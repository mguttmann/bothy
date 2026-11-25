# Package `platform/buildinfo`

Status: built (M1-10). Normative: [18-release.md](../../../../../docs/spec/18-release.md)
"Components" rule 5; on a difference the chapter wins.

## Purpose

The server knows its own version without linker flags: `Version()` returns a constant equal to
`services/server/VERSION`. A plain `go build` therefore reports the same version as the release
build. Users: `service.version` of the telemetry resource (M1-10, `platform/otel`), the
comparison with `instance.min_server_version` (M1-15) and the image labels (M1-23).

## Interface

- `Version() string`: SemVer 2.0.0 without a leading `v`, never empty.
- `Revision() string`: the commit the build paths embed with
  `-ldflags "-X github.com/OWNER/bothy/services/server/internal/platform/buildinfo.revision=<sha>"`
  (`services/server/Dockerfile` from the build argument `REVISION`, the one of the label
  `org.opencontainers.image.revision`, and `make -C services/server release-build`), or `unknown`
  for a plain `go build`. `bothy version` prints both (AUD-PKG-R2-owner-server).

## Rules

- A release changes `services/server/VERSION` and the constant `version` in `buildinfo.go` in the
  same commit. `TestVersionMatchesVERSIONFile` reads the file and fails when the two differ.
- No other package keeps its own copy of the version.

## Dependencies

None (platform package without project or vendor dependencies, `architecture/modules.yaml`).
