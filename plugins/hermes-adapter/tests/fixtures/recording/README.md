# Evidence of the recording (M2-32)

These files are written by `make -C plugins/hermes-adapter record` (never by hand). They prove
the answers in docs/spec/09-agents.md 17.10 against the version in `tests/hermes_pin.toml`.

| File | Content | Proves |
|---|---|---|
| `<scenario>.json` | scenario, pin, `inputs_sha256`, normalized calls of the probe, shape of the model requests with the kind of login, call pattern | the item that the scenario names in `tests/recording/scenarios/` |
| `observations.json` | pin, `inputs_sha256`, recording day, plugin discovery, surfaces, egress, calls of the keychain stand-ins, per item the facts and the computed verdict `holds` | all six items |

Rules:

- No third-party texts: system prompts, status texts and messages of Hermes appear only as
  `sha256:<hex>:<length>`; only scenario texts, identifiers and placeholders (`<id:n>`, `<tmp>`,
  `<home>`, `<host>`, `<user>`, `<ws>`, `<partial>`, `<cursor>`) appear in plain text.
- No login values: the fake logins live only in memory and in the throwaway home.
- Compared are the call patterns (`pattern`) and, in `observations.json`, plugin discovery,
  surfaces, egress and items. The number of repetitions of `send_draft` and of intermediate edits
  of a streamed message depends on the timing and does not count. Hermes calls
  `set_status_text` from the agent thread; its position among the other calls depends on thread
  scheduling, so the pattern holds only its last call, as the last entry.
- So that the pattern does not depend on the timing, every text answer of the scenarios is paced
  (a pause after each chunk), and scenarios with tool calls outside `progress_hooks` switch off
  the tool progress messages of the probe (`timing_problems` in
  `tests/hermes_harness/scenarios.py`).
- If the pin, the probe, a scenario or a harness module changes, the offline tests fail until the
  recording is made again (`inputs_sha256`).

Record again: `make -C plugins/hermes-adapter record`, review the diff, update 09 17.10.
