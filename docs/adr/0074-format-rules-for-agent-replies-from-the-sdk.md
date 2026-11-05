---
status: accepted
date: 2026-11-04
decision-makers: Owner, planning lead
consulted: Builder of F-AGENT-FORMAT
informed: all teams
---

# Format rules for agent replies from the SDK

Normative: [09-agents.md](../spec/09-agents.md) section 4.11 and R-AG-42; rendering:
[01-product.md](../spec/01-product.md) 6.4; rulings:
[decisions.d/F-AGENT-FORMAT.md](../decisions.d/F-AGENT-FORMAT.md).

## Context and Problem Statement

An agent wrote a text meant to be forwarded into a code block with hard line breaks after about
60 characters. The apps showed it as a narrow column, and the copy arrived as a bundle of short
lines. The owner asked on 2026-11-04 that code, tables and texts always look as they should. The
apps already render Markdown well (wrapping text, code blocks with language and wrap toggle,
tables); the agents do not know how a reply in Bothy should be written. Each adapter gives its
runtime instructions over its own channel: the Hermes adapter in the ephemeral system part of a
turn (17.6), the Claude-Code-Runner in its system prompt file (18.2). Where do the format rules
live, and how do they reach every agent, without touching the configuration of a runtime outside
this repository?

## Decision Drivers

* One definition, used by every adapter; no copy that can drift.
* The rules are instructions of the product, not context: they must not stand among the trust
  marked messages and data blocks of the run context (ADR 0054).
* They reach the runtime on every run, also for a resumed session and a command run.
* No change to a runtime's own configuration (a Hermes profile, user instruction files).

## Considered Options

* A text in the Python SDK (`bothy_sdk.message_format`), placed by each adapter in its
  instruction channel.
* A member of the run context (`GET /api/v1/runs/{id}/context`), sent by the server.
* A copy in each adapter (the runner's prompt file, a constant of the Hermes adapter).
* Part of the system text of `render_context`.

## Decision Outcome

Chosen option: "a text in the Python SDK", because both agent adapters already build on the SDK
and place their own fixed instructions themselves; one module is the only source, and each adapter
puts the text where its runtime takes instructions.

* The run context is data with trust marks for the context of one message; the server answers a
  command run's context with `500` (4.6), so a Hermes command run would get no rules, and an API
  member would need a schema change and three generated clients for a fixed text.
* `render_context` produces context that the runner sends as a user message, not as a system
  instruction, and its output is held by the shared golden files of both SDKs.
* A copy per adapter is what the owner's rule "one source" excludes.

### Consequences

* Good, because a change of the rules is one change in one module, covered by one test.
* Good, because the rules arrive as instructions, before the channel prompt, so an operator can
  still add to them.
* Bad, because an adapter that does not use the Python SDK must take the text over itself. The
  TypeScript SDK has no agent adapter today; the first one adds the text there with a test that
  compares it with the Python module.
* Neutral: the server does not check or rewrite replies; a reply that breaks a rule is shown as
  written.

### Confirmation

`sdk/python/tests/test_message_format.py` holds every rule. The Hermes adapter's unit test and
its Fake-Hermes test, and the runner's unit and scenario tests, check that each run carries the
text of the SDK word for word. The rendering of a reply in these rules on macOS, Windows, iOS and
Android is checked by `packages/design_system/test/components/markdown_agent_reply_test.dart` and
the golden `agent_reply`.
