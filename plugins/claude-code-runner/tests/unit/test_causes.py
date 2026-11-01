# SPDX-License-Identifier: Apache-2.0
"""The cause of a run that ends without a turn, as one sentence for the thread (RUNNER-NO-GIT)."""

from __future__ import annotations

import pytest

from claude_code_runner.causes import CAUSES, cause, turn_end_cause


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


def test_login_lost_at_a_turn_names_what_happened_and_what_follows() -> None:
    # FIX-runner-auth-failed-held: the owner learns why the run ended and that his next
    # message continues the same session.
    text = turn_end_cause("runtime_not_authenticated", 0)
    assert text is not None
    assert text.startswith("Claude Code lost its login on the runner host during this run")
    assert "next message" in text
    assert "same session" in text
    assert "background" not in text
    assert text.endswith("(`runtime_not_authenticated`)")


@pytest.mark.parametrize(
    ("lost", "sentence"),
    [
        (1, "1 background task was still running and was stopped; its result is lost."),
        (3, "3 background tasks were still running and were stopped; their results are lost."),
    ],
)
def test_lost_background_tasks_are_reported(lost: int, sentence: str) -> None:
    for reason in ("runtime_not_authenticated", "runtime_error"):
        text = turn_end_cause(reason, lost)
        assert text is not None
        assert sentence in text
        assert text.endswith(f"(`{reason}`)")
        assert chr(0x2013) not in text
        assert chr(0x2014) not in text
        assert len(text.encode()) <= 400


def test_an_error_without_lost_work_keeps_its_box_note_only() -> None:
    assert turn_end_cause("runtime_error", 0) is None
