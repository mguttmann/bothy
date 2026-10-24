# SPDX-License-Identifier: Apache-2.0
"""Append-only registries merge as a union of lines (.gitattributes, docs/DECISIONS.md PL/27).

Two branches that each append a line at the end of the same registry conflict in a plain
merge. With `merge=union` a local rebase, cherry-pick or merge keeps both lines. GitHub's
own merge ignores git merge drivers, so this helps the local rebuild of a branch, not the
mergeability shown on a pull request.

The tests read the real `.gitattributes` and the real registry files: every registry is
tracked and carries the attribute, files with a structure that a union would break do not,
and a cherry-pick in a fixture repository with that `.gitattributes` keeps both lines.
"""

from __future__ import annotations

import shutil
import subprocess
from pathlib import Path

import pytest
from git_fixtures import commit, git, init_repo

REPO = Path(__file__).resolve().parents[3]

# Append-only line registries: each line stands alone, new lines go at the end.
REGISTRIES = (
    "CHANGELOG.md",
    "services/server/CHANGELOG.md",
    "services/host/CHANGELOG.md",
    "sdk/python/CHANGELOG.md",
    "sdk/typescript/CHANGELOG.md",
    "packages/bothy_noise/CHANGELOG.md",
    "plugins/hermes-adapter/CHANGELOG.md",
    "plugins/grafana-bothy-live/CHANGELOG.md",
    "plugins/claude-code-runner/CHANGELOG.md",
    "docs/DECISIONS.md",
    "services/server/migrations.sha256",
    "tools/docs/language-allow.txt",
)

# Shared files whose structure a union of lines would break (code, JSON, YAML, tables
# with an order): they keep the normal three-way merge and conflict.
STRUCTURED = (
    "services/server/api/routes/real_test.go",
    "services/server/architecture/config-schema.json",
    "services/server/architecture/modules.yaml",
    "services/server/api/operations.yaml",
    "docs/quality/gates.md",
    "Makefile",
    "osv-scanner.toml",
)


def merge_attribute(repo: Path, path: str) -> str:
    out = git(repo, "check-attr", "merge", "--", path)
    return out.strip().rsplit(": ", 1)[1]


@pytest.mark.parametrize("path", REGISTRIES)
def test_registry_is_tracked_and_merges_as_union(path: str) -> None:
    assert git(REPO, "ls-files", "--error-unmatch", "--", path).strip() == path
    assert merge_attribute(REPO, path) == "union"


@pytest.mark.parametrize("path", STRUCTURED)
def test_structured_file_keeps_the_normal_merge(path: str) -> None:
    assert git(REPO, "ls-files", "--error-unmatch", "--", path).strip() == path
    assert merge_attribute(REPO, path) == "unspecified"


def test_every_union_entry_of_gitattributes_is_a_listed_registry() -> None:
    patterns = [
        line.split()[0]
        for line in (REPO / ".gitattributes").read_text(encoding="utf-8").splitlines()
        if "merge=union" in line.split()[1:]
    ]
    assert patterns, "no merge=union entry in .gitattributes"
    tracked = git(REPO, "ls-files", "-z")
    attributes = subprocess.run(
        ["git", "-C", str(REPO), "check-attr", "-z", "--stdin", "merge"],
        input=tracked,
        check=True,
        capture_output=True,
        text=True,
        timeout=60,
    ).stdout.split("\0")
    # -z output is path, attribute, value, repeated.
    union = {
        attributes[i] for i in range(0, len(attributes) - 2, 3) if attributes[i + 2] == "union"
    }
    assert union == set(REGISTRIES)


def _fixture(tmp_path: Path, path: str) -> Path:
    repo = init_repo(tmp_path / "repo")
    shutil.copyfile(REPO / ".gitattributes", repo / ".gitattributes")
    target = repo / path
    target.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(REPO / path, target)
    commit(repo, "base")
    return repo


def _append(repo: Path, path: str, line: str) -> None:
    with (repo / path).open("a", encoding="utf-8") as handle:
        handle.write(line + "\n")
    commit(repo, f"append {line}")


@pytest.mark.parametrize("path", REGISTRIES)
def test_cherry_pick_onto_a_moved_main_keeps_both_appended_lines(tmp_path: Path, path: str) -> None:
    repo = _fixture(tmp_path, path)
    git(repo, "switch", "-q", "-c", "ticket")
    _append(repo, path, "ticket line")
    git(repo, "switch", "-q", "main")
    _append(repo, path, "main line")

    git(repo, "cherry-pick", "ticket")

    lines = (repo / path).read_text(encoding="utf-8").splitlines()
    assert lines[-2:] == ["main line", "ticket line"]
    assert (REPO / path).read_text(encoding="utf-8").splitlines() == lines[:-2]


def test_migration_sums_stay_sorted_when_the_ticket_has_the_next_number(tmp_path: Path) -> None:
    path = "services/server/migrations.sha256"
    repo = _fixture(tmp_path, path)
    git(repo, "switch", "-q", "-c", "ticket")
    _append(repo, path, f"{'b' * 64}  99992_ticket.sql")
    git(repo, "switch", "-q", "main")
    _append(repo, path, f"{'a' * 64}  99991_main.sql")

    git(repo, "cherry-pick", "ticket")

    names = [line.split("  ", 1)[1] for line in (repo / path).read_text().splitlines()]
    assert names == sorted(names)
    assert names[-2:] == ["99991_main.sql", "99992_ticket.sql"]


def test_a_structured_file_still_conflicts(tmp_path: Path) -> None:
    path = "services/server/architecture/modules.yaml"
    repo = _fixture(tmp_path, path)
    git(repo, "switch", "-q", "-c", "ticket")
    _append(repo, path, "# ticket")
    git(repo, "switch", "-q", "main")
    _append(repo, path, "# main")

    run = subprocess.run(
        ["git", "-C", str(repo), "cherry-pick", "ticket"],
        check=False,
        capture_output=True,
        text=True,
        timeout=60,
    )

    assert run.returncode != 0
    assert git(repo, "diff", "--name-only", "--diff-filter=U").split() == [path]
