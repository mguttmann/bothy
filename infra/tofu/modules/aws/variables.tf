# Inputs of the module aws. The shared inputs (instance_name to tags) are the contract of every
# target module (infra/tofu/README.md "Structure" item 2) with the types and checks of
# generic-vm; the setup inputs pass through to generic-vm unchanged; the inputs below them are
# this module's own. Secrets are never inputs: the setup wizard generates them on the host, and
# the credentials of the provider belong to the root that configures it, so neither appears in
# an input, an output or the state.

# ---- Contract of the module inputs (shared by every target module) --------------------------

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
  description = "Version of the product release (SemVer without v), e.g. 1.2.3, whose installation bundle is installed; the bundle pins the server version (docs/spec/18-release.md 2.1). Never latest."
  type        = string
  validation {
    condition     = can(regex("^[0-9]+\\.[0-9]+\\.[0-9]+(-[0-9A-Za-z.-]+)?$", var.bothy_version))
    error_message = "bothy_version: SemVer without prefix."
  }
}

variable "image_registry" {
  description = "Registry path of the server images in lower case, e.g. ghcr.io/<owner>. The setup stops when the bundle pins another registry."
  type        = string
  validation {
    condition     = can(regex("^[a-z0-9][a-z0-9.:/_-]*$", var.image_registry))
    error_message = "image_registry: lower case letters, digits and . : / _ - only."
  }
}

variable "public_hostname" {
  description = "DNS name under which the apps reach the server, e.g. chat.example.com."
  type        = string
  validation {
    condition     = can(regex("^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$", var.public_hostname))
    error_message = "public_hostname: a lower-case DNS name with at least one dot."
  }
}

variable "edge_mode" {
  description = "reverse_proxy: a TLS proxy on the host or of the target in front of the server (install.sh --edge direct, port 443 public); tailscale: only in the tailnet through tailscale serve (--edge tailscale, no public port); private: only in the private network of the target (--edge direct, no public port)."
  type        = string
  default     = "reverse_proxy"
  validation {
    condition     = contains(["reverse_proxy", "tailscale", "private"], var.edge_mode)
    error_message = "edge_mode: reverse_proxy, tailscale or private."
  }
}

variable "database_mode" {
  description = "managed: managed PostgreSQL service of the target; container: PostgreSQL in the Compose stack with pgBackRest. aws accepts container only (docs/DECISIONS.md D-M7-16-4)."
  type        = string
  default     = "container"
  validation {
    condition     = contains(["managed", "container"], var.database_mode)
    error_message = "database_mode: managed or container."
  }
  validation {
    condition     = var.database_mode == "container"
    error_message = "database_mode = managed is not supported by aws: Amazon RDS for PostgreSQL offers PostgreSQL 18, but install.sh has no option for an external database (docs/DECISIONS.md D-M7-16-4)."
  }
}

variable "object_storage_mode" {
  description = "filesystem: volume on the host (install.sh --storage fs); seaweedfs: SeaweedFS in the Compose stack (--storage s3); s3: an S3 service of the target, refused by aws (docs/DECISIONS.md D-M7-16-4)."
  type        = string
  default     = "filesystem"
  validation {
    condition     = contains(["filesystem", "s3", "seaweedfs"], var.object_storage_mode)
    error_message = "object_storage_mode: filesystem, s3 or seaweedfs."
  }
  validation {
    condition     = var.object_storage_mode != "s3"
    error_message = "object_storage_mode = s3 is not supported by aws: install.sh has no option for an external S3 service such as Amazon S3; use filesystem or seaweedfs (docs/DECISIONS.md D-M7-16-4)."
  }
}

variable "backup_offsite" {
  description = "Off-site target of the backup that exists already. type: s3 (sftp is not supported by bothy-backupd); location: the bucket URL https://<host>[:<port>]/<bucket>[/<prefix>]. Credentials never here: they go to secrets/offsite/ on the host. null with offsite_region set: the module creates the bucket."
  type = object({
    type     = string
    location = string
  })
  default = null
  validation {
    condition     = var.backup_offsite == null ? true : can(regex("^https?://[^[:space:]'\"\\\\$`]+$", var.backup_offsite.location))
    error_message = "backup_offsite.location: an http(s) URL without spaces, quotes, backslash, $ or backtick."
  }
}

variable "telemetry_exporters" {
  description = "Overlays from infra/otel/exporters, at most two. generic-vm supports none and local (install.sh --observability on); the vendor exporters need credentials and are set up on the host."
  type        = list(string)
  default     = ["none"]
  validation {
    condition     = length(var.telemetry_exporters) <= 2 && alltrue([for e in var.telemetry_exporters : contains(["none", "local", "newrelic", "grafana", "datadog", "azure-monitor", "sentry"], e)])
    error_message = "telemetry_exporters: at most two of none, local, newrelic, grafana, datadog, azure-monitor, sentry."
  }
}

variable "admin_ssh_public_keys" {
  description = "Public SSH keys of the maintenance user bothy (cloud-init); empty means no SSH login."
  type        = list(string)
  default     = []
  validation {
    condition     = alltrue([for k in var.admin_ssh_public_keys : can(regex("^(ssh-ed25519|ecdsa-sha2-nistp[0-9]+|ssh-rsa|sk-[a-z0-9@.-]+) [A-Za-z0-9+/=]+( [^\n]*)?$", k))])
    error_message = "admin_ssh_public_keys: OpenSSH public keys, one line each."
  }
}

variable "tags" {
  description = "Additional tags or labels for every resource of the module."
  type        = map(string)
  default     = {}
}

# ---- Setup inputs, passed on to generic-vm --------------------------------------------------

variable "release_repository" {
  description = "GitHub repository <owner>/<name> of the product releases; it names the download address and, with keyless signatures, the signing identity of its release workflow."
  type        = string
  validation {
    condition     = can(regex("^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$", var.release_repository))
    error_message = "release_repository: <owner>/<name>."
  }
}

variable "release_base_url" {
  description = "Download address of the release files, ending in /; null means https://github.com/<release_repository>/releases/download/v<bothy_version>/. Integrity comes from the signatures, not from the transport."
  type        = string
  default     = null
  validation {
    condition     = var.release_base_url == null ? true : can(regex("^https?://[^[:space:]'\"\\\\$`]+/$", var.release_base_url))
    error_message = "release_base_url: an http(s) URL ending in / without spaces, quotes, backslash, $ or backtick."
  }
}

variable "release_public_key" {
  description = "PEM public key of the release signatures in the private phase (quality/release/cosign-private-phase.pub, ADR 0062); null verifies keyless against the release workflow of release_repository."
  type        = string
  default     = null
  validation {
    condition     = var.release_public_key == null ? true : can(regex("^-----BEGIN PUBLIC KEY-----\n[A-Za-z0-9+/=\n]+-----END PUBLIC KEY-----\n?$", var.release_public_key))
    error_message = "release_public_key: one PEM public key."
  }
}

variable "owner_handle" {
  description = "Handle of the first owner (install.sh --owner-handle)."
  type        = string
  validation {
    condition     = can(regex("^[a-z0-9][a-z0-9._-]*$", var.owner_handle))
    error_message = "owner_handle: a-z, 0-9, '.', '_' and '-', starting with a-z or 0-9."
  }
}

variable "owner_name" {
  description = "Display name of the first owner (install.sh --owner-name)."
  type        = string
  validation {
    condition     = length(var.owner_name) > 0 && !can(regex("[[:cntrl:]'\"\\\\$`]", var.owner_name))
    error_message = "owner_name: not empty, without control characters, quotes, backslash, $ or backtick."
  }
}

variable "virus_scan" {
  description = "auto: install.sh decides by the memory of the host (on from 4 GiB); on or off: install.sh --scan."
  type        = string
  default     = "auto"
  validation {
    condition     = contains(["auto", "on", "off"], var.virus_scan)
    error_message = "virus_scan: auto, on or off."
  }
}

# ---- Inputs of the cloud modules -----------------------------------------------------------

variable "ssh_allowed_cidrs" {
  description = "Networks (CIDR) from which the security rules of the target admit SSH to the host; empty admits none (maintenance then goes through the console of the provider or the tailnet). The whole internet (/0) is refused: only the edge port is public."
  type        = list(string)
  default     = []
  validation {
    condition     = alltrue([for c in var.ssh_allowed_cidrs : can(cidrhost(c, 0))])
    error_message = "ssh_allowed_cidrs: CIDR blocks such as 198.51.100.0/24 or 2001:db8::/32."
  }
  validation {
    condition     = alltrue([for c in var.ssh_allowed_cidrs : !endswith(c, "/0")])
    error_message = "ssh_allowed_cidrs: the whole internet (/0) is refused; only the edge port is public (infra/tofu/README.md rule 3)."
  }
}

# ---- Inputs of aws ------------------------------------------------------------------------------------

variable "region" {
  description = "AWS region of every resource of the host, e.g. eu-central-1. The module sets it on each resource, so the region of the provider configuration does not matter."
  type        = string
  validation {
    condition     = can(regex("^[a-z]{2}(-[a-z]+)+-[0-9]+$", var.region))
    error_message = "region: an AWS region name such as eu-central-1."
  }
}

variable "instance_type" {
  description = "EC2 instance type of the Compose host, e.g. t4g.medium; the Ubuntu image follows its architecture (arm64 or x86_64)."
  type        = string
}

variable "root_volume_size" {
  description = "Size of the encrypted system disk in GiB; it holds the Docker data, the volumes of the stack and the local backup repository."
  type        = number
  default     = 40
  validation {
    condition     = var.root_volume_size >= 20
    error_message = "root_volume_size: at least 20 GiB."
  }
}

variable "offsite_region" {
  description = "AWS region of an off-site bucket that the module creates, with Object Lock in compliance mode (another region than region); null creates none. Excludes backup_offsite."
  type        = string
  default     = null
  validation {
    condition     = var.offsite_region == null ? true : can(regex("^[a-z]{2}(-[a-z]+)+-[0-9]+$", var.offsite_region))
    error_message = "offsite_region: an AWS region name such as eu-north-1."
  }
  validation {
    condition     = var.offsite_region == null || var.backup_offsite == null
    error_message = "offsite_region and backup_offsite exclude each other: either the module creates the off-site bucket or it gets an existing one."
  }
}

variable "offsite_retention_days" {
  description = "Days for which Object Lock in compliance mode keeps every object version of the created off-site bucket (nobody, not even the account root, deletes a version earlier); a lifecycle rule removes noncurrent versions the same number of days after they became noncurrent. At least backup.objects.retention (default 90 days, docs/operations/backup-restore.md 2)."
  type        = number
  default     = 90
  validation {
    condition     = var.offsite_retention_days >= 1 && var.offsite_retention_days <= 36500 && floor(var.offsite_retention_days) == var.offsite_retention_days
    error_message = "offsite_retention_days: a whole number from 1 to 36500."
  }
}
