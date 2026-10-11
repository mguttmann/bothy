# SPDX-License-Identifier: Apache-2.0
"""Throw-away git repositories for tests, with their own identity.

Tests run without a git identity, like a fresh CI runner (tools/conftest.py). Every fixture
that commits or tags therefore builds its repository with `init_repo`, which writes a fixed
test identity into the repository's own configuration, together with the settings that keep
a developer's machine out of the result: no signing, no hooks. Code under test that reads
the identity (`git var`) sees the same one.
"""

from __future__ import annotations

import os
import subprocess
from collections.abc import Mapping
from pathlib import Path

NAME = "Alex Example"
EMAIL = "alex@example.com"

_REPO_CONFIG = (
    ("user.name", NAME),
    ("user.email", EMAIL),
    ("commit.gpgsign", "false"),
    ("tag.gpgsign", "false"),
    ("core.hooksPath", os.devnull),
)


def git(repo: Path, *args: str, env: Mapping[str, str] | None = None) -> str:
    """Run git in `repo` and return its standard output; `env` adds to the environment."""
    return subprocess.run(
        ["git", "-C", str(repo), *args],
        check=True,
        capture_output=True,
        text=True,
        env={**os.environ, **env} if env else None,
        timeout=60,
    ).stdout


def init_repo(path: Path) -> Path:
    """Create an empty repository on branch main with the test identity; return its path."""
    path.mkdir(parents=True, exist_ok=True)
    git(path, "init", "-q", "-b", "main")
    for key, value in _REPO_CONFIG:
        git(path, "config", key, value)
    return path


def commit(repo: Path, message: str, *, author: str | None = None, signoff: bool = False) -> None:
    """Stage every change and commit it, also when nothing changed.

    `author` ("Name <email>") replaces the test identity as author and committer.
    """
    env = None
    if author is not None:
        name, email = author.removesuffix(">").split(" <")
        env = {
            "GIT_AUTHOR_NAME": name,
            "GIT_AUTHOR_EMAIL": email,
            "GIT_COMMITTER_NAME": name,
            "GIT_COMMITTER_EMAIL": email,
        }
    git(repo, "add", "-A")
    git(repo, "commit", "-q", "--allow-empty", *(["-s"] if signoff else []), "-m", message, env=env)
