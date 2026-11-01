### Fixed

- CI: the stub PostgreSQL of the pgtest watchdog tests keeps a SIGINT that arrives right after it
  reported ready. Perl noticed such a signal only when its 60-second sleep ended, so the watchdog
  gave up after its stop wait and started the next server, and the restart test failed once with
  no fast shutdown request before the new start (issue #238).
