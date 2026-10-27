### Fixed

- `TestConformance3/AG-23/answer` and `AG-23/ignore` no longer fail with "the call suggest did
  not answer" on a loaded CI runner. The first human call of a case built the response validator
  over the whole contract, a one-time build that is slow under `-race` and slower still in a CI
  shard that runs the four conformance parts at once; in AG-23 that first call is async, so the
  build ran inside the patience of its await. The conformance groups now build the validator
  before any case starts, and `startCase` fails at once when it is missing.
