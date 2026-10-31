### Fixed

- The Windows app connects to an instance again: it no longer requires a step-up key, which the
  Windows device key plugin never creates by design. The device registers with its DPoP key
  alone, without `step_up_key`; a step-up with the device key on such a device fails with
  `StepUpUnavailable` before any request instead of a null error. Before, every "Connect" on
  Windows ended with "This device's keychain refused the app's keys".
- The E1 texts for a refused keychain and for local data that could not be opened no longer
  point to an app log, which a release build does not keep.
