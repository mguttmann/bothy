---
status: accepted
date: 2026-10-10
decision-makers: Planning lead, Team agent platform
---

# Run as its own object for the live box

## Context and Problem Statement

While an agent works, the person should see a live box: state, duration, context in percent,
costs, current tool, last text, plan, subagents, workflows, background jobs, API retries,
stalls. At the end it collapses to one line. How is this state modeled, stored and distributed
to devices without overloading the event log and the devices with a high rate of change?

## Decision Drivers

- High rate of change during a run, but few state transitions.
- Devices after a pause need the correct state, not every intermediate step.
- States control permissions, costs, statistics and "Running now"; they must be durable and
  ordered.
- No double start, no lost cancellation message.
- No flood of message revisions.

## Considered Options

1. Run as its own object (`runs`, `run_steps`, `run_followups`) with a state machine;
   state transitions as durable events, content as ephemeral JSON patches to focused
   threads; an anchor message `kind = run` in the history.
2. Live box as a continuously edited message (like the progress bubble in Discord).
3. Streaming as message drafts (`message.draft` events) without its own object.
4. Every step as a durable event (event sourcing of the run in the event log).

## Decision Outcome

Chosen: option 1, because it separates durable states and transient content (R-SY-05), allows
patches via stable paths (R-SY-06) and makes the run the carrier of costs, backend policy,
session and token.

### Consequences

- Good, because the event log carries only state transitions; patches go only to devices that
  show the thread.
- Good, because a device after a pause loads the whole document via REST and then applies
  patches.
- Good, because state transitions as compare-and-swap rule out double starts and late patches.
- Bad, because live box content that was distributed only ephemerally can be lost on a server
  restart between two save operations; the next patch or fetch heals that.
- Bad, because adapters need patch discipline (throttling, `base_version`); the SDK provides
  it (ADR 0026).

### Confirmation

- Tests of the state machine: every transition that is not allowed is rejected (docs/spec/03-data-model.md
  `runs`; docs/spec/09-agents.md section 5.1).
- Test AG-12: a patch on a forbidden path results in `400 validation_failed` with `errors[].code`
  `patch_path_forbidden` (ADR 0015; corrected by M2-10, D-M2-10-15), an old `base_version`
  results in `409` with the document.
- Load test: the number of durable events per run stays independent of the patch rate.
- Test that live box anchors create no revisions (03 `message_revisions`).
- M2-05 names the tests of the state machine and the run object: `TestCanTransition_AllPairs`,
  `TestReasonTable`, `TestNormalizeAdapterReason` and `TestTriggerStateMapping`
  (`services/server/internal/runs/domain`); `TestTransition_EveryDisallowedPairRejected`,
  `TestTransition_RaceOfTwoStartsOneWins`, `TestTransition_TriggerStatePerEndState`,
  `TestTransition_UnknownAdapterReasonBecomesAdapterError`, `TestOpen_AtMostOneActivePerThread`
  and `TestAcceptance_ThreadMessageOpensRun` (`services/server/test/integration/runs`).

## Pros and Cons of the Options

### Option 1: Own object with patches

- Good, because state, content and costs are one object with one version.
- Good, because the client draws only the latest state.
- Neutral, because a separate document schema (`run_document.v1`) is maintained.
- Bad, because two paths (durable and ephemeral) must be understood.

### Option 2: Edited message

- Good, because no new concept is needed and platforms like Discord solve it this way.
- Bad, because every edit would be a revision and a durable event.
- Bad, because state, costs and steps would only be text and could not be evaluated.

### Option 3: Message drafts

- Good, because text streaming is simple.
- Bad, because steps, subagents, plan and metrics have no place.
- Bad, because the state of the run (question open, canceled) is not modeled.

### Option 4: Every step durable

- Good, because the history of a run could be caught up without gaps.
- Bad, because the patch rate burdens the event log and all devices, including devices that do
  not show the thread at all.
- Bad, because the retention of the log would then be determined by agent traffic.

## More Information

- docs/spec/04-sync-protocol.md section 11; docs/spec/09-agents.md sections 5 and 6.
- `runs.plan`, `metrics.last_api_retry` and `run.start` for the transition out of `waiting_input`
  are in docs/spec/03-data-model.md (`runs`) and docs/spec/04-sync-protocol.md section 11.
