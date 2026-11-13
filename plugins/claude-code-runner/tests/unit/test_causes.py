# SPDX-License-Identifier: Apache-2.0
"""The cause of a run that ends without a turn, as one sentence for the thread (RUNNER-NO-GIT)."""

from __future__ import annotations

import pytest

from claude_code_runner.account import AccountEnd, AccountError
from claude_code_runner.causes import (
    CAUSES,
    adapter_stopped_cause,
    cause,
    definition_refused,
    turn_end_cause,
)


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


LIMIT = AccountError("usage_limit", 1794535800)
"""A used-up usage window that resets at 2026-11-13 02:10 UTC."""


def test_an_account_error_names_its_class_and_the_reset_time() -> None:
    # FIX-RUNNER-LIMIT-KEEPS-RUN: a run that an account error ended says which one, when the
    # limit resets and what the runner did, never a name of the account.
    text = turn_end_cause("runtime_error", 2, AccountEnd(LIMIT, retries=7, held=True))
    assert text == (
        "Claude Code could not use its account on the runner host: the usage limit of the "
        "account is reached and resets at 2026-11-13 02:10 UTC. The runner kept the session and "
        "tried the turn again 7 times (runner.account_retry_max_wait bounds that wait) before "
        "the run ended. 2 background tasks were still running and were stopped; their results "
        "are lost. Your next message starts a new run that continues the same session "
        "(`runtime_error`)"
    )
    assert len(text) <= 2000  # run.summary_max_chars
    once = turn_end_cause("runtime_error", 0, AccountEnd(LIMIT, retries=1, held=True))
    assert once is not None
    assert "tried the turn again 1 time (" in once


def test_an_account_error_without_background_work_says_why_nothing_waited() -> None:
    text = turn_end_cause("runtime_error", 0, AccountEnd(LIMIT))
    assert text == (
        "Claude Code could not use its account on the runner host: the usage limit of the "
        "account is reached and resets at 2026-11-13 02:10 UTC. This session had no background "
        "work to keep, so the run ended at once instead of waiting and trying again. Your next "
        "message starts a new run that continues the same session (`runtime_error`)"
    )
    # With the retry switched off a held process ends at once too; nothing more to explain.
    off = turn_end_cause("runtime_error", 1, AccountEnd(LIMIT, held=True))
    assert off is not None
    assert "no background work" not in off
    assert "tried the turn again" not in off
    assert "1 background task was still running" in off


def test_a_login_that_is_gone_keeps_its_advice() -> None:
    gone = AccountEnd(AccountError("not_logged_in"), retries=3, held=True)
    text = turn_end_cause("runtime_not_authenticated", 1, gone)
    assert text is not None
    assert text.startswith(
        "Claude Code could not use its account on the runner host: Claude Code is not logged in."
    )
    assert "tried the turn again 3 times" in text
    assert text.endswith(
        "Once the login works again, your next message starts a new run that continues the "
        "same session (`runtime_not_authenticated`)"
    )


def test_definition_missing_names_the_built_in_agent() -> None:
    # 26 5.6 item 4: a hosted agent's run without a definition the runner runs ends with
    # adapter_error and this cause in the summary (09 18.9).
    text = cause("definition_missing")
    assert text == (
        "The runner has no agent definition it can run for this built-in agent yet, so the "
        "run ended without a turn. Set up the agent's definition in the app "
        "(`definition_missing`)"
    )


# ---- A1-05: hosted turns (26 5.6, 5.8) -------------------------------------------------------


def test_cost_cap_names_the_cap_and_what_to_do() -> None:
    text = cause("cost_cap_reached")
    assert "cost cap" in text
    assert text.endswith("(`cost_cap_reached`)")


def test_a_refused_definition_names_the_field_codes_never_the_content() -> None:
    text = definition_refused(["inline_shell", "tool_class_too_low", "inline_shell"])
    assert "`inline_shell`" in text
    assert "`tool_class_too_low`" in text
    assert text.count("inline_shell") == 1
    assert text.endswith("(`tool_class_mismatch`)")
    assert chr(0x2013) not in text
    assert chr(0x2014) not in text
    assert "/" not in text


def test_a_refused_definition_keeps_its_summary_short() -> None:
    text = definition_refused([f"code_{i}" for i in range(60)])
    assert len(text.encode()) <= 400
    assert text.endswith("(`tool_class_mismatch`)")


# ---- FIX-runner-silent-stop (issue 553) ------------------------------------------------------


def test_a_run_that_ended_names_that_nothing_waits() -> None:
    # The owner sees at once that the thread stopped and what continues it.
    text = cause("run_ended_idle")
    assert "nothing is waiting" in text
    assert "sends a message" in text
    assert text.endswith("(`run_ended_idle`)")


def test_a_stop_without_background_work_keeps_its_cause() -> None:
    assert adapter_stopped_cause(0) == cause("adapter_stopped")


@pytest.mark.parametrize(
    ("lost", "sentence"),
    [
        (1, "1 background task was still running and was stopped; its result is lost."),
        (2, "2 background tasks were still running and were stopped; their results are lost."),
    ],
)
def test_a_stop_names_the_background_tasks_it_ended(lost: int, sentence: str) -> None:
    text = adapter_stopped_cause(lost)
    assert text.startswith("The runner was stopped (a restart or an update)")
    assert sentence in text
    assert text.index(sentence) < text.index("Send a message")
    assert text.endswith("(`adapter_stopped`)")
    assert chr(0x2013) not in text
    assert chr(0x2014) not in text
    assert len(text.encode()) <= 400
