# Vertrag der Modul-Eingaben. Noch keine Ressourcen: Die Implementierung folgt in eigenen
# Bau-Tickets (infra/tofu/README.md). Gemeinsame Eingaben sind in allen Zielmodulen gleich.
# Geheimnisse sind nie Eingaben dieses Moduls; sie entstehen auf dem Zielsystem
# (`bothy secrets init`) oder im Secret-Dienst des Ziels und erscheinen nie im State.

variable "instance_name" {
  description = "Kurzname der Instanz; Präfix aller Ressourcennamen."
  type        = string
  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{2,30}$", var.instance_name))
    error_message = "instance_name: Kleinbuchstaben, Ziffern, Bindestrich; 3 bis 31 Zeichen; beginnt mit Buchstaben."
  }
}

variable "environment" {
  description = "Umgebung der Instanz (docs/operations/environments-and-flags.md)."
  type        = string
  validation {
    condition     = contains(["dev", "staging", "prod"], var.environment)
    error_message = "environment: dev, staging oder prod."
  }
}

variable "bothy_version" {
  description = "Server-Version (SemVer ohne v), z. B. 1.2.3. Nie latest."
  type        = string
  validation {
    condition     = can(regex("^[0-9]+\\.[0-9]+\\.[0-9]+(-[0-9A-Za-z.-]+)?$", var.bothy_version))
    error_message = "bothy_version: SemVer ohne Präfix."
  }
}

variable "image_registry" {
  description = "Registry-Pfad der Bothy-Images in Kleinbuchstaben, z. B. ghcr.io/<owner>."
  type        = string
}

variable "public_hostname" {
  description = "DNS-Name, unter dem Apps den Server erreichen, z. B. chat.example.com."
  type        = string
}

variable "edge_mode" {
  description = "reverse_proxy: TLS-Proxy vor dem Server; tailscale: nur im Tailnet über tailscale serve; private: nur im privaten Netz des Ziels."
  type        = string
  default     = "reverse_proxy"
  validation {
    condition     = contains(["reverse_proxy", "tailscale", "private"], var.edge_mode)
    error_message = "edge_mode: reverse_proxy, tailscale oder private."
  }
}

variable "database_mode" {
  description = "managed: verwalteter PostgreSQL-Dienst des Ziels (PITR durch den Anbieter); container: PostgreSQL im Compose-Stack mit pgBackRest."
  type        = string
  default     = "container"
  validation {
    condition     = contains(["managed", "container"], var.database_mode)
    error_message = "database_mode: managed oder container."
  }
}

variable "object_storage_mode" {
  description = "filesystem: Volume auf dem Host; s3: S3-kompatibler Dienst des Ziels; seaweedfs: SeaweedFS im Compose-Stack."
  type        = string
  default     = "filesystem"
  validation {
    condition     = contains(["filesystem", "s3", "seaweedfs"], var.object_storage_mode)
    error_message = "object_storage_mode: filesystem, s3 oder seaweedfs."
  }
}

variable "backup_offsite" {
  description = "Off-site-Ziel der Sicherung. type: s3 oder sftp; location: Bucket-URL oder Host-Pfad. Zugangsdaten nie hier."
  type = object({
    type     = string
    location = string
  })
  default = null
}

variable "telemetry_exporters" {
  description = "Overlays aus infra/otel/exporters, höchstens zwei."
  type        = list(string)
  default     = ["none"]
  validation {
    condition     = length(var.telemetry_exporters) <= 2 && alltrue([for e in var.telemetry_exporters : contains(["none", "local", "newrelic", "grafana", "datadog", "azure-monitor", "sentry"], e)])
    error_message = "telemetry_exporters: höchstens zwei aus none, local, newrelic, grafana, datadog, azure-monitor, sentry."
  }
}

variable "admin_ssh_public_keys" {
  description = "Öffentliche SSH-Schlüssel für die Wartung des Hosts (nur VM-basierte Ziele)."
  type        = list(string)
  default     = []
}

variable "tags" {
  description = "Zusätzliche Tags oder Labels für alle Ressourcen."
  type        = map(string)
  default     = {}
}

variable "region" {
  description = "AWS-Region, z. B. eu-central-1."
  type        = string
}

variable "instance_type" {
  description = "EC2-Instanztyp für den Compose-Host."
  type        = string
}

variable "rds_instance_class" {
  description = "Instanzklasse für Amazon RDS for PostgreSQL (nur bei database_mode = managed)."
  type        = string
  default     = ""
}
