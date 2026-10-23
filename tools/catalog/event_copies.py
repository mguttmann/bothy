# SPDX-License-Identifier: Apache-2.0
"""Check the hand-kept lists of the durable event types against `docs/spec/actions.yaml`.

Generated copies of the event catalogue (the Go catalogues, the API clients, the tables of
chapter 06, `packages/domain`) have generator gates. The lists below are written by hand, and
each was guarded only by a test of its own component (the TypeScript type check, the Dart
catalogue test), which a change of another component does not run; so a durable type added to
the catalogue, or ported from main, could leave one of them behind (docs/DECISIONS.md
D-M3-19-23). This check reads all of them in Python, which `make codegen-check` runs for every
component selection:

- `sdk/typescript/src/tolerance.ts` `KNOWN_EVENT_TYPES` equals the durable types.
- `packages/sync_engine`: the applier keys of `lib/src/appliers/*.dart` and `skippedDurable` of
  `test/catch_up/catalogue_test.dart` are disjoint and together equal the durable types.
- The skipped list of `packages/sync_engine/README.md` equals `skippedDurable`.

Usage:
    event_copies.py [--root <repo>]

Exit codes: 0 every copy is complete; 1 a copy misses or adds a type, or a file cannot be read;
2 usage error.
"""

from __future__ import annotations

import argparse
import functools
import re
import sys
from pathlib import Path

import yaml

CATALOGUE = Path("docs/spec/actions.yaml")
TS_TOLERANCE = Path("sdk/typescript/src/tolerance.ts")
DART_APPLIERS = Path("packages/sync_engine/lib/src/appliers")
DART_CATALOGUE_TEST = Path("packages/sync_engine/test/catch_up/catalogue_test.dart")
DART_README = Path("packages/sync_engine/README.md")
README_START = "Skipped, the cursor advances (no local table):"
README_END = "and every type the catalogue does not know"

COPY_NAMES = {
    "ts": f"{TS_TOLERANCE} KNOWN_EVENT_TYPES",
    "dart": "packages/sync_engine appliers and skippedDurable",
    "readme": f"{DART_README} skipped list",
}

_TYPE = r"[a-z][a-z0-9_]*(?:\.[a-z][a-z0-9_]*)+"
_TS_LIST = re.compile(r"KNOWN_EVENT_TYPES\s*=\s*\[(.*?)\]", re.S)
_DART_SKIPPED = re.compile(r"skippedDurable\s*=\s*\{(.*?)\};", re.S)
_DART_KEY = re.compile(rf"^\s*'({_TYPE})'\s*:", re.M)


class CopyError(Exception):
    """A source or copy cannot be read; the message names the file."""


@functools.lru_cache(maxsize=4)
def durable_types(text: str) -> tuple[str, ...]:
    """Return the sorted durable event types of the `events:` list of actions.yaml text."""
    data = yaml.safe_load(text)
    items = data.get("events") if isinstance(data, dict) else None
    if not isinstance(items, list):
        raise CopyError(f"{CATALOGUE}: no `events:` list")
    return tuple(
        sorted({i["type"] for i in items if isinstance(i, dict) and i.get("kind") == "durable"})
    )


def ts_known(text: str) -> set[str]:
    """Return the types of `KNOWN_EVENT_TYPES` in tolerance.ts text."""
    m = _TS_LIST.search(text)
    if m is None:
        raise CopyError(f"{TS_TOLERANCE}: no KNOWN_EVENT_TYPES list")
    return set(re.findall(r'"([^"]+)"', m.group(1)))


def dart_skipped(text: str) -> set[str]:
    """Return the types of `skippedDurable` in catalogue_test.dart text."""
    m = _DART_SKIPPED.search(text)
    if m is None:
        raise CopyError(f"{DART_CATALOGUE_TEST}: no skippedDurable set")
    return set(re.findall(r"'([^']+)'", m.group(1)))


def readme_skipped(text: str) -> set[str]:
    """Return the backticked types of the skipped list in the sync_engine README text."""
    _, found, tail = text.partition(README_START)
    body, found_end, _ = tail.partition(README_END)
    if not found or not found_end:
        raise CopyError(f"{DART_README}: no skipped list")
    return set(re.findall(rf"`({_TYPE})`", body))


def _compare(name: str, want: set[str], got: set[str]) -> list[str]:
    return [f"{name}: missing {t}" for t in sorted(want - got)] + [
        f"{name}: unknown {t}" for t in sorted(got - want)
    ]


def check(root: Path) -> list[str]:
    """Return one line per difference between the catalogue and a hand-kept copy.

    Raises:
        CopyError: the catalogue or a copy is missing or has no list.
        OSError: a file cannot be read.
    """
    durable = set(durable_types((root / CATALOGUE).read_text(encoding="utf-8")))
    problems = _compare(
        COPY_NAMES["ts"], durable, ts_known((root / TS_TOLERANCE).read_text(encoding="utf-8"))
    )
    applied: set[str] = set()
    for f in sorted((root / DART_APPLIERS).glob("*.dart")):
        applied |= set(_DART_KEY.findall(f.read_text(encoding="utf-8")))
    skipped = dart_skipped((root / DART_CATALOGUE_TEST).read_text(encoding="utf-8"))
    both = sorted(applied & skipped)
    problems += [f"{COPY_NAMES['dart']}: applied and skipped {t}" for t in both]
    problems += _compare(COPY_NAMES["dart"], durable, applied | skipped)
    problems += _compare(
        COPY_NAMES["readme"], skipped, readme_skipped((root / DART_README).read_text("utf-8"))
    )
    return problems


def main(argv: list[str] | None = None) -> int:
    """Command line entry point; see the module docstring for usage and exit codes."""
    parser = argparse.ArgumentParser(prog="event_copies.py")
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[2])
    try:
        args = parser.parse_args(argv)
    except SystemExit as exc:  # argparse ends on usage errors (2) and on --help (0)
        return exc.code if isinstance(exc.code, int) else 2
    try:
        problems = check(args.root)
    except (OSError, yaml.YAMLError, CopyError) as exc:
        print(f"event-copies: {exc}", file=sys.stderr)
        return 1
    for p in problems:
        print(f"event-copies: {p}", file=sys.stderr)
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
