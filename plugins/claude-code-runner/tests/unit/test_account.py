# SPDX-License-Identifier: Apache-2.0
"""Account errors of a turn (docs/spec/09-agents.md 18.9, FIX-RUNNER-LIMIT-KEEPS-RUN).

The messages are the ones Fake-Claude writes from its fixtures, whose members are those the
pinned CLI wrote for each error (``tests/fake_claude/fixtures``).
"""

from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any

import pytest

from claude_code_runner.account import (
    ACCOUNT_ERRORS,
    AccountError,
    AccountRetry,
    account_error,
    retry_input,
    waiting_line,
)

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "fake_claude"))
import fake_claude

RESET = 1794535800
"""The reset time of the fixture ``usage_limit``: 2026-11-13 02:10 UTC."""


class _Recorder(fake_claude.Fake):
    def __init__(self) -> None:
        super().__init__({}, Path("unused"))
        self.out: list[dict[str, Any]] = []

    def emit(self, message: dict[str, Any]) -> None:
        self.out.append(json.loads(json.dumps(message)))


def cli_error(fixture: str) -> tuple[dict[str, Any], dict[str, Any]]:
    """The API error message and its result as Fake-Claude writes them for ``fixture``."""
    fake = _Recorder()
    fake.emit_api_error(*fake_claude._error(fixture))
    message, result = fake.out
    return message, result


@pytest.mark.parametrize(
    ("fixture", "kind"),
    [
        ("usage_limit", "usage_limit"),
        ("not_authenticated", "not_logged_in"),
        ("subscription_disabled", "subscription_disabled"),
        ("overloaded", "overloaded"),
    ],
)
def test_the_four_account_errors_are_recognised_by_their_member(fixture: str, kind: str) -> None:
    message, _ = cli_error(fixture)
    error = account_error(message)
    assert error is not None
    assert error.kind == kind
    assert set(ACCOUNT_ERRORS.values()) >= {kind}


def test_the_reset_time_comes_from_the_members_never_from_the_text() -> None:
    message, _ = cli_error("usage_limit")
    error = account_error(message)
    assert error == AccountError("usage_limit", RESET)
    assert error.reset_time() == "2026-11-13 02:10 UTC"
    assert error.describe() == (
        "the usage limit of the account is reached and resets at 2026-11-13 02:10 UTC"
    )
    # The CLI's own text is built from parts and names the host's time zone: never repeated.
    assert "2:10am" not in error.describe()


@pytest.mark.parametrize("fixture", ["server_error", "context_full", "cli_too_old"])
def test_any_other_api_error_is_no_account_error(fixture: str) -> None:
    message, _ = cli_error(fixture)
    assert account_error(message) is None


def test_only_the_turns_own_api_error_counts() -> None:
    message, result = cli_error("usage_limit")
    assert account_error({**message, "parent_tool_use_id": "toolu_1"}) is None  # a subagent's
    assert account_error({**message, "is_api_error_message": False}) is None  # the model's words
    assert account_error(result) is None
    assert account_error({**message, "error": 429}) is None


@pytest.mark.parametrize(
    "params",
    [
        None,
        {},
        {"rate_limit_info": None},
        {"rate_limit_info": {"resetsAt": "soon"}},
        {"rate_limit_info": {"resetsAt": True}},
        {"rate_limit_info": {"resetsAt": 0}},
    ],
)
def test_a_limit_without_a_reset_time_names_none(params: object) -> None:
    message, _ = cli_error("usage_limit")
    error = account_error({**message, "api_error_params": params})
    assert error == AccountError("usage_limit", None)
    assert error.describe() == "the usage limit of the account is reached"


def test_a_reset_time_out_of_range_is_left_out() -> None:
    assert AccountError("usage_limit", 10**18).reset_time() is None


def test_the_pauses_are_first_second_then_the_interval() -> None:
    schedule = AccountRetry(first=60.0, second=120.0, interval=180.0, max_wait=1200.0)
    assert [schedule.pause(n) for n in range(5)] == [60.0, 120.0, 180.0, 180.0, 180.0]


def test_the_waiting_line_names_the_class_the_reset_time_and_the_next_try() -> None:
    line = waiting_line(AccountError("usage_limit", RESET), 60.0)
    assert line == (
        "Waiting for the Claude Code account: the usage limit of the account is reached and "
        "resets at 2026-11-13 02:10 UTC. The session and its background work are kept; next try "
        "in about 1 minute."
    )
    assert waiting_line(AccountError("overloaded"), 180.0).endswith("in about 3 minutes.")
    assert waiting_line(AccountError("not_logged_in"), 0.2).endswith("next try in 1 second.")
    assert len(line) <= 500  # run.status_line_max_chars


@pytest.mark.parametrize("kind", sorted(set(ACCOUNT_ERRORS.values())))
def test_no_text_carries_a_dash_or_a_value_of_the_account(kind: str) -> None:
    error = AccountError(kind, RESET)
    for text in (error.describe(), waiting_line(error, 60.0), retry_input(error)):
        assert chr(0x2013) not in text
        assert chr(0x2014) not in text
        assert "@" not in text
        assert "/" not in text  # no path and no time zone name of the host


def test_the_retry_input_says_what_it_is_and_never_becomes_a_command() -> None:
    text = retry_input(AccountError("usage_limit", RESET))
    assert text.startswith("Automatic retry by the runner, not a message from a person.")
    assert "the usage limit of the account is reached" in text
    assert "do not start work again" in text
    assert "2026" not in text  # the reset time is for the person, not for the agent
