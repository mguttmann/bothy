# SPDX-License-Identifier: Apache-2.0
"""Every test ID of the specification is named in a test, or stands in a ratchet list.

The chapters under docs/spec and docs/security define test IDs in the first column of their test
tables (AR-T, BA-T, DEV-T, DP-T, ID-T, SEC-T, TM-T, VZ-T, and AZ-M and AZ-P of the permission
matrix). Only the terminal catalogue had a gate that a test exists for an ID
(tools/docs/terminal_catalogue.py); for the other families nothing tied an ID to a test, and some
were tested under another name or not at all (audit finding testing-16).

A test names an ID when a tracked test file (a Go, Dart, Python, TypeScript, Swift or Kotlin test,
or any file under a `test`, `tests` or `integration_test` folder) contains it, also in the
compact form of a Go test name (`TestAZM02`) or with an underscore (`ID_T30`). Whether the test
passes is the job of the test suites, not of this check.

The IDs that no test names today stand in KNOWN_UNNAMED with their reason (ratchet): a new ID must
be named by a test, and an entry whose ID a test names now, or that the spec no longer defines,
fails until it is removed.
"""

from __future__ import annotations

import re
import subprocess
from pathlib import Path

REPO = Path(__file__).resolve().parents[3]

_FAMILIES = r"(?:AR|BA|DEV|DP|ID|SEC|TM|VZ)-T[0-9]{1,3}|AZ-[MP][0-9]{1,3}"
_DEFINED = re.compile(rf"^\|\s*`?({_FAMILIES})`?\s*\|")
_NAMED = r"(AR|BA|DEV|DP|ID|SEC|TM|VZ|AZ)[-_]?(T|M|P)([0-9]{1,3})"
_TEST_PATHS = (
    "*_test.go",
    "*_test.dart",
    "test_*.py",
    "*_test.py",
    "*.test.ts",
    "*Tests.swift",
    "*Test.kt",
    "**/test/**",
    "**/tests/**",
    "**/integration_test/**",
)

_V2 = "chapter 21 is planned for V2"
_AR_OPEN = "stage AR ticket not built (AR-05 to AR-07, AR-09, AR-10; PLAN section 22)"
_OTHER = "not named in a test file; tested under another name or not at all (unverified)"

# ID -> why no test names it today (paths of Go tests under services/server/internal). Only
# removals are allowed.
KNOWN_UNNAMED: dict[str, str] = {
    "AR-T01": "tested in plugins/claude-code-runner/tests/scenarios/test_orderly_stop.py",
    "AR-T05": _AR_OPEN,
    "AR-T06": _AR_OPEN,
    "AR-T07": _AR_OPEN,
    "AR-T10": _AR_OPEN,
    "AR-T11": _AR_OPEN,
    "AR-T12": _AR_OPEN,
    "AR-T15": _AR_OPEN,
    "AZ-M16": _V2,
    "AZ-M17": _V2,
    "BA-T21": "end to end test of A1-10, not built",
    "DEV-T01": _V2,
    "DEV-T02": _V2,
    "DEV-T03": _V2,
    "DEV-T04": _V2,
    "DEV-T05": _V2,
    "DEV-T06": _V2,
    "DEV-T07": _V2,
    "DEV-T08": _V2,
    "DEV-T09": _V2,
    "DP-T11": "tested as TestMinGroup in stats/domain/statistics_test.go",
    "ID-T30": "tested in identity/adapters/httpapi/stepup_password_app_env_test.go",
    "ID-T32": _OTHER,
    "ID-T33": _OTHER,
    "ID-T34": _OTHER,
    "SEC-T02": "lint rules, not a test (SEC-TLS-4, SEC-NET-1, SEC-IN-5)",
    "SEC-T18": "an umbrella over TM-T01 to TM-T29, held by the terminal catalogue",
    "VZ-T06": "tested as TestLive_Revocation in wire/serve_stats_live_test.go",
}


def _git(*args: str) -> str:
    return subprocess.run(
        ["git", *args], cwd=REPO, capture_output=True, text=True, check=True
    ).stdout


def defined_ids() -> set[str]:
    """The test IDs that a table of docs/spec or docs/security defines in its first column."""
    out: set[str] = set()
    for name in _git("ls-files", "docs/spec/*.md", "docs/security/*.md").split():
        for line in (REPO / name).read_text(encoding="utf-8").splitlines():
            if m := _DEFINED.match(line):
                out.add(m.group(1))
    return out


def named_ids() -> set[str]:
    """The test IDs that a tracked test file names, in the form of the specification."""
    found = subprocess.run(
        ["git", "grep", "-ohIE", _NAMED, "--", *_TEST_PATHS],
        cwd=REPO,
        capture_output=True,
        text=True,
        check=False,
    ).stdout
    out = set()
    for token in found.split():
        if m := re.fullmatch(_NAMED, token):
            out.add(f"{m.group(1)}-{m.group(2)}{m.group(3)}")
    return out


def test_the_spec_defines_test_ids() -> None:
    # A broken table parser would find nothing and pass the other checks.
    assert len(defined_ids()) > 200


def test_every_test_id_is_named_by_a_test_or_listed() -> None:
    missing = defined_ids() - named_ids()
    assert sorted(missing - KNOWN_UNNAMED.keys()) == []


def test_the_ratchet_list_holds_only_ids_that_are_still_unnamed() -> None:
    stale = KNOWN_UNNAMED.keys() - (defined_ids() - named_ids())
    assert sorted(stale) == []
