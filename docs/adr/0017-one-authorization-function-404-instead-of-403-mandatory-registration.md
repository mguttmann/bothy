---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
---

# One authorization function, 404 instead of 403, mandatory registration

## Context and Problem Statement

The product has many entry points that need permissions: REST routes, WebSocket ops, delivery of
durable and ephemeral events, push, outgoing webhooks, search, counters, file retrievals,
agent invocations, component clicks and commands. Agents make errors expensive: an agent that
answers in the wrong channel or acts for the wrong person passes on content or triggers effects.
In an earlier system, exactly this error arose from scattered allow lists in several
places (Study B, lesson from an earlier chat platform).

How do we ensure that every entry point is checked, that the check decides the same way everywhere
and that those who are rejected learn nothing about foreign spaces?

## Decision Drivers

- One truth: the same question yields the same answer everywhere.
- Completeness can be checked: a test must detect when an entry point without a check is created.
- No information about the existence of foreign resources.
- Fail closed on errors and unknown cases.
- Latency: a check on every delivery, so very frequent.
- Explainability for administration ("Who has access?").

## Considered Options

1. One function `authorize(principal, action, resource)` in the module `authz`, a closed
   action catalogue, mandatory registration for every route, op and event type.
2. Checks per handler (middleware per route, separate queries in the modules).
3. External policy engine as a separate service with its own policy language or its own
   relationship model.
4. Row-Level Security in PostgreSQL as the main mechanism.

## Decision Outcome

Chosen option: 1, because it is the only one that makes completeness testable (every registration
needs an action from a closed catalogue), uses the same logic everywhere and decides in-process
without a network hop.

Rulings:

- Every route carries `x-authz-action`, every op and every event type an action or receive action
  ([../spec/05-actions.md](../spec/05-actions.md)). The server does not start if a
  registered route has no known action.
- No read permission results in `404`, regardless of whether the resource exists. `403` only if
  reading is allowed but the level, capability or scope is missing (R-AZ-01).
- Lists, counters and search filter in SQL with the set of visible channels from `authz` and
  additionally check every element with `authorize` ([../spec/07-permissions.md](../spec/07-permissions.md),
  section 7).
- Unknown action: `500`, alarm, never an allow.

### Consequences

- Good, because a forgotten entry point becomes a start error or a test failure instead of a gap.
- Good, because permission changes take effect in one place, also on push, webhooks and agents.
- Good, because `404` for everything invisible reveals no spaces, channels or IDs.
- Bad, because `authz` lies on the hot path of every delivery; this requires a permission cache
  with a version counter and load tests.
- Bad, because `404` is sometimes hard for people to understand. Compensation: "Who has access?"
  and "View as person" for administration.
- Bad, because every new feature first needs an action in the catalogue; this is intended friction.

### Confirmation

- Route test, op and event test, catalogue test (07, AZ-P1 to AZ-P3).
- 404 equality test for status, body, headers and response time (AZ-P4).
- Property test "Counters and search reveal nothing" (AZ-P5) and lint on the SQL predicate
  (AZ-P12).
- Lint rules `authz-action-required` and `authz-action-known` on the OpenAPI document (05,
  rule 3).

## Pros and Cons of the Options

### 1. One function with a catalogue and mandatory registration

- Good, because completeness can be checked by machine.
- Good, because the decision is made in-process (no additional latency over the network).
- Neutral, because the permission logic is our own code and needs its own tests.
- Bad, because the catalogue must be maintained.

### 2. Checks per handler

- Good, because quick to write.
- Bad, because exactly the scattered logic arises that led to the error in the earlier system.
- Bad, because completeness cannot be checked.

### 3. External policy engine

- Good, because rules are declarative and separate from the code.
- Bad, because an additional service lies in the path of every delivery (latency, outage,
  operations).
- Bad, because invariants such as I1 need transactions across grants, groups and operators
  that must be checked in the same database transaction.
- Bad, because a second language for permissions arises in the project.

### 4. Row-Level Security in PostgreSQL

- Good, because the database filters by itself, even with forgotten predicates.
- Bad, because push, webhooks, WebSocket delivery and agent invocations do not run through SQL
  and would still need a second implementation.
- Bad, because expiry times, guard rules and acting-for binding become hard to read and
  hard to test in policies.
- Neutral: possible later as an additional layer of defense, not as the truth.

## More Information

- Permission model and flow: [../spec/07-permissions.md](../spec/07-permissions.md).
- Action catalogue: [../spec/05-actions.md](../spec/05-actions.md).
- Grant model: ADR 0018.
- Study C (enforcement points P1 to P7) is the functional template; Baseline B7 fixes the function.
