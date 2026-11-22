# SPDX-License-Identifier: Apache-2.0
"""Workspace trust of a bound project folder (FIX-RUNNER-NEW-PROJECT-TRUST, `grant_trust`)."""

from __future__ import annotations

import json
import os
from pathlib import Path

import pytest

from claude_code_runner.workspacetrust import TRUST_KEY, global_config_file, grant_trust


@pytest.fixture
def tree(tmp_path: Path) -> tuple[Path, Path, Path]:
    """A projects root with the folder ``app`` and a global configuration file."""
    root = tmp_path / "projects"
    (root / "app").mkdir(parents=True)
    home = tmp_path / "home"
    (home / ".claude").mkdir(parents=True)
    config = home / ".claude.json"
    config.write_text(json.dumps({"userID": "u", "projects": {"/x": {"allowedTools": []}}}))
    config.chmod(0o600)
    return root, home, config


def grant(config: Path, folder: Path, root: Path, wait: float = 0) -> str:
    return grant_trust(config, folder, root, wait=wait, config_dir=config.parent / ".claude")


def entry(config: Path, folder: Path) -> object:
    return json.loads(config.read_text())["projects"].get(os.path.realpath(folder))


def test_the_bound_folder_gets_its_own_entry(tree: tuple[Path, Path, Path]) -> None:
    root, _, config = tree
    assert grant(config, root / "app", root) == "granted"
    assert entry(config, root / "app") == {TRUST_KEY: True}
    document = json.loads(config.read_text())
    assert document["userID"] == "u"
    assert document["projects"]["/x"] == {"allowedTools": []}
    assert config.stat().st_mode & 0o777 == 0o600
    assert not Path(f"{config}.lock").exists()
    assert grant(config, root / "app", root) == "trusted"


def test_an_entry_without_trust_keeps_its_members(tree: tuple[Path, Path, Path]) -> None:
    root, _, config = tree
    key = os.path.realpath(root / "app")
    config.write_text(json.dumps({"projects": {key: {TRUST_KEY: False, "lastCost": 1}}}))
    assert grant(config, root / "app", root) == "granted"
    assert entry(config, root / "app") == {TRUST_KEY: True, "lastCost": 1}


@pytest.mark.parametrize("where", ["root", "outside", "link_out", "parent"])
def test_nothing_outside_the_root_is_trusted(
    tree: tuple[Path, Path, Path], tmp_path: Path, where: str
) -> None:
    root, _, config = tree
    before = config.read_bytes()
    outside = tmp_path / "outside"
    outside.mkdir()
    folder = {
        "root": root,
        "outside": outside,
        "link_out": root / "link",
        "parent": tmp_path,
    }[where]
    if where == "link_out":
        folder.symlink_to(outside)  # a folder below the root that resolves outside it
    assert grant(config, folder, root) == "outside_root"
    assert config.read_bytes() == before


def test_a_link_inside_the_root_trusts_its_real_path(tree: tuple[Path, Path, Path]) -> None:
    root, _, config = tree
    (root / "alias").symlink_to(root / "app")
    assert grant(config, root / "alias", root) == "granted"
    assert entry(config, root / "app") == {TRUST_KEY: True}
    assert entry(config, root / "alias") in (None, {TRUST_KEY: True})


@pytest.mark.parametrize(
    ("content", "code"),
    [(None, "config_missing"), ("{not json", "config_unreadable"), ("[1]", "config_unreadable")],
)
def test_a_file_that_is_no_configuration_is_not_written(
    tree: tuple[Path, Path, Path], content: str | None, code: str
) -> None:
    root, _, config = tree
    if content is None:
        config.unlink()
    else:
        config.write_text(content)
    assert grant(config, root / "app", root) == code
    if content is None:
        assert not config.exists()
    else:
        assert config.read_text() == content
    assert not Path(f"{config}.lock").exists()


def test_a_legacy_configuration_is_not_written(tree: tuple[Path, Path, Path]) -> None:
    root, home, config = tree
    (home / ".claude" / ".config.json").write_text("{}")
    before = config.read_bytes()
    assert grant(config, root / "app", root) == "legacy_config"
    assert config.read_bytes() == before


def test_a_held_lock_is_waited_for_and_never_taken(tree: tuple[Path, Path, Path]) -> None:
    root, _, config = tree
    lock = Path(f"{config}.lock")
    lock.mkdir()
    before = config.read_bytes()
    assert grant(config, root / "app", root, wait=0.05) == "locked"
    assert config.read_bytes() == before
    assert lock.is_dir()


def test_a_write_under_the_lock_reads_the_file_again(
    tree: tuple[Path, Path, Path], monkeypatch: pytest.MonkeyPatch
) -> None:
    root, _, config = tree
    import claude_code_runner.workspacetrust as module

    reads = []
    real_read = module._read

    def read(path: Path) -> object:
        reads.append(path)
        if len(reads) == 2:  # the CLI wrote between the first read and the lock
            data = json.loads(path.read_text())
            data["numStartups"] = 9
            path.write_text(json.dumps(data))
        return real_read(path)

    monkeypatch.setattr(module, "_read", read)
    assert grant(config, root / "app", root) == "granted"
    document = json.loads(config.read_text())
    assert document["numStartups"] == 9
    assert entry(config, root / "app") == {TRUST_KEY: True}


def test_the_file_of_the_launch_environment() -> None:
    assert global_config_file({"HOME": "/h"}) == Path("/h/.claude.json")
    assert global_config_file({"HOME": "/h", "CLAUDE_CONFIG_DIR": "/c"}) == Path("/c/.claude.json")
    assert global_config_file({}) is None
