### Fixed

- The cost of a running run grows while its turn works. The runner patches `/cost` after every
  model call with the call's token usage priced from the server's start prices and settles on the
  CLI's exact amount at every turn end; before, a run showed `0.00` until it finished. The booked
  costs of `run.finish` are unchanged.
