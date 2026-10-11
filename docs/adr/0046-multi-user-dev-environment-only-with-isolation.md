---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
---

# Multi-user dev environment only with isolation

## Context and Problem Statement

Dev projects are forums with a project binding, served by a runner agent. The runner starts a
coding agent without tool permission prompts as a user of the host, without a sandbox. Today this
is safe enough because only the owner triggers. As soon as a second person may trigger,
"triggering" means shell access to the host with all secrets of the instance operator; the same
holds for every prompt injection in repository content. Baseline section 6 plans the multi-user
dev environment as a later milestone.

Under which conditions may further people trigger dev runs, and how does the server enforce
this?

## Decision Drivers

- No path from a person with `trigger` to the shell of the host or to secrets of the instance
  operator.
- Separation of people from each other.
- Enforcement in the server, not through configuration that can be forgotten.
- A boundary also against injection, not only against malicious intent.
- Feasible with existing tools and costs that a small team can bear.
- Compatible with the backend policy (I1) and the provider terms.

## Considered Options

1. Dev projects permanently for the owner only.
2. Shared runner, but with tool permission prompts switched on.
3. A separate operating system user per person on the host.
4. An environment per person with a hypervisor boundary (local VM or cloud VM), enabled through a
   checked record `dev_environments` and a guard rule in the server.

## Decision Outcome

Chosen option: 4, because only a VM boundary protects the host, secrets and other people even
when the agent fully follows the triggering person or an injection, and because the server can
check the enabling as a record with checks on every invocation.

Rulings ([../spec/21-dev-isolation.md](../spec/21-dev-isolation.md),
[../spec/07-permissions.md](../spec/07-permissions.md)):

- Guard rule SR3: a non-owner triggers in a dev project only if a ready environment of their own
  exists; the run runs exactly there; threads stay bound to their environment.
- Guard rule SR2: agents of the tool class `host` are triggered only by an owner or by a person in
  their own ready environment.
- Ready means: checks D1 to D10 passed and not older than `dev.reverify_interval`, red team
  passed for the kind and the image, approval by the owner with step-up.
- The default is one Linux VM per person; a macOS VM only when Apple tools are needed; a cloud VM
  is equivalent. The owner decides where the VMs run.
- LLM access of other people's environments in this milestone only through organization keys.

Until an environment is ready, the product behaves like option 1, without anyone having to
remember it.

### Consequences

- Good, because today's boundary ("owner only") stays in place without configuration until the
  isolation is proven.
- Good, because grants (`trigger` on a forum) and isolation are separate: a person can read along
  and write before they may trigger.
- Good, because a failed check disables the environment immediately.
- Bad, because VMs cause operations work, images, network rules and costs.
- Bad, because macOS VMs are limited per Mac.
- Bad, because a person's personal subscription in their own environment becomes possible only
  after a change to I1 (21, K-DV-2: deferred to the milestone; until then only `organization`,
  R-DM-24).

### Confirmation

- Tests DEV-T01 to DEV-T07 (21) and AZ-M15 to AZ-M20 (07).
- Red team P3 per environment kind and image ([../spec/19-security.md](../spec/19-security.md),
  section 13), result in the record (`red_team_passed_at`, `red_team_ref`).

## Pros and Cons of the Options

### 1. Owner only, permanently

- Good, because safe and without effort.
- Bad, because a requirement of the product (further people working on dev projects) is
  permanently missing.

### 2. Shared runner with tool prompts

- Good, because no new infrastructure.
- Bad, because in long runs the prompts get switched off or confirmed blindly.
- Bad, because a confirmed prompt still runs with the permissions of the host user.

### 3. Separate user per person

- Good, because feasible on every host without virtualization.
- Bad, because a shared kernel, local services and misconfigurations (readable files, a
  forwarded SSH agent) weaken the boundary.
- Bad, because network rules per user are hard to enforce.

### 4. VM per person with a readiness check

- Good, because the boundary holds against injection as well.
- Good, because resetting to a clean image is possible.
- Neutral, because several platforms (Linux, macOS, cloud) must be supported.
- Bad, because of effort and costs.

## More Information

- Study C: a dev environment for others only as a separate user or VM (VM stronger), their own
  login or their own key, their own Git access, network only to the provider, the Git host and
  package sources.
- Threat T-DEV-01: [../security/threat-model.md](../security/threat-model.md).
- Anthropic allows a person's own login in unmodified Claude Code, even when a platform operates
  it: https://code.claude.com/docs/en/legal-and-compliance, retrieved 2026-10-10
  [verified].
