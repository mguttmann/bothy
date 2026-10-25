# Package `test/guide`

Status: built (M2-37). Component `testsupport` (M1-05), never imported by production code.

The check run of the developer guide [Connect your own agent](../../../../docs/guides/connect-agent.md):
`TestGuide` reads the page, starts a fresh server and executes every block in order, so the guide
cannot drift away from the API, the SDK and the server unnoticed. It is exit criterion 7 of
milestone M2.

## What it proves

- The ten step sections carry exactly the bold titles of chapter 11 section 13, in order, framed
  by the sections Placeholders, Prerequisites, "How the repository checks this guide" and Next.
- Every request block names an operation of `docs/api/openapi.yaml` with its method and path, runs
  as the owner (after a fresh step-up) or as Alex or Sam, and gets the answer the text shows.
- Every command block exits 0; every output block equals the stdout of the command or the next
  lines of the example adapter.
- Every file of `sdk/python/examples/guide/` is shown once on the page, byte for byte, and the
  pin names `sdk/python/VERSION`.
- At the end: every adapter process exited 0 with all its lines matched; no output of a command
  or the adapter holds the agent token, a compact JWS or a message text the page sent; the run
  accepted before the restart of step 10 has exactly one `run.state_changed` to `running` and its
  thread exactly one run.

The first block that differs fails the test with its step, its title, its line and, for JSON, the
JSON Pointer of the difference; nothing after it runs. `TestGuide_DeviationFailsAtStep1` proves
that against the real server, `TestMutations_Static` with mutated copies of the real page.

## Prerequisites and running

- A Docker-compatible runtime (PostgreSQL through `pgtest`) and [uv](https://docs.astral.sh/uv/)
  on `PATH`; without either the test fails, it never skips.
- `cd services/server && go test ./test/guide/ -run TestGuide -v`. The static tests start no
  server and need no `uv`, but `TestMain` prepares the `pgtest` templates and so needs Docker.

The server is the shared harness (`test/harness`): the real wire graph with role `all`, a fresh
database, listeners on `127.0.0.1`, with `ServerConfig`: `ratelimit.*` raised, the scripted
clamd of the attachment tests for the file of step 9, and `command.autocomplete_timeout` above
`testkit.Patience` (with `http.request_timeout` above it), so no step is bounded by the speed of
the machine. The adapter folder
`{adapter_dir}` is a `t.TempDir()` with mode `0700`; every command runs in the repository root
with the test's environment minus every `BOTHY_` variable and with `PYTHONDONTWRITEBYTECODE=1`,
each in its own process group, and the test kills any group still running at its end.

## Block grammar

Every fenced block starts at column 0 with three backticks, a language and `title="..."`, and
stands in Prerequisites or a step section. The kinds:

| Language | Title | Kind |
|---|---|---|
| `http` | `<Actor>: <operationId>` (`Owner`, `Alex`, `Sam`) | Request: `<METHOD> /api/v1/<path> HTTP/1.1`, header lines (`Content-Type: application/json`, `If-Match`), a blank line and an optional JSON body. |
| `json` | `Response <status>` | The expected answer of the request directly before it: the status, and the body as a subset. A request without one must answer `2xx`. |
| `bash` | `Shell: <text>` | Run with `bash -euo pipefail -c`; must exit 0. |
| `bash` | `Adapter: start` | One command line, started in the background. A second start waits for the previous adapter, which must have been stopped or started with `--stop-after accept`. |
| `bash` | `Adapter: stop` | Body exactly `# Ctrl+C in the adapter's terminal`: `SIGINT` to the adapter's process group, exit code 0. |
| `text` | `Output` | After a `Shell:` block its complete stdout, otherwise the next stdout lines of the adapter. |
| `python` | `sdk/python/examples/guide/<file>` | The file, byte for byte (plus a final line break). |
| `toml` | `pyproject.toml (excerpt)` | `project.dependencies` holds exactly one entry `bothy-sdk==<sdk/python/VERSION>`. The check reads the TOML a pyproject excerpt needs (tables, strings, arrays of strings). |

Placeholders are `{name}` or `{secret:<file>}`. The runner binds the names of the table in
section Placeholders before the first block; every other name is captured once, by a response or
an output block, before its first use, and compared afterwards. In JSON a string that is exactly
`"{name}"` stands for the value with its JSON type; in a bash block a value is single-quoted.
`{secret:<file>}` stands only in a response block: the value is written to that file in the
adapter folder (mode `0600`), never compared, never logged and redacted from every error.

Matching: an expected JSON object matches when each of its members matches; an array element
matches exactly one distinct element of the answer, in any order, and an element that matches
several is ambiguous (add a distinguishing member). An output line must equal the actual line;
an unbound placeholder in it captures a run of non-space characters.

## Reading a failure

The message starts with the place, for example `Step 1, block "Owner: createAgent" (line 120):
Step 1, block "Response 201" (line 132): /billing_mode: want string "organization", got string
"personal"`. For an output line it shows the expected and the actual line, and the adapter's
stderr. Run the failing step by hand against a local instance, or fix the text, the example or
the server, whichever is wrong.
