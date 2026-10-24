# SPDX-License-Identifier: Apache-2.0
"""Cost reconciliation app (docs/spec/10-backend-policy.md section 12.4, ADR 0055).

The app runs as its own process outside the server. It reads the provider's admin key from its
own secret provider, fetches the daily cost report of the provider's cost API, distributes each
day's amount of a workspace over the estimated runs of the agents mapped to that workspace and
submits the result through the server's public API with an app token that holds only the scope
`costs:reconcile`. The admin key never leaves this process except towards the cost API.

Modules:
    config: the TOML configuration and its checks.
    secretref: `secret_ref` values (`file:`, `env:`) and the redacting secret type.
    transport: the HTTP opener without redirects and the request pacer.
    costapi: the client of the provider's cost report.
    amounts: exact cent sums and their conversion to micro units.
    server: the client of the server's two cost operations.
    distribute: the pure distribution rule (BR-10).
    reconcile: one pass.
    logs: JSON lines on stderr.
    uuid7: UUIDv7 values for `Idempotency-Key`.
"""

__version__ = "0.1.0"
