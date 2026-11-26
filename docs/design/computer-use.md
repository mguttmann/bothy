# Computer use: an agent operates a desktop of the owner

Status: design of 2026-11-26, revised the same day after the owner ruling OR-1 (section 0). Ticket
CU-01 turned it into the normative [chapter 27](../spec/27-computer-use.md), 07 2.15 and SR13, the
tables of 03, the threat model sections 5.18 and 6.10, [ADR 0078](../adr/0078-agent-computer-use-on-a-paired-desktop.md),
the rulings [decisions.d/CU-01.md](../decisions.d/CU-01.md), [PLAN](../PLAN.md) section 23 and the
briefs CU-02 to CU-07. Nothing of it is built yet. This note keeps the goal, the evidence, the
comparison of the approaches and the facts of the first draft that the source contradicted; where
it differs from chapter 27, the chapter applies.

## 0. Owner ruling

OR-1 (owner, 2026-11-26, verified owner word; wording in D-CU-01 of the rulings): the agent shall
control everything, the owner wants to tell it from the phone to do something on the Mac or on
the Windows computer without restrictions, it shall operate the terminal, the browser, any app and
the Mac settings: it shall operate the computer completely.

From this follow:

- **Full operation.** No app tiers, no view-only browser, no click-only terminal, no blocked app
  category, no block list of key combinations, no confirmation per consequential action. System
  settings are allowed; the Bothy windows are not locked (E6).
- **Start from afar.** The owner tells an agent from any client to work on a named desktop. There
  is no confirmation per session at the target; a one-time enablement per device that the owner
  gives at that machine and can switch off at any time takes its place (E9).
- **Only protection that removes nothing** stays: owner only, a visible indicator, stop from every
  client and with Esc at the machine, audit of start, end and app identity without screen content,
  no screenshot on the server, screenshots removed from the runner at the end of the run, the
  session bound to the run. End-to-end encryption and a server without insight stay.
- **Platforms:** Windows joins macOS in the first stage.
- **No paid penetration test** for this stage; a red team and attack tests are mandatory (E10).

## 1. Goal and done criterion

An agent in a Bothy thread sees the screen of an enabled desktop of the owner and operates its
mouse and keyboard without restriction: terminal, browser, every app and the system settings,
similar to computer use in Claude Cowork, without its tiers and questions. The owner orders it from
any client. macOS and Windows are controlled; mobile devices are not. It need not be real time, but
it should be brisk.

Done criterion of the first stage (macOS and Windows, owner only):

1. An agent of the Claude Code runner operates any app on the owner's Mac and Windows computer,
   among them a terminal, a browser and the system settings: screenshot, click, type, check.
2. "Do X on <desktop>" from any client opens the session on the named enabled desktop without a
   confirmation at the target. A run not started by the owner, a run without `origin = 'human'`, a
   desktop not enabled and another operating system account are refused.
3. Esc at the real keyboard, a stop in any client or at the indicator, switching the enablement off
   or any other revocation, `done` and the end of the run end the session at once. A lock pauses it
   and the agent reports it. No key stays pressed. Each case has a test against the fake engine.
4. Server, audit, telemetry, logs and event log hold no screen content (a sentinel test).
   Screenshots leave the runner host's disk at the end of the run (a test against the pinned CLI's
   layout).
5. The whole path runs in CI on Linux with Xvfb and XTest, without GUI tests on the owner's
   machines: runner, MCP server, relay, Noise, host session, coordinates.
6. ADR 0078 accepted; the `never` rows, SPEC sections 4 and 5, the threat model and the privacy
   chapter changed in the same change (CU-01).

## 2. Model: what Claude Cowork does

Sources, retrieved 2026-11-26:

- [S1] https://support.claude.com/en/articles/14128542-let-claude-use-your-computer-in-cowork
- [S2] https://platform.claude.com/docs/en/agents-and-tools/tool-use/computer-use-tool
- [S3] https://code.claude.com/docs/en/computer-use

Bothy takes over the loop, the tools and the stop with Esc, not the approval per app, the fixed
tiers, the block list or the questions before sending (OR-1).

- No sandbox: Claude works on the real desktop [S1, S3]. It loops: request a screenshot, choose an
  action, the client executes it, a new screenshot returns [S2].
- Screen control is the last resort after connectors, MCP, shell and browser; [S3] calls it "the
  broadest and slowest" option.
- Anthropic approves each app on first use and warns for terminals, IDEs, Finder and System
  Settings [S1, S3]; it gives browsers and trading apps view-only access, terminals and code editors
  click-only access, and blocks media apps [S1, S3]. Bothy takes none of this.
- Esc stops and is swallowed, so injected text cannot use it to dismiss dialogs [S3]; a lock file
  allows one controlling session per machine [S3]. Bothy takes both.
- Classifiers check screenshots for prompt injection; in the API they are on by default [S2].
  Whether they run on images an MCP tool returns is unverified; Bothy's defense does not rely on
  them.
- API [S2]: the tool set with batch actions that stop at the first failure ("Not executed: an
  earlier computer action in this turn failed."), coordinates in the pixel space of the returned
  screenshot, recommended sizes from 1024x768 to 1280x800 and at most 1920x1080, roughly 1,000 to
  1,800 input tokens per screenshot. The source warns that computer use may be too slow for human
  interaction; no source names seconds per action.
- Platforms: Desktop on macOS 14 or later and Windows [S1]; the CLI only on macOS, only
  interactive, not with `-p` [S3]. Whether background sessions of the CLI can use it is unverified
  and unlikely.

## 3. Decisions

### E1: "Tool first" with the run binding of "host first"

Rating of the two approaches of the first draft (1 weak, 5 strong):

| Criterion | A: host first | B: tool first | Reason |
|---|---|---|---|
| Fit with repository mechanisms | 3 | 4 | A moves the Noise end into a server broker, so the server would see screenshots (breaks R-TM-02 and SEC-TERM-1); the host service runs as a service user outside the desktop session and is built without cgo, so it cannot capture there. B keeps the server blind and uses the `stdio` rule (R-BA-05: `host`, then SR2). |
| Safety and consent | 4 | 4 | Both have Esc, indicator and revocation; A binds a session to exactly one run, B checks the lock state right before every action. |
| Latency | 4 | 4 | Both a few seconds per step, dominated by the model; B splits images into Noise messages of at most 65535 bytes, which A overlooked. |
| Size of the first stage | 4 | 3 | A still needs an app wrapper and signing; B needs a signed helper per platform and a `stdio` MCP server. |
| macOS and Windows | 2 | 4 | A depends on the owner account mode, which does not exist on Windows; B puts a helper into the interactive session and leaves the service as it is. |

Ruling: B, with A's binding of a session to exactly one run, the automatic image after every
changing action, and no holding of keys or buttons in the first stage.

### E2: Own MCP tools, not Claude Code's built-in computer use

The built-in tool acts on the runner's machine, needs an interactive session and does not run with
`-p` [S3]. Bothy ships `bothy-computer` with the names and parameters of the API tool set.

### E3: The owner's order in the thread is the consent

The agent opens the session itself, only in a run the owner started with an own message
(`origin = 'human'`) and only on a desktop of the owner with an active enablement. No card, no
lease, no confirmation at the target, no step-up per session. R-AZ-03 is untouched because
`computer.open` is no step-up action. T-TM-09 ("agents never reach a host") is reworded in ADR 0078:
an agent reaches a desktop only in the owner's run and only on a machine enabled locally.

### E4: Specification first

CU-01 before any code: OR-1 as a ruling, ADR 0078, the `never` rows of 01, SPEC sections 4 to 6,
the threat model. SPEC section 6 requires a new ADR for a weakened invariant, and the threat model
treats every new flow across a boundary without an entry as a merge blocker (section 9 there).

### E5: A helper per platform

The helper is the only process with screen recording and accessibility (macOS) or capture and
`SendInput` (Windows), and carries the switch "Allow computer control". Reasons, corrected against
the source: `bothy-host` runs as a service user outside the desktop session (R-TM-12) and is built
without cgo; the desktop app is sandboxed (`Release.entitlements`), and a sandboxed app cannot
drive other apps through accessibility. The first draft said `bothy-host` is unsigned; it is
signed and notarized by `release.yml`.

### E6: Full operation, the host enforces only protection without restriction

Any app, any key and combination, any field, the Bothy windows included. The host enforces only
the indicator, stop, pause while locked, one controller per machine and the run binding; no tool
switches the indicator, the Esc stop or the enablement off, and switching the enablement off through
the user interface only ends the agent's own session. A lock on the Bothy windows would only keep
the agent from stopping itself, which is the safe direction. Only Esc at the real keyboard stops,
never one the helper sends.

### E7: Foreground only, stop is final, a lock pauses

The agent takes the real pointer; activity at the machine does not hold actions back (Esc always
works). A stop ends the session; the agent may open a new one in the same run unless the stop
switched the enablement off. A background mode without the pointer is described by [S1] for macOS
15 only and is unverified for us (CU-11).

### E8: Limits only as named keys

No limit for actions, rate or duration; a session lives at most as long as its run; costs run
through the run's cost control.

### E9: One-time enablement per device

At the machine, in the account to be controlled: macOS screen recording and accessibility plus the
switch; Windows the signed helper with `uiAccess` (installed under `Program Files`) plus the
switch. The host reports the state; only then is the device listed. Switching off works locally,
from every client and through every revocation of 24 8.3; switching on only locally.

### E10: No paid penetration test

A red team and attack tests on T-CU-01 to T-CU-08 are mandatory before the stage merges
(D-CU-01-9).

### E11: A lock pauses, the agent reports it

Technical state (unverified): on macOS `loginwindow` lies above every app while locked and
synthetic input does not reach the user's apps; on Windows the input desktop switches to the secure
desktop, where `SendInput` from the user session has no effect. Ruling: lock, screen saver, user
switch and display sleep pause the session; the next tool call answers `screen_locked` and the
agent tells the thread; an unlock resumes. The helper keeps the display awake while a session is
open; whether that prevents the automatic lock is a probe of CU-02. The agent never unlocks (F13).

## 4. Facts of the first draft that the source contradicted

Checked on 2026-11-26 against `main` `ed7017063c` (D-CU-01-6 and D-CU-01-13 to D-CU-01-24):

| Draft said | Source says | Consequence |
|---|---|---|
| `bothy-host` is unsigned | `release.yml` signs and notarizes it (`build-apple`) and signs its MSI (`build-windows`) | E5 rests on the service user and the missing cgo, not on the signature |
| a section "Computer sessions" in 24 | new chapter 27 | rule and test IDs of their own (D-CU-01-13) |
| server module `computer` | module `terminal` | it owns hosts, control connection, relay, keys and revocation (D-CU-01-15) |
| threat model section 6.8 | 6.8 is visualization, 6.9 built-in agents | new 6.10 and 5.18 (D-CU-01-16) |
| R-AG-44 "no tool without an open run" | R-AG-44 is about background work of Hermes runs | the run binding is SR9 with `run_inactive` (D-CU-01-17) |
| R-BA-05 to R-BA-07 make `stdio` `host` | R-BA-05 and R-BA-06 do; R-BA-07 is about command-line flags | references corrected |
| the host op `computer.open` like a terminal op `terminal.open` | the terminal's host op is `session.open`; `terminal.open` is the action | new host ops `computer.*` (27 12.3) |
| the local channel with the peer rule `Rule.Allow` and a Team ID check | `localchan` and the confirmation endpoint check only user and process IDs; no signer check exists | the peer rule of 27 9.4 is built in CU-03 (D-CU-01-24) |
| pinning with the fingerprint from `Pair`/`Confirm` | devices pin on first use at their first opening (24 5.5) | per-runner pinning with the generation rule (D-CU-01-18) |
| `runner.mcp_stdio_runners` switches the tool on | it is the allow list of launcher commands (`npx`, `uvx`) for `stdio` servers of a definition | own key `runner.computer_use` (D-CU-01-19) |
| transcript cleanup of the runner | none exists; `claude rm` keeps the transcript | the end of the Claude Code session after a computer run (D-CU-01-20) |
| `host-platform.yml` has no visibility gate | both it and `e2e-desktop.yml` run only on a public repository, which it is | probe job first (D-CU-01-21) |
| contracts (`actions.yaml`, OpenAPI, AsyncAPI) in the specification ticket | the enable list and code generation accept them only with their handlers | they enter with CU-04 (D-CU-01-14) |

Confirmed: `services/host/internal/noise` (Noise `NK`, at most 65535 bytes per message),
`frame.go` with `allowedFrom` and frames of at most 65519 bytes, `wsrelay/relay.go` with `NewRelay`,
`Expect` and `armLimit`, `sessions_open.go`, `token.go`, `revocation.go` in the module `terminal`,
`session/manager.go`, `gate.go`, `verify.go`, `tool_class.py` of the Hermes adapter (an MCP toolset
counts as `host`), the sandboxed desktop app, `apps/client/lib/desktop/tray.dart`, the doctor checks
HD1 to HD11, `host_service.require_local_confirm`, the end path of M2-46.

## 5. Latency

Estimates, unverified; no source names seconds per action. A model turn of a few seconds
dominates; capture, scaling and encoding, the image chunks through the relay and the settle wait
add well under a second each; a step thus takes a few seconds, and a `batch` of several actions costs
one model turn. Levers: `batch` in the tool descriptions, the automatic image, JPEG for screenshots
and PNG for `zoom`, an image edge around 1280 pixels and never above 1920x1080 [S2], a Noise channel
that stays open for the whole session. Old images are not pruned, because [S2] says that breaks
later thinking blocks of current models; the CLI's compaction handles growth. CU-06 measures host and
relay; numbers are promised only after a measurement.

## 6. Risks

- **Full control means full effect of a hijacked agent.** Whoever takes the agent over through
  screen content, web pages, mail or files can do on the machine what the owner can. There is no
  barrier by design (OR-1); data marking, the system prompt, the run binding, the indicator, stop
  and audit make it visible and stoppable (T-CU-01, T-CU-02).
- **A stolen owner session starts work from afar** without a step-up (T-CU-08).
- **Classifiers** probably do not run on MCP images (unverified).
- **Owner account mode (T-TM-16):** on a machine whose host service runs in the owner's account,
  the agent reaches the host service itself through a terminal window.
- **Absence:** a display kept awake shows everyone in the room what the agent does.
- **Esc under Secure Input** is probably blind; the stop button and every client remain.
- **Signing:** a changed signature or designated requirement silently voids the macOS grants; the
  doctor and a release test watch it. The helper's signing paths are new on both platforms.
- **Transcript removal** depends on the CLI's file layout; pinned version and a test.
- **CI gap:** hosted runners may not prove native capture and input; then only the Linux fixture
  covers the path.
- **Costs:** every screenshot costs input tokens; no budget per session (E8).
- **Stuck keys** after a crash, a pause or a lost connection: released on every path, tested.
- **Codetermination and GDPR:** low while only the owner controls the owner's own machines; new
  assessment for CU-12.

## 7. Questions settled

F1 (lift the `never` rows): yes, for the owner, by OR-1. F3 (screenshots in the thread): no in this
stage. F13 (unlock): no. F14 (Windows system service for UAC): not in this stage; the agent reports
the prompt and the owner answers it. Settled by OR-1 and therefore gone: who may control (F2),
pause and resume (F4), "always allow" per app (F5), view-only browser (F6), work during absence (F7,
yes), an instance switch (F8, not needed while only the owner controls), runner transcripts (F9,
removed at the run's end), lease and action budget (F10, none), external penetration test (F11, no),
app identity in the audit (F12, yes).
