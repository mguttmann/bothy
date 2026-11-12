### Added

- Configuration key `auth.argon2id_max_concurrent`: the Argon2id computations of password checks
  that run at the same time, each with `auth.argon2id_memory_kib` of memory. Further password
  checks wait for a free one; a login that waits past its request deadline answers
  `503 unavailable`.
- Migration `00104` builds indexes for the retention jobs concurrently: the end time of
  `refresh_tokens`, `auth_sessions`, `devices`, `terminal_sessions` and of the messages that
  still hold content, and the foreign keys to `devices` of the personal sync tables and of
  `push_registrations`. A batch of a retention job and the deletion of a device no longer read
  whole tables.

### Security

- Setting up a passkey (`passkey.register`) or a password with TOTP (`local_credential.set`)
  needs a step-up from every session that can confirm one, also for the first passkey or
  password: on a device with a step-up key, in the web console, and for a person who already has
  a passkey or a password. A `cli` device sets up none. Before, a session without a step-up could
  set up the first passkey or password and then confirm its own step-up with it or log in to the
  console. Only a person without any sign-in data on a device without a step-up key still sets
  up the first one with the session alone. Clients answer the `401 step_up_required` with a
  step-up and repeat the request.
- A console step-up by a new OIDC login is refused when the ID token has no `auth_time`; the
  time the token was issued no longer stands in for it. A provider that ignored `prompt=login`
  could confirm a step-up with a silent session before.
- Parallel password logins no longer hold unbounded memory (see `auth.argon2id_max_concurrent`).
