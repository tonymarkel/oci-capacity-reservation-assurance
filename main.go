package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"github.com/oracle/oci-go-sdk/v65/workrequests"
)

const createdByTag = "oci-capacity-reservation-assurance"

type options struct {
	Shape              string
	OCPUs              float64
	MemoryGBs          float64
	CompartmentID      string
	Quantity           int64
	AvailabilityDomain string
	FaultDomain        string
	DisplayName        string
	PollInterval       time.Duration
	ReservationCheck   time.Duration
	Timeout            time.Duration
	DryRun             bool
	ResourceManagement string
}

func main() {
	opts, err := parseFlags(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}

	if err := run(context.Background(), opts); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags(args []string) (options, error) {
	var opts options
	fs := flag.NewFlagSet("reserve-capacity", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	fs.StringVar(&opts.Shape, "instance-type", "", "OCI instance shape, for example VM.Standard.E4.Flex")
	fs.StringVar(&opts.Shape, "shape", "", "alias for --instance-type")
	fs.Float64Var(&opts.OCPUs, "ocpus", 0, "number of OCPUs per reserved instance")
	fs.Float64Var(&opts.OCPUs, "ocpu", 0, "alias for --ocpus")
	fs.Float64Var(&opts.MemoryGBs, "memory-gbs", 0, "memory in GB per reserved instance")
	fs.Float64Var(&opts.MemoryGBs, "memory", 0, "alias for --memory-gbs")
	fs.StringVar(&opts.CompartmentID, "compartment-id", "", "target compartment OCID")
	fs.StringVar(&opts.CompartmentID, "compartment", "", "alias for --compartment-id")
	fs.Int64Var(&opts.Quantity, "quantity", 0, "number of instances to reserve")
	fs.StringVar(&opts.AvailabilityDomain, "availability-domain", "", "availability domain name, for example Uocm:US-ASHBURN-AD-1; defaults to the first AD in the tenancy")
	fs.StringVar(&opts.FaultDomain, "fault-domain", "", "optional fault domain, for example FAULT-DOMAIN-1")
	fs.StringVar(&opts.DisplayName, "display-name", "", "optional display name for the capacity reservation")
	fs.DurationVar(&opts.PollInterval, "poll-interval", 15*time.Second, "work request polling interval")
	fs.DurationVar(&opts.ReservationCheck, "reservation-check-interval", 30*time.Second, "how often to check reserved quantity after a work request succeeds")
	fs.DurationVar(&opts.Timeout, "timeout", 30*time.Minute, "maximum time to wait for the work request")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "print the create request body and exit without calling OCI")
	fs.StringVar(&opts.ResourceManagement, "resource-management", "", "optional internal shape resource management value: STATIC or DYNAMIC")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintln(fs.Output(), "Creates an OCI compute capacity reservation using ~/.oci/config DEFAULT profile, waits for the returned work request, and verifies the final reserved quantity.")
		fmt.Fprintln(fs.Output(), "\nRequired flags:")
		fmt.Fprintln(fs.Output(), "  --instance-type, --ocpus, --memory-gbs, --compartment-id, --quantity")
		fmt.Fprintln(fs.Output(), "\nExample:")
		fmt.Fprintf(fs.Output(), "  %s --instance-type VM.Standard.E4.Flex --ocpus 4 --memory-gbs 64 --compartment-id ocid1.compartment.oc1..example --quantity 3 --availability-domain Uocm:US-ASHBURN-AD-1\n\n", os.Args[0])
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return opts, err
	}

	if strings.TrimSpace(opts.Shape) == "" {
		return opts, errors.New("--instance-type is required")
	}
	if opts.OCPUs <= 0 {
		return opts, errors.New("--ocpus must be greater than 0")
	}
	if opts.MemoryGBs <= 0 {
		return opts, errors.New("--memory-gbs must be greater than 0")
	}
	if strings.TrimSpace(opts.CompartmentID) == "" {
		return opts, errors.New("--compartment-id is required")
	}
	if opts.Quantity <= 0 {
		return opts, errors.New("--quantity must be greater than 0")
	}
	if opts.PollInterval <= 0 {
		return opts, errors.New("--poll-interval must be greater than 0")
	}
	if opts.ReservationCheck <= 0 {
		return opts, errors.New("--reservation-check-interval must be greater than 0")
	}
	if opts.Timeout <= 0 {
		return opts, errors.New("--timeout must be greater than 0")
	}
	opts.ResourceManagement = strings.ToUpper(strings.TrimSpace(opts.ResourceManagement))
	switch opts.ResourceManagement {
	case "", "STATIC", "DYNAMIC":
	default:
		return opts, errors.New("--resource-management must be STATIC or DYNAMIC when provided")
	}
	if opts.DisplayName == "" {
		opts.DisplayName = defaultDisplayName(opts.Shape)
	}

	return opts, nil
}

func run(parent context.Context, opts options) error {
	ctx, cancel := context.WithTimeout(parent, opts.Timeout)
	defer cancel()

	provider := common.DefaultConfigProvider()

	region, err := provider.Region()
	if err != nil {
		return fmt.Errorf("read region from DEFAULT OCI profile: %w", err)
	}

	availabilityDomain, err := resolveAvailabilityDomain(ctx, provider, opts.AvailabilityDomain)
	if err != nil {
		return err
	}

	if opts.DryRun {
		return printDryRunPayload(opts, availabilityDomain)
	}

	computeClient, err := core.NewComputeClientWithConfigurationProvider(provider)
	if err != nil {
		return fmt.Errorf("create compute client: %w", err)
	}
	workRequestClient, err := workrequests.NewWorkRequestClientWithConfigurationProvider(provider)
	if err != nil {
		return fmt.Errorf("create work request client: %w", err)
	}

	fmt.Printf("Creating capacity reservation %q in region %s, availability domain %s\n", opts.DisplayName, region, availabilityDomain)
	createResp, err := createCapacityReservation(ctx, computeClient, opts, availabilityDomain)
	if err != nil {
		return fmt.Errorf("create capacity reservation: %w", err)
	}

	reservationID := value(createResp.ComputeCapacityReservation.Id)
	workRequestID := value(createResp.OpcWorkRequestId)
	fmt.Printf("Create request accepted\n")
	if reservationID != "" {
		fmt.Printf("Reservation ID: %s\n", reservationID)
	}
	if workRequestID == "" {
		return errors.New("create response did not include an opc-work-request-id")
	}
	fmt.Printf("Work request ID: %s\n", workRequestID)

	workRequest, err := waitForWorkRequest(ctx, workRequestClient, workRequestID, opts.PollInterval)
	if err != nil {
		return err
	}

	if workRequest.Status != workrequests.WorkRequestStatusSucceeded {
		return workRequestFailure(ctx, workRequestClient, workRequestID, workRequest.Status)
	}

	if reservationID == "" {
		reservationID = reservationIDFromWorkRequest(workRequest)
	}
	if reservationID == "" {
		return errors.New("work request succeeded, but no compute capacity reservation OCID was found")
	}

	return ensureReservedQuantity(ctx, computeClient, workRequestClient, opts, reservationID)
}

func createCapacityReservation(ctx context.Context, client core.ComputeClient, opts options, availabilityDomain string) (core.CreateComputeCapacityReservationResponse, error) {
	return client.CreateComputeCapacityReservation(ctx, core.CreateComputeCapacityReservationRequest{
		OpcRetryToken:                           common.String(fmt.Sprintf("%s-%d", createdByTag, time.Now().UnixNano())),
		CreateComputeCapacityReservationDetails: buildCreateDetails(opts, availabilityDomain),
	})
}

func updateCapacityReservation(ctx context.Context, client core.ComputeClient, opts options, reservationID string, etag *string) (core.UpdateComputeCapacityReservationResponse, error) {
	return client.UpdateComputeCapacityReservation(ctx, core.UpdateComputeCapacityReservationRequest{
		CapacityReservationId: common.String(reservationID),
		IfMatch:               etag,
		UpdateComputeCapacityReservationDetails: core.UpdateComputeCapacityReservationDetails{
			InstanceReservationConfigs: []core.InstanceReservationConfigDetails{buildInstanceReservationConfig(opts)},
		},
	})
}

func buildCreateDetails(opts options, availabilityDomain string) core.CreateComputeCapacityReservationDetails {
	config := buildInstanceReservationConfig(opts)

	return core.CreateComputeCapacityReservationDetails{
		AvailabilityDomain: common.String(availabilityDomain),
		CompartmentId:      common.String(opts.CompartmentID),
		DisplayName:        common.String(opts.DisplayName),
		FreeformTags: map[string]string{
			"created-by": createdByTag,
		},
		InstanceReservationConfigs: []core.InstanceReservationConfigDetails{config},
		IsDefaultReservation:       common.Bool(false),
	}
}

func buildInstanceReservationConfig(opts options) core.InstanceReservationConfigDetails {
	config := core.InstanceReservationConfigDetails{
		InstanceShape: common.String(opts.Shape),
		InstanceShapeConfig: &core.InstanceReservationShapeConfigDetails{
			Ocpus:       common.Float32(float32(opts.OCPUs)),
			MemoryInGBs: common.Float32(float32(opts.MemoryGBs)),
		},
		ReservedCount: common.Int64(opts.Quantity),
	}
	if opts.ResourceManagement != "" {
		config.InstanceShapeConfig.ResourceManagement = core.InstanceReservationShapeConfigDetailsResourceManagementEnum(opts.ResourceManagement)
	}
	if opts.FaultDomain != "" {
		config.FaultDomain = common.String(opts.FaultDomain)
	}

	return config
}

func printDryRunPayload(opts options, availabilityDomain string) error {
	req := core.CreateComputeCapacityReservationRequest{
		CreateComputeCapacityReservationDetails: buildCreateDetails(opts, availabilityDomain),
	}
	httpReq, err := req.HTTPRequest("POST", "/20160918/computeCapacityReservations", nil, nil)
	if err != nil {
		return fmt.Errorf("build dry-run request: %w", err)
	}
	body, err := io.ReadAll(httpReq.Body)
	if err != nil {
		return fmt.Errorf("read dry-run request body: %w", err)
	}
	fmt.Println("Dry run enabled; OCI create call was not sent.")
	fmt.Println(string(body))
	return nil
}

func resolveAvailabilityDomain(ctx context.Context, provider common.ConfigurationProvider, explicitAD string) (string, error) {
	if strings.TrimSpace(explicitAD) != "" {
		return explicitAD, nil
	}

	tenancyID, err := provider.TenancyOCID()
	if err != nil {
		return "", fmt.Errorf("read tenancy OCID from DEFAULT OCI profile while resolving availability domain: %w", err)
	}

	client, err := identity.NewIdentityClientWithConfigurationProvider(provider)
	if err != nil {
		return "", fmt.Errorf("create identity client while resolving availability domain: %w", err)
	}

	resp, err := client.ListAvailabilityDomains(ctx, identity.ListAvailabilityDomainsRequest{
		CompartmentId: common.String(tenancyID),
	})
	if err != nil {
		return "", fmt.Errorf("list availability domains for tenancy %s: %w", tenancyID, err)
	}
	if len(resp.Items) == 0 || resp.Items[0].Name == nil {
		return "", errors.New("no availability domains were returned; provide --availability-domain explicitly")
	}

	ad := *resp.Items[0].Name
	fmt.Printf("No --availability-domain provided; using first tenancy AD: %s\n", ad)
	return ad, nil
}

func waitForWorkRequest(ctx context.Context, client workrequests.WorkRequestClient, workRequestID string, pollInterval time.Duration) (workrequests.WorkRequest, error) {
	for {
		resp, err := client.GetWorkRequest(ctx, workrequests.GetWorkRequestRequest{
			WorkRequestId: common.String(workRequestID),
		})
		if err != nil {
			return workrequests.WorkRequest{}, fmt.Errorf("get work request %s: %w", workRequestID, err)
		}

		fmt.Printf("Work request status: %s", resp.Status)
		if resp.PercentComplete != nil {
			fmt.Printf(" (%.1f%%)", *resp.PercentComplete)
		}
		fmt.Println()

		switch resp.Status {
		case workrequests.WorkRequestStatusSucceeded, workrequests.WorkRequestStatusFailed, workrequests.WorkRequestStatusCanceled:
			return resp.WorkRequest, nil
		}

		select {
		case <-ctx.Done():
			return workrequests.WorkRequest{}, fmt.Errorf("timed out waiting for work request %s: %w", workRequestID, ctx.Err())
		case <-time.After(pollInterval):
		}
	}
}

func workRequestFailure(ctx context.Context, client workrequests.WorkRequestClient, workRequestID string, status workrequests.WorkRequestStatusEnum) error {
	resp, err := client.ListWorkRequestErrors(ctx, workrequests.ListWorkRequestErrorsRequest{
		WorkRequestId: common.String(workRequestID),
	})
	if err != nil {
		return fmt.Errorf("work request %s ended with status %s; also failed to list work request errors: %w", workRequestID, status, err)
	}
	if len(resp.Items) == 0 {
		return fmt.Errorf("work request %s ended with status %s", workRequestID, status)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "work request %s ended with status %s:", workRequestID, status)
	for _, item := range resp.Items {
		fmt.Fprintf(&b, "\n- %s: %s", value(item.Code), value(item.Message))
	}
	return errors.New(b.String())
}

func reservationIDFromWorkRequest(workRequest workrequests.WorkRequest) string {
	for _, resource := range workRequest.Resources {
		identifier := value(resource.Identifier)
		entityType := strings.ToLower(value(resource.EntityType))
		if strings.Contains(strings.ToLower(identifier), "capacityreservation") || strings.Contains(entityType, "capacityreservation") {
			return identifier
		}
	}
	return ""
}

func ensureReservedQuantity(ctx context.Context, computeClient core.ComputeClient, workRequestClient workrequests.WorkRequestClient, opts options, reservationID string) error {
	for {
		resp, err := computeClient.GetComputeCapacityReservation(ctx, core.GetComputeCapacityReservationRequest{
			CapacityReservationId: common.String(reservationID),
		})
		if err != nil {
			return fmt.Errorf("get capacity reservation %s: %w", reservationID, err)
		}

		reservation := resp.ComputeCapacityReservation
		reserved := reservedConfigCount(reservation)
		fmt.Printf("Reservation check: lifecycle=%s requested=%d reservedCount=%d\n", reservation.LifecycleState, opts.Quantity, reserved)

		if reserved == opts.Quantity {
			return validateReservedQuantity(opts, reservation)
		}

		switch reservation.LifecycleState {
		case core.ComputeCapacityReservationLifecycleStateDeleted, core.ComputeCapacityReservationLifecycleStateDeleting:
			return fmt.Errorf("reservation %s is %s before requested quantity was reserved", reservationID, reservation.LifecycleState)
		case core.ComputeCapacityReservationLifecycleStateCreating, core.ComputeCapacityReservationLifecycleStateUpdating, core.ComputeCapacityReservationLifecycleStateMoving:
			fmt.Printf("Reservation is still %s; checking again in %s\n", reservation.LifecycleState, opts.ReservationCheck)
			if err := waitForNextReservationCheck(ctx, opts.ReservationCheck); err != nil {
				return err
			}
			continue
		}

		fmt.Printf("Reserved quantity is %d, updating reservation to requested quantity %d\n", reserved, opts.Quantity)
		updateResp, err := updateCapacityReservation(ctx, computeClient, opts, reservationID, resp.Etag)
		if err != nil {
			if reason, retry := retryableReservationUpdateError(err); retry {
				fmt.Printf("Update is not ready yet: %s; checking again in %s\n", reason, opts.ReservationCheck)
				if err := waitForNextReservationCheck(ctx, opts.ReservationCheck); err != nil {
					return err
				}
				continue
			}
			return fmt.Errorf("update capacity reservation %s to reserved quantity %d: %w", reservationID, opts.Quantity, err)
		}

		updateWorkRequestID := value(updateResp.OpcWorkRequestId)
		if updateWorkRequestID == "" {
			fmt.Printf("Update accepted without a work request ID; checking again in %s\n", opts.ReservationCheck)
			if err := waitForNextReservationCheck(ctx, opts.ReservationCheck); err != nil {
				return err
			}
			continue
		}

		fmt.Printf("Update work request ID: %s\n", updateWorkRequestID)
		workRequest, err := waitForWorkRequest(ctx, workRequestClient, updateWorkRequestID, opts.PollInterval)
		if err != nil {
			return err
		}
		if workRequest.Status != workrequests.WorkRequestStatusSucceeded {
			return workRequestFailure(ctx, workRequestClient, updateWorkRequestID, workRequest.Status)
		}

		fmt.Printf("Update work request succeeded; checking reserved quantity again in %s\n", opts.ReservationCheck)
		if err := waitForNextReservationCheck(ctx, opts.ReservationCheck); err != nil {
			return err
		}
	}
}

func retryableReservationUpdateError(err error) (string, bool) {
	serviceErr, ok := common.IsServiceError(err)
	if !ok {
		return "", false
	}

	code := serviceErr.GetCode()
	status := serviceErr.GetHTTPStatusCode()
	if status != 409 && !strings.EqualFold(code, "IncorrectState") && !strings.EqualFold(code, "Conflict") {
		return "", false
	}

	return fmt.Sprintf("%s (%d): %s", code, status, serviceErr.GetMessage()), true
}

func waitForNextReservationCheck(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return fmt.Errorf("timed out waiting for reservation quantity to match request: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

func validateReservedQuantity(opts options, reservation core.ComputeCapacityReservation) error {
	reserved := reservedConfigCount(reservation)
	reservationID := value(reservation.Id)

	fmt.Printf("Reservation completed: %s\n", reservationID)
	fmt.Printf("Lifecycle state: %s\n", reservation.LifecycleState)
	fmt.Printf("Requested quantity: %d\n", opts.Quantity)
	fmt.Printf("Reserved count total: %d\n", reserved)

	for i, config := range reservation.InstanceReservationConfigs {
		fmt.Printf("Config %d: shape=%s reservedCount=%d", i+1, value(config.InstanceShape), int64Value(config.ReservedCount))
		if config.InstanceShapeConfig != nil {
			fmt.Printf(" ocpus=%.2f memoryGBs=%.2f", float32Value(config.InstanceShapeConfig.Ocpus), float32Value(config.InstanceShapeConfig.MemoryInGBs))
		}
		if config.FaultDomain != nil {
			fmt.Printf(" faultDomain=%s", *config.FaultDomain)
		}
		fmt.Println()
	}

	if reserved != opts.Quantity {
		return fmt.Errorf("reservation quantity mismatch for %s: requested %d, OCI reserved %d; the reservation remains in OCI, so review or delete it if this partial reservation is not desired", reservationID, opts.Quantity, reserved)
	}

	fmt.Println("Validation passed: reserved quantity matches requested quantity")
	return nil
}

func reservedConfigCount(reservation core.ComputeCapacityReservation) int64 {
	var total int64
	for _, config := range reservation.InstanceReservationConfigs {
		if config.ReservedCount != nil {
			total += *config.ReservedCount
		}
	}
	return total
}

func defaultDisplayName(shape string) string {
	clean := strings.NewReplacer(".", "-", "_", "-").Replace(shape)
	return fmt.Sprintf("cap-res-%s-%s", clean, time.Now().UTC().Format("20060102-150405"))
}

func value(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

func int64Value(ptr *int64) int64 {
	if ptr == nil {
		return 0
	}
	return *ptr
}

func float32Value(ptr *float32) float32 {
	if ptr == nil {
		return 0
	}
	return *ptr
}
