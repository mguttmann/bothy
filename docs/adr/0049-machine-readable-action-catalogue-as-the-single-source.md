---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Team A (architecture), Team B (API), Team C (permissions)
informed: all planning teams
---

# Machine-readable action catalogue as the single source for docs, lint and code

## Context and Problem Statement

The action catalogue in [05-actions.md](../spec/05-actions.md) is closed and normative: every
route, every op, every event and every delivery names an action from it (B7, rules 1.1 to
1.5). Today it exists as a Markdown table. The same lists appear again in
`docs/api/redocly.yaml` (lint rule `authz-action-known`), in the tables of
[02-architecture.md](../spec/02-architecture.md) (sections 4.6 and 4.7), in `x-receive-action` of
the AsyncAPI and, in the future, in the Go constants of `kernel/authorize` and the route table.
Every copy can diverge; the server's startup check and the route test need a list that a program
can read. Where does the catalogue live, and what is generated from it?

## Decision Drivers

- One source per fact; copies are only created by generation and are compared in the gate.
- Spec-first (ADR 0013): the specification leads, the code follows.
- The catalogue stays readable for people in 05; review sees every change.
- Startup check, route test and lint need the same data (05, rules 1.3 to 1.5).
- Tools in the repo are Python, the server is Go; both must read the source.

## Considered Options

- YAML file `docs/spec/actions.yaml` as the single source; the tables in 05, the lists in
  `redocly.yaml`, the Go constants and the route table are generated from it
- The Markdown tables in 05 stay the source and are read by tools
- Go code as the source (declarations in `kernel/authorize`), docs generated from it
- OpenAPI and AsyncAPI extensions as the source
- Several maintained lists with a reconciliation script (current state, gate G2 in 06)

## Decision Outcome

Chosen option: "`docs/spec/actions.yaml` as the single source", because people can read it,
Python and Go load it without parser tricks, and every copy is created mechanically.

1. Content of the file: all actions with the columns from 05 (ID, description, resource,
   permission, owner, step-up, audit, permissions, rate-limit class), the app scopes from 05,
   section 2.6, the rate-limit classes with key and purpose, the durable and ephemeral event types
   with their receive action (05, section 6) and the WebSocket ops with their action (section 7).
   Further attributes are added only through an ADR (for example the effect for the maintenance
   mode, ADR 0065).
2. Generated are: the tables of sections 3, 4, 6 and 7 in 05 between markers, the action lists in
   `docs/api/redocly.yaml` (without system actions, because no route may name them), the
   constants and attributes in `kernel/authorize` and the mapping of `operationId` to action in
   `api/routes` ([02-architecture.md](../spec/02-architecture.md), sections 4.5 and 6).
   Kept by hand are the table "Scopes for apps" (05, section 2.6; its rows contain prose) and the
   configuration values of 05 (source of the configuration reference); the check compares them
   with the file. Cells of the receive action that name several cases in words
   (for example "self (agent) and `agent.events.consume`") keep their text; the machine value
   stands next to it and matches `x-receive-action` in asyncapi.yaml.
3. The mapping of action to module is not here but in the module register (ADR 0050). A check
   requires that both files name exactly the same set of actions.
4. New actions are created first in `actions.yaml`, then in the contracts (06, lint).
5. Until build ticket M1-02 delivers the file, 05 is the source and the reconciliation gate G2
   from [06-api.md](../spec/06-api.md), section 15, keeps the copies equal (05, rule 1.3).

### Consequences

- Good, because a new action is created in one place and lint, the server startup check, the route
  test and the docs know it at the same time.
- Good, because the catalogue stays closed: an action that is not in the file does not exist in
  the code (05, rule 1.5).
- Bad, because a generator and a check must be maintained and changes to the tables in 05 go only
  through the YAML file; a manual change in 05 fails in the gate.
- Bad, because the description of an action is in the YAML file and reviewers see two places
  (source and generated table); the diff of the generated table shows the effect.
- Neutral, because 05 stays normative: the generated table is the text, the YAML file its source.

### Confirmation

- `pytest tools/catalog/tests`: a duplicate action ID, an unknown rate-limit class, an event type
  without a receive action, an op without an action and a diverging table block in 05 make the
  check fail; a test proves that the tables in 05 have the same rows before and after the rework.
- `make catalog-check` in the target `gates` (ADR 0038): generating again yields no diff in 05,
  `redocly.yaml`, `kernel/authorize` and `api/routes`.
- `make api-lint`: rules `authz-action-required` and `authz-action-known` with the generated list.
- Server startup check and route test from 05, section 9 (tests 1 and 2), against the generated
  constants.
- Set comparison with the module register (ADR 0050).
- Build ticket M1-02 in [PLAN.md](../PLAN.md).

## Pros and Cons of the Options

### YAML file as the source

- Good, because structured, versioned and readable by all tools.
- Bad, because another format next to Markdown and OpenAPI.

### Markdown tables as the source

- Good, because no second format is created.
- Bad, because tables with running text, references and code spans break when read by a program
  as soon as someone changes the formatting; attributes such as scopes are in other sections.

### Go code as the source

- Good, because the compiler checks the list.
- Bad, because the docs then follow from the code and not the other way round (against ADR 0013),
  and because docs tools would need a Go environment.

### OpenAPI and AsyncAPI extensions as the source

- Good, because the contracts name the actions anyway.
- Bad, because system actions, internal delivery checks (`push.deliver`, `webhook_out.deliver`)
  and actions without a route have no place there (05, rule 1.10 and R-AZ-11); the attributes
  (permission, step-up, audit) would be scattered over many operations.

### Several lists with reconciliation

- Good, because possible right away without a generator; this is the state until M1-02.
- Bad, because every change happens by hand in several places and the reconciliation only
  reports, it does not prevent.

## More Information

- Resolution in the core contract: 05, rule 1.3; [02-architecture.md](../spec/02-architecture.md),
  section 20 ("decided, until then reconciliation in the docs gate").
- Related: ADR 0013 (spec-first), ADR 0017 (one authorization function), ADR 0050
  (module register), ADR 0065 (maintenance mode).
