# Host setup of a Linux host for Bothy (brief M7-15, ADR 0012, infra/tofu/README.md).
#
# One script, two delivery paths (docs/DECISIONS.md D-M7-15-1): the static script
# files/bothy-setup.sh plus four rendered files (setup.env, install-options, firewall.nft and,
# in the private signing phase, release.pub). New hosts get them as cloud-init user data
# (output user_data, passed by the target modules to their VM resource); an existing host gets
# the same files over SSH from terraform_data.ssh_setup. The script installs Docker, verifies the
# release files and then runs the setup wizard install.sh --bundle, which generates every secret
# on the host; this module never writes .env or secrets/ itself.

locals {
  # cosign verifies the release signatures on the host. Pinned like every tool
  # (tools/bin/tools.lock.json); tools/ci/tests/test_tofu_generic_vm.py holds the values equal.
  cosign_version = "3.1.3"
  cosign_sha256 = {
    amd64 = "4629c757b7618056f8ddd7e2625ae9fdd94c0372a65049520bc7d9df9efc7f71"
    arm64 = "c5d324e091826b0d7a78eb16fef316450b4eb9aaec045611c08ba06f5e73220a"
  }

  # Mapping of the inputs to the options of install.sh (docs/spec/16-betrieb.md 3.1).
  edge          = var.edge_mode == "tailscale" ? "tailscale" : "direct"
  edge_port     = var.edge_mode == "reverse_proxy" ? "443" : ""
  storage       = var.object_storage_mode == "seaweedfs" ? "s3" : "fs"
  observability = contains(var.telemetry_exporters, "local") ? "on" : "off"
  offsite       = var.backup_offsite == null ? "" : "s3:${var.backup_offsite.location}"
  public_url    = "https://${var.public_hostname}"

  base_url = coalesce(
    var.release_base_url,
    "https://github.com/${var.release_repository}/releases/download/v${var.bothy_version}/",
  )
  # The signing identity of release.yml for product tags vX.Y.Z (docs/spec/18-release.md 6).
  identity_regexp = "^https://github\\.com/${replace(var.release_repository, ".", "\\.")}/\\.github/workflows/release\\.yml@refs/tags/v"

  install_options = concat(
    ["--yes", "--public-url", local.public_url, "--edge", local.edge],
    var.virus_scan == "auto" ? [] : ["--scan", var.virus_scan],
    ["--storage", local.storage, "--observability", local.observability],
    local.offsite == "" ? [] : ["--offsite", local.offsite],
    ["--owner-handle", var.owner_handle, "--owner-name", var.owner_name],
  )

  # KEY='value' lines in a fixed order; the inputs are checked to contain no quote or newline.
  config = [
    ["SETUP_VERSION", var.bothy_version],
    ["SETUP_BASE_URL", local.base_url],
    ["SETUP_SIGNING", var.release_public_key == null ? "keyless" : "key"],
    ["SETUP_IDENTITY_REGEXP", local.identity_regexp],
    ["SETUP_OIDC_ISSUER", "https://token.actions.githubusercontent.com"],
    ["SETUP_REGISTRY", var.image_registry],
    ["SETUP_STACK_DIR", var.stack_dir],
    ["SETUP_EDGE", local.edge],
    ["SETUP_EDGE_PORT", local.edge_port],
    ["SETUP_OWNER_HANDLE", var.owner_handle],
    ["SETUP_COSIGN_VERSION", local.cosign_version],
    ["SETUP_COSIGN_SHA256_AMD64", local.cosign_sha256.amd64],
    ["SETUP_COSIGN_SHA256_ARM64", local.cosign_sha256.arm64],
  ]

  files = merge(
    {
      "/usr/local/lib/bothy/bothy-setup" = {
        content = file("${path.module}/files/bothy-setup.sh")
        mode    = "0755"
      }
      "/etc/bothy/setup.env" = {
        content = join("", concat(
          ["# Rendered by the OpenTofu module generic-vm for ${var.instance_name} (${var.environment}); no secrets.\n"],
          [for kv in local.config : "${kv[0]}='${kv[1]}'\n"],
        ))
        mode = "0644"
      }
      "/etc/bothy/install-options" = {
        content = join("", [for o in local.install_options : "${o}\n"])
        mode    = "0644"
      }
      "/etc/bothy/firewall.nft" = {
        content = templatefile("${path.module}/templates/firewall.nft.tftpl", { edge_port = local.edge_port })
        mode    = "0644"
      }
    },
    var.release_public_key == null ? {} : {
      "/etc/bothy/release.pub" = {
        content = var.release_public_key
        mode    = "0644"
      }
    },
  )

  users = length(var.admin_ssh_public_keys) == 0 ? [] : [{
    name                = var.ssh_user
    groups              = "sudo"
    sudo                = "ALL=(ALL) NOPASSWD:ALL"
    shell               = "/bin/bash"
    lock_passwd         = true
    ssh_authorized_keys = var.admin_ssh_public_keys
  }]

  cloud_config = merge(
    length(local.users) == 0 ? {} : { users = local.users },
    {
      write_files = [for path in sort(keys(local.files)) : {
        path        = path
        content     = local.files[path].content
        permissions = local.files[path].mode
        owner       = "root:root"
      }]
      runcmd = [["/usr/local/lib/bothy/bothy-setup"]]
    },
  )

  # The SSH delivery: a shell script that writes the same files and runs the same setup.
  ssh_payload = join("", concat(
    [
      "#!/bin/sh\n",
      "# Written by the OpenTofu module generic-vm (SSH delivery of the host setup).\n",
      "set -eu\n",
      "umask 022\n",
      "put() { mkdir -p \"$(dirname \"$1\")\"; printf '%s' \"$3\" | base64 -d >\"$1.tmp\"; chmod \"$2\" \"$1.tmp\"; mv -f \"$1.tmp\" \"$1\"; }\n",
    ],
    [for path in sort(keys(local.files)) : "put '${path}' '${local.files[path].mode}' '${base64encode(local.files[path].content)}'\n"],
    var.release_public_key == null ? ["rm -f /etc/bothy/release.pub\n"] : [],
    ["exec /usr/local/lib/bothy/bothy-setup\n"],
  ))

  # Inputs of the shared contract that this module cannot honour; the plan stops on them.
  supported = {
    database  = var.database_mode == "container"
    storage   = var.object_storage_mode != "s3"
    telemetry = alltrue([for e in var.telemetry_exporters : contains(["none", "local"], e)])
    offsite   = var.backup_offsite == null ? true : var.backup_offsite.type == "s3"
  }
}

# The SSH delivery for an existing host. The payload is written to a file of mktemp (owned by
# ssh_user, mode 0600) and run with sudo, so another user of the host cannot change it, and the
# setup does not read its own script from standard input. The private key comes from the SSH
# agent; it never is an input or part of the state.
resource "terraform_data" "ssh_setup" {
  count = var.provision_over_ssh ? 1 : 0

  triggers_replace = sha256(local.ssh_payload)

  connection {
    type    = "ssh"
    host    = var.ssh_host
    port    = var.ssh_port
    user    = var.ssh_user
    agent   = true
    timeout = "5m"
  }

  provisioner "remote-exec" {
    inline = [
      "f=$(mktemp) && printf '%s' '${base64encode(local.ssh_payload)}' | base64 -d >\"$f\" && sudo sh \"$f\"; rc=$?; rm -f \"$f\"; exit $rc",
    ]
  }

  lifecycle {
    precondition {
      condition     = local.supported.database
      error_message = "database_mode = \"managed\" is not supported by generic-vm: install.sh has no option for an external database."
    }
    precondition {
      condition     = local.supported.storage
      error_message = "object_storage_mode = \"s3\" is not supported by generic-vm: install.sh has no option for an external S3 service; use filesystem or seaweedfs."
    }
    precondition {
      condition     = local.supported.telemetry
      error_message = "telemetry_exporters: generic-vm supports none and local; a vendor exporter needs credentials and is set up on the host."
    }
    precondition {
      condition     = local.supported.offsite
      error_message = "backup_offsite.type = \"sftp\" is not supported: bothy-backupd writes to an S3 bucket only."
    }
  }
}
