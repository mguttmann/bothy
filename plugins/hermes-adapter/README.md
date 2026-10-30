# hermes-adapter

Status: built (M2-33) on the recording of M2-32; files, images, voice and video from Hermes and
voice input to Hermes since HERMES-MEDIA; Hermes commands and the reports of Hermes cron jobs
since HERMES-CRON-COMMANDS (09 17.3, 17.9).

## Purpose

Reference adapter for Hermes Agent (https://github.com/NousResearch/hermes-agent), an open-source
agent framework. The adapter is a Hermes platform plugin (`kind: platform`) that connects one
Hermes profile to the server as one agent principal: invocations come through the gateway of the
Python SDK (`sdk/python`), answers, live box, cards and session reports go through the public API.

Hermes is one adapter among several. The server does not depend on Hermes, and Hermes is not a
dependency of this component: it runs only in the Fake-Hermes tests and in the operator's
installation.

## Contract

- General: docs/spec/09-agents.md sections 4 to 16.
- Hermes-specific: docs/spec/09-agents.md section 17 (package, mapping of the
  `BasePlatformAdapter` methods, session per thread, `busy_input_mode`, flow of an invocation,
  access layers, reported mode, tool class).
- Backend policy: docs/spec/10-backend-policy.md section 10.1.
- Rulings: docs/decisions.d/M2-33.md (`D-M2-33-*`), docs/decisions.d/HERMES-COMPAT.md and
  docs/decisions.d/HERMES-CRON-COMMANDS.md.

## Layout

| Path | Content |
|---|---|
| `src/bothy_hermes_adapter/entry.py` | the plugin entry (`make_register`); imports neither Hermes nor the SDK, so loading the plugin is fast and never races the gateway's own imports |
| `src/bothy_hermes_adapter/platform.py` | the platform plugin (`ServerPlatform`); one of the two modules that import Hermes, imported when Hermes builds the adapter |
| `src/bothy_hermes_adapter/hermes_config.py` | Hermes' own YAML reader and writer, the credential resolution, the enabled toolsets; the other module that imports Hermes |
| `src/bothy_hermes_adapter/runtime.py` | the protocol between the logic and Hermes (`Runtime`, `RuntimeEvents`) |
| `src/bothy_hermes_adapter/handler.py` | the runtime side of the SDK loop: start, turn, end, follow-ups, interactions |
| `src/bothy_hermes_adapter/actual_mode.py`, `tool_class.py` | the actual billing mode (17.8) and the tool class (17.11) |
| `src/bothy_hermes_adapter/media.py` | files of the answer (check, upload session, message kind) and the audio attachments to transcribe (17.3, 17.6) |
| `src/bothy_hermes_adapter/context_format.py` | the run context as the fixed system part (17.6) |
| `src/bothy_hermes_adapter/hermes_commands.py` | the command table of the pinned version, the control and skill commands the adapter declares (17.3) |
| `src/bothy_hermes_adapter/reports.py` | reports without a run: targets, a new forum post or the main thread, the standalone sender (17.9) |
| `src/bothy_hermes_adapter/live_box.py`, `cards.py`, `store.py`, `heartbeat.py` | live box, cards, resolved callbacks and open cards, heartbeat |
| `src/bothy_hermes_adapter/config.py`, `defaults.py` | the configuration from `PlatformConfig.extra` and its defaults |
| `src/bothy_hermes_adapter/installer.py`, `__main__.py` | the installer |
| `typings/` | type stubs of exactly the Hermes symbols the two Hermes modules use |
| `tests/unit/` | tests without Hermes: a fake runtime on the SDK's FakeServer, and the harness tests of M2-32 |
| `tests/fake_hermes/` | the pinned Hermes in a subprocess with the adapter, the fake model and the FakeServer |
| `tests/hermes_harness/`, `tests/recording/`, `tests/fixtures/recording/` | the recording of M2-32 |

## Installation into a Hermes profile

The adapter runs on Python 3.11 and newer, in the Python of the profile's gateway; it is tested
against Hermes Agent `v0.22.5` (the pin, on Python 3.14, the only Python its lock installs).

Neither this component nor the SDK is packaged before the release chain (M7). Hermes replaces
the Python environment of a profile on many of its commands (a new environment, the old one is
collected), so nothing of the adapter goes into that environment: no `.pth` file, no `attrs`.
Run the installer with the Python of the profile's gateway and put what it needs on
`PYTHONPATH` for that one command: the two source trees of a checkout and a directory with
`attrs` made outside every Hermes environment (the SDK's other runtime dependencies are part of
the Hermes environment at versions the SDK accepts):

```sh
python -m pip install --target /tmp/bothy-attrs 'attrs>=26.1.0'   # any Python, any directory
export PYTHONPATH=<checkout>/sdk/python/src:<checkout>/plugins/hermes-adapter/src:/tmp/bothy-attrs
```

Then, with that Python:

```sh
python -m bothy_hermes_adapter install \
  --profile-home <hermes home of the profile> \
  --server-url https://example.com/api/v1 \
  --instance-id <instance ID> \
  --token-file <file with the agent's API token, mode 0600> \
  --busy-input-mode steer          # or queue; interrupt is refused
```

`--dpop-key-file <file>` (with `--create-dpop-key` to create one) binds the token to a DPoP key.
`--report-channel <channel ID>` (optionally with `--report-thread <thread ID>`) makes that channel
the platform's home channel, the target of `deliver=<platform key>` cron jobs and of `hermes send
--to <platform key>`.
`install` places the plugin in `<home>/plugins/<platform key>` as a self-contained directory,
the place Hermes itself discovers directory plugins: `packages.json` names the directory of each
package the plugin needs (the source trees by their path, so the checkout must stay where it
is), and `attrs` is copied into `vendor/` (as is a package that lay inside Hermes' environment),
so the attrs directory may go after the install. At every start the plugin entry serves exactly
those packages, the adapter's own before the environment and `attrs` only when the environment
lacks it; a missing package stops the plugin with an error in the gateway log that names the
package and says to run `install` again. A `.pth` file of an earlier install may stay; it no
longer matters. Run `install` again after updating the checkout's `attrs` pin or moving the
checkout. `install` also writes the platform section,
`display.busy_input_mode`, `tool_progress: off`, the own access policy and the home channel (the
report channel, or a placeholder that refuses every report), and refuses to change an existing
platform key: every thread would lose its Hermes session. Restart the gateway of the profile
afterwards.

## Checking an install

`python -m bothy_hermes_adapter check --profile-home <home>`, with the same Python and
`PYTHONPATH` as `install`, is the doctor of the install: besides the billing state it prints
`adapter: loadable`, or `adapter: not loadable: <reasons>` and exits with `1`. The reasons: the
plugin is not listed in `plugins.enabled`, a file of the plugin entry or a package it names is
missing, or a dependency Hermes ships (`cryptography`, `httpx`, `jwt`, `websockets`) is gone from
its environment. The output names no path or other value.

## Commands

At every connect the adapter declares its commands and the SDK reconciles them with the server
(09 10.2). `/stop`, `/new`, `/resume`, `/title` and `/status` are the server's built-in commands
and are never registered; the adapter carries out the stop and the session changes they cause
through its own paths (09 8, 9). The
control commands `/context` and `/version` run in the thread's Hermes session from the adapter's
control sender; Hermes' reply becomes a message of the agent in the thread, without a run, also
during a run. Every other Hermes command is refused (the reasons are in
`hermes_commands.COMMAND_CLASSES`). Each skill Hermes offers for the platform is a model command
with an optional `input`: the server opens a run, and the turn is Hermes' own skill invocation. A
skill installed later is declared after the next gateway start.

## Reports without a run

Cron jobs and `hermes send` deliver to the platform (09 14, 17.9): `<platform key>` (the home
channel), `<platform key>:<channel ID>` or `<platform key>:<channel ID>:<thread ID>`. A forum gets
a new post per report, titled by the report's first line; a text channel gets the report in its
main thread; a thread target gets it in that thread. The server checks every report with the
agent's own rights (`write` on the channel); a refusal reaches Hermes as a failure with the problem
code. In the gateway process the report goes out with the adapter's client, in a process without
the gateway with a client of its own from the platform section (`User-Agent`
`hermes-adapter-standalone`). Reports with attachments are refused (a report carries text only;
files go with a turn's answer).

## Configuration keys

In `gateway.platforms.<platform key>.extra`, written by the installer (09 21):
`hermes_adapter.platform_key` (`bothy`), `hermes_adapter.consumer` (`default`),
`hermes_adapter.server_url`, `hermes_adapter.instance_id`, `hermes_adapter.token_file`,
`hermes_adapter.dpop_key_file`, `hermes_adapter.max_message_bytes` (change it together with the
server's `message.max_content_bytes`; a longer answer goes out as one message with its rest as
one attachment `answer.txt`, and Hermes never splits an answer, M4-47), `hermes_adapter.attachment_text_max_bytes`, and the SDK keys
`adapter.invocation_token_leeway`, `adapter.jwks_refresh_min_interval`, `sdk.http_timeout`,
`sdk.retry_max_attempts`, `sdk.retry_backoff_base`, `sdk.retry_backoff_cap`. The adapter never
reads the process environment.

## Billing mode

`python -m bothy_hermes_adapter switch-mode organization|personal --profile-home <home>
[--credential-holder <principal ID>]` changes the profile environment (10 10.1): `organization`
needs `ANTHROPIC_API_KEY`, empties the OAuth variables and Hermes' allow list and refuses a login
file of another tool in the profile's home; `personal` empties the API key and writes Hermes'
allow list with exactly the credential holder. `check` shows the state. Every command prints the
mode Hermes would now resolve and never a value; none restarts a process. Restart the gateway of
the profile yourself, and note that a gateway serving several profiles restarts all of them.

The adapter determines the actual mode from the kind of credential Hermes resolves, before every
start and in every heartbeat; an undeterminable mode fails every run with `billing_mode_mismatch`
and the heartbeat reports `degraded`.

## Files and voice

Files Hermes names in its answer (`MEDIA:` tags, local images, voice replies, videos) reach the
thread as attachments: each one passes Hermes' own media delivery policy again (keep
`HERMES_MEDIA_DELIVERY_STRICT` and `HERMES_MEDIA_ALLOW_DIRS` as for Hermes' other platforms),
must be a regular file within the server's `file_max_upload_bytes`, goes through an upload
session of the thread's channel and then as one message of the run with the file attached; the
server scans it. Audio from `send_voice` is attached to a text message too: the server accepts a
voice message only once its scan has made the file final, which happens after the upload. The
agent needs the right `upload`, and the owner switch `supports_files_out` must stay on; a refused
file sends nothing and Hermes posts its own failure notice. An image by web address is sent as
the address.

Voice messages (and other audio attachments) of the triggering message without a transcript of
the server are transcribed with the profile's own speech to text (`stt` in Hermes'
configuration, the same provider its other platforms use); the transcript is the person's text
and is stored on the server as the agent's transcript. Without a transcript Hermes' own note
reaches the turn. The adapter reports `voice_input` only while speech to text is on and finds a
provider, so in the server's mode `agent` a voice message reaches it only then (09 13). An
external speech to text provider receives the audio; choose it as for Hermes' other platforms.

## Inbox, state and restart

The state directory `<home>/<platform key>` (mode `0700`) holds the SDK's inbox and outbox and the
adapter's file of resolved callbacks, open cards and the Hermes session of each thread; none holds
a content or a key value. After a
restart, runs that were accepted but not started start again (an expired token is renewed);
runs whose turn was running are not resumed (the server ends them `interrupted`). The adapter
connects only after Hermes finished its startup restore.

## What the tests prove and what they do not

- `tests/unit`: the logic against the SDK's FakeServer with a fake runtime: intake, start
  answers, restart, live box, answer, costs, title, follow-ups, cards, stop, sessions, heartbeat,
  the `Invocation-Token` header, logs without content, configuration, the installer's plan, the
  command table, control and skill commands, reports and the standalone sender.
- `tests/fake_hermes`: the pinned Hermes in a subprocess (throwaway home, fake model on
  `127.0.0.1`, the FakeServer, under the macOS sandbox, an empty managed scope): plugin discovery,
  a full turn, AG-08 and AZ-P6 (no turn without a matching token), AG-11 and the unknown mode,
  the reported mode equals the credential the model endpoint receives, AG-13 (steer, queue,
  stop), AG-15 (concurrent clicks resolve once), AG-20 (trust marks reach the model as data
  blocks), BR-13 (no key value in any request, frame, log, output or state file), the installer,
  the stub contract, the toolset table, the command table and the unchanged pin; files of the
  answer (`MEDIA:` tags for a document, an MP3 voice, a video and an image) arrive as attachments
  with the uploaded bytes, the profile's `.env` stays out, and a voice message reaches Hermes'
  speech to text (a command provider of the profile) and its transcript the model and the server,
  or Hermes' note when speech to text is off; a control command idle and during a turn, a skill
  command, and reports through `hermes send`, a cron job in a process without the gateway and one
  the gateway's own scheduler fires. They need the network for the pinned downloads and fail without it; they never skip.
- `tests/unit/test_media.py`: upload in parts, voice kind, size and file type refusals, an
  aborted upload, a refused attachment, the capabilities, transcripts and their submission.
- The recording of M2-32 has no scenario for files and voice: Hermes' calls are proven by the
  Fake-Hermes tests against the same pin (D-HERMES-MEDIA-5).
- Not proven: the newest Hermes release (`pytest --hermes-version=latest` is a manual switch with
  an unpinned download, never part of `make check`; it takes the highest version tag and ranks
  every version tag above the date tags Hermes used before `v0.22`), the adapter against the real
  server, a send of a turn into another channel or thread (Hermes `v0.22` has no agent tool for
  it; the routing is the same as a report's), Hermes releases older than the pin (the adapter
  declares surfaces of `v0.22` that older releases ignore, but no test runs them), other
  credential resolutions than the tested ones, the effect of the markers on a real model.

## The pin and its update

The pinned version is in `tests/hermes_pin.toml`. After a new pin: record again
(`make -C plugins/hermes-adapter record`), review the diff in `tests/fixtures/recording/`,
update 09 17.10, then run the tests: the stub contract, the toolset table and the module list
fail when the new version changed a surface the adapter uses.

## Recording (M2-32)

Before the adapter was built, M2-32 measured how the pinned Hermes version calls the surfaces of
section 17 (09 17.10): no call of `format_tool_event`; tool steps through `set_status_text`,
cumulative drafts through `send_draft`; `steer` only through the global key and `/steer`; no
public surface for the credential type; `/stop` via `handle_message` returns `CANCELLED`; `/new`
with confirmation, `/resume` in the thread, foreign sessions only with `--all` for an admin
sender, the session ID from the hook `pre_llm_call`; `enforces_own_access_policy` with
`dm_policy = "allowlist"` works. Recorded again against `v0.22.5` (09 17.10): the own access
policy needs `authorization_is_upstream` next to it, `/resume --all` does not move a session that
is live in another thread of the same gateway, and the global `queue` steers. The recording is
part of the normal test run and needs the network for the pinned downloads.

The harness guarantees: the checkout of the pinned commit lies read-only in a throwaway
directory outside the repository; the installation is made from a `git archive` export into a
throwaway environment; checkout and installed files are unchanged after the run. Every Hermes
process gets an environment from an allow-list, its own homes, an empty keyring, refusing
programs for the keychain and services, and a proxy that rejects everything; on macOS the process
tree also runs in `sandbox-exec`. The harness opens only ports on `127.0.0.1` with port 0 and ends
only its own process group.
