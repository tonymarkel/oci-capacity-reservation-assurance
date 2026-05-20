resource "oci_core_compute_capacity_reservation" "this" {
  depends_on = [terraform_data.shape_preflight]

  availability_domain    = local.availability_domain
  compartment_id         = var.compartment_id
  display_name           = local.display_name
  defined_tags           = var.defined_tags
  freeform_tags          = merge(var.freeform_tags, { "created-by" = "oci-capacity-reservation-assurance" })
  is_default_reservation = var.is_default_reservation

  dynamic "instance_reservation_configs" {
    for_each = local.reservation_configs_by_index

    content {
      instance_shape             = instance_reservation_configs.value.instance_shape
      reserved_count             = instance_reservation_configs.value.reserved_count
      fault_domain               = instance_reservation_configs.value.fault_domain
      cluster_placement_group_id = instance_reservation_configs.value.cluster_placement_group_id

      dynamic "instance_shape_config" {
        for_each = instance_reservation_configs.value.instance_shape_config == null ? [] : [instance_reservation_configs.value.instance_shape_config]

        content {
          ocpus         = instance_shape_config.value.ocpus
          memory_in_gbs = instance_shape_config.value.memory_in_gbs
        }
      }

      dynamic "cluster_config" {
        for_each = instance_reservation_configs.value.cluster_config == null ? [] : [instance_reservation_configs.value.cluster_config]

        content {
          hpc_island_id     = cluster_config.value.hpc_island_id
          network_block_ids = cluster_config.value.network_block_ids
        }
      }
    }
  }

  timeouts {
    create = var.create_timeout
    update = var.update_timeout
    delete = var.delete_timeout
  }
}

resource "terraform_data" "reserved_count_assurance" {
  count = var.run_reserved_count_assurance ? 1 : 0

  triggers_replace = [
    oci_core_compute_capacity_reservation.this.id,
    sha256(jsonencode(local.oci_cli_instance_reservation_configs)),
    tostring(var.update_until_match),
    tostring(var.reservation_check_interval_seconds),
    tostring(var.validation_timeout_seconds),
  ]

  input = {
    capacity_reservation_id = oci_core_compute_capacity_reservation.this.id
    requested_total         = local.requested_total
    requested_configs       = local.oci_cli_instance_reservation_configs
  }

  provisioner "local-exec" {
    command     = "${path.module}/scripts/ensure_reserved_counts.py"
    interpreter = ["python3"]

    environment = {
      OCI_CLI_PATH           = var.oci_cli_path
      OCI_CONFIG_FILE        = var.oci_config_file
      OCI_PROFILE            = var.oci_profile
      RESERVATION_ID         = oci_core_compute_capacity_reservation.this.id
      REQUESTED_CONFIGS_JSON = jsonencode(local.oci_cli_instance_reservation_configs)
      POLL_INTERVAL_SECONDS  = tostring(var.reservation_check_interval_seconds)
      TIMEOUT_SECONDS        = tostring(var.validation_timeout_seconds)
      UPDATE_UNTIL_MATCH     = tostring(var.update_until_match)
    }
  }

  depends_on = [oci_core_compute_capacity_reservation.this]
}
