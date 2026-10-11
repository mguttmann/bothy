# Open questions for the owner

As of: end of M0 (planning). This page holds only questions that the owner alone can answer and
that apply to the product: law and provider terms, costs, external accounts, public commitments of
the project and real product decisions. The planning decides technical questions itself and records
them in the sections "Decisions of this chapter" of the [chapters](spec/00-glossary.md) and in the
[ADRs](adr/README.md).

Every question has a default. The [plan](PLAN.md) keeps building with it until the owner decides
otherwise; no milestone waits for an answer as long as the deadline has not been reached. The
deadline names the ticket or milestone before whose start the answer must be settled, because after
that a change becomes more expensive or has an external effect.

Questions that concern only a single instance (own accounts, own hosts, own operations) are in the
private operations repository of that instance, not here.

**Rules.**

1. An answer of the owner is implemented in the same change that closes the question here:
   ADR or entry in the ruling log, affected chapters, plan tickets.
2. A closed question moves with date and answer to the section
   [Decided](#decided). IDs are never reassigned.
3. A new question comes here only if it passes the test question: Can the planning decide it itself
   with a reason? Then it is not an owner question.

## Overview

Sorted by deadline.

| ID | Question | Default | Due by |
|---|---|---|---|
| OQ-01 | Second owner while personal agents run | strict I1 without an owner exception | before M1-29 |
| OQ-02 | Visibility of the subscription holder on the agent card | visible to everyone with `agent.read` | before M1-51 |
| OQ-03 | The holder's own long-lived subscription token in the account of the session user for the terminal profile `claude` | allowed as own use; the product does not touch the token; M2-45 checks the documentation before the build | before M2-45 |
| OQ-04 | Content recording of terminal sessions | no, only metadata | before M2-46 |
| OQ-05 | Automatic following and silent quiet hours | yes, as in chapter 13 | before M3-19 |
| OQ-06 | No presence and no read receipts for people | leave it as it is | before M4 |
| OQ-07 | Milestone for multi-node operation | M7 (M7-17); terminal relay already in M2 | before M7-17 |
| OQ-08 | Publisher accounts for signing and distributing the apps | test publisher until the decision | before M7-12 and M7-13 |
| OQ-09 | Live tests in third-party clouds | only `tofu validate` and a local VM | before M7-16 |
| OQ-10 | Order of migration and publication | migration first | before M8-09 |
| OQ-11 | External penetration test | commission it before the publication | before V1-02 |
| OQ-12 | Location of the repository and Go module path | placeholder `OWNER`; recommendation: organization | before V1-03 |
| OQ-13 | External review services before the publication | off until the publication | before V1-05 |
| OQ-14 | Public commitments on deprecation and version overlap | default values from chapter 06 | before V1-05 |
| OQ-15 | Commitments in SECURITY.md and GOVERNANCE.md | default values from chapter 23 | before V1-05 |
| OQ-16 | Final name and trademark check | Bothy stays the working name | before V1-06 |
| OQ-17 | Package names and accounts for SDKs at PyPI and npm | reserve only after the choice of name | before the first SDK publication, at the latest V1-06 |
| OQ-18 | Personal subscription in dev environments | no; V2-06 is not built | before V2-06 |
| OQ-19 | Direct messages invisible to owners | yes, access only through the legal route | before V3-01 |
| OQ-20 | Grafana Cloud account for signing the plugin | unsigned and local until the publication | before V3-17 |
| OQ-21 | Problem types as absolute URIs | relative URIs in API v1 | before API v2 |
| OQ-22 | API documentation from the code or from the contract | spec-first according to ADR 0013 | before M1-17 |

## Questions

### OQ-01: Second owner with personal agents

- **Question:** Should a second owner be possible without first switching all agents with
  `billing_mode = personal` to `organization`? That would only work with an owner exception
  in I1: other owners would not count toward the set of triggering people and would be rejected by I2.
- **Why only the owner:** I1 is a requirement of the owner and depends on the provider terms
  (personal subscription only for own use). An exception changes "the owner may do everything".
- **Default:** No. I1 stays strict: `personal` only as long as the `credential_holder` is the
  only owner. Otherwise `owner.add` is rejected; the guided flow `resolve_i1` switches over
  beforehand.
- **What the plan does with it:** M1-29 builds I1 without an exception, M2-21 the guided flow.
  [Chapter 10](spec/10-backend-policy.md), [ADR 0024](adr/0024-backend-policy-personal-organization-with-invariant.md).
  The terminal has its own, narrower rule with an owner exception (I6,
  [chapter 24](spec/24-terminal.md) section 7.3); it restricts owners and does not change I1.
- **Due by:** before M1-29. A later exception would be a new ADR with a migration of the checks.

### OQ-02: Visibility of the subscription holder

- **Question:** May the agent card show everyone who sees the agent whose personal subscription it
  uses (`credential_holder_id`)?
- **Why only the owner:** A trade-off between transparency and privacy toward the people of the
  organization; not a technical question.
- **Default:** Yes, visible to everyone with `agent.read`. Reason: The badge "Personal" is
  visible anyway, and the information explains why only one person may trigger.
- **What the plan does with it:** The schema `Agent` in [openapi.yaml](api/openapi.yaml) contains the
  field; M1-51 delivers it with `getAgent` and `listAgents`. With "no", only those who may use `agent.update` see it.
- **Due by:** before M1-51.

### OQ-03: Long-lived subscription token in the terminal

- **Question:** Is it allowed that the holder of a subscription login (`llm_credentials = personal`)
  logs in, in the account of the session user, with their own long-lived token from `claude setup-token`
  (variable `CLAUDE_CODE_OAUTH_TOKEN`) for their interactive sessions of the profile `claude`?
  The product does not store, read or pass on the token (D-W-17).
- **Why only the owner:** Interpretation of the provider terms; the owner bears the risk. The
  documentation describes the token for environments without a browser, but does not say whether an
  interactive session of the same person is meant ([chapter 24](spec/24-terminal.md) section 7.4, [assumed]).
- **Default:** Yes, as the holder's own use in the unmodified coding agent.
- **What the plan does with it:** M2-45 checks the current provider documentation before the build (ruling T10). If
  that route is excluded there, with `personal` only `/login` in a session remains; if that does not
  work either for a user without a login session, the host offers the profile `claude` only with
  `organization`.
- **Due by:** before M2-45.

### OQ-04: Content recording of terminal sessions

- **Question:** Should it be possible to record the content of terminal sessions, for example for
  record-keeping obligations?
- **Why only the owner:** A question of law and of record-keeping, privacy and codetermination.
- **Default:** No. There is only metadata (`terminal_sessions`, audit) with the
  retention class `security`.
- **What the plan does with it:** M2-46 builds audit and metrics without content; 01 lists recording
  as "never". [Chapter 24](spec/24-terminal.md) section 10.4, [chapter 20](spec/20-privacy.md)
  (R-DP-07), [chapter 01](spec/01-product.md). With "yes" it needs its own ADR: Under
  [ADR 0068](adr/0068-terminal-through-a-host-service-with-noise-stream-and-invariant-i6.md) the server must
  not read the content, so a recording would have to be created in the host service.
- **Due by:** before M2-46.

### OQ-05: Automatic following and quiet hours

- **Question:** Should it be like Discord: whoever writes in a thread follows it automatically, and
  quiet hours make notifications silent instead of muted?
- **Why only the owner:** Product decision about the notification behavior.
- **Default:** Yes, as planned.
- **What the plan does with it:** M3-19 builds automatic following and bundling according to
  [chapter 13](spec/13-push-notifications.md) section 3.2; the person can change "Follow".
- **Due by:** before M3-19.

### OQ-06: Presence and read receipts

- **Question:** People never have a presence ("online") and never read receipts for others; the
  typing indicator exists. Is this intended?
- **Why only the owner:** Privacy and codetermination (performance monitoring) concern the
  organization.
- **Default:** Yes, leave it as it is.
- **What the plan does with it:** The clients from M4 on are built without presence of people and
  without read receipts. [Chapter 01](spec/01-product.md) (R-PR-02, R-PR-03).
- **Due by:** before M4.

### OQ-07: Milestone for multi-node operation

- **Question:** In which milestone should operation with several api nodes be production-ready?
- **Why only the owner:** Scope and time: cloud operation with failover versus operation on
  a single host.
- **Default:** In M7. The code stays node-neutral from M1 on.
- **What the plan does with it:** M7-17 builds stage 2 from [chapter 02](spec/02-architecture.md)
  section 16; the terminal relay across several nodes already comes with M2-47 (P-25,
  [chapter 24](spec/24-terminal.md) section 9).
- **Due by:** before M7-17.

### OQ-08: Publisher accounts for the apps

- **Question:** With which accounts does the project sign and distribute its apps (Apple Developer
  Program, signing for Windows, route for Android)?
- **Why only the owner:** external accounts, running costs, name of the publisher. The accounts of a
  single instance are the business of its instance operator.
- **Default:** Publisher model from
  [ADR 0037](adr/0037-app-distribution-and-auto-update-channels.md) (status `proposed`): each
  organization signs itself. Until the decision, the tickets build and check only with a
  test publisher.
- **What the plan does with it:** M7-12 and M7-13 deliver the release chain, the `distribution.yaml`
  template and the runbook; real signatures are only created after the answer. [Chapter 18](spec/18-release.md).
- **Due by:** before M7-12 and M7-13.

### OQ-09: Live tests in third-party clouds

- **Question:** Should the OpenTofu modules be tested against real accounts at cloud providers?
- **Why only the owner:** Costs and external accounts.
- **Default:** No. Only `tofu validate` and the portability test on a local VM.
- **What the plan does with it:** M7-16 delivers the modules with these checks; a live test would be
  a separate ticket with a cost budget. [Chapter 16](spec/16-operations.md),
  [cloud migration](operations/cloud-migration.md).
- **Due by:** before M7-16.

### OQ-10: Order of migration and publication

- **Question:** Does the first migration of an existing installation (M8-09) run before the
  publication of the repository (V1) or after it? And when exactly does it become public?
- **Why only the owner:** The publication has an external effect and cannot be undone.
- **Default:** Migration first (P-15). A migration with real data before the
  publication provides evidence.
  Until the publication, the release signatures from
  [ADR 0062](adr/0062-release-signatures-while-the-repo-is-private.md) apply at no cost; Artifact
  Attestations come with the publication, because GitHub offers them in the Free, Pro and Team plans
  only for public repositories ([chapter 18](spec/18-release.md) section 6).
- **What the plan does with it:** V1 depends on the build tickets of M8, not on carrying out M8-09;
  the owner can bring V1 forward. [Plan](PLAN.md) sections 15 and 19,
  [Checklist: publishing](quality/public-switch.md).
- **Due by:** before M8-09.

### OQ-11: External penetration test

- **Question:** Is an external penetration test commissioned before the publication?
- **Why only the owner:** Costs and choice of a service provider.
- **Default:** Yes, stage P2 from [chapter 19](spec/19-security.md) "Penetration test plan",
  on a test instance with synthetic data.
- **What the plan does with it:** V1-02 is a prerequisite for V1-04; without a test there is no
  publication.
- **Due by:** before V1-02.

### OQ-12: Location of the repository and Go module path

- **Question:** Under which account or which organization does the public repository appear, and
  which Go module path follows from that?
- **Why only the owner:** external account and public effect; only the owner can create and name an
  organization.
- **Default:** The earlier requirement of the owner applies (personal account). Until then all files
  use the placeholder `OWNER` (P-05). Recommendation: create a neutral organization before the first
  public commit, because a later move breaks the Go module path of all imports and a personal name
  would otherwise be in the module path.
- **What the plan does with it:** V1-03 replaces `OWNER` and the provisional module path; a test checks
  that no placeholder remains. [ADR 0044](adr/0044-working-name-and-branding-as-configuration.md).
- **Due by:** before V1-03.

### OQ-13: External review services before publication

- **Question:** May diffs of the still private repository go to external services: the CodeRabbit CLI
  and the hosted Renovate app?
- **Why only the owner:** Outflow of private code to third parties.
- **Default:** No. The CodeRabbit CLI is off until `BOTHY_CODERABBIT=1` is set;
  Renovate is activated when the repository goes public.
- **What the plan does with it:** [Push discipline](quality/push-discipline.md),
  [ADR 0041](adr/0041-dependency-updates-with-renovate.md),
  [Checklist: publishing](quality/public-switch.md) step 16. Until then the
  local gates check alone.
- **Due by:** before V1-05. The default costs nothing; allowing it earlier is possible at any
  time.

### OQ-14: Commitments on deprecation and version overlap

- **Question:** Which deadlines does the project commit to for integrators: minimum notice before
  shutting down a deprecated operation, and minimum duration that an old major version runs next to the new one?
- **Why only the owner:** public commitment to third parties with maintenance effort.
- **Default:** the default values of `api.deprecation_min_notice` and
  `api.major_version_overlap` from [chapter 06](spec/06-api.md).
- **What the plan does with it:** [ADR 0014](adr/0014-api-versioning-and-deprecation-policy.md) and
  the breaking-change check M7-11 work with these values.
- **Due by:** before V1-05.

### OQ-15: Commitments in SECURITY.md and GOVERNANCE.md

- **Question:** Can the project commit to the deadlines for vulnerability reports (receipt,
  assessment, fix, disclosure) and for issues (first response, closing when feedback is missing)?
- **Why only the owner:** public commitment of the project.
- **Default:** the default values of `security.*` and `triage.*` from
  [chapter 23](spec/23-quality.md); a test keeps the files in line.
- **What the plan does with it:** The community files name these values; V1-05 makes them public.
- **Due by:** before V1-05.

### OQ-16: Final name and trademark check

- **Question:** Does it stay Bothy, or does it become one of the other candidates? Is a
  trademark search by a professional in classes 9 and 42 commissioned?
- **Why only the owner:** Trademark and legal question, cost of the search, product decision.
- **Default:** Bothy stays the working name; commission the search before the publication.
- **What the plan does with it:** The displayed name is configuration (`branding.product_name`); in
  the code it appears only in the places that
  [ADR 0044](adr/0044-working-name-and-branding-as-configuration.md) lists, and a test holds the
  list. V1-06 sets the final name.
- **Due by:** before V1-06. OQ-17 and OQ-20 depend on it.

### OQ-17: Package names and accounts for the SDKs

- **Question:** Under which names are the SDKs registered at PyPI and npm, and who holds the
  accounts (npm scope or organization)?
- **Why only the owner:** external accounts; depends on the name (OQ-16).
- **Default:** reserve only after the final choice of name.
- **What the plan does with it:** The SDKs are created in M2-29 and M2-30 without publication; the
  release chain publishes only once the name is fixed.
  [ADR 0026](adr/0026-sdks-generated-core-plus-hand-written-layer.md), [chapter 18](spec/18-release.md).
- **Due by:** before the first publication of an SDK, at the latest V1-06.

### OQ-18: Personal subscription in dev environments

- **Question:** May a person use their personal subscription in their own isolated dev environment?
  That requires I1 per environment instead of per agent.
- **Why only the owner:** changes the requirement I1 and touches provider terms.
- **Default:** No. Dev environments have `organization` (own key with a
  spending limit per person) or no LLM login.
- **What the plan does with it:** V2-06 is not built without a decision.
  [Chapter 21](spec/21-dev-isolation.md) (K-DV-2),
  [ADR 0046](adr/0046-multi-user-dev-environment-only-with-isolation.md).
- **Due by:** before V2-06.

### OQ-19: Direct messages invisible to owners

- **Question:** Do direct messages between people stay invisible to owners as well? That deviates from
  "the owner may do everything".
- **Why only the owner:** Confidentiality between people versus control by the
  organization; a legal question.
- **Default:** Yes, invisible. Access only through a data subject access route or a legal route with
  step-up and audit (ruling L21).
- **What the plan does with it:** V3-01 builds direct messages with guard rule SR6.
  [Chapter 07](spec/07-permissions.md), [chapter 05](spec/05-actions.md) (R-AZ-02).
- **Due by:** before V3-01.

### OQ-20: Grafana Cloud account for the plugin signature

- **Question:** When and under which organization name is the account at Grafana Cloud created
  that signing the panel plugin needs? The first part of the plugin ID must match the account.
- **Why only the owner:** external account and name at the provider; depends on OQ-16.
- **Default:** Until the publication unsigned and provisioned locally; only exactly this plugin ID
  is allowed (ruling T11). Account and application as a community plugin after the
  final choice of name.
- **What the plan does with it:** M7-25 and M7-26 deliver the plugin unsigned; V3-17 signs it.
  [Chapter 25](spec/25-visualization.md) section 8,
  [ADR 0069](adr/0069-visualization-on-two-levels-without-iframes-in-apps.md).
- **Due by:** before V3-17; earlier only if the plugin is to run in Grafana Cloud.

### OQ-21: Problem types as absolute URIs

- **Question:** Should the error types of the API later get absolute URIs under a public
  project domain?
- **Why only the owner:** requires a public domain of the product and depends on OQ-16.
- **Default:** The relative URI `/api/v1/problems/{code}` stays for API v1.
- **What the plan does with it:** nothing until v2; a switch would be a breaking change.
  [Chapter 06](spec/06-api.md) section 5,
  [ADR 0015](adr/0015-error-format-rfc-9457-idempotency-pagination.md).
- **Due by:** before the planning of API v2.

### OQ-22: API documentation from the code or from the contract

- **Question:** Fixed build rule 3 requires "API documentation is generated from code (OpenAPI from the
  routes)". ADR 0013 makes `openapi.yaml` normative and generates the code from it. Does the reading
  "one source, spec-first" (D-W-43) apply to the server?
- **Why only the owner:** The build rule is an owner requirement; only the owner can interpret or
  change it.
- **Default:** Yes, spec-first. Server stubs, clients and the API reference are generated from
  the contract; a test (`make codegen-check`) keeps code and contract in line. For SDKs and
  Dart packages the rule applies literally.
- **What the plan does with it:** M1-17 and all API tickets work spec-first. If the
  owner answers "from the routes", a new ADR is needed that supersedes ADR 0013, and M1-17 changes
  the code generation.
- **Due by:** before M1-17.

## Decided

Questions from the team reports and questions of this page that have since been decided. They are
here so that nobody asks them again. A closed question of this page keeps its ID in the
column "Question"; the ID is not reassigned.

| Question | Decision | Who, when | Effect |
|---|---|---|---|
| Brand color and design (dark petrol, yellow accent) | accepted | Owner, 2026-10-10, on the previews in [design](design/README.md) | Palette and tokens stay; accent variants per instance come with M4-03. |
| Does the acceptance apply to the seven preview images generated again afterwards (D-W-46)? | yes, without a new acceptance: The acceptance applies to the images in `docs/design/previews`, and the seven are content corrections within the accepted design (run states as words, `amber.100` in the warning banner, no truncated message). | Owner decision "Design on `docs/design/previews` accepted", passed to the planning in this wording on 2026-10-11, after the seven images had been generated again on the same day (D-W-46, D-W-77) | The tickets build with these versions. |
| OQ-23: Are the preview images terminal and statistics (`terminal-*`, `viz-*`) considered accepted? | yes, without an open owner question: They are in `docs/design/previews` and apply the accepted design system (same tokens, same rules). They were created after the acceptance of 2026-10-10: both pairs on the same day (D-W-13), `viz-*` generated again on 2026-10-11, last with D-W-55. | Owner decision as in the row above; ruling T14 of the lead, 2026-10-11 (D-W-77) | The tickets build with these versions; the earlier deadline before M4-03 no longer applies. |
| Commit identity in the public repository | The commits carry the identity chosen by the owner; the product repository does not name it. | Owner, 2026-10-10 | No rewriting of the authors needed. What is published is a commit without parents; the planning history stays private (D-W-93, step 2 of the [Checklist: publishing](quality/public-switch.md)). |
| Android, voice messages, sharing from other apps, login through OIDC | Part of the core | Owner, planning basis | [Chapter 01](spec/01-product.md), [chapter 08](spec/08-identity.md); tickets in M3 to M5. |
| Revisions of edited messages and default values of the core contract (retention of the event log, session duration, sizes) | not an owner question | Planning | Revisions follow the class `content` in [chapter 20](spec/20-privacy.md); the values are named configuration values that every instance can change ([configuration reference](operations/configuration.md)). |
| View with data about individual subscription seats or credentials | not in the product | Planning, ruling T8 | The product shows only the backend view ([chapter 25](spec/25-visualization.md) section 6.2) and knows no credentials (I5). |
