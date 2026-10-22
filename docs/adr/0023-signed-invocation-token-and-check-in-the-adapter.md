---
status: accepted
date: 2026-10-10
decision-makers: Planning lead, Team agent platform
---

# Signed invocation token and check in the adapter

## Context and Problem Statement

The server decides whether a person may trigger an agent (`agent.invoke`, I2). The adapter
outside the server then starts a runtime with real costs and side effects. How does the server
prove this decision to the adapter so that the adapter starts nothing without a valid decision,
double starts are impossible and the adapter can check whether its backend matches the configured
mode (I3)?

## Decision Drivers

- The adapter must not do anything without a valid decision (mandatory test from 07).
- The decision must apply at the time of delivery, not at the time of the message
  (revoking permissions takes effect immediately).
- The adapter must be able to compare the configured mode and the reported mode of the backend
  policy (I3).
- An adapter must not be able to forge a decision, not even with its own token.
- Key rotation without downtime.

## Considered Options

1. Asymmetrically signed JWT (JWS, EdDSA) per delivery, keys via JWKS, check in the
   adapter and again at run start.
2. HMAC verification tag with a shared secret per agent.
3. No token; the adapter trusts the authenticated gateway connection.
4. The adapter checks back with the server before every start ("may I?"), without a token.

## Decision Outcome

Chosen: option 1, because only it combines forgery resistance toward the adapter, a verifiable
proof with all fields for I3 and key rotation without a shared secret. The token is issued anew
with every delivery (R-SY-07); the server checks it again at run start (`jti`, expiry, configured
mode equals reported mode).

### Consequences

- Good, because an adapter token alone cannot start a run.
- Good, because the claim `billing_mode` enforces the comparison of configured mode against
  reported mode at the location of the runtime.
- Good, because an emergency key rotation delays at most one delivery: the next
  delivery carries a token from the new key.
- Bad, because every adapter must verify JOSE correctly (algorithm allow-list, `typ`,
  time window). The SDK provides the check ready-made (ADR 0026).
- Bad, because the clocks of server and adapter must roughly agree (tolerance as a
  configuration value).

### Confirmation

- Scenario AG-08 in docs/spec/09-agents.md: a missing token, a wrong signature, a different
  `alg`, a foreign `aud`, a wrong `typ` lead to no start.
- Scenario AG-28: An expired, still current token does not lead to an abort; the adapter
  renews it with `run.token.renew`, the server decides anew in doing so, and the run starts
  exactly once (D-W-15).
- Scenarios AG-31 and AG-32: A consumed start token resumes a waiting run neither through
  `run.token.renew` nor through `resumeRun`; a lost response of `run.token.renew`
  does not leave the run hanging, and it starts exactly once (D-W-44).
- Scenario AG-22: scheduled rotation without failed runs, emergency with at most one
  delayed delivery.
- Scenario AG-09: a duplicate delivery leads to exactly one successful `run.start`.
- A schema test keeps the claims in line with docs/spec/05-actions.md section 5.
- M2-06: the schema test `TestInvocationClaims_MatchSpec` (`internal/runs/domain`) holds the claim
  set of the code equal to 05 section 5 and 09 4.5; the reference verifier
  (`internal/runs/adapters/runstest`, `TestReferenceVerifier_RejectsAlgAndTyp`,
  `TestReferenceVerifier_Points`, `TestReferenceVerifier_JWKSCache`) and the server verifier
  (`TestVerify_Reasons`, `TestInvocationToken_ForeignPurposeKey`) reject every other `alg`, `typ`
  and key purpose; AG-22 at token level: `TestAG22_ScheduledRotationFailsNoToken` and
  `TestAG22_EmergencyRotationTokenLevel` (`test/integration/invocationtoken`); the run level is
  M2-31's.

## Pros and Cons of the Options

### Option 1: Asymmetric JWT per delivery

- Good, because the adapter can verify but not forge (R-SY-08).
- Good, because standard libraries exist and RFC 8725 defines the rules (explicit
  `typ`, algorithm allow-list;
  https://www.rfc-editor.org/rfc/rfc8725.html, retrieved 2026-10-10, [verified]).
- Neutral, because the JWKS must be published and cached.
- Bad, because signing per delivery costs computing time (EdDSA is fast for this).

### Option 2: HMAC with a shared secret

- Good, because simple and fast.
- Bad, because whoever can verify can also forge: a compromised adapter could issue
  tokens to itself.
- Bad, because the secret must be distributed and rotated per agent.

### Option 3: Trust in the connection

- Good, because no additional mechanism is needed.
- Bad, because the adapter has no proof of the configured mode, the triggering person and the
  expiry; I3 would not be verifiable.
- Bad, because a delivery through another path (REST fetch, webhook) would prove nothing.

### Option 4: Check back before every start

- Good, because the decision is always completely fresh.
- Bad, because it needs an additional synchronous call per run and blocks the start
  during network problems.
- Bad, because the adapter can trust the result only through its connection, so, like
  option 3, it has no independent proof.

## More Information

- Baseline I2, I3; docs/spec/04-sync-protocol.md section 15.3; docs/spec/05-actions.md
  section 5; docs/spec/09-agents.md section 4.5.
- The claims including `space_id`, `trigger_kind`, `command_id`, `tool_class` and the header
  `typ` are in docs/spec/05-actions.md section 5.
