---
status: accepted
date: 2026-11-04
decision-makers: Owner, planning lead
consulted: Builder check (fresh reviewer)
informed: all teams
---

# Search assistant through an agent over the full-text search

Extends [ADR 0032](0032-full-text-search-in-postgresql.md) (full-text search in PostgreSQL) and
applies [ADR 0022](0022-agents-and-plugins-outside-the-server-process.md) (agents outside the
server process) to search. Normative: [14-search.md](../spec/14-search.md) 4.10 and
[09-agents.md](../spec/09-agents.md) 10.6; design note:
[search-assistant.md](../design/search-assistant.md); rulings:
[decisions.d/F-INTELLIGENTE-SUCHE.md](../decisions.d/F-INTELLIGENTE-SUCHE.md).

## Context and Problem Statement

The owner has many projects and chats and looks for sessions by what happened in them, not by the
words they contain (2026-11-04, translated): "I tell it what or which session I am looking for
... it searches everything ... and gives me the link to the session." The full-text search of
ADR 0032 answers word queries: all words must match, and a paraphrase finds nothing. Understanding a
question needs a language model. The server holds no model credentials (B10, I5), every read is
filtered by `authz` (B7), and agents run outside the server (ADR 0022). Where does the model run,
what does it see, and what happens without it?

## Decision Drivers

- Permissions: a hit only from what the person may read; the model sees nothing the person or the
  agent that runs it may not read.
- No model credentials and no new paid service in the server.
- Bounded cost and time per question; a usable answer without the model.
- Nothing stored: the search is a short-lived conversation, not a thread.

## Considered Options

1. Server pipeline over the full-text search, with one model call per question through an assistant
   agent over the gateway (like autocompletion, 09 10.5).
2. A search agent with tools: the runtime calls the search API itself, several times, inside a run.
3. A model driver inside the server (an HTTP model endpoint with a key from the secret provider).
4. Semantic search with embeddings (a vector extension or an external embedding service).

## Decision Outcome

Chosen: option 1. The server turns the question into terms, runs the thread title and message
searches of ADR 0032 with the person's scope and page check, fuses them per thread, keeps what the
assistant agent may read too, and sends the candidates as plain excerpts with opaque references to
one live connection of the assistant agent named in `search.assist.agent_id`. The agent makes one
model call, answers picks with a reason and further terms, and reports its usage. The server accepts
only references it sent, runs one more full-text pass with the terms and returns the picks first.
Without an answer the person gets the fused full-text hits.

### Consequences

- Good, because the permission filter stays in SQL and in one function (`authz`); the model can
  only choose among authorized candidates and cannot name anything else.
- Good, because the model runs on the agent's own account (for the Claude Code runner the owner's
  subscription); the server stays without model credentials and without a new service.
- Good, because a question costs one model call and at most `search.assist.timeout`; the full-text
  answer is always there.
- Bad, because the model chooses only among what full text finds for the person's words; terms the
  model adds find more, but those hits come without a reason.
- Bad, because the cost of a question is shown as an estimate but not booked in `cost_entries`,
  which belong to runs.
- Bad, because open requests live in the memory of one api node (as for autocompletion).

### Confirmation

- Server tests: a person never gets a hit from a channel only another person may read, and the
  payload to the assistant holds no excerpt of it, also when the assistant names a reference it was
  not sent; the fixture questions of the owner's example find the expected thread among the first
  hits; without an assistant, without a connection and after the timeout the answer is full text.
- Adapter tests of the Claude Code runner: the prompt marks the candidates as data, the answer is
  parsed and checked, and a failing model call answers nothing.
- Client widget tests: the popup shows the hits with their reasons and a tap opens the thread at the
  message.

## Pros and Cons of the Options

### Server pipeline with one model call through an agent

- Good, because of the drivers above.
- Neutral, because a new ephemeral event and an answer endpoint are needed; both follow
  autocompletion.

### Search agent with tools

- Good, because the model could search freely and several times.
- Bad, because it needs a tool bridge into the runtime and a run with its binding (SR9) per
  question, and several model calls; slower and dearer.

### Model driver inside the server

- Good, because it has the shortest path.
- Bad, because the server would hold a model key and call a paid service, against B10 and
  ADR 0022.

### Embeddings

- Good, because paraphrases are found without words in common.
- Bad, because no local embedding model or vector extension is shipped, an external service needs
  an owner release and a processor contract, and every edit and deletion must reach a second index.

## More Information

- The reference adapter is the Claude Code runner (09 18.16).
- Revisit when a local embedding model ships with the product or when search questions need costs
  per space (then `cost_entries` needs a source other than a run).
