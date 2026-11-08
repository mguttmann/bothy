### Changed

- The Hermes Agent pin follows the newest upstream main commit instead of release tags, without
  a minimum age (D-HERMES-PIN-1). A daily workflow proposes the next main commit as one pull
  request, recorded again and tested against it (D-HERMES-PIN-2); Renovate no longer tracks the
  pin.
