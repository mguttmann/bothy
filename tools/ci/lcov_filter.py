# SPDX-License-Identifier: Apache-2.0
"""Drop the records of generated Dart files from an lcov report (brief M4-01, Design note 4).

Generated code is excluded from coverage (docs/DECISIONS.md D-M4-01-4), with the same patterns
as the doc and style lint in the root analysis_options.yaml: `*.g.dart`, `*.freezed.dart`,
`*.drift.dart`, everything below `lib/src/generated/` and the build output `build/`. The drift
table definitions below `lib/src/tables/` leave too: drift's generator reads them and the
generated `*.g.dart` holds the code that runs, so their getters never execute (D-M4-12-11).
The report of `flutter test --coverage` names files relative to the member (`SF:lib/x.dart`);
absolute paths are matched by their file name and the `lib/src/generated/` and `lib/src/tables/`
segments.

Usage:
  lcov_filter.py <lcov.info>            print the report without generated records
  lcov_filter.py --totals <lcov.info>   print "<lines found> <lines hit>" of that report
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path, PurePosixPath

GENERATED_SUFFIXES = (".g.dart", ".freezed.dart", ".drift.dart")
GENERATED_DIR = ("lib", "src", "generated")
GENERATOR_INPUT_DIR = ("lib", "src", "tables")


def is_generated(path: str) -> bool:
    """Whether the source file `path` of a record is generated code."""
    parts = PurePosixPath(path.replace("\\", "/")).parts
    if parts and parts[-1].endswith(GENERATED_SUFFIXES):
        return True
    if not path.startswith("/") and parts[:1] == ("build",):
        return True
    for segment in (GENERATED_DIR, GENERATOR_INPUT_DIR):
        n = len(segment)
        if any(parts[i : i + n] == segment for i in range(len(parts) - n)):
            return True
    return False


def _records(text: str) -> list[list[str]]:
    records: list[list[str]] = []
    current: list[str] = []
    for line in text.splitlines():
        current.append(line)
        if line == "end_of_record":
            records.append(current)
            current = []
    if current:
        records.append(current)
    return records


def _source(record: list[str]) -> str:
    return next((line[3:] for line in record if line.startswith("SF:")), "")


def filter_lcov(text: str) -> str:
    """The report `text` without the records of generated files."""
    kept = [r for r in _records(text) if not is_generated(_source(r))]
    return "".join("\n".join(r) + "\n" for r in kept)


def totals(text: str) -> tuple[int, int]:
    """The sums of `LF` (lines found) and `LH` (lines hit) over every record of `text`."""
    found = hit = 0
    for line in text.splitlines():
        if line.startswith("LF:"):
            found += int(line[3:])
        elif line.startswith("LH:"):
            hit += int(line[3:])
    return found, hit


def main(argv: list[str] | None = None) -> int:
    """Command line entry point; see the module documentation."""
    parser = argparse.ArgumentParser(description="Drop generated Dart files from lcov.")
    parser.add_argument("--totals", action="store_true", help="print the sums only")
    parser.add_argument("lcov", type=Path)
    args = parser.parse_args(argv)
    try:
        text = args.lcov.read_text(encoding="utf-8")
    except OSError as exc:
        print(f"lcov_filter: cannot read {args.lcov}: {exc.strerror}", file=sys.stderr)
        return 2
    kept = filter_lcov(text)
    if args.totals:
        found, hit = totals(kept)
        print(f"{found} {hit}")
    else:
        sys.stdout.write(kept)
    return 0


if __name__ == "__main__":
    sys.exit(main())
