# FIX-DEP-LICENSES-FAILOPEN A failing license scan passes only with a finding that explains it

Follow-up to AUD-ROOT-01/2 (docs/decisions.d/AUD-root-01.md). An automated security review of
that change reported that the dependency license step of `make licenses` excuses a failed run of
OSV-Scanner as soon as a Gradle lock file lists any package that does not ship.

The report was checked at the source. It holds for the code and not for a run of the pinned
scanner:

* The code: since AUD-ROOT-01 `judge` in `tools/ci/dep_licenses.py` failed an exit code 1 without
  a finding only when no package had been set aside as not shipping (`... and not dropped`). The
  build and test packages of a Gradle lock file alone, all under allowed licenses and without an
  advisory, turned a scan that cannot decide into a pass. A test with such a lock file and a
  forced exit code 1 passed the step.
* The pinned scanner: OSV-Scanner 2.6.0 exits 1 only for a vulnerability, a license violation or
  a deprecated package (`determineReturnErr`,
  [osvscanner.go](https://github.com/google/osv-scanner/blob/v2.6.0/pkg/osvscanner/osvscanner.go#L374-L413)),
  ends its own failures with 127 to 130
  ([run.go](https://github.com/google/osv-scanner/blob/v2.6.0/cmd/osv-scanner/internal/cmd/run.go#L114-L136)),
  which the step refuses before it reads a report, and names in its report only packages with one
  of these findings unless `--all-packages` is set
  ([filter.go](https://github.com/google/osv-scanner/blob/v2.6.0/pkg/osvscanner/filter.go#L20);
  all three retrieved 2026-11-12). The step sets neither `--all-packages` nor the flag for
  deprecated packages, so every package it set aside came with a finding that explained the exit
  code: a run of the pinned scanner over the tracked lock files of the apps on 2026-11-12 named no
  package without a violation or a vulnerability. No run of the gate is known to have passed
  wrongly.

So the gap was latent: it opens with a scanner release or an argument that lists packages
without a finding, or with anything between the scanner and the step that exits 1.

| No | Date | Ruling | Reason |
|---|---|---|---|
| FIX-DLF/1 | 2026-11-12 | In the license step a package of a Gradle lock file that does not ship explains an exit code 1 of the scanner only by a license violation or a vulnerability of its own. An exit code 1 that no package of the whole report explains, shipped or not, fails with `osv-scanner failed, but its report names no license violation`, as it does for every other lock file since PL/32. Nothing else changes: a violation of a package that does not ship is still counted in one line per lock file and does not fail, and a vulnerability still fails only `make osv`. | PL/32 rules that a scan that cannot decide never passes, and the step must not depend on what a scanner release happens to list. The scanner exits 1 for a finding of any package it read, so the exemption AUD-ROOT-01/2 needs is "the finding sits in a package that does not ship", not "the lock file has such packages". `test_a_failing_scan_without_a_finding_fails_next_to_packages_that_do_not_ship` in `tools/ci/tests/test_dep_licenses.py` failed before the change (the step returned 0) and passes now; `test_an_advisory_of_a_build_or_test_package_alone_passes` holds the case the exemption is for, and the run over the tracked lock files above passes the step as before. |
| FIX-DLF/2 | 2026-11-12 | `tools/ci/advisories.py` stays as it is. | It never had the gap: `shipped_findings` counts, per lock file, only the packages set aside that have a failing finding, and `osv_gate` and `osv_full` fail an exit code 1 when neither a shipped finding nor such a count explains it. |
