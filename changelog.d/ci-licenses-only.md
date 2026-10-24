### Fixed

- `make licenses` fails only for dependency licenses outside the allowlist, no longer for a
  known vulnerability that OSV-Scanner reports in the same scan; advisories are judged by the
  `osv` gate alone, so a new public advisory no longer turns the license step red on every pull
  request.
