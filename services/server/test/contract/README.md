# Package `test/contract`

Status: built (M1-71). Component `testsupport`. G7 of
[06-api.md](../../../../docs/spec/06-api.md) "Contract tests and gate".

## Run

```sh
make contract-test                      # quality.contract.max_examples and quality.contract.seed
CONTRACT_SEED=<seed> make contract-test # draw other cases
```

The seed is fixed (named value `quality.contract.seed`): every run draws the same cases in the
owner and user passes and in the stateful phase, so a failure repeats on a rerun instead of
passing with another seed (flake list #238). The test fails when `CONTRACT_SEED` is not a decimal
unsigned integer; it never draws a seed of its own.

`TestG7_SchemathesisAllEnabledOperations` (build tags `contract_validate`, `contract_live`) starts
the harness server with `EgressDenyAll`, APNs enabled for `registerPush` (topic `com.example.chat`, the adapter pointed at a Fake-APNs that egress denial keeps unreached; the test fails if a request reaches it) and every rate-limit class raised (23 R-QA-11: G7 checks
conformance, M1-19 owns throttling), builds the matrix world, starts the credential broker and runs
`uv run --project tools --locked pytest -p no:cacheprovider tools/contract/live_contract.py` from the
repository root. It prints `CONTRACT_SEED` and fails when pytest exits non-zero. The server runs
with G8 (tag `contract_validate`), so a response outside the contract also ends the run.

## Credential broker

`broker.go`, an HTTP server of the test on `127.0.0.1:0`, never part of the product:

- `POST /principals` with `{"kind":"owner"}` returns a new device of the first owner (recovery code
  of the setup service), with `{"kind":"user"}` a new device of Alex of the matrix world
  (`createEnrollmentCode`): `{principal_id, device_id, session_id, access_token, refresh_token,
  dpop_private_jwk}`.
- `POST /step-up` with `{"session_id"}` performs a fresh step-up for a session it handed out.
- Requests from other addresses than loopback get `403`.

## What is not proven

- The stateful phase runs over the transitions Schemathesis 4.30.1 infers from openapi.yaml, which
  has no `links`; the run prints `inferred_transitions` in its summary line. When it is 0, the
  stateful part is not proven (06 G7).
- The Compose stack: the smoke test of M1-73 and M7 cover it.
