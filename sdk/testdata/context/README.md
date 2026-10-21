# sdk/testdata/context

Shared golden files of the run context rendering (docs/spec/09-agenten.md 4.6, ADR 0054,
docs/DECISIONS.md D-M2-30-1): both SDKs render the same input with the same fixed block
boundary and must produce the same bytes. Python renders with `bothy_sdk.context.render_context`
(`sdk/python/tests/test_context_golden.py`), TypeScript with `renderContext` of `runs`
(`sdk/typescript/test/context.test.ts`).

Each case is one directory:

| File | Content |
|---|---|
| `input.json` | `boundary` (the fixed test boundary, 16 lowercase hexadecimal characters), `context` (a `RunContext` body as `getRunContext` returns it) and optionally `file_texts` (extracted file texts keyed by `file_id`) |
| `system.txt` | the expected system part, followed by one line feed |
| `trigger.txt` | the expected trigger part, followed by one line feed |

The line feed at the end of each text file is not part of the rendered text; it is there because
every text file of the repository ends with one (the `end-of-file-fixer` hook). Outside tests the
boundary is random per call; the fixed one exists only so that the golden files are stable.

| Case | What it holds |
|---|---|
| `minimal` | only the triggering message of Alex |
| `thread_marks` | `truncated_before_seq`; a channel message of Robin (`authorized`); in the thread, deliberately out of order: an own message of Lotse (`self`, `already_delivered`), a message of Sam, a person without `trigger` (`unverified`, with a forged end marker of another boundary and a time with an offset), a message of the other agent Texter (`unverified`) with a voice transcript, an import (`origin: import`, `unverified`), a system message (`system`) and a truncated message of Robin whose display name holds a quote character |
| `trigger_upload_and_voice` | a thread message of Sam (`authorized`) with a file text; the triggering voice message of Alex with an uploaded file whose text is given (marked data, also for the triggering person), a voice note with a transcript (in the trigger part, mark `trigger`) and an image without text; plus a file text of a file that is no attachment of the context (ignored) |

Regenerate the expected files after a deliberate change of the rendering with
`BOTHY_SDK_RECORD_CONTEXT=1` set while running `sdk/python/tests/test_context_golden.py`
(`cd sdk/python && BOTHY_SDK_RECORD_CONTEXT=1 uv run --locked pytest tests/test_context_golden.py`),
then check the difference and run the TypeScript test, which must pass unchanged.
