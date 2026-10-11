---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
---

# Grant model: levels, capabilities, inheritance, no deny

## Context and Problem Statement

People, groups, agents and apps need access to spaces and channels. Agents require a level of
their own: reading and writing is something different from triggering an agent that uses tools
and incurs costs. Administration should succeed correctly in a small team without expert
knowledge, and every decision should be explainable in one sentence ("Sam may write here, because
...").

Which model represents access so that it is secure, explainable and checkable for invariants such
as I1?

## Decision Drivers

- Explainability in plain language for "Who has access?" and the preview.
- Checkable invariants (I1: trigger set of a personal agent).
- Few concepts; the lesson from an earlier chat platform: too many features.
- Time-limited grants, checked on every call.
- Delegation to space managers without escalation of permissions.

## Considered Options

1. Ascending levels `read` < `write` < `trigger`, capabilities (`upload`, `approve_actions`,
   `export`, `manage`), inheritance instance -> space -> channel, the highest valid level applies,
   no deny rules.
2. Roles with allow and deny overrides per channel (pattern of large chat platforms).
3. Relationship-based model with tuples (Zanzibar pattern).
4. Flat access list per channel without inheritance.

## Decision Outcome

Chosen option: 1, because it has the fewest concepts, makes every decision explainable as the
maximum over a short list of valid grants and makes the trigger set for I1 directly computable.

Rulings (details in [../spec/07-permissions.md](../spec/07-permissions.md)):

- Subjects: person, group (only people), agent and app (at most `write`, as a capability only
  `upload`).
- Validity per call: not revoked, not expired, subject active, membership valid.
- Grants are immutable; a change is a revocation plus a new grant (R-DM-09).
- Delegation with `manage` only up to one's own level and one's own capabilities, revocation within
  the same scope; grants to agents and apps only by the owner.
- Guard rules (SR1 to SR12 in 07) take precedence over grants.

### Consequences

- Good, because every decision is explainable: highest level, origin, expiry.
- Good, because there are no ordering errors between allow and deny.
- Good, because I1 can be checked as a set computation over grants, groups, owners and operators.
- Bad, because "everything except X" cannot be expressed; one grants access to the individual
  channels instead of to the space.
- Bad, because a grant on a space also opens future channels of the space; the
  preview when a channel is created shows who inherits it.

### Confirmation

- Test matrix principal x target x action, including expired, revoked, overlapping, group,
  agent in a foreign space (07, section 11.1).
- Difference test of the cache against a slow reference in random worlds (07, AZ-P8).
- I1 test per kind of change (AZ-P7).
- Check in `grants`: agents and apps never `trigger`, capabilities only `upload` (03 `grants`,
  R-DM-20).

## Pros and Cons of the Options

### 1. Levels, capabilities, inheritance, no deny

- Good, because small and explainable.
- Good, because Study C proposes exactly this model and it ends with the agent level `trigger`.
- Neutral, because capabilities are a second axis; it stays closed (four values).
- Bad, because exceptions downward are not possible.

### 2. Roles with allow and deny overrides

- Good, because exceptions downward are possible.
- Bad, because the effect depends on the order of the overrides and is hard to explain.
- Bad, because role hierarchies are deliberately not part of the product (Baseline section 4).

### 3. Relationship-based model

- Good, because very expressive (shared documents, nested groups).
- Bad, because the expressiveness is not needed here and makes explainability and the I1 check
  harder.
- Bad, because it usually requires a separate service or a separate library with its own
  model.

### 4. Flat list per channel

- Good, because simplest.
- Bad, because every new channel needs all grants anew; spaces as a unit are missing.

## More Information

### Partial decision: filtering of lists and search

Options:

1. Permission cache in `authz` per principal, valid for one permission version and until the next
   expiry time; queries get the list of visible channels as a parameter.
2. Materialized view of effective permissions in the database (table or materialized view), with
   a join in every query.

Chosen: 1 (same as R-DM-19 in the core contract), because the cache comes from the same function as
`authorize` and cannot deviate; expiry through time stays correct without a write (the
cache ends at `valid_until`); group changes trigger no mass writes. Option 2 would be a second
truth, would still need a time check in every query and silently delivers wrong results after a
maintenance error. Limit of option 1: very long lists of visible channels; the remedy is a
temporary table from the same snapshot in the same transaction (07, 7.4).

Further sources: Baseline section 3 (grant), Study C (table `freigabe`, no deny rows, expiry
checked per call).
