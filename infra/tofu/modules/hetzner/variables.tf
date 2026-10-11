# Contract of the module inputs. No resources yet: the implementation follows in its own
# build tickets (infra/tofu/README.md). Shared inputs are the same in every target module.
# Secrets are never inputs of this module; they are created on the target system
# (`bothy secrets init`) or in the secret service of the target and never appear in the state.

variable "instance_name" {
  description = "Short name of the instance; prefix of every resource name."
  type        = string
  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{2,30}$", var.instance_name))
    error_message = "instance_name: lower-case letters, digits, hyphen; 3 to 31 characters; starts with a letter."
  }
}

variable "environment" {
  description = "Environment of the instance (docs/operations/environments-and-flags.md)."
  type        = string
  validation {
    condition     = contains(["dev", "staging", "prod"], var.environment)
    error_message = "environment: dev, staging or prod."
  }
}

variable "bothy_version" {
  description = "Server version (SemVer without v), e.g. 1.2.3. Never latest."
  type        = string
  validation {
    condition     = can(regex("^[0-9]+\\.[0-9]+\\.[0-9]+(-[0-9A-Za-z.-]+)?$", var.bothy_version))
    error_message = "bothy_version: SemVer without prefix."
  }
}

variable "image_registry" {
  description = "Registry path of the Bothy images in lower case, e.g. ghcr.io/<owner>."
  type        = string
}

variable "public_hostname" {
  description = "DNS name under which the apps reach the server, e.g. chat.example.com."
  type        = string
}

variable "edge_mode" {
  description = "reverse_proxy: TLS proxy in front of the server; tailscale: only in the tailnet through tailscale serve; private: only in the private network of the target."
  type        = string
  default     = "reverse_proxy"
  validation {
    condition     = contains(["reverse_proxy", "tailscale", "private"], var.edge_mode)
    error_message = "edge_mode: reverse_proxy, tailscale or private."
  }
}

variable "database_mode" {
  description = "managed: managed PostgreSQL service of the target (PITR by the provider); container: PostgreSQL in the Compose stack with pgBackRest."
  type        = string
  default     = "container"
  validation {
    condition     = contains(["managed", "container"], var.database_mode)
    error_message = "database_mode: managed or container."
  }
}

variable "object_storage_mode" {
  description = "filesystem: volume on the host; s3: S3-compatible service of the target; seaweedfs: SeaweedFS in the Compose stack."
  type        = string
  default     = "filesystem"
  validation {
    condition     = contains(["filesystem", "s3", "seaweedfs"], var.object_storage_mode)
    error_message = "object_storage_mode: filesystem, s3 or seaweedfs."
  }
}

variable "backup_offsite" {
  description = "Off-site backup target. type: s3 or sftp; location: bucket URL or host path. Never credentials here."
  type = object({
    type     = string
    location = string
  })
  default = null
}

variable "telemetry_exporters" {
  description = "Overlays from infra/otel/exporters, at most two."
  type        = list(string)
  default     = ["none"]
  validation {
    condition     = length(var.telemetry_exporters) <= 2 && alltrue([for e in var.telemetry_exporters : contains(["none", "local", "newrelic", "grafana", "datadog", "azure-monitor", "sentry"], e)])
    error_message = "telemetry_exporters: at most two of none, local, newrelic, grafana, datadog, azure-monitor, sentry."
  }
}

variable "admin_ssh_public_keys" {
  description = "Public SSH keys for maintaining the host (VM based targets only)."
  type        = list(string)
  default     = []
}

variable "tags" {
  description = "Additional tags or labels for every resource."
  type        = map(string)
  default     = {}
}

variable "location" {
  description = "Hetzner location, e.g. nbg1."
  type        = string
}

variable "server_type" {
  description = "Server type for the Compose host."
  type        = string
}
