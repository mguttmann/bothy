# SPDX-License-Identifier: Apache-2.0
"""License gate over a ScanCode report: `make scancode` (docs/quality/licenses.md section 4).

Every scanned file whose detected license expression (`detected_license_expression_spdx`)
is not satisfied by the allowed licenses of quality/licenses/allowed.txt is a finding, and so is
a file ScanCode could not scan. An expression is satisfied when `A OR B` has one satisfied
side, `A AND B` both, and `A WITH exception` its license A; `LicenseRef-*` (unknown or
non-SPDX texts) is never allowed. Paths reviewed once and listed with a reason in
quality/licenses/scancode-exceptions.txt (documentation that names licenses, test data of a
license tool) do not count. A finding stops the release before any tag (release.yml job
`scancode`).

Usage:
  scancode_policy.py --report <scancode.json> [--allowed <allowed.txt>]
                     [--exceptions <scancode-exceptions.txt>]
Exit codes: 0 no finding, 1 findings (listed), 2 unreadable input.
"""

from __future__ import annotations

import argparse
import fnmatch
import json
import re
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Any

REPO_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_ALLOWED = REPO_ROOT / "quality" / "licenses" / "allowed.txt"
DEFAULT_EXCEPTIONS = REPO_ROOT / "quality" / "licenses" / "scancode-exceptions.txt"
_TOKEN = re.compile(r"\(|\)|[A-Za-z0-9.+:-]+")


class PolicyError(Exception):
    """An input of the gate is missing or malformed."""


@dataclass(frozen=True)
class Waiver:
    """A reviewed path pattern (fnmatch, `*` also matches `/`) and why it does not count."""

    pattern: str
    reason: str


@dataclass(frozen=True)
class Finding:
    """A file the release must not ship as it is."""

    path: str
    problem: str


class _Parser:
    """Recursive descent over an SPDX expression; precedence WITH, then AND, then OR."""

    def __init__(self, text: str, allowed: frozenset[str]) -> None:
        self.tokens = _TOKEN.findall(text)
        self.pos = 0
        self.allowed = allowed

    def _peek(self) -> str | None:
        return self.tokens[self.pos] if self.pos < len(self.tokens) else None

    def _take(self) -> str:
        token = self._peek()
        if token is None:
            raise PolicyError("unexpected end of expression")
        self.pos += 1
        return token

    def parse(self) -> bool:
        value = self._or()
        if self._peek() is not None:
            raise PolicyError(f"unexpected '{self._peek()}'")
        return value

    def _or(self) -> bool:
        value = self._and()
        while self._peek() == "OR":
            self._take()
            value = self._and() or value
        return value

    def _and(self) -> bool:
        value = self._with()
        while self._peek() == "AND":
            self._take()
            value = self._with() and value
        return value

    def _with(self) -> bool:
        value = self._atom()
        if self._peek() == "WITH":
            self._take()
            self._take()  # the exception widens a license, it never needs allowing itself
        return value

    def _atom(self) -> bool:
        item = self._take()
        if item == "(":
            value = self._or()
            if self._take() != ")":
                raise PolicyError("missing ')'")
            return value
        if item in ("AND", "OR", "WITH", ")"):
            raise PolicyError(f"unexpected '{item}'")
        return item in self.allowed


def allowed_expression(expression: str, allowed: frozenset[str]) -> bool:
    """Return whether the allowed licenses satisfy an SPDX license expression.

    Args:
        expression: for example `(MIT OR GPL-2.0-only) AND Apache-2.0`.
        allowed: SPDX identifiers, compared with their exact case.

    Returns:
        True when the expression holds with the allowed licenses; False for a malformed one.

    Example: `allowed_expression("GPL-2.0-only OR MIT", frozenset({"MIT"}))` is True.
    """
    try:
        return _Parser(expression, allowed).parse()
    except PolicyError:
        return False


def load_allowed(path: Path) -> frozenset[str]:
    """Read allowed.txt: one SPDX identifier per line, `#` comments."""
    lines = path.read_text(encoding="utf-8").splitlines()
    return frozenset(line.strip() for line in lines if line.strip() and not line.startswith("#"))


def load_exceptions(path: Path) -> list[Waiver]:
    """Read the exceptions file: `<pattern> | <reason>` per line, `#` comments.

    Raises:
        PolicyError: a line has no pattern or no reason.
    """
    out = []
    for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        text = line.strip()
        if not text or text.startswith("#"):
            continue
        pattern, sep, reason = (part.strip() for part in text.partition("|"))
        if not sep or not pattern or not reason:
            raise PolicyError(f"{path.name} line {number}: expected '<pattern> | <reason>'")
        out.append(Waiver(pattern, reason))
    return out


def load_report(path: Path) -> dict[str, Any]:
    """Read a ScanCode JSON report.

    Raises:
        PolicyError: the file is not JSON or has no `files` list.
    """
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError) as exc:
        raise PolicyError(f"{path}: {exc}") from exc
    if not isinstance(data, dict) or not isinstance(data.get("files"), list):
        raise PolicyError(f"{path} is not a ScanCode report (no 'files' list)")
    return data


def findings(
    report: dict[str, Any], allowed: frozenset[str], waivers: list[Waiver]
) -> list[Finding]:
    """Return the findings of a ScanCode report, sorted by path."""
    out = []
    for entry in report["files"]:
        if entry.get("type") != "file":
            continue
        path = str(entry.get("path", ""))
        if any(fnmatch.fnmatchcase(path, w.pattern) for w in waivers):
            continue
        errors = entry.get("scan_errors") or []
        if errors:
            out.append(Finding(path, f"scan error: {errors[0]}"))
            continue
        expression = entry.get("detected_license_expression_spdx")
        if expression and not allowed_expression(expression, allowed):
            out.append(Finding(path, f"license {expression}"))
    return sorted(out, key=lambda f: f.path)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="License gate over a ScanCode report.")
    parser.add_argument("--report", type=Path, required=True)
    parser.add_argument("--allowed", type=Path, default=DEFAULT_ALLOWED)
    parser.add_argument("--exceptions", type=Path, default=DEFAULT_EXCEPTIONS)
    args = parser.parse_args(argv)
    try:
        report = load_report(args.report)
        found = findings(report, load_allowed(args.allowed), load_exceptions(args.exceptions))
    except (OSError, PolicyError) as exc:
        print(f"scancode-policy: {exc}", file=sys.stderr)
        return 2
    scanned = sum(1 for e in report["files"] if e.get("type") == "file")
    for f in found:
        print(f"  {f.path}: {f.problem}")
    print(f"scancode-policy: {scanned} files, {len(found)} finding(s)")
    return 1 if found else 0


if __name__ == "__main__":
    sys.exit(main())
