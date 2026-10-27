# SPDX-License-Identifier: Apache-2.0
"""Command line of the showcase: strip metadata, keep the manifest, check the folder (M7-28).

The capture drivers call it after every capture; `make check` calls `check`. The module is not
named `showcase.py`: under pytest's importlib mode the folder `tools/showcase` itself becomes
the package `showcase`, which would hide a module of that name.

Usage:

    showcase_cli.py strip <file>...
    showcase_cli.py record --file F --kind image|video --platform P --view V --theme light|dark
                       --script S --commit C --seed N
    showcase_cli.py drop-platform <platform>
    showcase_cli.py check

`strip` rewrites each file in place without its metadata, PNG or MP4 by content (media.py).
`record` adds or replaces the manifest entry of one file (relative to docs/assets/showcase).
`drop-platform` removes a platform's entries and files before its fresh capture. `check`
prints one line per finding (manifest.py). `--root` names the repository root; it defaults to
the repository this file is in.

Exit codes: 0 done or clean; 1 findings, or a file that `strip` cannot read or strip; 2 usage,
including values that break the manifest schema.
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

import manifest
import media

REPO_ROOT = Path(__file__).resolve().parents[2]


def parse_args(argv: list[str]) -> argparse.Namespace:
    """Parse the command line.

    Args:
        argv: the arguments without the program name.

    Returns:
        The parsed arguments; `command` names the subcommand.

    Raises:
        SystemExit: with code 2 on a usage error (argparse).
    """
    parser = argparse.ArgumentParser(description="Showcase media: strip, record, check.")
    parser.add_argument("--root", type=Path, default=REPO_ROOT, help="repository root")
    sub = parser.add_subparsers(dest="command", required=True)
    strip = sub.add_parser("strip", help="remove metadata in place")
    strip.add_argument("files", nargs="+", type=Path)
    rec = sub.add_parser("record", help="add or replace one manifest entry")
    rec.add_argument("--file", required=True, help="path relative to docs/assets/showcase")
    rec.add_argument("--kind", required=True, choices=sorted(manifest.KINDS))
    rec.add_argument("--platform", required=True)
    rec.add_argument("--view", required=True)
    rec.add_argument("--theme", required=True, choices=manifest.THEMES)
    rec.add_argument("--script", required=True, help="repository path of the capture script")
    rec.add_argument("--commit", required=True, help="40-hex commit of the capture")
    rec.add_argument("--seed", required=True, type=int, help="seed of the demo instance")
    drop = sub.add_parser("drop-platform", help="remove a platform's entries and files")
    drop.add_argument("platform")
    sub.add_parser("check", help="check the folder against the manifest and the metadata rule")
    return parser.parse_args(argv)


def _strip(files: list[Path]) -> int:
    failed = 0
    for path in files:
        try:
            data = path.read_bytes()
            stripped = media.strip(data)
        except (OSError, media.MediaError) as exc:
            print(f"showcase: {path}: {exc}", file=sys.stderr)
            failed += 1
            continue
        if stripped != data:
            path.write_bytes(stripped)
    return 1 if failed else 0


def _check(root: Path) -> int:
    findings = manifest.check(root)
    for line in findings:
        print(line)
    if findings:
        print(
            f"showcase: {len(findings)} finding(s); run `showcase_cli.py strip` and "
            "`showcase_cli.py record` (tools/showcase/README.md)"
        )
        return 1
    count = len(manifest.load(root / manifest.MANIFEST))
    print(f"showcase: clean ({count} file{'' if count == 1 else 's'})")
    return 0


def main(argv: list[str] | None = None) -> int:
    """Run one subcommand.

    Args:
        argv: the arguments without the program name; `None` reads `sys.argv`.

    Returns:
        The exit code (see the module docstring).

    Raises:
        SystemExit: with code 2 on a usage error (argparse).
    """
    args = parse_args(sys.argv[1:] if argv is None else argv)
    root: Path = args.root
    try:
        match args.command:
            case "strip":
                return _strip(args.files)
            case "record":
                manifest.record(
                    root,
                    manifest.Asset(
                        file=args.file,
                        kind=args.kind,
                        platform=args.platform,
                        view=args.view,
                        theme=args.theme,
                        script=args.script,
                        commit=args.commit,
                        demo_seed=args.seed,
                    ),
                )
                return 0
            case "drop-platform":
                for file in manifest.drop_platform(root, args.platform):
                    print(f"showcase: removed {file}")
                return 0
            case _:
                return _check(root)
    except manifest.ManifestError as exc:
        print(f"showcase: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
