# SPDX-License-Identifier: Apache-2.0
"""Run `flutter gen-l10n` in a temporary copy of a Dart member (gate `codegen-check`).

Brief M4-06 Design note 2 and docs/DECISIONS.md D-M4-06-2: a missing translation breaks the
build, and the checked-in localizations equal a fresh generation. The tool copies the
member's `pubspec.yaml`, `l10n.yaml` and ARB directory into a temporary directory, adds an
`untranslated-messages-file` inside that directory to the copied `l10n.yaml` (the generator
ignores command-line options while an `l10n.yaml` exists), runs the generator there and then

- fails when the untranslated-messages file lists any message or is missing;
- fails when the generated output directory differs from the checked-in one (a file
  changed, added or removed), so a stale generation fails.

The member itself is never written; the copy is removed at the end.

Usage: l10n_check.py --dir <member> [--flutter <program>]

Exit codes: 0 ok; 1 untranslated messages or stale generated code; 2 usage error (no
`l10n.yaml`, an option missing, or an `untranslated-messages-file` already set); the
generator's own exit code when it fails (127 when it cannot be found).
"""

from __future__ import annotations

import argparse
import hashlib
import json
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

import yaml

UNTRANSLATED = "untranslated-messages-file"


def _hashes(directory: Path) -> dict[str, str]:
    """Map every file below `directory` (relative POSIX path) to the sha256 of its bytes."""
    if not directory.is_dir():
        return {}
    return {
        p.relative_to(directory).as_posix(): hashlib.sha256(p.read_bytes()).hexdigest()
        for p in sorted(directory.rglob("*"))
        if p.is_file()
    }


def _options(member: Path) -> dict[str, object]:
    """Read the member's l10n.yaml; raise ValueError when it is unusable for the check."""
    config = member / "l10n.yaml"
    if not config.is_file():
        raise ValueError(f"{config} is missing")
    loaded = yaml.safe_load(config.read_text(encoding="utf-8")) or {}
    if not isinstance(loaded, dict):
        raise ValueError(f"{config} is not a mapping")
    if UNTRANSLATED in loaded:
        raise ValueError(f"{config} sets {UNTRANSLATED}; the check sets it in its copy only")
    for key in ("arb-dir", "output-dir"):
        if not isinstance(loaded.get(key), str):
            raise ValueError(f"{config} has no {key}")
    return loaded


def check(member: Path, flutter: str) -> int:
    """Generate into a temporary copy of `member` and compare; returns the exit code."""
    options = _options(member)
    arb_dir = str(options["arb-dir"])
    output_dir = str(options["output-dir"])
    with tempfile.TemporaryDirectory(prefix="l10n-check-") as tmp:
        copy = Path(tmp) / member.name
        copy.mkdir()
        shutil.copy2(member / "pubspec.yaml", copy / "pubspec.yaml")
        shutil.copytree(member / arb_dir, copy / arb_dir)
        untranslated = Path(tmp) / "untranslated.json"
        text = (member / "l10n.yaml").read_text(encoding="utf-8").rstrip("\n")
        (copy / "l10n.yaml").write_text(
            f"{text}\n{UNTRANSLATED}: {json.dumps(str(untranslated))}\n", encoding="utf-8"
        )
        try:
            result = subprocess.run([flutter, "gen-l10n"], cwd=copy, check=False)
        except FileNotFoundError:
            print(f"l10n-check: {flutter} not found", file=sys.stderr)
            return 127
        if result.returncode != 0:
            return result.returncode
        failed = False
        if not untranslated.is_file():
            print(f"l10n-check: {member}: the generator wrote no {UNTRANSLATED}")
            failed = True
        else:
            listed = json.loads(untranslated.read_text(encoding="utf-8") or "{}")
            if listed:
                print(f"l10n-check: {member}: untranslated messages (add them to every ARB file):")
                for locale, keys in sorted(listed.items()):
                    print(f"  {locale}: {', '.join(keys)}")
                failed = True
        fresh = _hashes(copy / output_dir)
        checked_in = _hashes(member / output_dir)
        drift = [
            f"  changed: {output_dir}/{p}"
            for p in sorted(fresh)
            if p in checked_in and fresh[p] != checked_in[p]
        ]
        drift += [f"  added: {output_dir}/{p}" for p in sorted(fresh.keys() - checked_in.keys())]
        drift += [f"  removed: {output_dir}/{p}" for p in sorted(checked_in.keys() - fresh.keys())]
        if drift:
            print(f"l10n-check: {member}: generated code is stale; run `flutter gen-l10n` there:")
            print("\n".join(drift))
            failed = True
    if failed:
        return 1
    print(f"l10n-check: {member.name} ok")
    return 0


def main(argv: list[str] | None = None) -> int:
    """Parse the arguments and run the check; see the module docstring for the exit codes."""
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--dir", required=True, type=Path, help="the Dart member with l10n.yaml")
    parser.add_argument("--flutter", default="flutter", help="the flutter program")
    args = parser.parse_args(argv)
    try:
        return check(args.dir, args.flutter)
    except ValueError as exc:
        print(f"l10n-check: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
