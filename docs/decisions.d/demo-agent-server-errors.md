# demo-agent-server-errors Every server error answer of the demo agent is classified by its status

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-demo-agent-server-errors-1 | 2026-10-27 | Supersedes the status list of D-demo-agent-resilience-1: the demo agent waits out `429`, every `5xx` and a request without an HTTP answer (`is_transient` in `sdk/python/src/bothy_sdk/testing/demo_agent.py`); every other `4xx` ends the agent it hit. | After one restart of the API, the TLS proxy answered `502` without a problem body and all demo agents stopped; a status list of single 5xx codes misses the next one. |
| D-demo-agent-server-errors-2 | 2026-10-27 | The Fake-Agent answers a server error that the SDK raises as a `ProblemError` (for example a body that is no problem) like any other refused answer: a successful reply with `status` and `code`, not an `internal` error. Only a `ProblemError` without an HTTP status (a gateway code the SDK does not know) stays `internal`. | A driver can only classify an answer by its status; an `internal` error hid the `502` from the demo agent. |
