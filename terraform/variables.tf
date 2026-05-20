variable "oci_profile" {
  description = "OCI config profile to use from ~/.oci/config."
  type        = string
  default     = "DEFAULT"
}

variable "oci_config_file" {
  description = "OCI CLI config file used by the local reserved-count assurance step."
  type        = string
  default     = "~/.oci/config"
}

variable "compartment_id" {
  description = "Target compartment OCID for the capacity reservation."
  type        = string
}

variable "tenancy_ocid" {
  description = "Tenancy OCID used only when availability_domain is omitted and Terraform must list ADs."
  type        = string
  default     = null
}

variable "availability_domain" {
  description = "Availability domain name, for example Uocm:US-ASHBURN-AD-1. If omitted, the first tenancy AD is used."
  type        = string
  default     = null
}

variable "display_name" {
  description = "Optional capacity reservation display name."
  type        = string
  default     = null
}

variable "config_file" {
  description = "Optional JSON file containing a raw array of configs or an object with instanceReservationConfigs."
  type        = string
  default     = null
}

variable "reservation_configs" {
  description = "Optional native Terraform list of reservation configs. Use either this, config_file, or the single-config variables."
  type        = any
  default     = null
}

variable "instance_type" {
  description = "Single-config instance shape, for example VM.Standard.E4.Flex."
  type        = string
  default     = null
}

variable "ocpus" {
  description = "Single-config OCPU count."
  type        = number
  default     = null
}

variable "memory_gbs" {
  description = "Single-config memory in GB."
  type        = number
  default     = null
}

variable "quantity" {
  description = "Single-config reserved instance count."
  type        = number
  default     = null
}

variable "fault_domain" {
  description = "Optional single-config fault domain, for example FAULT-DOMAIN-1."
  type        = string
  default     = null
}

variable "is_default_reservation" {
  description = "Whether this capacity reservation should be the default reservation."
  type        = bool
  default     = false
}

variable "defined_tags" {
  description = "Defined tags for the reservation."
  type        = map(string)
  default     = {}
}

variable "freeform_tags" {
  description = "Freeform tags for the reservation."
  type        = map(string)
  default     = {}
}

variable "skip_preflight" {
  description = "Skip the OCI shape availability preflight."
  type        = bool
  default     = false
}

variable "run_reserved_count_assurance" {
  description = "Run the post-create local polling/update loop that validates instanceReservationConfigs[].reserved-count."
  type        = bool
  default     = true
}

variable "update_until_match" {
  description = "When the reserved counts are short and the reservation is ACTIVE, update the reservation back to the requested configs until it matches."
  type        = bool
  default     = true
}

variable "reservation_check_interval_seconds" {
  description = "Seconds between reserved-count checks in the local assurance loop."
  type        = number
  default     = 30
}

variable "validation_timeout_seconds" {
  description = "Maximum seconds for the local reserved-count assurance loop."
  type        = number
  default     = 1800
}

variable "oci_cli_path" {
  description = "OCI CLI executable used by the local assurance loop."
  type        = string
  default     = "oci"
}

variable "create_timeout" {
  description = "Terraform provider timeout for capacity reservation create."
  type        = string
  default     = "30m"
}

variable "update_timeout" {
  description = "Terraform provider timeout for capacity reservation update."
  type        = string
  default     = "30m"
}

variable "delete_timeout" {
  description = "Terraform provider timeout for capacity reservation delete."
  type        = string
  default     = "30m"
}
