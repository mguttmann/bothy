---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Team F (operations), Team E (apps)
informed: Team C (security)
---

# Feature flag registration with a cleanup gate

## Context and Problem Statement

New features are to be enabled step by step (for individual people or groups), and
the owner needs permanent product switches such as `dm.enabled` or `voice.enabled`. Feature flags
do both, but experience shows that they leave dead switches behind and tempt people to make limits
or security rules switchable. Where do flags live, who evaluates them, and what enforces their
removal?

## Decision Drivers

- No dead flags: a rollout flag has an end, and the gate enforces it.
- Flags switch features, never security boundaries, limits or secrets
  ([16-operations.md](../spec/16-operations.md), section 6.2).
- Evaluation only in the server; clients get evaluated values (no permission logic in the client,
  [15-client-architecture.md](../spec/15-client-architecture.md), section 18).
- Changing a flag is an administrative action with step-up and audit.
- No additional service in operations.

## Considered Options

- Registration of every flag in code (key, kind `release` or `option`, description, default,
  `remove_by` for `release`), values in the table `feature_flags`, gate check against `VERSION`
- External flag service through a standard interface (OpenFeature with a provider such as flagd or
  Unleash)
- Flags only in the database, without registration in code
- Flags as configuration values (environment or file), without rollout per person
- No flags; features only with the finished release

## Decision Outcome

Chosen option: "Registration in code, values in `feature_flags`, gate check", because the
registration describes every flag with kind and end and the gate enforces the end. Normative in
[16-operations.md](../spec/16-operations.md), section 6.2 (R-OP-14); guide in
[environments-and-flags.md](../operations/environments-and-flags.md), section 2.

1. **Registration.** Every flag is defined in one place in the server with key, kind, description
   (English, for the console), default, responsible module and, for `release`, the version
   `remove_by`. `option` flags have no `remove_by`.
2. **Values.** Table `feature_flags` ([03-data-model.md](../spec/03-data-model.md), section
   4.1): global value and `rollout` with people and groups for whom the value applies inverted.
   If a row is missing, the registered default applies. Keys without registration are off and
   appear in the console as orphaned.
3. **Evaluation** only in the server (`flags.Enabled(principal, key)`), with valid
   group memberships. Clients get the values evaluated for them in `instance.read` and
   the snapshot; `instance.updated` names only changed keys.
4. **Changing** with `feature_flag.update`: owner, step-up, reason, audit
   ([05-actions.md](../spec/05-actions.md), section 4.2).
5. **Cleanup.** A check in the gate fails as soon as the version in the server's `VERSION` reaches
   `remove_by` of a `release` flag that is still registered. Whoever removes the flag removes
   code and registration and deletes the row by migration in the same change. A
   `release` flag never becomes `option`; a permanent switch gets a new key.

`remove_by` is a version, not a date: a date would turn builds of old states red on an arbitrary
day; the version changes only with a release and can be checked in the repository.

### Consequences

- Good, because every rollout flag has an end in the form of a version that the gate enforces.
- Good, because the console can show every flag with kind and description and orphaned rows
  stand out.
- Good, because no external service gets user data for the rollout.
- Bad, because the rollout knows only people and groups, no percentages; for a
  team chat with named people that is enough.
- Bad, because a forgotten flag blocks a release; that is intended, but costs time at the
  wrong moment. Mitigation: set `remove_by` generously and show it during release preparation
  (ADR 0060).
- Neutral, because agents and apps experience flags like people; `rollout` names only people and
  groups.

### Confirmation

- Build ticket M1-47 in [PLAN.md](../PLAN.md): evaluation globally, per person and per group with
  an expired membership; unknown keys are off; changes only by an owner with step-up
  and audit.
- `make flag-check` (tool `tools/flagcheck`, part of `gates`): fails when `VERSION` reaches
  `remove_by` of a registered `release` flag; also fails for `release` without
  `remove_by` and for `option` with `remove_by`.
- Held by the form: a flag is only a switch (`feature_flags.enabled boolean`); a flag cannot carry
  numbers, lists or secrets. Whether a switch touches a security boundary is checked by the review
  of every ticket and by the red team gate
  ([red-team-gate.md](../quality/red-team-gate.md)); no mechanism exists for this.

## Pros and Cons of the Options

### Registration in code with a gate

- Good, because code, registration and cleanup rule come together in one place.
- Bad, because only simple target audiences are possible.

### External flag service

- Good, because OpenFeature is a vendor-neutral interface with a Go SDK and providers
  (CNCF project, maturity level Incubating) [Q1, verified] and brings percentage rollouts.
- Bad, because another service must be operated and secured or user identifiers go to
  a provider; the cleanup rule is still missing.

### Database only

- Good, because flags come into being without a release.
- Bad, because nobody knows which code reads a flag and when it can go; that is exactly how
  dead flags come about.

### Flags as configuration

- Good, because no new mechanism is needed (ADR 0011).
- Bad, because configuration needs a restart and knows no rollout per person.

### No flags

- Good, because there is nothing to manage.
- Bad, because step-by-step enabling and permanent product switches of the owner are missing.

## More Information

- Source (retrieved 2026-10-10): Q1 OpenFeature, https://openfeature.dev/docs/reference/intro
  [verified].
- Related: ADR 0011 (configuration), ADR 0036 (releases), ADR 0060 (release preparation).
