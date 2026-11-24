### Changed

- The preview of a new built-in agent (desktop and web console) says where each secret goes on the
  runner host: an `env:NAME` secret is the file `NAME` (mode `0600`) in
  `<runner.hosted_secrets_dir>/<agent ID>/`, filled once the agent exists.
