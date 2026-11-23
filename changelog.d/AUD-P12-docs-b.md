### Added

- Plan rows V3-46 to V3-61 for the parts that the specification and the guides promised but that
  are not built or not proven (among them a rehearsed disaster recovery, service operation of the
  runner and the adapters, data protection procedures, the handover of hosted turns, the
  reassignment of the search language, the operations view of the search, the steps after
  disabling a person, version signals from the first release, the reconcile view for costs and
  scheduled fuzzing).
- A guide for setting further server keys in the Compose stack through `compose.override.yaml`,
  a section on the search assistant in the Compose guide and a troubleshooting page.
- Tests that hold documents to the built artifacts: the `secrets init` commands of the guides
  against the Compose file, the test IDs of the specification against the test files (with a
  list of known gaps that may only shrink) and the CI job table of gates.md against `ci.yml`.

### Fixed

- "View as person" in the administration closes itself after a cancelled step-up instead of
  showing an empty frame.
- The specification, the operations guides and the compliance templates describe what is built:
  the content of "View as person", the push kinds and badge, the hosted turns at a restart, the
  search language job and dashboard, the steps after `user.disable`, the status of chapters 04
  and 15, the audit of the co-determination switches, `Location` and the cross-route headers,
  the version signals before the first release, the reconcile button, the performance targets,
  the restore runbooks, the import cancel, the ScanCode exceptions of the fonts and the
  definition of "The Bothy Authors".
