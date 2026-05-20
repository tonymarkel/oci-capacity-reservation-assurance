# oci-capacity-reservation-assurance
Runs a script locally to ensure that a capacity reservation exists in the specified compartment for the requested shape and quantity. This script will incur changes on OCI. Run Responsibly.

## What it does

This Go script:

1. Uses the `DEFAULT` profile in `~/.oci/config`.
2. Uses your tenancy-specific availability domain name, such as `Dfpy:US-PHOENIX-AD-2`.
3. Creates an OCI compute capacity reservation for either one CLI-specified config or many configs from a JSON file.
4. Waits for the returned work request to reach a terminal state.
5. Checks the reservation every 30 seconds until each requested `instanceReservationConfigs[].reservedCount` matches OCI's returned config.
6. If the reservation is active but any capacity configuration is still short, updates the capacity configs back to the requested values and repeats the check/update cycle until they match or `--timeout` expires.

If OCI cannot reach the requested quantity before `--timeout`, the script exits non-zero and leaves the reservation in OCI for you to review or delete.

Use `oci iam availability-domain list` to get your tenancy-specific availability domain names.
The `region` in your `DEFAULT` profile must match the availability domain region, for example `us-ashburn-1` with `...:US-ASHBURN-AD-1`.

## Usage

Single config:

```bash
go run . \
  --instance-type VM.Standard.E4.Flex \
  --ocpus 4 \
  --memory-gbs 64 \
  --compartment-id ocid1.compartment.oc1..example \
  --quantity 3 \
  --availability-domain Uocm:US-ASHBURN-AD-1
```

Multiple configs from a file:

```bash
go run . \
  --config-file example.json \
  --compartment-id ocid1.compartment.oc1..example \
  --availability-domain Uocm:US-ASHBURN-AD-1
```

The config file can be either a raw JSON array or an object with `instanceReservationConfigs`:

```json
{
  "instanceReservationConfigs": [
    {
      "instanceShape": "VM.Standard.E4.Flex",
      "instanceShapeConfig": {
        "ocpus": 4,
        "memoryInGBs": 64
      },
      "reservedCount": 3
    },
    {
      "instanceShape": "VM.Standard.E4.Flex",
      "instanceShapeConfig": {
        "ocpus": 8,
        "memoryInGBs": 128
      },
      "reservedCount": 2
    }
  ]
}
```

`--availability-domain` is recommended because capacity reservations are AD-specific. If you omit it, the script lists availability domains in the tenancy from the `DEFAULT` profile and uses the first one returned.

Optional flags:

```text
--config-file example.json
--fault-domain FAULT-DOMAIN-1
--display-name my-capacity-reservation
--dry-run
--preflight-only
--skip-preflight
--poll-interval 15s
--reservation-check-interval 30s
--resource-management STATIC
--timeout 30m
```

Use `--dry-run` to print the exact SDK-marshaled create request body without sending it to OCI.
Use `--preflight-only` to check that each requested shape is available for capacity reservations in the compartment/AD, print the request body, and exit before creating anything.
If preflight says a shape is not available, change the config to one of the listed shapes or use a compartment/AD where the shape appears. Passing an `ocid1.tenancy...` value targets the root compartment; use the child `ocid1.compartment...` OCID when the reservation should live in a child compartment.
Use `--skip-preflight` only when you want to bypass the shape availability check and send the create request anyway; OCI may still reject the reservation.
Use `--reservation-check-interval` to change how often the script re-reads the reservation after a work request succeeds.
When using `--config-file`, put per-config values such as `faultDomain` inside the JSON file instead of using the single-config flags.
`--resource-management` is a single-config debug flag and is intentionally omitted by default because the public OCI CLI schema does not include it.

Aliases are also available for the single-config inputs: `--shape`, `--ocpu`, `--memory`, and `--compartment`.
