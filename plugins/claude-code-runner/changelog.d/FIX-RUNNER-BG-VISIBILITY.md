### Fixed

- Background mode: a Bash command the session runs with `run_in_background` is a
  `background_job` step of the run under its call, titled with the command's first line, until
  its notification ends it or the session is idle; between turns `/metrics/background_jobs`
  counts the open commands. Before, Live showed the run card without a step for it.
- Background mode: an answer after a request without the stored login while the run waits for
  its account after a lost login is withheld and the wait goes on, instead of stopping the
  session with its background work and ending the run `billing_mode_mismatch`. Nothing of such
  an answer is posted; every other output without a proven login still ends the run.
