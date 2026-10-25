# tools/contract

The Python side of G7 ([06-api.md](../../docs/spec/06-api.md) "Contract tests and gate"):
Schemathesis 4.30.1 against the server that the Go test
`services/server/test/contract` starts (`make contract-test`). Never run alone: the Go test sets the
environment (base URL, broker URL, paths of `openapi.yaml` and the enable list, examples, seed and
the lifetimes of the session).

| Module | Does |
|---|---|
| `dpop.py` | ES256 DPoP proofs (RFC 9449): header `typ=dpop+jwt`, `alg`, public `jwk`; claims `jti`, `htm`, `htu` without query and fragment, `iat`, `ath`, `nonce`; raw `r||s` signature with `cryptography`. Command line for reproductions: `PYTHONPATH=tools/contract uv run --project tools python -m dpop --key <file> --htm <M> --htu <URL> [--nonce N]` (with `BOTHY_CONTRACT_AUTH="DPoP <token>"` the proof carries `ath`). |
| `broker_client.py` | `POST /principals` and `POST /step-up` of the credential broker; raw loopback HTTP. |
| `auth.py` | state of one pass: credentials and a step-up from the broker, renewed at half of `min(auth.step_up_max_age, auth.access_token_ttl)`; the newest `DPoP-Nonce` (a `getMe` probe when none is known or it may be older than `auth.dpop_nonce_ttl`); new credentials after `session_revoked`, `unauthenticated` or `token_expired`; the Schemathesis auth provider (`get`, `set`); the scheme `pushPreviewToken` of `getPushPreview`: per request a fresh `registerPush` of an APNs target of the pass's device and `Authorization: DPoP <preview token>` with a proof of the registering session's key, `ath` over the preview token. |
| `checks.py` | project checks: `problem_format`, `correlation_id`, `no_json_null`, `not_found_equality` (skipped for a malformed path parameter, which is no ID). |
| `operations.py` | the enabled operation IDs; an ID missing from `openapi.yaml` fails. |
| `reporting.py` | `curl` reproduction with `"$BOTHY_CONTRACT_AUTH"` and a fresh proof instead of the credentials, plus the seed. |
| `live_contract.py` | the live tests: one test per enabled operation and pass (owner, Alex; a positive `unregisterPush` removes a fresh registration, a positive `getPushPreview` asks for every second `nid` for a readable message of the world, and a pass that called it fails unless a preview token was accepted once), the stateful phase over the links Schemathesis infers, SEC-T06 (`x_unknown_field` gives `400 validation_failed`, a duplicate member `400 malformed_request`; the two post operations of the incoming webhooks answer the uniform `404 not_found` instead, because their token check runs before the body is read and the pass holds no webhook token) and the coverage check that fails when an enabled operation was not called in a pass. A positive case of an operation with the action `user.disable` or `owner.remove` never names a principal of a pass: `_shield` gives it an unknown person, so a generated call cannot end the owner or user the broker issues sessions for (D-M6-05-8). Its name does not match `test_*.py`, so the default `pytest` run never collects it. |

Unit tests: `tests/` (run by `make test`). A run prints one line `G7 summary:` with the seed, the
inferred transitions and the answers per pass and status class. To narrow a reproduction, set
`CONTRACT_SEED` and `CONTRACT_PYTEST_K` (a pytest `-k` expression) for `make contract-test`; a
narrowed run fails its coverage check by design.
