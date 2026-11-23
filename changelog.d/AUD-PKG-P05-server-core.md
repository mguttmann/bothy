### Changed

- 06 section 2 records that undeclared query parameters are ignored, 06 section 3.3 that every
  401 carries a challenge, and 02 names all four in-band routes.
- upgrade.md rule M4: one index per migration without a transaction, and a unique index is
  replaced in two migrations.
- The Discord migration guide says that a full import copies messages as they are and should run
  while the agents are idle, and how a full run is stopped.

### Added

- A tools test checks that every sqlc block of the server writes `.gen.go` files and omits unused
  structs, and that no stale generated file is left.
