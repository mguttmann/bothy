---
status: accepted
date: 2026-10-10
decision-makers: Planning lead, Team G (platform services)
---

# Object storage behind an S3 interface, local driver as the default

## Context and Problem Statement

Bothy stores uploaded files, thumbnails and voice messages. The first target is a
single Mac with a container runtime; later targets are Linux VMs and clouds (Baseline B4, B12).
Where do the bytes live, and how does the server stay portable between these targets without
forcing an additional storage service on a small target?

## Decision Drivers

- Portability between Mac, Linux VM and cloud without code changes.
- Small installation without an additional service.
- Every retrieval through `authorize` (B7); bytes never without a check.
- No vendor lock-in; only widely supported operations.
- Simple backup and restore together with PostgreSQL.
- License and maintenance situation of the self-hosted S3 services (MinIO archived).

## Considered Options

- Port `ObjectStore` with a file system driver (default) and an S3 driver that uses only core S3
- Only S3, locally with a self-hosted S3 service
- Files in PostgreSQL (`bytea` or large objects)
- Only file system

## Decision Outcome

Chosen option: "Port `ObjectStore` with a file system driver (default) and an S3 driver that uses
only core S3", because it serves the small target without a further service and uses any
S3-compatible storage in the cloud. Core S3 means: PUT, GET (with range), HEAD, DELETE of single
objects, multipart upload and presigned URLs for upload parts. No listing, no copying,
no batch delete, no versioning, no bucket policies. Details:
[12-files.md](../spec/12-files.md) section 4.

### Consequences

- Good, because an installation on a single host needs only the server, PostgreSQL and ClamAV.
- Good, because moving to a cloud is only configuration and copying the objects; the
  key scheme is the same in both drivers.
- Good, because the S3 driver, with few operations, also works with services outside of AWS
  (for example SeaweedFS).
- Good, because the server knows every key from the database and never has to list.
- Bad, because two drivers must be tested. A shared conformance test suite
  covers this.
- Bad, because the file system driver only scales with one `api` node or a shared
  volume. Several nodes need the S3 driver.
- Bad, because downloads run through the server (decision R-FI-03 in 12-files); this
  costs bandwidth, but is necessary for `authorize` at retrieval time.

### Confirmation

- A conformance test suite runs against the file system driver and against SeaweedFS in a
  test container (Put, Get with range, Head, Delete of missing objects, Multipart with abort).
  The suite is `filestest.RunObjectStoreConformance` in
  `services/server/internal/files/adapters/filestest`; the file system driver runs it in
  `TestFSConformance` (M3-01), the S3 driver follows with M3-02.
- The S3 driver depends only on a Go interface with exactly the core S3 operations; an
  architecture rule allows the import of the S3 SDK only in `internal/files/adapters/s3`.
- A property test checks that no object key contains channel IDs, space IDs or
  names.
  The property test is `TestObjectKeysCarryNoContext` in
  `services/server/internal/files/domain`.

## Pros and Cons of the Options

### Port with a file system driver (default) and an S3 driver with core S3

- Good, because the smallest operational footprint on a single host.
- Good, because cloud storage can be used without vendor lock-in.
- Neutral, because presigned uploads only work with S3; they are optional.
- Bad, because two implementations must be maintained.

### Only S3, locally with a self-hosted S3 service

- Good, because only one driver exists.
- Bad, because every installation must run and back up a further service.
- Bad, because the obvious choice, MinIO, is archived and no longer ships binaries
  [Q1, verified]; alternatives such as Garage are under AGPLv3 and have limited S3 features
  [Q2, verified].

### Files in PostgreSQL

- Good, because backup and transaction are one unit.
- Bad, because large binary data bloats the database, WAL, backups and replication.
- Bad, because range requests for audio and video are cumbersome and cloud object storage
  stays unused.

### Only file system

- Good, because simplest.
- Bad, because several nodes and managed cloud storage are ruled out and a move
  needs code changes.

## More Information

- Sources (retrieved 2026-10-10): Q1 https://github.com/minio/minio [verified]; Q2
  https://garagehq.deuxfleurs.fr/documentation/reference-manual/s3-compatibility/ and
  https://raw.githubusercontent.com/deuxfleurs-org/garage/main-v2/LICENSE [verified];
  SeaweedFS https://github.com/seaweedfs/seaweedfs [verified]; AWS SDK checksums
  https://docs.aws.amazon.com/sdkref/latest/guide/feature-dataintegrity.html [verified].
- The S3 driver sets the SDK checksums to `when_required`, because the default checksums of the
  SDKs break several S3-compatible services; our own SHA-256 per file replaces them.
- Baseline B4 and ruling R-Objektspeicher; [12-files.md](../spec/12-files.md).
