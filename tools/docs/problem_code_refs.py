# SPDX-License-Identifier: Apache-2.0
"""Every "status plus problem code" in the documentation names a registered problem code.

The problem codes of the API are a closed list: `x-problem-codes` at the schema `Problem` in
docs/api/openapi.yaml (06 section 5, R-API-23). Chapters, briefs and the plan often write an
error as an HTTP status followed by a code, for example `409 conflict` or `403` `forbidden`. Such
a pair must name a registered code with exactly that status. A finer cause goes into `reason`
(403, 409, 410, 503) or into `errors[].code` of `400 validation_failed`, never into a new code
written only in prose. Without this check a chapter can invent a code that no operation
documents and no client knows.

Recognised forms (the code is snake_case in backticks, the status a 4xx or 5xx number):
`409 conflict`, `409` `conflict` and 409 `conflict`; whitespace may include a line break. Other
phrasings (for example "`409` with code `x`") are not recognised; 06 section 5 item 7 asks the
chapters to avoid them.

Usage: python tools/docs/problem_code_refs.py [--openapi PATH] FILE...  (exit 1 on a finding)
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

import yaml

DEFAULT_OPENAPI = Path("docs/api/openapi.yaml")

# Forms: "`409 conflict`", "`409` `conflict`", "409 `conflict`".
_GAP = r"[ \t]*\n?[ \t]*"
_PAIR = re.compile(
    r"(?<![\w.])(?P<open>`?)(?P<status>[45]\d\d)"
    r"(?P<sep>`" + _GAP + r"`|" + _GAP + r"`|[ \t]+\n?[ \t]*|\n[ \t]*)"
    r"(?P<code>[a-z][a-z0-9_]*)`"
)


def load_codes(openapi: Path) -> dict[str, int]:
    """Return the registered problem codes with their status from `x-problem-codes`."""
    spec = yaml.safe_load(openapi.read_text(encoding="utf-8"))
    entries = spec["components"]["schemas"]["Problem"]["x-problem-codes"]
    return {entry["code"]: int(entry["status"]) for entry in entries}


def _is_pair(match: re.Match[str]) -> bool:
    """A bare separator without backticks only counts inside one code span (`409 conflict`)."""
    sep = match.group("sep")
    if "`" in sep:
        # "`409` `x`" needs the opening backtick, "409 `x`" must not have one.
        return sep.count("`") == 2 if match.group("open") else sep.count("`") == 1
    return match.group("open") == "`"


def check_text(name: str, text: str, codes: dict[str, int]) -> list[str]:
    """Return one message per status-code pair that is not registered with that status."""
    problems: list[str] = []
    for match in _PAIR.finditer(text):
        if not _is_pair(match):
            continue
        status, code = int(match.group("status")), match.group("code")
        if codes.get(code) == status:
            continue
        line = text.count("\n", 0, match.start("status")) + 1
        known = codes.get(code)
        why = f"registered with status {known}" if known else "not in x-problem-codes"
        problems.append(f"{name}:{line}: `{status} {code}`: {why} (06 section 5)")
    return problems


def main(argv: list[str] | None = None) -> int:
    """Check that the files cite only problem codes of openapi.yaml; returns the exit code."""
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--openapi", type=Path, default=DEFAULT_OPENAPI)
    parser.add_argument("files", nargs="*", type=Path)
    args = parser.parse_args(sys.argv[1:] if argv is None else argv)
    codes = load_codes(args.openapi)
    problems: list[str] = []
    for path in args.files:
        problems += check_text(str(path), path.read_text(encoding="utf-8"), codes)
    for problem in problems:
        print(problem, file=sys.stderr)
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
