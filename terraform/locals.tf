locals {
  config_file_path = var.config_file == null ? "" : trimspace(var.config_file)
  config_file_body = local.config_file_path == "" ? null : jsondecode(file(local.config_file_path))
  file_configs     = local.config_file_path == "" ? null : try(local.config_file_body.instanceReservationConfigs, local.config_file_body.instance_reservation_configs, local.config_file_body)

  single_config_input = [
    {
      instance_shape = var.instance_type
      reserved_count = var.quantity
      fault_domain   = var.fault_domain
      instance_shape_config = {
        ocpus         = var.ocpus
        memory_in_gbs = var.memory_gbs
      }
    }
  ]

  raw_reservation_configs = local.config_file_path != "" ? local.file_configs : (
    var.reservation_configs == null ? local.single_config_input : var.reservation_configs
  )

  reservation_configs = [
    for config in local.raw_reservation_configs : {
      instance_shape             = try(trimspace(tostring(try(config.instance_shape, config.instanceShape))), "")
      reserved_count             = try(coalesce(tonumber(try(config.reserved_count, config.reservedCount)), 0), 0)
      fault_domain               = try(trimspace(tostring(try(config.fault_domain, config.faultDomain))), "") == "" ? null : try(trimspace(tostring(try(config.fault_domain, config.faultDomain))), null)
      cluster_placement_group_id = try(trimspace(tostring(try(config.cluster_placement_group_id, config.clusterPlacementGroupId))), "") == "" ? null : try(trimspace(tostring(try(config.cluster_placement_group_id, config.clusterPlacementGroupId))), null)

      instance_shape_config = try(config.instance_shape_config, config.instanceShapeConfig, null) == null ? null : {
        ocpus         = try(tonumber(try(config.instance_shape_config.ocpus, config.instanceShapeConfig.ocpus)), null)
        memory_in_gbs = try(tonumber(try(config.instance_shape_config.memory_in_gbs, config.instance_shape_config.memoryInGBs, config.instanceShapeConfig.memory_in_gbs, config.instanceShapeConfig.memoryInGBs)), null)
      }

      cluster_config = try(config.cluster_config, config.clusterConfig, null) == null ? null : {
        hpc_island_id     = try(trimspace(tostring(try(config.cluster_config.hpc_island_id, config.cluster_config.hpcIslandId, config.clusterConfig.hpc_island_id, config.clusterConfig.hpcIslandId))), "")
        network_block_ids = try(tolist(try(config.cluster_config.network_block_ids, config.cluster_config.networkBlockIds, config.clusterConfig.network_block_ids, config.clusterConfig.networkBlockIds)), null)
      }
    }
  ]

  mixed_single_config_flags = compact([
    var.instance_type == null ? "" : "instance_type",
    var.ocpus == null ? "" : "ocpus",
    var.memory_gbs == null ? "" : "memory_gbs",
    var.quantity == null ? "" : "quantity",
    var.fault_domain == null ? "" : "fault_domain",
  ])

  explicit_config_source_count = (local.config_file_path == "" ? 0 : 1) + (var.reservation_configs == null ? 0 : 1)

  single_config_mode     = local.explicit_config_source_count == 0
  requested_shapes       = sort(distinct([for config in local.reservation_configs : config.instance_shape if config.instance_shape != ""]))
  requested_total        = sum([for config in local.reservation_configs : config.reserved_count])
  availability_domain    = var.availability_domain == null || trimspace(var.availability_domain) == "" ? data.oci_identity_availability_domains.available[0].availability_domains[0].name : trimspace(var.availability_domain)
  display_name_base      = length(local.reservation_configs) == 1 ? local.reservation_configs[0].instance_shape : "multi"
  display_name           = var.display_name == null || trimspace(var.display_name) == "" ? "cap-res-${replace(replace(local.display_name_base, ".", "-"), "_", "-")}" : var.display_name
  available_shapes       = var.skip_preflight ? [] : sort([for shape in data.oci_core_compute_capacity_reservation_instance_shapes.available[0].compute_capacity_reservation_instance_shapes : shape.instance_shape])
  missing_shapes         = var.skip_preflight ? [] : sort(tolist(setsubtract(toset(local.requested_shapes), toset(local.available_shapes))))
  tenancy_compartment_id = var.tenancy_ocid == null || trimspace(var.tenancy_ocid) == "" ? var.compartment_id : var.tenancy_ocid

  reservation_configs_by_index = {
    for idx, config in local.reservation_configs : tostring(idx) => config
  }

  oci_cli_instance_reservation_configs = [
    for config in local.reservation_configs : merge(
      {
        instanceShape = config.instance_shape
        reservedCount = config.reserved_count
      },
      config.fault_domain == null ? {} : {
        faultDomain = config.fault_domain
      },
      config.cluster_placement_group_id == null ? {} : {
        clusterPlacementGroupId = config.cluster_placement_group_id
      },
      config.instance_shape_config == null ? {} : {
        instanceShapeConfig = merge(
          config.instance_shape_config.ocpus == null ? {} : {
            ocpus = config.instance_shape_config.ocpus
          },
          config.instance_shape_config.memory_in_gbs == null ? {} : {
            memoryInGBs = config.instance_shape_config.memory_in_gbs
          }
        )
      },
      config.cluster_config == null ? {} : {
        clusterConfig = merge(
          {
            hpcIslandId = config.cluster_config.hpc_island_id
          },
          config.cluster_config.network_block_ids == null ? {} : {
            networkBlockIds = config.cluster_config.network_block_ids
          }
        )
      }
    )
  ]
}
