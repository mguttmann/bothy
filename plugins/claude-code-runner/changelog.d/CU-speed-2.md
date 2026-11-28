### Changed

- The system prompt section `## Computer use` (docs/spec/27-computer-use.md 10.3) asks for one
  `batch` up to the next decision that needs a look at the screen (for example a whole form with
  Tab between the fields), a `wait` with the longest time the step may need as its last item,
  since the host ends it on a quiet screen, and keys instead of the pointer in every program.
