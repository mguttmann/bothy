### Fixed

- `TestDemoViews` no longer fails with "step-up challenge: context deadline exceeded" on a slow
  runner. One `testkit.Context` (three minutes) bounded the whole test, so when the demo command
  and the waits for its runs took longer, the context had run out before the terminal step. The
  command and the stack now run on the test's own context (the command bounds its requests and
  waits itself), and the step-up and the terminal open get their own `testkit.Context`.
