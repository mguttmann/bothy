### Added

- The API contract gate `make api-checks` has the rule `client-routes`: every operation the
  Python SDK and the Python plugins call must exist in `openapi.yaml`, be enabled on the server
  and be registered in its route table.
- The CI job `test-adapters-e2e` runs the end-to-end suites of the Hermes adapter and the Claude
  Code runner against the server of this tree; `ci-ok` requires it whenever the SDK, a plugin,
  the server or the API contract changes.
