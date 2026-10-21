# Inputs of the module generic-vm. The shared inputs (instance_name to tags) are the contract of
# every target module (infra/tofu/README.md); the inputs below them are this module's own.
# Secrets are never inputs of this module: the setup wizard install.sh generates them on the host
# (`secrets-init`, docs/spec/16-operations.md 5.3), so they never appear in the state or the plan.
# Every value that reaches the rendered host setup is checked here, because the setup files quote
# them in single quotes and one value per line.

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
  description = "managed: managed PostgreSQL service of the target (PITR by the provider); container: PostgreSQL in the Compose stack with pgBackRest. generic-vm supports container only (install.sh has no option for an external database)."
  type        = string
  default     = "container"
  validation {
    condition     = contains(["managed", "container"], var.database_mode)
    error_message = "database_mode: managed or container."
  }
}

variable "object_storage_mode" {
  description = "filesystem: volume on the host (install.sh --storage fs); s3: S3-compatible service of the target (not supported by generic-vm); seaweedfs: SeaweedFS in the Compose stack (--storage s3)."
  type        = string
  default     = "filesystem"
  validation {
    condition     = contains(["filesystem", "s3", "seaweedfs"], var.object_storage_mode)
    error_message = "object_storage_mode: filesystem, s3 or seaweedfs."
  }
}

variable "backup_offsite" {
  description = "Off-site target of the backup. type: s3 (sftp is not supported by bothy-backupd); location: the bucket URL https://<host>[:<port>]/<bucket>[/<prefix>]. Credentials never here: they go to secrets/offsite/ on the host."
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
  description = "Public SSH keys of ssh_user for the maintenance of the host (cloud-init delivery only)."
  type        = list(string)
  default     = []
  validation {
    condition     = alltrue([for k in var.admin_ssh_public_keys : can(regex("^(ssh-ed25519|ecdsa-sha2-nistp[0-9]+|ssh-rsa|sk-[a-z0-9@.-]+) [A-Za-z0-9+/=]+( [^\n]*)?$", k))])
    error_message = "admin_ssh_public_keys: OpenSSH public keys, one line each."
  }
}

variable "tags" {
  description = "Additional tags or labels for all resources. generic-vm creates no tagged resource; the target modules use them."
  type        = map(string)
  default     = {}
}

# ---- Inputs of generic-vm --------------------------------------------------------------------

variable "ssh_host" {
  description = "Address of the Linux host (Ubuntu or Debian with systemd), the target of the SSH delivery and of the output ssh_endpoint."
  type        = string
}

variable "ssh_user" {
  description = "User for the maintenance by SSH; with cloud-init it is created with admin_ssh_public_keys and password-less sudo, with the SSH delivery it must exist with sudo."
  type        = string
  default     = "bothy"
  validation {
    condition     = can(regex("^[a-z_][a-z0-9_-]{0,31}$", var.ssh_user))
    error_message = "ssh_user: a POSIX user name."
  }
}

variable "ssh_port" {
  description = "SSH port of the host for the SSH delivery and the output ssh_endpoint."
  type        = number
  default     = 22
}

variable "sshd_port" {
  description = "Port sshd listens on, on the host; the firewall admits it. null means ssh_port; set it only when ssh_port is a forwarded port in front of the host (NAT)."
  type        = number
  default     = null
}

variable "provision_over_ssh" {
  description = "false: the module only renders user_data, which the caller passes to its VM resource (new hosts); true: the same setup runs on the existing host ssh_host over SSH (the key comes from the SSH agent, never from an input)."
  type        = bool
  default     = false
}

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

variable "stack_dir" {
  description = "Directory of the installation on the host (install.sh --dir), owned by the user bothy."
  type        = string
  default     = "/srv/bothy"
  validation {
    condition     = can(regex("^/[A-Za-z0-9._/-]+$", var.stack_dir))
    error_message = "stack_dir: an absolute path of letters, digits and . _ / -."
  }
}
