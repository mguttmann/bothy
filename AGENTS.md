# Rules for AI coding agents

These rules apply to every AI agent that works in this repository, whether it was started
by a maintainer, a script or another agent. They add to [CONTRIBUTING.md](CONTRIBUTING.md);
where the two differ, this file wins for agents.

## This repository is public

- Everything you commit, push, or write into a pull request, an issue or a review comment
  is public and permanent.
- Never add company-internal or private information: names of employers or customers,
  internal host names, IP addresses, accounts, people other than the example persons
  (Alex, Sam, Robin), billing or plan details, credentials, or anything about the
  machines and tools the maintainers use privately. Use `example.com` and the example
  values described in CONTRIBUTING.md.
- The white-label scan with the maintainers' private deny list and the secret scan are
  gates, not a replacement for judgement. If you are unsure whether something is
  private, leave it out.

## Language

Everything in the repository is English: code, comments, docstrings, tests,
documentation, specification, ADRs, changelogs, commit messages and pull request texts.
This replaces the older rule in CONTRIBUTING.md that planning documents are German.

## How work lands on `main`

- One branch and one pull request per ticket or fix, created from the current `main`.
  The ticket briefs live in `docs/briefs/`.
- Conventional Commits with a DCO sign-off (`git commit -s`). Do not add co-author
  trailers for tools or bots.
- Run `make check` before you push; the pre-push hook needs the review report in
  `.review/<sha>.md` (see "Review before every push" in CONTRIBUTING.md).
- The required status check is `ci-ok`. Pull requests are squash-merged.
- SonarQube Cloud and CodeRabbit review every pull request. Fix their findings and
  comments, or answer them with a reason in the thread, before the pull request is
  merged; all review threads must be resolved.
- Never push to `main` directly, never force-push a shared branch, never change
  repository settings, secrets or branch protection.

## Working style

- Reproduce a bug with a failing test before you fix it, and add a test for every change.
- Keep a change inside the scope of its ticket; record rulings in `docs/DECISIONS.md`.
- No en dashes or em dashes anywhere; use a hyphen or rephrase.
