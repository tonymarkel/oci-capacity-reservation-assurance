terraform {
  required_version = ">= 1.5.0"

  required_providers {
    oci = {
      source  = "oracle/oci"
      version = ">= 7.10.0, < 8.0.0"
    }
  }
}

provider "oci" {
  config_file_profile = var.oci_profile
}
