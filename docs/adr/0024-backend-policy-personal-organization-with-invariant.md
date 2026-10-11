---
status: accepted
date: 2026-10-10
decision-makers: Planning lead, Team agent platform
consulted: Owner (owner exception rejected, ruling L22, see More Information)
---

# Backend policy personal/organization with invariant

## Context and Problem Statement

An agent can run on a person's personal subscription or on an organization API key.
The Anthropic terms provide personal subscriptions for ordinary, individual use and prohibit
third-party developers from routing requests through Free, Pro or Max credentials on behalf of
their users or from collecting, storing or brokering Claude.ai credentials
(https://code.claude.com/docs/en/legal-and-compliance, retrieved 2026-10-10, [verified]). How does
the product ensure that a personal subscription never carries out requests from other people,
without knowing any credentials itself?

## Decision Drivers

- No path on which another person triggers a personal agent, not even through groups,
  inheritance, owner permissions or a change of operator.
- The server knows no credentials (B10, I5).
- A check where permissions change, where an invocation happens and where the runtime starts.
- Switching must be deliberate, justified and traceable.
- Make errors visible, do not prevent them silently.

## Considered Options

1. Field `billing_mode` per agent with three checks: invariant I1 transactionally on every
   permission change, I2 before every invocation, I3 in the adapter with a signed token; switching
   I4 with step-up and a reason.
2. Only a check at invocation (I2), without an invariant on permission changes.
3. Only organization keys, no personal mode.
4. Choice of the backend per trigger by the triggering person.

## Decision Outcome

Chosen: option 1 (Baseline section 5). It prevents the failure case at three independent
places and keeps the permission model honest: Nobody is shown a level `trigger` that they could
never use on a personal agent.

### Consequences

- Good, because a misconfiguration (grant, group, second owner) fails on save, with a code
  and a hint, instead of later as a rejected invocation.
- Good, because the adapter checks its runtime against the configured mode with the claim
  `billing_mode`.
- Good, because the emergency switch "all to organization" works at any time.
- Bad, because every owner has `trigger` everywhere: `personal` works only as long as the holder
  is the only owner (docs/spec/03-data-model.md section 6.4).
- Bad, because the I1 check needs additional queries on permission changes; it runs only
  for agents with `personal` and under the lock of the permission version.

### Confirmation

- Tests BR-01 to BR-08 in docs/spec/10-backend-policy.md (one violating change per action
  from 05 rule 1.6; race; revocation between acceptance and delivery; mismatch between configured
  and reported mode).
- Tests 5 and 6 from docs/spec/05-actions.md section 9.
- A leak test checks that no response and no log contains key values (BR-13).

## Pros and Cons of the Options

### Option 1: Invariant plus invocation and adapter check

- Good, because three independent places catch the same error.
- Good, because denials are recorded in the audit log.
- Neutral, because the console needs a guided flow "switch first, then grant".
- Bad, because several owners rule out the personal mode.

### Option 2: Only a check at invocation

- Good, because simpler to build.
- Bad, because people see permissions that fail on every attempt; the error shows only
  to the user, not to the admin.
- Bad, because a gap in the invocation check alone leads to a violation.

### Option 3: Only organization keys

- Good, because there is no subscription risk.
- Bad, because a person could not use their own subscription for their own agents,
  although the terms provide for individual use; the requirement from Baseline section 5
  (mode `personal` for individual use) would be missed.

### Option 4: Choice per trigger

- Good, because flexible.
- Bad, because the runtime (for example a Hermes profile) cannot safely switch its login per
  turn, and every trigger would demand a decision with cost consequences from the user.
- Bad, because the relation "whoever triggers pays on whose subscription" would remain unclear
  to third parties.

## More Information

- Baseline section 5 (I1 to I5); docs/spec/10-backend-policy.md.
- Owner exception: It was considered that owners other than the holder do not count toward the
  I1 set and are rejected by I2. The lead rejected this (ruling L22): `personal` works only as
  long as the `credential_holder` is the only owner, because owners have `trigger` everywhere and
  "the owner may do everything" should not get an exception. A change would be a new ADR that
  supersedes this one.
