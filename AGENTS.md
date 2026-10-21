# Rules for AI coding agents

These rules apply to every AI agent that works in this repository, whether it was started
by a maintainer, a script or another agent. They add to [CONTRIBUTING.md](CONTRIBUTING.md);
where the two differ, this file wins for agents.

## This repository is public

- Everything you commit, push, or write into a pull request, an issue or a review comment
  is public and permanent.
- Never add company-internal or private information: names of employers or customers,
  internal host names, IP addresses, private accounts, people other than the example persons
  (Alex, Sam, Robin), billing or plan details, credentials, or anything about the
  machines and tools the maintainers use privately. Use `example.com` and the example
  values described in CONTRIBUTING.md. The project's own public repository location and
  maintainer handles are fine where the repository needs them (for example
  `.github/CODEOWNERS`, `go.mod`, the commit sign-off).
- The white-label scan with the maintainers' private deny list and the secret scan are
  gates, not a replacement for judgement. If you are unsure whether something is
  private, leave it out.

## Language

Everything in the repository is English: code, comments, docstrings, tests,
documentation, specification, ADRs, changelogs, commit messages and pull request texts.
This rule takes precedence over any older instruction that asks for German, for example
in a ticket brief or a review bot comment. The fixed review token `Urteil: JA` stays as it
is.

## How work lands on `main`

- Use a dedicated branch and pull request for each ticket or fix, created from the current
  `main`.
  The ticket briefs live in `docs/briefs/`. This section takes precedence over older
  instructions in `docs/PLAN.md` and the briefs that say to merge locally and never open a
  pull request.
- Conventional Commits with a DCO sign-off (`git commit -s`). Do not add co-author
  trailers for tools or bots.
- Run `make check` before you push. The pre-push hook needs the review report in
  `.review/<sha>.md` (see "Review before every push" in CONTRIBUTING.md). That report is
  written by an independent, read-only reviewer, never by the agent that did the work.
- The required status check is `ci-ok`. Pull requests are squash-merged: the pull request
  title must be a Conventional Commit header, because it becomes the commit on `main`.
- The pull request body becomes the message of that commit on `main`, and a commit on
  `main` is never rewritten. Write title and body fresh, in English, in your own words:
  never paste raw reviewer, bot or agent output (the review report stays in `.review/`;
  name only the reviewed commit, the verdict and the fixes), and no local paths, user or
  machine names, temporary folder names or notes about your tools. The deny-list job in
  CI scans title and body on every edit and fails `ci-ok` on a finding; check the planned
  text before the push with `PR_TITLE=... PR_BODY_FILE=<file> make denylist`.
- CodeRabbit reviews every pull request that is ready for review. Fix its findings and
  comments, or answer them with a reason in the thread, before the pull request is
  merged; all review threads must be resolved. SonarQube Cloud runs inside `ci-ok` once
  product code exists; a failed quality gate is fixed, not answered.
- Never push to `main` directly, never force-push a shared branch, never change
  repository settings, secrets or branch protection.

## Working style

- Reproduce a bug with a failing test before you fix it, and add a test for every change.
- Keep a change inside the scope of its ticket; record rulings in
  `docs/decisions.d/<ticket>.md` and changelog entries in `changelog.d/<ticket>.md` next to the
  component's `CHANGELOG.md`, never in the shared files (docs/quality/gates.md, "Fragments").
- No en dashes or em dashes anywhere; use a hyphen or rephrase.
