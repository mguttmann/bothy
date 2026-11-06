# Byte-equal baseline of the runner

These files hold what the Claude Code runner produces without any key of stage A1
(`runner.hosted_agents` and the later keys of `docs/spec/26-builtin-agents.md` 14). They were
recorded from the runner before any change of ticket A1-04, and
`tests/unit/test_byte_equal_baseline.py` compares the current runner with them byte for byte
(BA-T20, 26 3.2 item 4 and 11 item 3).

| File | What |
|---|---|
| `command_line_chat.json`, `command_line_scoped.json`, `command_line_host.json` | The argument list of one turn per tool class (without the program path), with Fake-Claude in the forum Projekt-A, which carries channel skills of both forms (`claude-code:review` and `other-adapter:notes`) |
| `heartbeat_start.json` | The heartbeat body after the start, once the first folder check ran |
| `heartbeat_with_run.json` | The heartbeat body while one turn runs |
| `heartbeat_degraded.json` | The heartbeat body with a CLI below the supported version |
| `served_channels.json` | `listServedChannels` as the runner's channel cache holds it |

## How they were recorded

With the scenario world of `tests/scenarios/conftest.py` (FakeServer and Fake-Claude, the
FakeServer's clock and IDs), from the directory of the runner:

```sh
CLAUDE_CODE_RUNNER_RECORD_BASELINE=1 uv run --locked pytest -q tests/unit/test_byte_equal_baseline.py
```

Before the comparison the test replaces what differs between two runs of the same scenario: the
temporary folder of the scenario by `<root>`, the forums, the agent and the instance by their
names, every other UUID by `<id-N>` in the order of its first appearance, and the runner's own
release version by `<runner-version>`. Everything else is compared as it is.

## Changing them

These files change only with a ruling in `docs/decisions.d/` that names the file, the changed
bytes and the reason. A change of the runner that makes the comparison fail changes the
behaviour of a running installation without a key, which stage A1 forbids (26 11).
