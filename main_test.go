package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

func TestParseFlagsConfigFileSetsMultipleConfigs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configs.json")
	body := []byte(`{
		"instanceReservationConfigs": [
			{
				"instanceShape": "VM.Standard.E4.Flex",
				"instanceShapeConfig": {"ocpus": 4, "memoryInGBs": 64},
				"reservedCount": 3
			},
			{
				"instanceShape": "VM.Standard.E4.Flex",
				"instanceShapeConfig": {"ocpus": 8, "memoryInGBs": 128},
				"reservedCount": 2
			}
		]
	}`)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	opts, err := parseFlags([]string{
		"--config-file", path,
		"--compartment-id", "ocid1.compartment.oc1..example",
	})
	if err != nil {
		t.Fatalf("parseFlags returned error: %v", err)
	}

	if got := len(opts.Configs); got != 2 {
		t.Fatalf("config count = %d, want 2", got)
	}
	if got := opts.Quantity; got != 5 {
		t.Fatalf("quantity total = %d, want 5", got)
	}
	if usesConfigAvailabilityDomains(opts) {
		t.Fatal("plain config file unexpectedly uses config availability domains")
	}
}

func TestParseFlagsConfigFileCapturesAvailabilityDomains(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configs.json")
	body := []byte(`{
		"instanceReservationConfigs": [
			{
				"instanceShape": "VM.Standard.E6.Flex",
				"availabilityDomain": "DfpY:US-ASHBURN-AD-1",
				"instanceShapeConfig": {"ocpus": 3, "memoryInGBs": 12},
				"reservedCount": 3
			},
			{
				"instanceShape": "VM.Standard.E6.Flex",
				"availabilityDomain": "DfpY:US-ASHBURN-AD-2",
				"instanceShapeConfig": {"ocpus": 3, "memoryInGBs": 12},
				"reservedCount": 2
			}
		]
	}`)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	opts, err := parseFlags([]string{
		"--config-file", path,
		"--compartment-id", "ocid1.compartment.oc1..example",
	})
	if err != nil {
		t.Fatalf("parseFlags returned error: %v", err)
	}

	if !usesConfigAvailabilityDomains(opts) {
		t.Fatal("config file availabilityDomain values were not detected")
	}
	wantADs := []string{"DfpY:US-ASHBURN-AD-1", "DfpY:US-ASHBURN-AD-2"}
	for i, want := range wantADs {
		if got := opts.ConfigADs[i]; got != want {
			t.Fatalf("ConfigADs[%d] = %q, want %q", i, got, want)
		}
	}
}

func TestParseFlagsRejectsPartialConfigAvailabilityDomains(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configs.json")
	body := []byte(`{
		"instanceReservationConfigs": [
			{
				"instanceShape": "VM.Standard.E6.Flex",
				"availabilityDomain": "DfpY:US-ASHBURN-AD-1",
				"instanceShapeConfig": {"ocpus": 3, "memoryInGBs": 12},
				"reservedCount": 3
			},
			{
				"instanceShape": "VM.Standard.E6.Flex",
				"instanceShapeConfig": {"ocpus": 3, "memoryInGBs": 12},
				"reservedCount": 2
			}
		]
	}`)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	_, err := parseFlags([]string{
		"--config-file", path,
		"--compartment-id", "ocid1.compartment.oc1..example",
	})
	if err == nil {
		t.Fatal("parseFlags succeeded with partial availabilityDomain values")
	}
}

func TestCompareReservationConfigsDetectsWrongSizeWithSameTotal(t *testing.T) {
	requested := []core.InstanceReservationConfigDetails{
		requestedConfig("VM.Standard.E4.Flex", 4, 64, 3),
		requestedConfig("VM.Standard.E4.Flex", 8, 128, 2),
	}
	actual := []core.InstanceReservationConfig{
		actualConfig("VM.Standard.E4.Flex", 4, 64, 2),
		actualConfig("VM.Standard.E4.Flex", 8, 128, 3),
	}

	comparison := compareReservationConfigs(requested, actual)
	if comparison.matches() {
		t.Fatal("comparison matched even though reservedCount values were assigned to the wrong sizes")
	}
	if comparison.RequestedTotal != 5 || comparison.ActualTotal != 5 {
		t.Fatalf("totals = requested %d actual %d, want both 5", comparison.RequestedTotal, comparison.ActualTotal)
	}
}

func TestSplitConfigsAcrossAvailabilityDomainsDividesEvenly(t *testing.T) {
	configs := []core.InstanceReservationConfigDetails{
		requestedConfig("VM.Standard.E6.Flex", 3, 12, 6),
	}

	split := splitConfigsAcrossAvailabilityDomains(configs, 3)
	want := []int64{2, 2, 2}
	for i := range want {
		if got := requestedConfigTotal(split[i]); got != want[i] {
			t.Fatalf("AD %d total = %d, want %d", i, got, want[i])
		}
	}
}

func TestSplitConfigsAcrossAvailabilityDomainsAssignsRemainderToEarlierADs(t *testing.T) {
	configs := []core.InstanceReservationConfigDetails{
		requestedConfig("VM.Standard.E6.Flex", 3, 12, 5),
	}

	split := splitConfigsAcrossAvailabilityDomains(configs, 3)
	want := []int64{2, 2, 1}
	for i := range want {
		if got := requestedConfigTotal(split[i]); got != want[i] {
			t.Fatalf("AD %d total = %d, want %d", i, got, want[i])
		}
	}
}

func TestBuildAvailabilityDomainPlansSkipsZeroCountADs(t *testing.T) {
	opts := options{
		DisplayName: "cap-res-test",
		Quantity:    2,
		Configs: []core.InstanceReservationConfigDetails{
			requestedConfig("VM.Standard.E6.Flex", 3, 12, 2),
		},
	}

	plans, err := buildAvailabilityDomainPlans(opts, []string{
		"DfpY:US-ASHBURN-AD-1",
		"DfpY:US-ASHBURN-AD-2",
		"DfpY:US-ASHBURN-AD-3",
	})
	if err != nil {
		t.Fatalf("buildAvailabilityDomainPlans returned error: %v", err)
	}
	if got := len(plans); got != 2 {
		t.Fatalf("plan count = %d, want 2", got)
	}
	for i, plan := range plans {
		if got := requestedConfigTotal(plan.Options.Configs); got != 1 {
			t.Fatalf("plan %d total = %d, want 1", i, got)
		}
		if plan.Options.DisplayName == opts.DisplayName {
			t.Fatalf("plan %d display name was not made AD-specific", i)
		}
	}
}

func TestBuildAvailabilityDomainPlansUsesConfigDistribution(t *testing.T) {
	opts := options{
		DisplayName: "cap-res-test",
		Quantity:    5,
		Configs: []core.InstanceReservationConfigDetails{
			requestedConfig("VM.Standard.E6.Flex", 3, 12, 3),
			requestedConfig("VM.Standard.E6.Flex", 3, 12, 2),
		},
		ConfigADs: []string{
			"DfpY:US-ASHBURN-AD-1",
			"DfpY:US-ASHBURN-AD-2",
		},
	}

	plans, err := buildAvailabilityDomainPlans(opts, availabilityDomainsFromConfigDistribution(opts.ConfigADs))
	if err != nil {
		t.Fatalf("buildAvailabilityDomainPlans returned error: %v", err)
	}
	if got := len(plans); got != 2 {
		t.Fatalf("plan count = %d, want 2", got)
	}
	wantTotals := []int64{3, 2}
	for i, want := range wantTotals {
		if got := requestedConfigTotal(plans[i].Options.Configs); got != want {
			t.Fatalf("plan %d total = %d, want %d", i, got, want)
		}
	}
}

func requestedConfig(shape string, ocpus float32, memory float32, count int64) core.InstanceReservationConfigDetails {
	return core.InstanceReservationConfigDetails{
		InstanceShape: common.String(shape),
		InstanceShapeConfig: &core.InstanceReservationShapeConfigDetails{
			Ocpus:       common.Float32(ocpus),
			MemoryInGBs: common.Float32(memory),
		},
		ReservedCount: common.Int64(count),
	}
}

func actualConfig(shape string, ocpus float32, memory float32, count int64) core.InstanceReservationConfig {
	return core.InstanceReservationConfig{
		InstanceShape: common.String(shape),
		InstanceShapeConfig: &core.InstanceReservationShapeConfigDetails{
			Ocpus:       common.Float32(ocpus),
			MemoryInGBs: common.Float32(memory),
		},
		ReservedCount: common.Int64(count),
	}
}
