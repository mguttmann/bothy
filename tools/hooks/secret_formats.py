# SPDX-License-Identifier: Apache-2.0
"""Reject values in the Stripe webhook secret format that GitHub secret scanning reports.

GitHub scans every pushed file for the secret formats of its partners and opens an alert for
each match, whatever the text around it says; a gitleaks stopword such as EXAMPLE does not
count there. The first public push raised alerts of type stripe_webhook_signing_secret for
documentation placeholders with 31 and 32 characters after the prefix `whsec_`, which Stripe
shares with Standard Webhooks. Example values therefore stay short of the format
(`whsec_EXAMPLE`, CONTRIBUTING.md), and tests assemble their samples at runtime. Other
third-party formats are left to gitleaks, whose EXAMPLE marker exempts only this product's
own token formats and generic key-like values (.gitleaks.toml).

GitHub does not publish its patterns. The check reports `whsec_` followed by 16 or more
base64 characters: a real secret has at least 32, GitHub reported 31, and the margin covers a
shorter pattern. Binary files (NUL byte in the first block) are skipped.

Usage:
  secret_formats.py FILE...   check the given files (pre-commit passes staged files)
  secret_formats.py --all     check every tracked file of the repository
"""

from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass
from pathlib import Path

from text_rules import tracked_files

FORMATS = {"stripe_webhook_signing_secret": re.compile(r"whsec_[A-Za-z0-9+/]{16,}")}
"""Provider formats by their GitHub secret type."""

ALLOWED_FIXTURES: frozenset[str] = frozenset()
"""Repository paths of test fixtures that must hold such a value verbatim, each with its
reason in a comment. Empty: tests assemble their samples at runtime."""

ROOT = Path(__file__).resolve().parents[2]
_BINARY_PROBE = 8192


@dataclass(frozen=True)
class Finding:
    """One value in a provider format with its file, line, column and GitHub secret type."""

    path: Path
    line: int
    column: int
    secret_type: str


def scan_file(path: Path) -> list[Finding]:
    """Return every value in a provider format in a text file; binary files give none."""
    data = path.read_bytes()
    if b"\x00" in data[:_BINARY_PROBE]:
        return []
    text = data.decode("utf-8", errors="replace")
    return [
        Finding(path, lineno, match.start() + 1, secret_type)
        for lineno, line_text in enumerate(text.splitlines(), start=1)
        for secret_type, pattern in FORMATS.items()
        for match in pattern.finditer(line_text)
    ]


def _allowed(path: Path) -> bool:
    try:
        relative = path.resolve().relative_to(ROOT.resolve())
    except ValueError:
        return False
    return relative.as_posix() in ALLOWED_FIXTURES


def main(argv: list[str] | None = None) -> int:
    """Check the given or all tracked files; returns 1 if a value in the whsec_ format was found."""
    parser = argparse.ArgumentParser(
        description="Reject the Stripe webhook secret format whsec_ (GitHub secret scanning)."
    )
    parser.add_argument("--all", action="store_true", help="check every tracked file")
    parser.add_argument("files", nargs="*", type=Path)
    args = parser.parse_args(argv)
    files = tracked_files(ROOT) if args.all else args.files
    findings = [
        f for path in files if path.is_file() and not _allowed(path) for f in scan_file(path)
    ]
    for f in findings:
        print(
            f"{f.path}:{f.line}:{f.column}: {f.secret_type} format, which GitHub secret "
            "scanning reports; shorten the example (whsec_EXAMPLE) or assemble it at runtime"
        )
    return 1 if findings else 0


if __name__ == "__main__":
    sys.exit(main())
