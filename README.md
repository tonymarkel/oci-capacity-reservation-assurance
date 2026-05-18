# oci-capacity-reservation-assurance
Runs a script locally to ensure that a capacity reservation exists in the specified compartment for the requested shape and quantity. This script will incur changes on OCI. Run Responsibly.

## What it does

This Go script:

1. Uses the `DEFAULT` profile in `~/.oci/config`.
2. Uses your tenancy-specific xxxx:CN-REGION-#-# nomenclature for your availability domain (e.g. Dfpy:US-PHOENIX-AD-2) s
    * use `oci iam availability-domain list` to get your availability domains
3. Creates an OCI compute capacity reservation for the requested shape, OCPUs, memory, compartment, and quantity.
4. Waits for the returned work request to reach a terminal state.
5. Checks the reservation every 30 seconds until the total `reservedCount` from `instanceReservationConfigs` matches the requested `--quantity`.
6. If the reservation is active but the capacity configuration `reservedCount` total is still short, updates the capacity configuration back to the requested quantity and repeats the check/update cycle until it matches or `--timeout` expires.

If OCI cannot reach the requested quantity before `--timeout`, the script exits non-zero and leaves the reservation in OCI for you to review or delete.

## Usage

```bash
go run . \
  --instance-type VM.Standard.E4.Flex \
  --ocpus 4 \
  --memory-gbs 64 \
  --compartment-id ocid1.compartment.oc1..example \
  --quantity 3 \
  --availability-domain Uocm:US-ASHBURN-AD-1
```

`--availability-domain` is recommended because capacity reservations are AD-specific. If you omit it, the script lists availability domains in the tenancy from the `DEFAULT` profile and uses the first one returned.

Optional flags:

```text
--fault-domain FAULT-DOMAIN-1
--display-name my-capacity-reservation
--dry-run
--poll-interval 15s
--reservation-check-interval 30s
--resource-management STATIC
--timeout 30m
```

Use `--dry-run` to print the exact SDK-marshaled create request body without sending it to OCI.
Use `--reservation-check-interval` to change how often the script re-reads the reservation after a work request succeeds.
`--resource-management` is intentionally omitted by default because the public OCI CLI schema does not include it; it is available only for debugging SDK/schema drift.

Aliases are also available for the required inputs: `--shape`, `--ocpu`, `--memory`, and `--compartment`.
