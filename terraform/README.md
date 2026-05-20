# OCI Capacity Reservation Terraform

This Terraform configuration creates an OCI compute capacity reservation using the `DEFAULT` profile in `~/.oci/config` and preserves the Go script's multi-config behavior.

It does three things:

1. Reads one reservation config from variables, a native Terraform `reservation_configs` value, or a JSON file like `../example.json`.
2. Preflights the requested shapes with `oci_core_compute_capacity_reservation_instance_shapes`.
3. Creates `oci_core_compute_capacity_reservation`, then runs a local OCI CLI polling step that checks each returned capacity config's `reserved-count` until it matches the requested `reservedCount`. If the reservation is `ACTIVE` but short, it updates the reservation back to the requested configs and keeps checking.

## Usage

```bash
cd terraform
cp terraform.tfvars.example terraform.tfvars
terraform init
terraform apply
```

For your existing JSON file:

```hcl
compartment_id      = "ocid1.compartment.oc1..example"
availability_domain = "Uocm:US-ASHBURN-AD-1"
config_file         = "../example.json"
```

The JSON file can be either:

```json
{
  "instanceReservationConfigs": [
    {
      "instanceShape": "VM.Standard.E6.Flex",
      "instanceShapeConfig": {
        "ocpus": 3,
        "memoryInGBs": 12
      },
      "reservedCount": 3
    }
  ]
}
```

Or a raw array of those config objects.

Single-config mode is also available:

```hcl
compartment_id      = "ocid1.compartment.oc1..example"
availability_domain = "Uocm:US-ASHBURN-AD-1"
instance_type       = "VM.Standard.E6.Flex"
ocpus               = 3
memory_gbs          = 12
quantity            = 3
```

## Notes

The local reserved-count assurance step requires the OCI CLI and `python3` on your machine. It uses `~/.oci/config` with the `DEFAULT` profile by default.

Use `skip_preflight = true` only when you want to bypass the shape availability check. OCI can still reject the reservation if the shape is not reservable in that compartment and AD.

If you omit `availability_domain`, set `tenancy_ocid` so Terraform can list availability domains and use the first one returned.
