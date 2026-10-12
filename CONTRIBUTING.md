# Contributing to Bothy

Thank you for helping. This guide covers the development setup, the gates every change
passes, and how commits, pull requests and decisions work. The detailed rules live in
[docs/quality/](docs/quality/README.md).

## Development setup

You need git, GNU make, [uv](https://docs.astral.sh/uv/) and Node.js (an LTS release;
`make bootstrap` checks the minimum version). Working on the server needs Go, working on
the apps needs Flutter; their versions come from `services/server/go.mod` and the app's
`pubspec.yaml`. A Docker-compatible runtime is needed for the local SonarQube and for server
work: integration tests run against PostgreSQL in throwaway containers, and `make perf` runs
the load tests. As soon as a Go module exists, every server gate fails without Go, a
Docker-compatible runtime or the pinned tools; there is no skip. Windows contributors use WSL.
Work on the server image (`make image`, `make image-test`) needs Docker with the buildx plugin
and a builder that builds several platforms: Docker Engine with the containerd image store does,
otherwise create one with `docker buildx create --name <name> --driver docker-container` and
pass `IMAGE_BUILDER=<name>` (services/server/README.md, section "Image").

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

The pre-push hook first checks the review evidence of the pushed commit (see "Review
before every push"), then runs `make check`. Do not bypass it with `--no-verify`.

Maintainers additionally run the white-label scan with the project's private deny list.
Contributors from forks do not have that list: set `export DENYLIST_FLAGS=--allow-missing`
in your shell, so that `make check` runs the public generic rules only. If you do not run
the local SonarQube, also set `export SONAR_SKIP=1`, and without an independent reviewer
`export REVIEW_SKIP=1`. CI checks fork pull requests the same way; a maintainer runs the full
scan, SonarQube and the review before merging, and every push to `main` is scanned again
with the private list (`denylist-main.yml`).

## Database migrations

Server migrations are plain SQL files run by goose (ADR 0007). Create the next free number of
the base branch as `migrations/NNNNN_<slug>.sql` (five digits, a lower-case slug) and run
`make migrations-lock`; it records the checksum of the new file in
`<module>/migrations.sha256`. Never edit or delete a migration that is merged into the base
branch; write a new one. If another branch took your number first, renumber your file, remove
your own line from `migrations.sha256` and lock again before your final `make check`. A
timestamp version is fine for a draft, but `make migrations-lint` rejects it in every run.
Details: [docs/quality/gates.md](docs/quality/gates.md#migrations).

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

## Review before every push

Before you push, an independent, read-only reviewer (a fresh AI review session without the
context of the work, or a second person) checks the diff of your branch against `main`
following [docs/quality/red-team-gate.md](docs/quality/red-team-gate.md). The report goes to
`.review/<sha>.md`, where `<sha>` is the commit you push (`git rev-parse HEAD`); the folder
is git-ignored and the report stays local: in the section "Review" of the pull request,
name the reviewed commit and the verdict in your own words, never the raw report (the body
becomes the commit message on `main`). The last line of the report is `Urteil: JA` or
`Urteil: NEIN <reason>`. The pre-push hook refuses a push without a report or with any other
last line. If you change the commit after the review, the reviewer checks the delta and
writes the report for the new commit. `make review` creates the folder, prints the expected
file name and, with `BOTHY_CODERABBIT=1`, also runs the CodeRabbit CLI.

## Pull requests

- One pull request per finished feature or fix. Draft pull requests are welcome for
  early feedback.
- Fill in the template. Describe what you verified and how. Title and body become the
  squash commit on `main`: write them fresh, without pasted tool output, local paths or
  host names. CI scans both with the deny list and runs again when you edit them; check
  the planned text before the push with `PR_TITLE=... PR_BODY_FILE=<file> make denylist`.
- The required status check is `ci-ok`. Maintainers merge with squash only, so the history
  of `main` stays linear and every pull request is one commit.
- Reviews from automated reviewers (CodeRabbit, SonarQube Cloud) are part of the review.
  If one of them finds something the local gates missed, we add a local rule so that it
  is caught before the next push ([push discipline](docs/quality/push-discipline.md)).

## Documentation is part of the change

- A change to behavior, an API, an event, a permission or a configuration value updates
  the specification under `docs/spec/` and `docs/api/` in the same pull request.
- Limits, time windows and thresholds are named configuration values with their default
  in the chapter's table; prose uses the name, not the number.
- All repository content is English (D-W-94; terms in [docs/glossary.md](docs/glossary.md)).
- Every public module, class, function and interface has a doc comment; the linters of each
  language enforce it (ruff `D` for Python, see chapter 23 "Documentation in code").
- No en dashes or em dashes anywhere (the `text-rules` hook rejects them); use a hyphen
  or rephrase.
- Example credentials in documentation and tests contain the word `EXAMPLE`, so that the
  secret scan can tell them from real ones. The marker exempts only formats that GitHub
  secret scanning does not know (Bothy's own tokens, generic key-like values): GitHub
  ignores the word, so an example for a third-party provider's credential stays shorter than
  that provider's format (for example `whsec_EXAMPLE`), and a test that needs a full-length
  sample assembles it at runtime. gitleaks (for the formats its rules know) and the
  `secret-formats` hook (`whsec_`) reject such values; not every GitHub pattern is covered
  ([what the gates do not cover](docs/quality/gates.md#what-the-gates-do-not-cover), with
  GitHub's [supported patterns](https://docs.github.com/en/code-security/secret-scanning/introduction/supported-secret-scanning-patterns), retrieved 2026-10-11).
  Identifiers in examples are UUIDs, people are Alex, Sam and Robin, domains are
  `example.com`.
- Diagrams are Mermaid sources (`.mmd`) with the rendered `.svg` next to them, or
  ```` ```mermaid ```` blocks in Markdown; `make diagrams` renders all of them.
- Every component has a `CHANGELOG.md` in [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
  format; add your entry under `Unreleased`.

## Architecture decisions

Decisions with lasting effect are recorded as ADRs in `docs/adr/` (MADR 4, ADR 0001):

1. Copy `docs/adr/0000-template-madr-4.md` to the next free number,
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
