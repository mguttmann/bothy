---
status: accepted
date: 2026-10-10
decision-makers: Maintainer
---

# Local gates first: one command identical to CI

## Context and Problem Statement

Every error should show up before the push: "fix CI" commits on `main` count as
process errors. How do we organize the checks so that the same thing happens locally and
in CI?

## Decision Drivers

* Locally green means CI green; no second list of checks that drifts apart.
* Fast feedback on commit, complete check before the push.
* Reproducible: the same tool versions on every machine and in CI.

## Considered Options

* `make check` as the only gate command, hooks and CI call the same make targets
* CI first: GitHub-hosted jobs on every push, locally only partial checks
* Only local, without CI
* pre-commit as the only gate system, CI calls `pre-commit run --all-files`

## Decision Outcome

Chosen option: "`make check` as the only gate command", because then local and CI are equal by
construction: `check` consists of `gates`, `denylist` and `sonar`,
and the three CI jobs call exactly these targets. pre-commit provides the fast
file checks and calls `make check` before the push; `make lint` runs the pre-commit hooks
over all files, so that no check exists only in the hook.

### Consequences

* Good, because a developer knows exactly one command and CI finds nothing new.
* Good, because tools are pinned (uv lock, npm lock, sha256 lock for binaries).
* Bad, because `make check` takes minutes before every push. That is intended: pushes are
  rare (one pull request per finished feature).
* Bad, because components without code skip today. That is visible (`SKIP` line)
  and ends automatically with the first code.

### Confirmation

* `Makefile`: `check: gates denylist sonar`; `.github/workflows/ci.yml` calls
  `make gates`, `make denylist`, `make sonar`.
* `.pre-commit-config.yaml`: pre-push hook `make check`.
* `tools/ci/check_runners.py` (gate `runners-check`) forbids `push` triggers on
  branches, requires `timeout-minutes` and fixed GitHub-hosted runners (ADR 0039).

## Pros and Cons of the Options

### `make check` as the only gate command

* Good, because there is one definition for local and CI.
* Good, because make is available everywhere and the component detection lives in the Makefile.
* Neutral, because Windows contributors need WSL.

### CI first

* Good, because nothing has to be installed locally.
* Bad, because errors show up only after the push.

### Only local, without CI

* Good, because it is simple.
* Bad, because outside contributions and releases would stay unchecked and no
  required check protects `main`.

### pre-commit as the only system

* Good, because it is a widespread tool.
* Bad, because tests, docs build, SonarQube and component logic fit poorly into
  file hooks.

## More Information

Gate list: docs/quality/gates.md. Push rules: docs/quality/push-discipline.md.
