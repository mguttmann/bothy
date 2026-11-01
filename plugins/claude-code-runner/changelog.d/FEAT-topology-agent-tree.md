### Added

- The agents of a workflow run appear as child steps of the workflow step, with their label,
  phase, current tool and usage, from the `workflow_progress` of the stream; the prompt and
  result previews never leave the runner (09 18.4).

### Changed

- Foreground commands of workflow agents no longer add a background job step at the root of the
  run.
