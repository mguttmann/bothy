# SPDX-License-Identifier: Apache-2.0
"""Copy the files a commit could contain into a directory, for scanners that walk directories.

Some scanners (trivy, `gitleaks dir`) take a directory and walk the file system. In a
working copy that also holds ignored content (other worktrees under `.claude/worktrees/`,
caches, `node_modules/`, the built site), their result would depend on the machine and
differ from CI. This tool copies every tracked file and every untracked file that
`.gitignore` does not exclude (`git ls-files --cached --others --exclude-standard`), in
its working-tree state, so a scanner sees what CI sees plus the changes not yet committed.

Usage: python tools/ci/tracked_tree.py [--repo DIR] DEST   (DEST must be empty or absent)
"""

from __future__ import annotations

import argparse
import shutil
import subprocess
import sys
from pathlib import Path


def committable(root: Path) -> list[str]:
    """Tracked plus untracked, not ignored paths that exist as files (or symlinks)."""
    out = subprocess.run(
        [
            "git",
            "-C",
            str(root),
            "ls-files",
            "-z",
            "--cached",
            "--others",
            "--exclude-standard",
            "--deduplicate",
        ],
        check=True,
        capture_output=True,
    ).stdout
    names = sorted({p for p in out.decode("utf-8").split("\0") if p})
    return [n for n in names if (root / n).is_symlink() or (root / n).is_file()]


def snapshot(root: Path, dest: Path) -> int:
    """Copy `committable(root)` to `dest`; return the number of copied files."""
    if dest.exists() and any(dest.iterdir()):
        raise ValueError(f"{dest} is not empty")
    names = committable(root)
    for name in names:
        target = dest / name
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(root / name, target, follow_symlinks=False)
    return len(names)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Copy the committable files of a repository.")
    parser.add_argument("--repo", type=Path, default=Path.cwd())
    parser.add_argument("dest", type=Path)
    args = parser.parse_args(argv)
    try:
        count = snapshot(args.repo, args.dest)
    except ValueError as exc:
        print(f"tracked_tree: {exc}", file=sys.stderr)
        return 2
    print(f"tracked_tree: {count} file(s) copied to {args.dest}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
