data "oci_identity_availability_domains" "available" {
  count          = var.availability_domain == null || trimspace(var.availability_domain) == "" ? 1 : 0
  compartment_id = local.tenancy_compartment_id
}

data "oci_core_compute_capacity_reservation_instance_shapes" "available" {
  count               = var.skip_preflight ? 0 : 1
  compartment_id      = var.compartment_id
  availability_domain = local.availability_domain
}

resource "terraform_data" "input_validation" {
  input = {
    config_file                  = local.config_file_path
    requested_total              = local.requested_total
    reservation_config_count     = length(local.reservation_configs)
    reservation_configs_sha256   = sha256(jsonencode(local.oci_cli_instance_reservation_configs))
    single_config_flag_names     = local.mixed_single_config_flags
    explicit_config_source_count = local.explicit_config_source_count
  }

  lifecycle {
    precondition {
      condition     = local.explicit_config_source_count <= 1
      error_message = "Use only one config source: config_file, reservation_configs, or the single-config variables."
    }

    precondition {
      condition     = local.config_file_path == "" || length(local.mixed_single_config_flags) == 0
      error_message = "config_file cannot be combined with single-config variables: ${join(", ", local.mixed_single_config_flags)}."
    }

    precondition {
      condition     = !local.single_config_mode || (var.instance_type != null && var.ocpus != null && var.memory_gbs != null && var.quantity != null)
      error_message = "When config_file and reservation_configs are omitted, set instance_type, ocpus, memory_gbs, and quantity."
    }

    precondition {
      condition     = length(local.reservation_configs) > 0
      error_message = "At least one instance reservation config is required."
    }

    precondition {
      condition     = alltrue([for config in local.reservation_configs : config.instance_shape != ""])
      error_message = "Each reservation config must include instanceShape or instance_shape."
    }

    precondition {
      condition     = alltrue([for config in local.reservation_configs : config.reserved_count > 0])
      error_message = "Each reservation config must include reservedCount or reserved_count greater than 0."
    }

    precondition {
      condition = alltrue([
        for config in local.reservation_configs :
        config.instance_shape_config == null || config.instance_shape_config.ocpus == null || config.instance_shape_config.ocpus > 0
      ])
      error_message = "When provided, instance shape config ocpus must be greater than 0."
    }

    precondition {
      condition = alltrue([
        for config in local.reservation_configs :
        config.instance_shape_config == null || config.instance_shape_config.memory_in_gbs == null || config.instance_shape_config.memory_in_gbs > 0
      ])
      error_message = "When provided, instance shape config memory must be greater than 0."
    }

    precondition {
      condition = alltrue([
        for config in local.reservation_configs :
        config.cluster_config == null || config.cluster_config.hpc_island_id != ""
      ])
      error_message = "When cluster_config is provided, hpc_island_id is required."
    }
  }
}

resource "terraform_data" "shape_preflight" {
  input = {
    requested_shapes = local.requested_shapes
    available_shapes = local.available_shapes
    missing_shapes   = local.missing_shapes
  }

  depends_on = [terraform_data.input_validation]

  lifecycle {
    precondition {
      condition     = var.skip_preflight || length(local.missing_shapes) == 0
      error_message = "Requested shape(s) are not available for compute capacity reservations in compartment ${var.compartment_id}, availability domain ${local.availability_domain}: ${join(", ", local.missing_shapes)}. Available shapes: ${join(", ", local.available_shapes)}."
    }
  }
}
