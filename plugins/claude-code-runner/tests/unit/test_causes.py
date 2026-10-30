# SPDX-License-Identifier: Apache-2.0
"""The cause of a run that ends without a turn, as one sentence for the thread (RUNNER-NO-GIT)."""

from __future__ import annotations

import pytest

from claude_code_runner.causes import CAUSES, cause


def test_unbound_forum_names_what_to_do() -> None:
    text = cause("project_not_bound")
    assert "no project folder" in text
    assert "Bind a folder" in text
    assert "runner.default_workspace" in text
    assert "`project_not_bound`" in text


def test_default_workspace_problems_name_the_key() -> None:
    for code in ("workspace_missing", "not_writable", "outside_root"):
        text = cause(code, default_workspace=True)
        assert "runner.default_workspace" in text
        assert f"`{code}`" in text
        assert text != cause(code)


@pytest.mark.parametrize("code", sorted(CAUSES))
def test_every_cause_is_a_sentence_with_its_code(code: str) -> None:
    text = cause(code)
    assert text.endswith(f"(`{code}`)")
    assert text[0].isupper()
    assert chr(0x2013) not in text  # no en dash
    assert chr(0x2014) not in text  # no em dash
    assert len(text.encode()) <= 400


def test_unknown_code_still_names_it() -> None:
    assert cause("something_new") == (
        "The runner could not start Claude Code; its log has the details (`something_new`)"
    )


def test_a_value_is_never_part_of_the_text() -> None:
    # Only codes and key names leave the runner: a path of the host would be private.
    assert all("/" not in text for text in CAUSES.values())
