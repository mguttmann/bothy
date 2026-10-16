# Package `platform/httpserver/contractcheck`

Status: built (M1-71). G8 of [06-api.md](../../../../../../docs/spec/06-api.md) "Contract tests and
gate": in the test build every response of the public listener is validated against
[`docs/api/openapi.yaml`](../../../../../../docs/api/openapi.yaml).

## How it is installed

- Every file of the package has the build tag `contract_validate`. The file
  `../contractcheck_install.go` (same tag) sets the response hook of `Chain.Public`, which hands the
  parsed contract and the mounted operations of the chain's request validator to `Attach`; the
  untagged production build has neither the package nor a reference to it
  (`TestG8_ProductionBuildExcludesValidator` holds that with `go list -deps ./cmd/bothy`,
  `TestG8_ChainInstallsTheCheck` the installation).
- `Attach` checks only a chain whose contract is byte for byte `docs/api/openapi.yaml` (SHA-256);
  chains over test fixture contracts stay unchecked, and the contract is parsed once per process.
- `make test` runs `go test -race -tags contract_validate ./...`, so G8 runs in every Go
  integration test that serves through `Chain.Public` (harness, wire, httpservertest, module
  worlds).

## What it checks

- The contract is `docs/api/openapi.yaml`, found by walking up from the working directory; a missing
  file ends the process with exit code 3. The operations are those the chain mounts (the enable list
  `services/server/api/operations.yaml` in the server).
- A request whose method and path match an enabled operation (the same `net/http` routing as the
  server) gets its status, headers, media type and body validated with
  `github.com/pb33f/libopenapi-validator` (OpenAPI 3.1, JSON Schema 2020-12, format assertions on;
  `oneOf` with discriminator and `format: uuid` are covered by `TestG8_ValidatesDiscriminatorAndUUID`).
  `application/x-ndjson` bodies are validated line by line. The route decides the operation, also
  where a literal path of another method exists (`GET /grants/preview` is `getGrant`).
- Unmatched requests (internal listener, unknown paths, other methods) pass unchecked; AZ-P1 already
  proves that every registered route is an operation.

## On a violation

The report goes to standard error, one line per finding with `operationId`, method, route, status,
JSON pointer, schema keyword, reason and the offending value (truncated to 200 bytes), and the
process ends with exit code 3.

Ruling (M1-71, [DECISIONS.md](../../../../../../docs/DECISIONS.md) M1-71/2): a hard exit is the only
mechanism that fails every integration test regardless of how it asserts, without touching each
test; it exists only in the test build.

## Interface

- `Attach(specSum, base, validator, routes)`: the check of the chain (exit on violation).
- `FromValidator`, `Load()`, `New(spec, operationIDs)`, `Checker.Wrap`, `Checker.WithFailure`: the
  same check over an own build, with a recording failure handler for tests;
  `EnabledOperations(list)` reads an enable list.
