---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Team D (agent platform), Team C (security)
informed: Team B (API)
---

# Origin and trust marking of foreign content in the agent context

## Context and Problem Statement

An agent gets context on every run: the triggering message, earlier messages of the thread,
attachments and transcripts. These contain texts from people without trigger permission, from other
agents, apps and imports. Each of these texts can contain instructions that the runtime takes for
a task (prompt injection, [threat-model.md](../security/threat-model.md), T-AI-01 and
T-AI-03). The runtime cannot tell senders apart by itself, and only the server knows who has which
permission at the time of the run. Who builds the context, and how does the runtime learn which
content it may follow?

## Decision Drivers

- Only `authz` knows the permissions of the authors; the adapter should not rebuild them (B7).
- Instructions come only from the triggering person; everything else is material (SEC-AI-1 in
  [19-security.md](../spec/19-security.md)).
- Permissions apply at the time of the run, not at the time of writing.
- The mark must reach every runtime, including third-party adapters (Hermes Agent, Claude Code
  runner, SDK adapter).
- Prompt injection cannot be fully prevented [Q1, verified]; marking is one of several boundaries,
  not the only one.

## Considered Options

- The server builds the context with role and trust mark per element (`run.context.read`,
  schema `run_context.v1`); adapters pass marked content on as data in a fixed system part
- The adapter builds the context itself from events and `thread.read`
- The server filters: content of untrusted authors never reaches the agent
- Detection of injections with a classifier before passing content on
- No marking; only tool class, approval cards and acting-for binding limit the damage

## Decision Outcome

Chosen option: "The server builds the context with role and trust mark", because only it can set
the mark correctly and every adapter then passes it on the same way. Normative in
[09-agents.md](../spec/09-agents.md), section 4.6 (R-AG-06).

1. The adapter fetches the context with `run.context.read` (`GET /api/v1/runs/{id}/context`, only
   for its own active runs). Every element carries `author`, `origin`, `role` (`user`, `assistant`,
   `other`) and `trust`.
2. The server sets `trust` at query time: `trigger` (the triggering person), `authorized` (another
   person with effective `trigger` on the channel), `self` (this agent), `unverified` (a person
   without `trigger`, other agents, apps, imports), `system`.
3. Required for every adapter: content with `unverified` goes to the runtime only as marked
   data with a fixed note before it, never as an instruction. The context is a fixed system part
   with author and mark, separate from the text of the triggering person.
4. Content of files (fetched through `file.read`) is always data, even if the triggering person
   uploaded it: a document from the web can contain instructions that do not come from the person
   (T-AI-03). The transcript of a voice message, in contrast, carries the mark of its message,
   because it is the words of its author.
5. The triggering message is never truncated; the rest of the context has the limits
   `agent.context_message_max_bytes` and `agent.context_max_bytes`.
6. The other boundaries remain: agents never trigger (SR4), tool class (SR2), approval cards
   only for the triggering person (SEC-AI-4), acting-for binding (SR9 in
   [07-permissions.md](../spec/07-permissions.md)).

### Consequences

- Good, because the mark is created in one place and permission changes take effect immediately:
  if an author loses `trigger`, their messages are `unverified` in the next context.
- Good, because adapters need no permission logic and the payload does not have to fit into events.
- Good, because foreign content stays available: an agent can summarize a customer email without
  obeying it. A filter would destroy the value.
- Bad, because the effect depends on the runtime: in a study, marking the origin clearly lowered
  the success rate of attacks [Q2, verified], but it does not prevent them [Q1, verified]. The
  residual risk is in the threat model.
- Bad, because every adapter implements the formatting itself; errors only show in the
  conformance test. Mitigation: the required test AG-20 for every reference adapter and a
  formatting helper in the hand-written layer of the SDKs (ADR 0026).
- Neutral, because one context fetch per run adds load; it replaces reading along all channel
  events in the adapter.

### Confirmation

- Required scenario AG-20 ([09-agents.md](../spec/09-agents.md), section 16.2): an author without
  `trigger`, another agent and an import result in `trust = unverified`; the adapter test shows the
  text only in the marked block.
- Required scenario AG-21: context above `agent.context_max_bytes` is truncated from old to new,
  the triggering message stays complete.
- Test: an author loses `trigger` between two runs; in the second context their message is
  `unverified`.
- Test in the reference adapter: the content of an attachment appears only as marked data, even
  when the triggering person uploaded it.
- Build ticket M2-13 in [PLAN.md](../PLAN.md) (SEC-AI-1, size limits).

## Pros and Cons of the Options

### Server builds the context with marks

- Good, because permissions and mark come from one source.
- Bad, because one more operation and a versioned schema must be maintained.

### Adapter builds the context

- Good, because the server does less.
- Bad, because the adapter would have to know the permissions of every author; it sees only its own
  grants, not those of other people.

### Server filters foreign content

- Good, because untrusted text never reaches the runtime.
- Bad, because agents are supposed to work on exactly such content (summarizing, answering); the
  context would have gaps and answers would be wrong.

### Classifier for injections

- Good, because conspicuous texts could be detected in advance.
- Bad, because the server would have to call models for that (against B10 and R-AG-14) and
  detection can be bypassed; OWASP names no reliable detection [Q1, verified].

### No marking

- Good, because there is nothing to build.
- Bad, because the runtime cannot tell instructions in foreign texts apart from tasks;
  OWASP recommends separating and labeling foreign content [Q1, verified].

## More Information

- Sources (retrieved 2026-10-10): Q1 OWASP Gen AI Security Project, "LLM01:2025 Prompt Injection",
  https://genai.owasp.org/llmrisk/llm01-prompt-injection/ [verified]; Q2 Hines et al., "Defending
  Against Indirect Prompt Injection Attacks With Spotlighting" (2024),
  https://arxiv.org/abs/2403.14720 [verified; measured with GPT models, transferability to other
  models derived].
- Format in the reference adapter for Hermes Agent: [09-agents.md](../spec/09-agents.md),
  section 17.6.
- Related: ADR 0022 (agents outside the server), ADR 0023 (invocation token), ADR 0025 (run),
  ADR 0026 (SDKs).
