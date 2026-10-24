# SPDX-License-Identifier: Apache-2.0
"""License step of `make licenses`: fail only for dependency licenses outside the allowlist.

OSV-Scanner has no mode that checks licenses alone: `scan source --licenses=...` also matches
every package against the advisory database and exits 1 for a vulnerability. Called directly,
the license step then failed every pull request whenever a new public advisory hit a lock
file, although the advisory gate `osv` had already let the merge base's findings pass
(docs/DECISIONS.md PL/25 and PL/32). This tool runs the same scan in JSON mode and judges only
`license_violations`; vulnerabilities stay the business of `make osv`.

A scan that cannot decide never passes: an exit code other than 0 or 1, a report that is not
JSON, and a failing scan whose report names neither a license violation nor a vulnerability.

Usage:
  dep_licenses.py --osv-scanner PATH --licenses LIST -L LOCK [-L LOCK]...
Exit codes: 0 every license allowed, 1 a violation or an error.
"""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
from typing import Any


class GateError(Exception):
    """The scanner failed or printed no usable report; the gate cannot decide and fails."""


def scan(osv: str, allowed: str, locks: list[str]) -> tuple[int, dict[str, Any]]:
    """Run OSV-Scanner with the allowlist in JSON mode; its exit code (0 or 1) and report."""
    args = [osv, "scan", "source", "--format", "json", f"--licenses={allowed}"]
    for lock in locks:
        args += ["-L", lock]
    done = subprocess.run(args, check=False, capture_output=True, text=True)
    sys.stderr.write(done.stderr)
    if done.returncode not in (0, 1):
        raise GateError(f"osv-scanner exited with {done.returncode}")
    if not done.stdout.strip():
        if done.returncode == 0:
            return 0, {}
        raise GateError("osv-scanner failed and printed no report")
    try:
        report = json.loads(done.stdout)
    except json.JSONDecodeError as err:
        raise GateError(f"osv-scanner printed no JSON: {err}") from err
    if not isinstance(report, dict):
        raise GateError("osv-scanner printed no JSON object")
    return done.returncode, report


def judge(rc: int, report: dict[str, Any]) -> list[str]:
    """Error lines for the license violations of the report; vulnerabilities do not count."""
    errors: list[str] = []
    vulnerable = 0
    for result in report.get("results") or []:
        src = (result.get("source") or {}).get("path", "")
        for pkg in result.get("packages") or []:
            if pkg.get("vulnerabilities"):
                vulnerable += 1
            violations = pkg.get("license_violations") or []
            if violations:
                p = pkg.get("package") or {}
                errors.append(
                    f"license not allowed: {src}: {p.get('ecosystem', '')} {p.get('name', '')} "
                    f"{p.get('version', '')}: {', '.join(violations)}"
                )
    if rc == 1 and not errors and not vulnerable:
        errors.append("osv-scanner failed, but its report names no license violation")
    if vulnerable and not errors:
        print(
            f"licenses: {vulnerable} package(s) with a known vulnerability; "
            "advisories are judged by `make osv`, not by the license step"
        )
    return errors


def main(argv: list[str] | None = None) -> int:
    """Command line entry; see the module docstring."""
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--osv-scanner", required=True)
    parser.add_argument("--licenses", required=True)
    parser.add_argument("-L", dest="locks", action="append", default=[], required=True)
    args = parser.parse_args(argv)
    if not args.licenses.strip(","):
        errors = ["licenses: the allowlist is empty, so --licenses would not limit anything"]
    else:
        try:
            errors = judge(*scan(args.osv_scanner, args.licenses, args.locks))
        except GateError as err:
            errors = [str(err)]
    for line in errors:
        print(f"::error::{line}")
    return 1 if errors else 0


if __name__ == "__main__":
    sys.exit(main())
