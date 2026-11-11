# SPDX-License-Identifier: Apache-2.0
"""The tracked shell scripts of the repository, and ShellCheck over all of them (`make lint`).

A shell script is a tracked file whose name ends in `.sh` or whose first line names `sh`,
`bash`, `dash` or `ksh` as its interpreter, directly or through `env`: the dialects ShellCheck
reads. The scope comes from git, so a new script is checked without an entry in a list
(docs/decisions.d/AUD-root-01.md). A tracked file that is missing in the working tree is left
out.

Usage:
  shell_scripts.py [--repo DIR]                    print the scripts, one path per line
  shell_scripts.py [--repo DIR] --shellcheck PATH  run ShellCheck over them
Exit codes: 0, or the exit code of ShellCheck.
"""

from __future__ import annotations

import argparse
import subprocess
import sys
from pathlib import Path

_DIALECTS = (b"sh", b"bash", b"dash", b"ksh")


def _interpreter(first_line: bytes) -> bytes:
    """The program a `#!` line starts, without its folder; empty without such a line."""
    if not first_line.startswith(b"#!"):
        return b""
    words = first_line[2:].split()
    if not words:
        return b""
    name = words[0].rsplit(b"/", 1)[-1]
    if name == b"env" and len(words) > 1:
        return words[1].rsplit(b"/", 1)[-1]
    return name


def tracked(repo: Path) -> list[str]:
    """The tracked shell scripts of `repo`, as sorted paths relative to it."""
    listed = subprocess.run(
        ["git", "-C", str(repo), "ls-files", "-z"], check=True, capture_output=True
    ).stdout
    scripts = []
    for raw in listed.split(b"\0"):
        if not raw:
            continue
        name = raw.decode("utf-8", "surrogateescape")
        path = repo / name
        if not path.is_file():
            continue
        if not name.endswith(".sh"):
            with path.open("rb") as fh:
                if _interpreter(fh.readline(256)) not in _DIALECTS:
                    continue
        scripts.append(name)
    return sorted(scripts)


def main(argv: list[str] | None = None) -> int:
    """Command line entry; see the module docstring."""
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--repo", type=Path, default=Path.cwd())
    parser.add_argument("--shellcheck")
    args = parser.parse_args(argv)
    scripts = tracked(args.repo)
    if args.shellcheck is None:
        for name in scripts:
            print(name)
        return 0
    if not scripts:
        print("shellcheck: no tracked shell script")
        return 0
    return subprocess.run([args.shellcheck, *scripts], cwd=args.repo, check=False).returncode


if __name__ == "__main__":
    sys.exit(main())
