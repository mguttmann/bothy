# SPDX-License-Identifier: Apache-2.0
"""Run a code generator and fail if it changed what git would see (gate `codegen-check`).

Generated files (sqlc code, `.go-arch-lint.yml`, API code) must equal a fresh generation
(docs/spec/02-architecture.md "Other checks in the gate", ADR 0007 "Confirmation"). The
tool hashes every file git would see under the directory (tracked plus untracked, not
ignored), runs the generator there, hashes again and compares.

Why snapshots and not `git diff --exit-code`: a developer runs `make check` on a dirty
tree, and an uncommitted hand edit is not drift. Only changes the generator makes count.

Usage: codegen_check.py --dir <path> -- <command> [args...]

Exit codes: 0 no drift; 1 the generator changed, added or removed files; the generator's
own exit code when it fails (126 when it is not executable, 127 when it cannot be found,
128 + n when signal n ended it); 2 usage error.
"""

from __future__ import annotations

import argparse
import hashlib
import os
import shlex
import subprocess
import sys
from pathlib import Path

USAGE = "codegen_check.py --dir <path> -- <command> [args...]"

# git exports these to hooks (the pre-push hook runs `make check`); with GIT_DIR set, git
# would take the directory for the top of the working tree and skip the .gitignore files
# above it. Without them git finds the repository from the directory, as it does by hand.
_GIT_LOCATION_VARIABLES = frozenset(
    {"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_PREFIX"}
)


def snapshot(directory: Path) -> dict[str, str]:
    """Hash every file git would see under `directory`.

    Lists tracked and untracked, not ignored paths (`git ls-files --cached --others
    --exclude-standard`) relative to `directory`; a tracked file missing from the working
    tree is left out, a symbolic link is hashed by its target. Variables that point git at
    another repository (as git sets them for hooks) are not passed on.

    Args:
        directory: a directory inside a git working tree.

    Returns:
        Path relative to `directory` mapped to the sha256 of its content.

    Raises:
        subprocess.CalledProcessError: `directory` is not inside a git working tree.
    """
    listing = subprocess.run(
        ["git", "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", "."],
        cwd=directory,
        env={k: v for k, v in os.environ.items() if k not in _GIT_LOCATION_VARIABLES},
        check=True,
        capture_output=True,
    ).stdout
    hashes: dict[str, str] = {}
    for name in sorted({p for p in os.fsdecode(listing).split("\0") if p}):
        path = directory / name
        if path.is_symlink():
            hashes[name] = hashlib.sha256(b"symlink:" + os.fsencode(path.readlink())).hexdigest()
        elif path.is_file():
            hashes[name] = hashlib.sha256(path.read_bytes()).hexdigest()
    return hashes


def compare(before: dict[str, str], after: dict[str, str]) -> list[str]:
    """Describe the difference of two snapshots.

    Args:
        before: snapshot taken before the generator ran.
        after: snapshot taken after it.

    Returns:
        Sorted lines `changed: <path>`, `added: <path>` and `removed: <path>`; empty when
        both snapshots are equal.

    Example:
        >>> compare({"a.go": "1"}, {"a.go": "2", "b.go": "3"})
        ['added: b.go', 'changed: a.go']
    """
    lines = [f"changed: {p}" for p in before.keys() & after.keys() if before[p] != after[p]]
    lines += [f"added: {p}" for p in after.keys() - before.keys()]
    lines += [f"removed: {p}" for p in before.keys() - after.keys()]
    return sorted(lines)


def _run(command: list[str], directory: Path) -> int:
    """Run the generator in `directory`; return an exit code a shell would report."""
    try:
        code = subprocess.run(command, cwd=directory, check=False).returncode
    except FileNotFoundError:
        print(f"codegen-check: {command[0]}: command not found", file=sys.stderr)
        return 127
    except PermissionError:
        print(f"codegen-check: {command[0]}: permission denied", file=sys.stderr)
        return 126
    return 128 - code if code < 0 else code


def main(argv: list[str] | None = None) -> int:
    """Command line entry point; see the module docstring for usage and exit codes."""
    args = list(sys.argv[1:] if argv is None else argv)
    split = args.index("--") if "--" in args else len(args)
    options, command = args[:split], args[split + 1 :]
    parser = argparse.ArgumentParser(prog="codegen_check.py", usage=USAGE)
    parser.add_argument("--dir", type=Path, required=True, help="directory to run in")
    try:
        directory: Path = parser.parse_args(options).dir
    except SystemExit as exc:  # argparse ends on usage errors (2) and on --help (0)
        return exc.code if isinstance(exc.code, int) else 2
    if not command:
        print(f"usage: {USAGE}\ncodegen-check: no generator command given", file=sys.stderr)
        return 2
    if not directory.is_dir():
        print(f"codegen-check: {directory} is not a directory", file=sys.stderr)
        return 2
    try:
        before = snapshot(directory)
    except subprocess.CalledProcessError:
        print(f"codegen-check: {directory} is not inside a git working tree", file=sys.stderr)
        return 2
    shown = shlex.join(command)
    code = _run(command, directory)
    if code != 0:
        print(f"codegen-check: {shown} failed with exit code {code}", file=sys.stderr)
        return code
    drift = compare(before, snapshot(directory))
    if drift:
        print("\n".join(drift))
        print(
            f"codegen-check: generated code in {directory} was stale; commit the regenerated files"
        )
        return 1
    print(f"codegen-check: {directory} up to date ({shown})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
