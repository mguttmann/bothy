---
status: accepted
date: 2026-10-10
decision-makers: Planning lead, Team F (operations)
consulted: Team A (architecture), Team H (quality)
informed: all teams
---

# Multi-arch OCI images, Compose as the installation, OpenTofu per target

## Context and Problem Statement

The first target is a Mac; after that, Linux VMs and clouds (Azure, AWS, Hetzner and others). The
server is a Go binary with roles (B1), plus PostgreSQL 18, object storage, virus scanning, a
collector and optionally an observability stack. How is an instance installed so that the same way
works on all targets and a move is only configuration?

## Decision Drivers

* one command for a complete instance
* no Mac dependency in the server (B12)
* the same artifacts for amd64 and arm64 (Mac with Apple silicon, cloud)
* moving between targets without changes to the code (portability test)
* infrastructure as code for cloud targets

## Considered Options

* OCI images for linux/amd64 and linux/arm64, Docker Compose as the installation, OpenTofu modules
  per target, Helm later
* Kubernetes and Helm as the first way
* Native packages and system services without containers (Homebrew, deb, LaunchAgent, systemd)

## Decision Outcome

Chosen option: "OCI images, Compose, OpenTofu per target, Helm later", because Compose runs the
same on a Mac, a VM and in every cloud, and OpenTofu keeps the differences between the targets
outside the code.

Core of the implementation:

1. Images `bothy-server` (static Go binary, without a shell, without root) and `bothy-postgres`
   (`postgres:18` plus pgBackRest and `bothy-backupd`), each as a multi-arch manifest
   ([18-release.md](../spec/18-release.md) 7).
2. [infra/compose/compose.yaml](../../infra/compose/compose.yaml) with profiles for edge, scan, S3
   and observability; ports only on `127.0.0.1`; PostgreSQL only in the internal network.
3. OpenTofu modules `generic-vm`, `hetzner`, `aws`, `azure` with the same interface; the
   target modules call `generic-vm`. A managed database and object storage are options.
4. OpenTofu instead of Terraform: OpenTofu is under MPL-2.0 (GitHub license information of the
   repository opentofu/opentofu, retrieved 2026-10-10) [verified]; the Terraform repository reports
   no OSI license identifier ("NOASSERTION", same query) [verified]. In addition, the tool has
   state encryption.

### Consequences

* Good, because an instance operator with basic Docker knowledge can run an instance.
* Good, because the same stack runs in CI on a fresh runner as a portability test.
* Good, because auxiliary services (ClamAV, SeaweedFS, collector) stay separate processes (license
  and failure boundaries).
* Bad, because Compose offers no high availability; several `api` processes across hosts
  need Helm or a cloud platform later.
* Bad, because a VM is needed on the Mac (Docker Desktop or Colima) and its memory must suffice for
  ClamAV.
* Neutral, because `bothy-postgres` is a separate image that is versioned with the server.

### Confirmation

* `docker compose config --quiet` in the gate (all profiles).
* Portability test in CI ([16-operations.md](../spec/16-operations.md) 11.1) on amd64 and arm64.
* `tofu validate` of the modules in the gate.
* Image check in the release: multi-arch manifest, user without root, no shell in
  `bothy-server`.

## Pros and Cons of the Options

### OCI images, Compose, OpenTofu

* Good, because one way for all targets, including the Mac.
* Good, because Compose files are readable and checkable.
* Bad, because Compose features (`subpath`, `configs.content`) require a current Docker version.

### Kubernetes and Helm first

* Good, because high availability and scaling are built in.
* Bad, because a Mac or a small VM needs a disproportionate amount of operations work for it.
* Bad, because the first target (a Mac) is served worst by it.

### Native packages without containers

* Good, because no container runtime is needed and there is performance without a VM.
* Bad, because every target needs its own packages, services and paths; the Mac would become a
  special case in the code (contradicts B12).
* Bad, because ClamAV, PostgreSQL and the collector then depend on the operating system.

## More Information

* Operations: [16-operations.md](../spec/16-operations.md); modules: [infra/tofu/README.md](../../infra/tofu/README.md);
  Helm: [infra/helm/README.md](../../infra/helm/README.md).
* OpenTofu encrypts state and plan files
  (https://opentofu.org/docs/language/state/encryption/, retrieved 2026-10-10).
