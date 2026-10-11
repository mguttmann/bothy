# SPDX-License-Identifier: Apache-2.0
"""Reject forbidden characters in text files.

Forbidden: en dash (U+2013) and em dash (U+2014). The project writes plain hyphens
or rephrases (docs/quality/push-discipline.md). Binary files (NUL byte in the first
block) are skipped; text that is not valid UTF-8 is reported.

Usage:
  text_rules.py FILE...   check the given files (pre-commit passes staged files)
  text_rules.py --all     check every tracked file of the repository
"""

from __future__ import annotations

import argparse
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

FORBIDDEN = {chr(0x2013): "U+2013", chr(0x2014): "U+2014"}
_BINARY_PROBE = 8192


@dataclass(frozen=True)
class Finding:
    """One forbidden character with its file, line, column and code point."""

    path: Path
    line: int
    column: int
    codepoint: str


def scan_file(path: Path) -> list[Finding]:
    """Return every en or em dash in a text file; binary files give no finding."""
    data = path.read_bytes()
    if b"\x00" in data[:_BINARY_PROBE]:
        return []
    try:
        text = data.decode("utf-8")
    except UnicodeDecodeError as exc:
        line = data[: exc.start].count(b"\n") + 1
        return [Finding(path, line, 0, "invalid-utf-8")]
    findings = []
    for lineno, line_text in enumerate(text.splitlines(), start=1):
        for col, char in enumerate(line_text, start=1):
            if char in FORBIDDEN:
                findings.append(Finding(path, lineno, col, FORBIDDEN[char]))
    return findings


def tracked_files(root: Path) -> list[Path]:
    """Return the tracked files of the repository at `root`."""
    out = subprocess.run(
        ["git", "-C", str(root), "ls-files", "-z"], check=True, capture_output=True
    ).stdout
    return [root / p for p in out.decode("utf-8").split("\0") if p]


def main(argv: list[str] | None = None) -> int:
    """Check the given or all tracked files; returns 1 if a dash was found."""
    parser = argparse.ArgumentParser(description="Reject en and em dashes in text files.")
    parser.add_argument("--all", action="store_true", help="check every tracked file")
    parser.add_argument("files", nargs="*", type=Path)
    args = parser.parse_args(argv)
    root = Path(__file__).resolve().parents[2]
    files = tracked_files(root) if args.all else args.files
    findings = [f for path in files if path.is_file() for f in scan_file(path)]
    for f in findings:
        print(f"{f.path}:{f.line}:{f.column}: {f.codepoint} is not allowed, use '-' or rephrase")
    return 1 if findings else 0


if __name__ == "__main__":
    sys.exit(main())
