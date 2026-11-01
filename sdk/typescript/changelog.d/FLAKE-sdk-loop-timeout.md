### Fixed

- The adapter no longer drops the first of two quick answers to one question: when the server
  refuses its `resumeRun` with `superseded` before the newer answer reached the process, the
  adapter waits for that answer, resumes with its token and hands both answers over in order
  (09 4.5). The wait ends when the run is cancelled or the adapter stops.
