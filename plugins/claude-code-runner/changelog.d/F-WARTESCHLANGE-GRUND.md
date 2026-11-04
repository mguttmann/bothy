### Added

- The runner reports its slot use (`run_slots`, used of `runner.max_parallel_runs`) in every
  heartbeat and every run that waits in prepare for a slot (`slots_full`) or for the thread or
  session lock of another turn (`thread_busy`) in `waiting_runs`, with a heartbeat at once when
  a run starts or stops waiting (09 18.2, 5.2.1). The person sees why a run waits and which
  runs hold the slots.
