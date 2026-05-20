output "capacity_reservation_id" {
  description = "OCID of the compute capacity reservation."
  value       = oci_core_compute_capacity_reservation.this.id
}

output "capacity_reservation_state" {
  description = "Current Terraform-observed lifecycle state of the reservation."
  value       = oci_core_compute_capacity_reservation.this.state
}

output "availability_domain" {
  description = "Availability domain used for the reservation."
  value       = local.availability_domain
}

output "requested_reserved_count_total" {
  description = "Sum of requested reservedCount values."
  value       = local.requested_total
}

output "requested_shapes" {
  description = "Distinct requested shapes."
  value       = local.requested_shapes
}

output "available_reservation_shapes" {
  description = "Shapes returned by the preflight data source. Empty when skip_preflight is true."
  value       = local.available_shapes
}

output "instance_reservation_configs" {
  description = "Terraform-observed capacity reservation configs. Run terraform refresh/apply again if you want state refreshed after the local assurance step updates OCI."
  value       = oci_core_compute_capacity_reservation.this.instance_reservation_configs
}
