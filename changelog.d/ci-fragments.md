### Changed

- Changelog entries and ticket rulings are fragments, one file per ticket:
  `changelog.d/<ticket>.md` next to each component's `CHANGELOG.md` and
  `docs/decisions.d/<ticket>.md`. `make release-prep` assembles a component's fragments under
  "Unreleased" and deletes them, `make changelog-preview COMPONENT=<name>` shows the result, and
  the docs site lists the ruling files on the generated page "Ruling log: tickets". The gate
  `fragments-check` refuses malformed fragments, ticket sections in `docs/DECISIONS.md` and, in
  a pull request, entries added to a `CHANGELOG.md` directly. The ticket sections of the
  ruling log moved unchanged into `docs/decisions.d/`.
