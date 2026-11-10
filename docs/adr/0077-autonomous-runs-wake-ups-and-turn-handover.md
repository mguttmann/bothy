---
status: accepted
date: 2026-11-06
decision-makers: Owner, planning lead
consulted: Red team of the design note
informed: all teams
---

# Autonomous runs: wake-ups and turn handover

Extends [ADR 0022](0022-agents-and-plugins-outside-the-server-process.md) (agents outside the
server process), [ADR 0023](0023-signed-invocation-token-and-check-in-the-adapter.md) (signed
invocation token, check in the adapter) and [ADR 0025](0025-run-as-its-own-object-for-the-live-box.md)
(run as its own object) to sessions that keep working without a new message. Normative:
[09-agents.md](../spec/09-agents.md) 5.8 and 18.19, [07-permissions.md](../spec/07-permissions.md)
2.14, 03 `run_wakes` and `run_autonomy`; design note:
[autonomous-runs.md](../design/autonomous-runs.md); rulings:
[decisions.d/AR-01.md](../decisions.d/AR-01.md).

## Context and Problem Statement

The owner asked on 2026-11-06 that every session, chat or development, keeps working on its own as
long as it has work, so that a steering session can in principle work on indefinitely. Three
failures stood in the way, each read from the code and the pinned CLI and the second reproduced:
an agent that promises to continue after an outside event ends its turn and nothing wakes it; a
runner restart cancels its run tasks, leaves the CLI processes orphaned with their stdin closed,
and the CLI stops all background work at its ceiling; and limits such as the usage window of the
account end work silently. How does a session ask to be woken, where does that request live, how
does a turn survive a restart of the runner, and how is autonomous work kept from spending money
without limit?

## Decision Drivers

- No silent end: every end of autonomous work is visible to the person whose work it is.
- An update of the runner is routine and must cost no running work.
- The server stays the only place that starts a run, issues its token and applies the trigger and
  billing rules (I1 to I5, SR2, SR4).
- Cost safety: who may start autonomous work, a visible cap and an idle cap that cannot be
  switched off.
- Additive and off by default; an installation that sets none of the new keys behaves byte for
  byte as before.

## Considered Options

1. Wake-ups as server records, evaluated by the server (`at`) or by the runner (host conditions)
   and fired with the checks of a new run; a keeper per turn that leads its own session and holds
   the turn's named pipe, so the runner hands live turns over at a restart and the next runner
   adopts them with full rights after replaying their state from the stream.
2. Wake-ups only in the runner's state folder, as the task text first proposed.
3. Refuse every runner restart while turns run.
4. Kill and resume: end turns at a restart and start them again with `--resume`.
5. The CLI's own timers (`CronCreate`, `ScheduleWakeup`) and a CLI background ceiling of 0.
6. A separately installed supervisor service that owns the turns.

## Decision Outcome

Chosen: option 1. A wake-up is a row of `run_wakes` with the person of its source run; the server
job fires `at` wake-ups, the runner evaluates `pr_checks` and `command` wake-ups and asks
`run.wake.check` before every host execution; a fired wake-up starts a wake run, or reaches the
active run of the same person as a wake input with its own token. Autonomous inputs need an owner
or the channel switch `allow_autonomous_runs` and stop visibly at a chain cap and an idle cap
counted per thread and person. With `runner.turn_handover` the runner starts a keeper per launch
with `start_new_session`; the keeper holds the write end of a named pipe, starts the CLI after a
launch handshake that persists every identity first, and survives the service's stop. On SIGTERM
the runner exits 75 and the next runner adopts each turn from its record, replays the per-turn
state (`ProcessState`) from the stream and handles the end of an adopted process like its own.
Lost background work leads to a recovery run that only informs, the usage window of the account
marks a run usage-limited and wakes it at the reset, and every run ends with a `STATE:` marker.

### Consequences

- Good, because a wake-up survives the runner, its state folder and a crash, is visible and
  cancellable in the thread, and passes the same checks as a message.
- Good, because a restart no longer touches a running turn: background work, follow-ups and wake
  inputs go on, and the server never sees the run lost.
- Good, because each limit that can end work is either removed, made irrelevant or a named key
  with a visible end.
- Bad, because the launch path of a turn becomes a handshake over files and a second process, and
  the adoption rule adds recovery for adopted ends; both need crash-point tests for every step.
- Bad, because the promise depends on the service's stop leaving the keeper alone: launchd without
  `AbandonProcessGroup` was probed; systemd needs `KillMode=process`.
- Bad, because an agent can still decide not to ask for a wake-up; the end-state check makes that
  visible, it cannot prevent it.
- Neutral, because `chat` sessions continue only through recovery and usage-window wake-ups and
  replies; they have no tool to ask for a wake-up.

### Confirmation

- Runner scenarios AR-T01 to AR-T16 of 09 18.19.13 with a real runner process and the fake Claude:
  SIGTERM and SIGKILL to the whole process group with a live background job, a crash point after
  every step of the launch handshake for every attempt kind, the replayed decisions equal to the
  old runner's for every recorded stream and checkpoint position, byte-equal fixtures without the
  keys.
- Server tests for every refusal and end state of a wake-up, the caps and the launch check.
- The probe of the keeper under a scratch launchd job becomes a macOS test (AR-09).
- An end-to-end run on the demo stack across a runner restart (AR-10).

## Pros and Cons of the Options

### Server records and a keeper per turn

- Good, because of the drivers above.
- Neutral, because new tables, actions, keys and a heartbeat capability are needed; all are
  additive.

### Wake-ups in the runner's state folder

- Good, because nothing in the server changes.
- Bad, because a wake-up would be invisible, not cancellable, lost with the folder, and the runner
  would still need a server operation to start a run.

### Refuse restarts while turns run

- Good, because it is simple.
- Bad, because a held turn may run for days; such a gate blocks every update for that long, and a
  gate that operators switch off protects nothing. It stays only for the restarts that install the
  handover (ruling D-AR-01-4).

### Kill and resume

- Good, because no second process is needed.
- Bad, because it loses every background task at every update.

### The CLI's own timers and ceiling

- Good, because the CLI already has them.
- Bad, because a session cron is dropped while input is closed and dies with the process, and a
  ceiling of 0 does not stop the CLI from sweeping background shells after a few seconds.

### A supervisor service

- Good, because turns would live outside the runner.
- Bad, because it is a second program to install and update, which ADR 0076 already rejected for
  built-in agents.

## More Information

- The CLI facts behind the decision are read from the pinned CLI (2.1.288 when the decision was
  taken) and recorded in 09 18.19.12; they are checked again when the pin moves. Read again in
  2.1.321 on 2026-11-10: a background shell now keeps the CLI alive after the end of input
  (whether a pending loop wake-up alone does is not established), which changes nothing in the
  decision (D-FIX-RUNNER-CLI-PIN-321-5).
- Revisit when the CLI offers a durable wake-up of its own, when the runner runs under a service
  manager that kills whole sessions, or when `chat` sessions should ask for wake-ups.
