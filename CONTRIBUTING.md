# Contributing to Bothy

Thank you for helping. This guide covers the development setup, the gates every change
passes, and how commits, pull requests and decisions work. The detailed rules live in
[docs/quality/](docs/quality/README.md).

## Development setup

You need git, GNU make, [uv](https://docs.astral.sh/uv/) and Node.js (an LTS release;
`make bootstrap` checks the minimum version). Working on the server needs Go, working on
the apps needs Flutter; their versions come from `services/server/go.mod` and the app's
`pubspec.yaml`. A Docker-compatible runtime is only needed for the local SonarQube.
Windows contributors use WSL.

```sh
git clone https://github.com/OWNER/bothy.git
cd bothy
make bootstrap   # pinned tools, git hooks (pre-commit, commit-msg, pre-push)
make check       # every gate, exactly as CI runs it
```

`make help` lists all targets. Gates of a component that has no code yet print a `SKIP`
line; a missing tool is an error, never a skip.

## The gates

`make check` runs formatting, linters, type checks, all tests with coverage thresholds,
the documentation build, link and diagram checks, the API lint, the workflow policy
check, secret and white-label scans, Semgrep, Trivy, OSV-Scanner, the license gate and
SonarQube. CI runs the same make targets, so a green `make check` means a green pull
request. The list with every threshold: [docs/quality/gates.md](docs/quality/gates.md).

The pre-push hook runs `make check`. Do not bypass it with `--no-verify`.

Maintainers additionally run the white-label scan with the project's private deny list.
Contributors from forks do not have that list: set `export DENYLIST_FLAGS=--allow-missing`
in your shell, so that `make check` runs the public generic rules only. If you do not run
the local SonarQube, also set `export SONAR_SKIP=1`. CI checks fork pull requests the same
way, and a maintainer runs the full scan and SonarQube before merging.

## Branches and commits

- Work on a branch named `<type>/<short-topic>`, for example `feat/thread-move` or
  `fix/reconnect-cursor`. Never commit to `main` directly; it is protected.
- Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/):
  `type(scope): description` with the types `feat`, `fix`, `docs`, `refactor`, `perf`,
  `test`, `build`, `ci`, `chore`, `style`, `revert`. Mark breaking changes with `!`.
- Every commit carries a Developer Certificate of Origin sign-off. Commit with
  `git commit -s`; the line `Signed-off-by: Your Name <you@example.com>` must match the
  commit author. By signing off you certify the
  [Developer Certificate of Origin 1.1](https://developercertificate.org/). There is no
  separate contributor license agreement; contributions are licensed under Apache-2.0.
- Before you push, squash your work into meaningful commits (`git rebase -i main` in your
  terminal). Commits such as "fix CI" or "typo" do not belong in `main`.

The commit-msg hook checks format and sign-off; CI checks them again for the whole pull
request.

## Pull requests

- One pull request per finished feature or fix. Draft pull requests are welcome for
  early feedback.
- Fill in the template. Describe what you verified and how.
- The required status check is `ci-ok`. Maintainers merge with rebase or squash, so the
  history of `main` stays linear.
- Reviews from automated reviewers (CodeRabbit, SonarQube Cloud) are part of the review.
  If one of them finds something the local gates missed, we add a local rule so that it
  is caught before the next push ([push discipline](docs/quality/push-discipline.md)).

## Documentation is part of the change

- A change to behaviour, an API, an event, a permission or a configuration value updates
  the specification under `docs/spec/` and `docs/api/` in the same pull request.
- Limits, time windows and thresholds are named configuration values with their default
  in the chapter's table; prose uses the name, not the number.
- Planning and specification documents are written in German, community files and code
  comments in English. Identifiers are English everywhere.
- No en dashes or em dashes anywhere (the `text-rules` hook rejects them); use a hyphen
  or rephrase.
- Example credentials in documentation and tests contain the word `EXAMPLE` (for example
  `whsec_EXAMPLE...`), so that the secret scan can tell them from real ones. Identifiers
  in examples are UUIDs, people are Alex, Sam and Robin, domains are `example.com`.
- Diagrams are Mermaid sources (`.mmd`) with the rendered `.svg` next to them, or
  ```` ```mermaid ```` blocks in Markdown; `make diagrams` renders all of them.
- Every component has a `CHANGELOG.md` in [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
  format; add your entry under `Unreleased`.

## Architecture decisions

Decisions with lasting effect are recorded as ADRs in `docs/adr/` (MADR 4, ADR 0001):

1. Copy `docs/adr/0000-vorlage-madr-4.md` to the next free number,
   `NNNN-title-in-kebab-case.md`.
2. Fill in context, drivers, at least two real options, the outcome and its
   confirmation (the test or check that keeps the decision true). Status `proposed`.
3. Open a pull request; the discussion happens there. The merged ADR is `accepted`.
4. Run `make adr-index` to update the index.

Smaller rulings go into `docs/DECISIONS.md`.

## Reporting security issues

See [SECURITY.md](SECURITY.md). Never in a public issue.

## Code of conduct

Everyone in this project follows the [Code of Conduct](CODE_OF_CONDUCT.md).
