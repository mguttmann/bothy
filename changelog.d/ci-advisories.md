### Changed

- The advisory gate `osv` of a pull request fails only for vulnerable dependencies the pull
  request adds or changes; findings its merge base has too (a new public advisory) print as
  warnings. A push to `main` and the new daily workflow `advisories.yml` still scan everything.
