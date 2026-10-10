# SPDX-License-Identifier: Apache-2.0
"""Repository license gate (docs/quality/licenses.md).

1. LICENSE must be the canonical Apache-2.0 text, byte for byte.
2. Every tracked hand-written source file carries `SPDX-License-Identifier: Apache-2.0`
   within its first HEADER_SEARCH_LINES lines. Generated code is exempt (GENERATED).

Dependency licenses are checked per component by `make licenses` (go-licenses, the
Dart, Python and npm checkers); this script covers the project's own files.
"""

from __future__ import annotations

import argparse
import fnmatch
import hashlib
import subprocess
import sys
from pathlib import Path

# sha256 of https://www.apache.org/licenses/LICENSE-2.0.txt (retrieved 2026-10-10).
APACHE_2_SHA256 = "cfc7749b96f63bd31c3c42b5c471bf756814053e847c10f3eb003417bc523d30"
SPDX = "SPDX-License-Identifier: Apache-2.0"
HEADER_SEARCH_LINES = 5
SOURCE_SUFFIXES = frozenset(
    {".go", ".py", ".dart", ".ts", ".tsx", ".js", ".mjs", ".cjs", ".sh", ".kt", ".kts", ".swift"}
)
GENERATED = (
    "*/node_modules/*",
    "*.g.dart",
    "*.freezed.dart",
    "*.gen.go",
    "*_gen.go",
    "*.pb.go",
    "*/generated/*",
    "*/gen/*",
)


def license_findings(path: Path) -> list[str]:
    if not path.is_file():
        return [f"{path}: missing"]
    if hashlib.sha256(path.read_bytes()).hexdigest() != APACHE_2_SHA256:
        return [f"{path}: not the canonical Apache-2.0 text"]
    return []


def _is_generated(name: str) -> bool:
    candidate = "/" + name
    return any(fnmatch.fnmatch(candidate, pattern) for pattern in GENERATED)


def header_findings(root: Path, names: list[str]) -> list[str]:
    findings = []
    for name in names:
        if Path(name).suffix not in SOURCE_SUFFIXES or _is_generated(name):
            continue
        path = root / name
        if not path.is_file():
            continue
        with path.open(encoding="utf-8", errors="replace") as fh:
            head = [fh.readline() for _ in range(HEADER_SEARCH_LINES)]
        if not any(SPDX in line for line in head):
            findings.append(f"{name}: missing '{SPDX}'")
    return findings


def tracked(root: Path) -> list[str]:
    out = subprocess.run(
        ["git", "-C", str(root), "ls-files", "-z"], check=True, capture_output=True
    ).stdout
    return [p for p in out.decode("utf-8").split("\0") if p]


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Check LICENSE and SPDX headers.")
    parser.add_argument("--repo", type=Path, default=Path.cwd())
    args = parser.parse_args(argv)
    findings = license_findings(args.repo / "LICENSE") + header_findings(
        args.repo, tracked(args.repo)
    )
    for finding in findings:
        print(finding)
    return 1 if findings else 0


if __name__ == "__main__":
    sys.exit(main())
