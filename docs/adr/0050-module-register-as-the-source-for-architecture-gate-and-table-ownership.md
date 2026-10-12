---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Team A (architecture)
informed: all planning teams
---

# Module register as the source for the architecture gate and table ownership

## Context and Problem Statement

The server is a modular monolith with hexagonal modules (ADR 0004, ADR 0008). Every table
belongs to exactly one module, only this module reads and writes it via SQL
([03-data-model.md](../spec/03-data-model.md), principle 12), and every action belongs to exactly
one module ([02-architecture.md](../spec/02-architecture.md), section 4.1, rule 6). ADR 0008 chose
go-arch-lint as the tool for package dependencies. Still open: which file holds the
facts about modules, which checks are generated from it, and how is table ownership enforced,
which go-arch-lint does not see?

## Decision Drivers

- One source for module boundaries: tables, actions, allowed dependencies, third-party libraries.
- Table ownership is checked, not only described (principle 12, B7: one truth for permissions).
- New packages, tables and actions stand out when they are not assigned to a module.
- The tables in 02 (sections 4.2, 4.4, 4.6) must not diverge from the code.
- Many build sessions work in parallel; violations must show up in the local gate (ADR 0038).
- A Unit of Work spans several modules in one transaction (02, section 8).

## Considered Options

- Register `services/server/architecture/modules.yaml` as the single source; the configuration of
  go-arch-lint and all further checks are generated from it
- Hand-written go-arch-lint configuration plus separate lists in the docs
- Enforce table ownership in PostgreSQL: one schema and one database role per module
- Declarations in Go code per module, read by a tool

## Decision Outcome

Chosen option: "Register `modules.yaml` as the single source", because it holds all module facts
in one readable place and every check in the local gate is generated from it. Format and example:
[02-architecture.md](../spec/02-architecture.md), section 7.1.

1. Per module the register names `tables`, `actions`, `events`, `ephemeral_events`, `ws_ops`,
   `depends_on` and `adapter_vendors`; in addition the kernel packages (`kernel.core`,
   `kernel.ports`), one entry per platform package with `depends_on` and `vendors`, the entries
   `wire`, `cmd` and `api` with their vendor groups and the list `vendors` with the import paths per
   third-party library. Technical tables (River in the schema `jobs.schema`, the version table of
   goose; 03 section 4.13) belong to no module and are not listed under `modules`; a query of a
   module that names them is a finding of the table ownership check.
2. Generated from it are:
    - `.go-arch-lint.yml` with four components per module (port, `domain`, `app`, `adapters`, ADR
      0008), one component per platform package and the components `wire`, `cmd`, `api`; only in
      this way can a third-party library be limited to exactly one platform package (02 section
      7.2);
    - the table ownership check: every SQL file under
      `internal/<module>/adapters/postgres/queries/` names only tables of this module;
    - the completeness check: every table of the migrations and from 03 is in the register exactly
      once, every action of the catalogue exactly once (set comparison with
      `docs/spec/actions.yaml`, ADR 0049);
    - the route check: the handler of an operation lies in the module that owns its action;
    - the direction of the in-tx subscriptions: a module subscribes only to events of modules it
      depends on (golden file in `internal/wire`, 02 section 8.3);
    - the docs reconciliation: the tables 4.2, 4.4, 4.6 and 4.7 in 02 match the register.
3. A new package without a component makes go-arch-lint abort ("not attached to any component");
   this is intended (02, section 7.2).

### Consequences

- Good, because a change of the module boundaries changes exactly one file and all checks follow.
- Good, because table ownership violations stand out before the merge, with file and table,
  instead of in review.
- Good, because completeness is checked against the migrations, that is against the real schema.
- Bad, because the table ownership check reads SQL files and is not PostgreSQL: it does not see a
  table that is reached only through a view or a function. Mitigation: views and functions are
  listed in the register like tables.
- Bad, because the generator and the checks must be maintained; the fallback for go-arch-lint is
  generating depguard rules from the same register (ADR 0008).
- Neutral, because sqlc works per module (`sqlc.yaml`, one entry per module) and the register can
  provide the list of modules for it.

### Confirmation

- `make arch` (go-arch-lint, exit code 0) and `make codegen-check` (generated `.go-arch-lint.yml`
  equal to the generator result) in the target `gates`.
- `go test ./tools/archgen/...` with fixture modules (PLAN M1-05, form with platform components):
  `TestArchLintSkeletonClean` (clean skeleton, exit code 0), `TestArchLintDomainImportsDriver`
  (domain imports a driver), `TestArchLintAppImportsForeignPort` (app imports a foreign port),
  `TestArchLintAppImportsForeignApp` (app imports a foreign app),
  `TestArchLintPackageWithoutComponent` (package without component); the four violations fail with
  exit code 1 and the package path ([02-architecture.md](../spec/02-architecture.md), section 7.2).
  `TestArchLintTestSupport`: `test/**` may use everything, no production component may use it.
  `TestArchLintChapter02Layout`: the layout from 02 with several packages per component (`sqlcgen`,
  `pgtest`, subpackages, `api/routes` calls `api/rest`) passes; `TestArchLintLayersStayClosed`:
  the self entries open no other edge; `TestArchLintVendorPatternIsLiteral`: a vendor pattern
  allows only the named libraries.
- Generator: `TestGenerateGolden`, `TestGenerateIsDeterministic`, `TestRegisterRejects` (one case
  per check rule of the register), `TestImportPathRule` (vendor patterns only as a Go import path),
  `TestRunRejectsExcludePrefixOfOtherDirectories` (`exclude` acts as a string prefix),
  `TestRepositoryRegisterIsValid` and
  `TestRepositoryConfigIsCurrent` (the checked-in `.go-arch-lint.yml` equals the generator result
  from `architecture/modules.yaml` byte for byte), `TestChapter02ExamplesMatchGenerator`
  (register example and generated excerpt in 02, sections 7.1 and 7.2, match the generator).
- Fixture for table ownership: a query in module A names a table of module B; the
  check fails with file and table.
- Completeness check against the migrations, 03 and `actions.yaml`; docs reconciliation with 02.
- Build ticket M1-05 in [PLAN.md](../PLAN.md).

## Pros and Cons of the Options

### Register as the single source

- Good, because people and tools (Go generator, Python docs checks) read the same file.
- Bad, because the generated configuration is large; it is never edited by hand.

### Hand-written configuration and separate lists

- Good, because no generator is needed.
- Bad, because three lists (tool, 02, 03) drift apart and table ownership is not checked at
  all.

### Schema and role per module in PostgreSQL

- Good, because the database rejects another module's access at runtime.
- Bad, because a Unit of Work runs several modules in one transaction on one connection
  (02, section 8.1); roles per module would need `SET ROLE` before every statement or separate
  connections and thus separate transactions [derived].
- Bad, because the phase hooks (permission version, event log, audit) write other modules' tables
  in every transaction; they would need exceptions that weaken the rule again.

### Declarations in Go code

- Good, because they stand next to the code.
- Bad, because the facts are spread over many files and docs tools would have to evaluate Go
  source code.

## More Information

- [02-architecture.md](../spec/02-architecture.md), sections 4, 6 and 7; decision R-AR-19.
- Related: ADR 0004 (modular monolith), ADR 0007 (sqlc and migrations), ADR 0008 (hexagonal
  modules, go-arch-lint), ADR 0049 (action catalogue).
