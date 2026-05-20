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
