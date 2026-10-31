# SPDX-License-Identifier: Apache-2.0
"""Accepted OSV findings are time-limited and named, never blanket (docs/quality/gates.md).

OSV-Scanner ignores an `[[IgnoredVulns]]` entry without `ignoreUntil` forever, and a string
in place of a TOML date would not be read as an expiry. The test reads every tracked
`osv-scanner.toml` and holds each accepted finding to one advisory id, a reason and an expiry
date.
"""

from __future__ import annotations

import datetime
import re
import subprocess
import tomllib
from pathlib import Path

import pytest

REPO = Path(__file__).resolve().parents[3]
ADVISORY_ID = re.compile(
    r"^(GHSA(-[23456789cfghjmpqrvwx]{4}){3}|CVE-\d{4}-\d{4,}|GO-\d{4}-\d{4,}|PYSEC-\d{4}-\d+)$"
)


def tracked_configs() -> list[Path]:
    out = subprocess.run(
        ["git", "ls-files", "--", "osv-scanner.toml", "*/osv-scanner.toml"],
        cwd=REPO,
        check=True,
        capture_output=True,
        text=True,
    ).stdout.split()
    return [REPO / p for p in out]


def ignores() -> list[tuple[str, dict]]:
    found = []
    for path in tracked_configs():
        data = tomllib.loads(path.read_text(encoding="utf-8"))
        for entry in data.get("IgnoredVulns", []):
            found.append((f"{path.relative_to(REPO)}:{entry.get('id', '?')}", entry))
    return found


def test_configs_are_found() -> None:
    assert tracked_configs(), "no tracked osv-scanner.toml"


@pytest.mark.parametrize(("where", "entry"), ignores(), ids=[w for w, _ in ignores()])
def test_ignore_is_one_advisory_with_reason_and_expiry(where: str, entry: dict) -> None:
    assert ADVISORY_ID.match(str(entry.get("id", ""))), f"{where}: id must be one advisory id"
    until = entry.get("ignoreUntil")
    assert isinstance(until, datetime.date), f"{where}: ignoreUntil must be a TOML date"
    assert str(entry.get("reason", "")).strip(), f"{where}: reason is missing"
