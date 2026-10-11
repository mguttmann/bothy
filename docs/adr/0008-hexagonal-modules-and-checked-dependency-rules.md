---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Architecture team
informed: all planning teams
---

# Hexagonal modules and checked dependency rules

## Context and Problem Statement

The server is a modular monolith (ADR 0004). Many build sessions work in parallel. Without
enforced boundaries, a module reads other modules' tables, domain code imports database drivers, and
modules depend on each other in a circle. How are modules structured, and how are the rules
kept automatically?

## Decision Drivers

* Business logic testable without infrastructure (B1: hexagonal).
* Drivers replaceable (object storage, identity provider, push, database details).
* A dependency graph without cycles, so that modules stay separable.
* Violations show up in the local gate with file and line, not in the review.
* One source for module boundaries that feeds code, docs and checks.

## Considered Options

* go-arch-lint with a configuration generated from `modules.yaml`
* depguard (in golangci-lint) with rules per file pattern
* Only nested Go `internal` directories
* Custom test with `golang.org/x/tools/go/packages`

## Decision Outcome

Chosen option: "go-arch-lint with a configuration generated from `modules.yaml`", because it
directly expresses components, allowed dependencies and allowed third-party libraries per component
and reported all four kinds of violations in a trial run.

Structure per module (02-architecture.md, section 5): port in the root package, `domain`, `app`,
`adapters/<kind>`. `services/server/architecture/modules.yaml` names tables, actions, allowed
dependencies and third-party libraries per module; a generator produces `.go-arch-lint.yml` from it.

### Consequences

* Good, because violations appear in the gate with file and line.
* Good, because new packages without an entry in the register stand out ("not attached to any
  component").
* Good, because the same source feeds the checks for table ownership, actions and docs
  (02, 7.3).
* Bad, because the generated configuration is large; it is never edited by hand.
* Bad, because another tool must be maintained. Fallback: the generator can produce
  depguard rules from the same register.

### Confirmation

* `go-arch-lint check` in the local gate (ADR 0038); an exit code other than 0 aborts.
* The gate checks that `.go-arch-lint.yml` matches the generator result from `modules.yaml`.
* Self-test of the gate: a skeleton with built-in violations (third-party library in `domain`, port
  of a module that is not allowed, `app` of another module, package without a component) must fail.
* Trial run on 2026-10-10 (Go 1.27.2, go-arch-lint v1.19.0) on a skeleton with all modules:
  clean "No warnings found"; with the four violations four messages, exit code 1.

## Pros and Cons of the Options

### go-arch-lint with generated configuration

* Good, because the configuration (version 3) knows components via path patterns, `mayDependOn`,
  `canUse` for third-party libraries and `commonComponents`. [verified, Q1]
* Good, because the project publishes regularly in 2026 (v1.17.0 on 2026-08-05, v1.18.0 on
  2026-08-21, v1.19.0 on 2026-09-07) and is MIT-licensed. [verified, Q2]
* Bad, because an empty `mayDependOn` is an error; the generator omits such entries
  (found in the trial run).

### depguard with rules per file pattern

* Good, because depguard is part of golangci-lint and knows rules with file patterns, allow and
  deny lists and list modes (`strict`, `lax`). [verified, Q3]
* Bad, because a dependency graph between modules must be written as lists per file pattern;
  packages without a rule do not stand out. [derived]

### Only nested Go `internal` directories

* Good, because the compiler enforces the rule.
* Bad, because `internal` only expresses "private to the parent tree", no allowed graph
  between modules and no rules for third-party libraries. [derived]

### Custom test with go/packages

* Good, because full control.
* Bad, because a custom tool must be built and maintained, which go-arch-lint already is.

## More Information

Sources, retrieved 2026-10-10:

* Q1 https://github.com/fe3dback/go-arch-lint/blob/master/docs/syntax/README.md
* Q2 https://github.com/fe3dback/go-arch-lint/releases and
  https://proxy.golang.org/github.com/fe3dback/go-arch-lint/@latest
* Q3 https://pkg.go.dev/github.com/OpenPeeDeeP/depguard/v2

Planning basis B1. [02-architecture.md](../spec/02-architecture.md), sections 4, 5 and 7.
