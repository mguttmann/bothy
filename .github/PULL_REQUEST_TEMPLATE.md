## What and why

<!-- One finished feature or fix per pull request. Link the issue: Closes #123 -->

## How it was verified

<!-- Commands you ran and what they showed. `make check` is required. -->

- [ ] `make check` passes locally (the same gates as CI; docs/quality/gates.md)
- [ ] Tests cover the change; a bug fix comes with a test that failed before the fix
- [ ] Commits follow Conventional Commits and carry a DCO sign-off (`git commit -s`)
- [ ] Commits are squashed into meaningful steps (no "fix CI" or "typo" commits)

## Review report

<!-- Paste the full report of the independent review (`.review/<sha>.md`, CONTRIBUTING.md
"Review before every push"). The folder is git-ignored, so the text goes here. Name the
commit it covers; its last line is `Urteil: JA`. -->

- [ ] The report above covers the head commit of this pull request

## Contract and documentation

- [ ] Public contract changes (API, events, SDK, configuration) are reflected in
      `docs/api/` and the specification, or this pull request changes none
- [ ] New or changed limits are named configuration values, not numbers in prose
- [ ] A decision with lasting effect has an ADR in `docs/adr/`
- [ ] `CHANGELOG.md` of every affected component has an entry under `Unreleased`

## Security and privacy

- [ ] No secrets, no real personal data, nothing instance-specific (names, hosts, IDs)
- [ ] Every new route, event or delivery goes through `authorize` with a catalogued action
