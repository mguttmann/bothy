# sdk/python/examples

Examples built on the Python SDK, hand-written code of the component `sdk-python`: the same lint,
type, security and coverage rules as `src/` apply.

## guide/

The example adapter of the guide [Connect your own agent](../../../docs/guides/connect-agent.md)
(M2-37): one program in small modules, one per step of the guide. The guide shows every file whole,
and `services/server/test/guide` runs the guide against a fresh server, so the text, these files
and the server stay in step.

| File | Step |
|---|---|
| `keygen.py` | 2: the adapter's DPoP key and its thumbprint |
| `agent.py`, `connect.py` | 4: the command line, the handler and the gateway connection |
| `heartbeat.py` | 5: capabilities, billing mode and evidence |
| `commands.py` | 6: a model command with suggestions and a control command |
| `invocation.py` | 7: actual-mode comparison and the context with trust marks |
| `runtime.py`, `run.py` | 8: the rule-based runtime, the live box, the answer and the usage |
| `interactive.py` | 9: follow-ups, questions, approvals, stop and control commands |
| `tests/test_agent.py` | 10: the scenarios against the SDK's FakeServer |

Run from the repository root (the SDK comes from this checkout until it is published):

```sh
uv sync --project sdk/python --locked
uv run --project sdk/python --locked pytest sdk/python/examples/guide/tests
PYTHONPATH=sdk/python/src uv run --project sdk/python --locked python \
  sdk/python/examples/guide/agent.py --help
```

The runtime is a set of fixed rules instead of a model: nothing here calls a model or a third
party.
