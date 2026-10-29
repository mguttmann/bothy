### Fixed

- The installation and backup runbooks name commands that exist: the first backup is
  `bothy-backupd backup pg_full` and the first restore test `bothy-backupd restore-test`, and a
  point-in-time restore runs `pgbackrest restore` through the service `postgres`. A test keeps
  every documented `bothy-backupd` command in line with the program's usage text.
