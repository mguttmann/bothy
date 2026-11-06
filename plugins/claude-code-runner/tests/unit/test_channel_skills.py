# SPDX-License-Identifier: Apache-2.0
"""Channel skills behind ``runner.channel_skills_dir``, only in ``host`` turns (26 5.9, BA-T22).

A channel skill folder carrying ``allowed-tools: Bash``, ``hooks``, both inline shell forms and a
``hooks/`` folder is bound to channels whose turns run with ``chat``, ``scoped`` and ``host``:
only the ``host`` resolution hands it out as a plugin folder; below ``host`` the reference is
skipped, logged and named for the live box. References of another adapter and names that fail
the pattern are skipped with a debug line only; a missing folder is named, not fatal. Without
the key nothing is read at all. The command lines of both agents per class are in
``tests/scenarios/test_hosted_turns.py``.
"""

from __future__ import annotations

import json
import logging
from pathlib import Path

import pytest

from claude_code_runner.channel_skills import ChannelSkills, resolve_channel_skills

DANGEROUS = (
    "---\nname: review\ndescription: Reviews\nallowed-tools: Bash\n"
    "hooks:\n  PreToolUse:\n    - command: touch /tmp/x\n---\n"
    "Run !`touch marker` now.\n\n```!\ntouch marker\n```\n"
)


@pytest.fixture
def skills_dir(tmp_path: Path) -> Path:
    base = tmp_path / "channel-skills"
    folder = base / "review"
    (folder / "hooks").mkdir(parents=True)
    (folder / "SKILL.md").write_text(DANGEROUS)
    (folder / "hooks" / "hooks.json").write_text('{"hooks": {}}')
    (base / "empty").mkdir()
    return base


REFS = ["claude-code:review", "other-adapter:notes", "claude-code:Bad_Name", "claude-code:gone"]


def test_host_gets_the_folder_and_names_a_missing_one(skills_dir: Path) -> None:
    found = resolve_channel_skills(REFS, skills_dir, "host")
    assert found.plugin_dirs == (skills_dir / "review",)
    assert found.skipped == ()
    assert found.missing == ("claude-code:gone",)
    assert found.notice() == "Channel skill not found on the runner host: claude-code:gone"


@pytest.mark.parametrize("tool_class", ["chat", "scoped"])
def test_below_host_every_claude_code_reference_is_skipped(
    skills_dir: Path, tool_class: str, caplog: pytest.LogCaptureFixture
) -> None:
    with caplog.at_level(logging.DEBUG, logger="claude_code_runner"):
        found = resolve_channel_skills(REFS, skills_dir, tool_class)
    assert found.plugin_dirs == ()
    assert found.skipped == ("claude-code:review", "claude-code:gone")
    assert found.missing == ()
    assert found.notice() == (
        "Channel skills need tool class host, so this turn skipped: "
        "claude-code:review, claude-code:gone"
    )
    lines = [json.loads(r.getMessage()) for r in caplog.records]
    skipped = [line for line in lines if line["event"] == "channel_skill_skipped"]
    assert [line["skill"] for line in skipped] == ["review", "gone"]
    assert all(line["reason"] == "needs_host" for line in skipped)
    foreign = [line for line in lines if line["event"] == "channel_skill_foreign"]
    assert len(foreign) == 2  # the other adapter's reference and the name off the pattern
    assert all(r.levelno == logging.DEBUG for r in caplog.records if "foreign" in r.getMessage())
    assert "Bad_Name" not in caplog.text


def test_without_the_key_nothing_is_read(tmp_path: Path) -> None:
    found = resolve_channel_skills(REFS, None, "host")
    assert found == ChannelSkills()
    assert found.notice() is None


def test_a_folder_without_skill_md_counts_as_missing(skills_dir: Path) -> None:
    found = resolve_channel_skills(["claude-code:empty"], skills_dir, "host")
    assert found.plugin_dirs == ()
    assert found.missing == ("claude-code:empty",)


def test_both_notes_in_one_line(skills_dir: Path) -> None:
    found = ChannelSkills(skipped=("claude-code:a",), missing=("claude-code:b",))
    assert found.notice() == (
        "Channel skills need tool class host, so this turn skipped: claude-code:a. "
        "Channel skill not found on the runner host: claude-code:b"
    )


def test_a_reference_is_loaded_once(skills_dir: Path) -> None:
    found = resolve_channel_skills(["claude-code:review", "claude-code:review"], skills_dir, "host")
    assert found.plugin_dirs == (skills_dir / "review",)
