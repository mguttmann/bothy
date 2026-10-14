# SPDX-License-Identifier: Apache-2.0
"""Integrity of the read-only checkout and of the installed files."""

from __future__ import annotations

import json
import os
import subprocess
from pathlib import Path

import pytest
from hermes_harness.integrity import (
    CheckoutState,
    IntegrityError,
    assert_unchanged,
    installed_digest,
    make_read_only,
    restore_writable,
    snapshot_checkout,
    tree_digest,
)


def _tree(root: Path) -> Path:
    (root / "pkg").mkdir(parents=True)
    (root / "pkg" / "a.py").write_text("a = 1\n", encoding="utf-8")
    (root / "pkg" / "b.txt").write_text("b\n", encoding="utf-8")
    (root / "top.txt").write_text("top\n", encoding="utf-8")
    (root / "link").symlink_to("top.txt")
    return root


def _git(cwd: Path, *args: str) -> str:
    env = {
        "PATH": os.environ["PATH"],
        "HOME": str(cwd),
        "GIT_CONFIG_NOSYSTEM": "1",
        "GIT_AUTHOR_NAME": "Alex",
        "GIT_AUTHOR_EMAIL": "alex@example.com",
        "GIT_COMMITTER_NAME": "Alex",
        "GIT_COMMITTER_EMAIL": "alex@example.com",
    }
    return subprocess.run(
        ["git", *args], cwd=cwd, env=env, check=True, capture_output=True, text=True
    ).stdout


def test_tree_digest_is_stable(tmp_path: Path) -> None:
    root = _tree(tmp_path / "t")
    first = tree_digest(root)
    assert first == tree_digest(root)
    assert len(first) == 64
    other = _tree(tmp_path / "u")
    assert tree_digest(other) == first


@pytest.mark.parametrize("change", ["modified", "added", "removed", "mode", "symlink"])
def test_tree_digest_detects_change(tmp_path: Path, change: str) -> None:
    root = _tree(tmp_path / "t")
    before = tree_digest(root)
    if change == "modified":
        (root / "pkg" / "a.py").write_text("a = 2\n", encoding="utf-8")
    elif change == "added":
        (root / "pkg" / "c.py").write_text("", encoding="utf-8")
    elif change == "removed":
        (root / "pkg" / "b.txt").unlink()
    elif change == "mode":
        (root / "top.txt").chmod(0o755)
    else:
        (root / "link").unlink()
        (root / "link").symlink_to("pkg/b.txt")
    assert tree_digest(root) != before


def test_tree_digest_sees_an_added_empty_directory_and_honours_excludes(tmp_path: Path) -> None:
    root = _tree(tmp_path / "t")
    before = tree_digest(root)
    (root / "empty").mkdir()
    assert tree_digest(root) != before
    assert tree_digest(root, exclude=("empty",)) == before


def test_tree_digest_names_the_paths_it_covers(tmp_path: Path) -> None:
    root = _tree(tmp_path / "t")
    entries = tree_digest(root, entries=True)
    assert isinstance(entries, dict)
    assert sorted(entries) == ["link", "pkg/", "pkg/a.py", "pkg/b.txt", "top.txt"]


def test_checkout_state_detects_moved_head_and_untracked_files(tmp_path: Path) -> None:
    repo = tmp_path / "repo"
    repo.mkdir()
    _git(repo, "init", "-q")
    (repo / "f.txt").write_text("one\n", encoding="utf-8")
    _git(repo, "add", "f.txt")
    _git(repo, "commit", "-q", "-m", "one")
    before = snapshot_checkout(repo)
    assert len(before.head) == 40
    assert before.status == ""
    assert_unchanged(before, snapshot_checkout(repo))

    (repo / "untracked.txt").write_text("x\n", encoding="utf-8")
    with pytest.raises(IntegrityError, match=r"untracked\.txt"):
        assert_unchanged(before, snapshot_checkout(repo))
    (repo / "untracked.txt").unlink()

    (repo / "f.txt").write_text("two\n", encoding="utf-8")
    _git(repo, "commit", "-q", "-am", "two")
    with pytest.raises(IntegrityError, match="HEAD"):
        assert_unchanged(before, snapshot_checkout(repo))


def test_read_only_checkout_refuses_writes_and_release_restores_them(tmp_path: Path) -> None:
    root = _tree(tmp_path / "t")
    make_read_only(root)
    with pytest.raises(PermissionError):
        (root / "pkg" / "a.py").write_text("changed\n", encoding="utf-8")
    with pytest.raises(PermissionError):
        (root / "pkg" / "new.py").write_text("", encoding="utf-8")
    restore_writable(root)
    (root / "pkg" / "a.py").write_text("changed\n", encoding="utf-8")
    (root / "pkg" / "new.py").write_text("", encoding="utf-8")


def test_assert_unchanged_names_each_path(tmp_path: Path) -> None:
    before = CheckoutState(head="a" * 40, status="", digest="d1", entries={"x": "1", "y": "1"})
    after = CheckoutState(head="a" * 40, status="", digest="d2", entries={"x": "2", "z": "1"})
    with pytest.raises(IntegrityError) as caught:
        assert_unchanged(before, after)
    text = str(caught.value)
    assert "changed: x" in text
    assert "added: z" in text
    assert "removed: y" in text


def test_assert_unchanged_compares_plain_digests() -> None:
    assert_unchanged("abc", "abc")
    with pytest.raises(IntegrityError, match="installed files"):
        assert_unchanged("abc", "abd")


def _dist(site: Path, name: str, files: dict[str, str], editable: Path | None = None) -> None:
    info = site / f"{name}-1.0.dist-info"
    info.mkdir(parents=True)
    rows = []
    for rel, text in files.items():
        path = site / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
        rows.append(f"{rel},,")
    rows.append(f"{info.name}/RECORD,,")
    (info / "RECORD").write_text("\n".join(rows) + "\n", encoding="utf-8")
    if editable is not None:
        direct = {"url": editable.as_uri(), "dir_info": {"editable": True}}
        (info / "direct_url.json").write_text(json.dumps(direct), encoding="utf-8")


def test_installed_digest_covers_records_and_editable_sources(tmp_path: Path) -> None:
    venv = tmp_path / "venv"
    site = venv / "lib" / "python3.13" / "site-packages"
    source = tmp_path / "export"
    (source / "pkg").mkdir(parents=True)
    (source / "pkg" / "mod.py").write_text("x = 1\n", encoding="utf-8")
    _dist(site, "dep", {"dep/__init__.py": "y = 1\n"})
    _dist(site, "tool", {"__editable__.tool.pth": str(source)}, editable=source)
    first = installed_digest(venv)
    assert first == installed_digest(venv)

    (source / "pkg" / "mod.py").write_text("x = 2\n", encoding="utf-8")
    second = installed_digest(venv)
    assert second != first

    (site / "dep" / "__init__.py").write_text("y = 2\n", encoding="utf-8")
    third = installed_digest(venv)
    assert third != second

    _dist(site, "lazy", {"lazy/__init__.py": ""})
    assert installed_digest(venv) != third


def test_installed_digest_requires_a_site_packages_directory(tmp_path: Path) -> None:
    with pytest.raises(IntegrityError, match="site-packages"):
        installed_digest(tmp_path)
