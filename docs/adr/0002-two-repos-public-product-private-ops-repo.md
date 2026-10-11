---
status: accepted
date: 2026-10-10
decision-makers: Maintainer
---

# Two repos: public product, private ops repo

## Context and Problem Statement

The product should be open source and usable by every organization. The first
installation, however, belongs to a specific organization with its own host names, accounts,
agent assignments, migration data and runbooks. This information must never become public,
not even in the Git history. Where does what live, and how do we prevent
instance knowledge from getting into the product?

## Decision Drivers

* White label: no reference to an instance in the product, not even in examples, tests, commits.
* The instance still needs versioned configuration and runbooks.
* A boundary that a machine can check, not just an agreement.
* The product repository should be able to go public later without a history cleanup.

## Considered Options

* Two repositories: public product, private ops repository
* One repository with private folders that are removed before going public
* One private monorepo from which a public mirror is exported

## Decision Outcome

Chosen option: "Two repositories", because only then does the boundary exist from the moment of
writing, and the product history never contains instance knowledge.

The product repository contains server, apps, SDKs, extensions, docs, CI and examples
with neutral values (example.com, people Alex, Sam, Robin). The ops repository contains
the instance configuration, deployment runbooks, assignments to agent profiles,
migration data with real identifiers and the private deny list with which the
product repository is checked (ADR 0042).

### Consequences

* Good, because the product can go public at any time; the check for this is
  `make denylist-history`.
* Good, because instance knowledge stays versioned, just privately.
* Bad, because a change sometimes needs two pull requests (product feature and
  instance configuration). The configuration reference in the product describes every key,
  the ops repository only sets values.

### Confirmation

The deny-list scanner (`make denylist`, part of `make check` and CI) checks every push
with the list from the ops repository; before going public, `make denylist-history` checks
the whole history (docs/quality/public-switch.md).

## Pros and Cons of the Options

### Two repositories

* Good, because instance knowledge never gets into the product history.
* Good, because access permissions are separate (the ops repository stays private).
* Bad, because two repositories are maintained.

### One repository with private folders

* Good, because everything is in one place.
* Bad, because the history keeps the private folders; before going public a rewrite of
  the whole history would be needed, and every gap is then public.

### Private monorepo with a public mirror

* Good, because one place of work.
* Bad, because the export filter is the only boundary and every filter error leaks.
* Bad, because contributions from outside land on the mirror and must be ported back.

## More Information

Basis: planning basis section 0 and requirement 22 (white label).
