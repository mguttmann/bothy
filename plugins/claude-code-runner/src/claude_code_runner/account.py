# SPDX-License-Identifier: Apache-2.0
"""Account errors of a turn: their class, the retry schedule and the texts (09 18.5, 18.9).

The pinned CLI writes an API error as an ``assistant`` message with ``is_api_error_message`` and
names its kind in the member ``error`` (the enumeration of the CLI's message schema). Four kinds
are errors of the account or of the API's capacity and say nothing about the turn: a used-up
usage window or a rate limit (``rate_limit``), a login that is gone (``authentication_failed``),
subscription access the organization switched off (``oauth_org_not_allowed``) and an overloaded
API (``overloaded``). The same request can succeed a moment later without any change to the
session. `account_error` recognises them by that member, never by the error text, which the
CLI builds from parts.

A process that holds or held background work is therefore not ended at such an error: the
runner keeps it, shows the reason and writes one input line after a short pause, which makes the
CLI run a turn again (`AccountRetry`, `retry_input`; the state is
`claude_code_runner.processstate.ProcessState`). The reset time the CLI names for a usage limit
is shown to the person and never used as a waiting time.

No text of this module carries a value of the host or of the account: the class, a time and
configuration key names only.

Examples:
    >>> error = account_error({"type": "assistant", "is_api_error_message": True,
    ...                        "parent_tool_use_id": None, "error": "overloaded",
    ...                        "message": {"content": []}})
    >>> error.kind, error.describe()
    ('overloaded', 'the API is overloaded')
"""

from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass
from datetime import UTC, datetime
from typing import Any

from claude_code_runner.failures import api_error_text

__all__ = [
    "ACCOUNT_ERRORS",
    "AccountEnd",
    "AccountError",
    "AccountRetry",
    "account_error",
    "retry_input",
    "waiting_line",
]

ACCOUNT_ERRORS: dict[str, str] = {
    "rate_limit": "usage_limit",
    "authentication_failed": "not_logged_in",
    "oauth_org_not_allowed": "subscription_disabled",
    "overloaded": "overloaded",
}
"""The class of an account error by the ``error`` member of the CLI's API error message.

Read in the file of the pinned CLI 2.1.321 on 2026-11-13 (the ``error`` enumeration of its
message schema and the places that build the message): a used-up usage window gives
``rate_limit``, a missing login ``authentication_failed``, an answer that refuses OAuth
authentication for the organization ``oauth_org_not_allowed`` and an answer with status 529
``overloaded`` (09 18.19.12, docs/decisions.d/FIX-RUNNER-LIMIT-KEEPS-RUN.md)."""

_WHAT = {
    "usage_limit": "the usage limit of the account is reached",
    "not_logged_in": "Claude Code is not logged in",
    "subscription_disabled": (
        "the organization of the account has switched off subscription access for Claude Code"
    ),
    "overloaded": "the API is overloaded",
}
_MINUTE = 60.0


@dataclass(frozen=True)
class AccountError:
    """One account error of a turn.

    Attributes:
        kind: The class: ``usage_limit``, ``not_logged_in``, ``subscription_disabled`` or
            ``overloaded``.
        resets_at: When the usage window resets (seconds since the epoch), if the CLI named it.
    """

    kind: str
    resets_at: int | None = None

    def reset_time(self) -> str | None:
        """The reset time as a text in UTC, or ``None`` when the CLI named none.

        Returns:
            For example ``2026-11-13 02:10 UTC``.
        """
        if self.resets_at is None:
            return None
        try:
            moment = datetime.fromtimestamp(self.resets_at, UTC)
        except (OverflowError, OSError, ValueError):
            return None
        return moment.strftime("%Y-%m-%d %H:%M UTC")

    def describe(self) -> str:
        """The error in words for a person, with the reset time when there is one.

        Returns:
            A clause without a full stop, for example "the usage limit of the account is
            reached and resets at 2026-11-13 02:10 UTC".
        """
        text = _WHAT[self.kind]
        reset = self.reset_time()
        return text if reset is None else f"{text} and resets at {reset}"


@dataclass(frozen=True)
class AccountRetry:
    """The pauses of the retries after an account error and their total bound, in seconds.

    Attributes:
        first: ``runner.account_retry_first``: the pause before the first retry.
        second: ``runner.account_retry_second``: the pause before the second retry.
        interval: ``runner.account_retry_interval``: the pause before every further retry.
        max_wait: ``runner.account_retry_max_wait``: once this time has passed since the first
            account error of the wait and no turn runs and no input is queued, the error counts
            as an ordinary error of the turn.
    """

    first: float
    second: float
    interval: float
    max_wait: float

    def pause(self, retries: int) -> float:
        """The pause before the next retry.

        Args:
            retries: The retries written so far in this wait.

        Returns:
            The pause in seconds, counted from the last account error or retry.

        Examples:
            >>> schedule = AccountRetry(60.0, 120.0, 180.0, 1200.0)
            >>> [schedule.pause(n) for n in range(4)]
            [60.0, 120.0, 180.0, 180.0]
        """
        if retries <= 0:
            return self.first
        if retries == 1:
            return self.second
        return self.interval


@dataclass(frozen=True)
class AccountEnd:
    """How an account error ended a turn process, for the run's summary.

    Attributes:
        error: The account error of the failed turn.
        retries: The retries the runner wrote before the process ended; ``None`` when it
            wrote none (the process never had background work, the retry is switched off, or
            the process ended or could not be written to before the first retry).
        held: The process holds or held background work.
    """

    error: AccountError
    retries: int | None = None
    held: bool = False


def account_error(message: Mapping[str, Any]) -> AccountError | None:
    """The account error an API error message of the CLI carries, else ``None``.

    Only the turn's own API error counts (`claude_code_runner.failures.api_error_text`): the
    model's words and an API error of a subagent never start a wait.

    Args:
        message: A stream message.

    Returns:
        The error with its class and reset time, or ``None`` for any other message and for an
        API error of another kind (a full context, an invalid request, a server error).
    """
    if api_error_text(message) is None:
        return None
    code = message.get("error")
    kind = ACCOUNT_ERRORS.get(code) if isinstance(code, str) else None
    if kind is None:
        return None
    return AccountError(kind, _resets_at(message.get("api_error_params")))


def _resets_at(params: object) -> int | None:
    """``rate_limit_info.resetsAt`` of the error's parameters, when it is a time."""
    info = params.get("rate_limit_info") if isinstance(params, Mapping) else None
    resets = info.get("resetsAt") if isinstance(info, Mapping) else None
    if isinstance(resets, bool) or not isinstance(resets, int) or resets <= 0:
        return None
    return resets


def _about(seconds: float) -> str:
    """A pause in words: seconds below a minute, else whole minutes."""
    if seconds < _MINUTE:
        whole = max(1, round(seconds))
        return f"{whole} second" if whole == 1 else f"{whole} seconds"
    minutes = round(seconds / _MINUTE)
    return "about 1 minute" if minutes <= 1 else f"about {minutes} minutes"


def waiting_line(error: AccountError, pause: float) -> str:
    """The status line of a run that waits for the account (``/status_line``, 09 5.3).

    Args:
        error: The account error of the failed turn.
        pause: The pause before the next retry, in seconds.

    Returns:
        The text for the live box.

    Examples:
        >>> line = waiting_line(AccountError("overloaded"), 60.0)
        >>> line.split(". ")[0]
        'Waiting for the Claude Code account: the API is overloaded'
        >>> line.endswith("next try in about 1 minute.")
        True
    """
    return (
        f"Waiting for the Claude Code account: {error.describe()}. The session and its "
        f"background work are kept; next try in {_about(pause)}."
    )


def retry_input(error: AccountError) -> str:
    """The input line of a retry after an account error.

    The failed turn's own input (a message, or the notification of a background task) is still
    part of the conversation and has no answer; a further input line makes the CLI run a turn
    whose request carries both (run against the pinned CLI on 2026-11-13,
    ``tests/real_cli/test_refused_request.py``).

    Args:
        error: The account error of the failed turn.

    Returns:
        The text of the user line; it never starts with ``/``.
    """
    return (
        "Automatic retry by the runner, not a message from a person. The previous turn of this "
        f"session ended early because Claude Code could not use its account ({_WHAT[error.kind]}); "
        "the session and its background tasks were kept. Continue now where that turn stopped: "
        "answer what is still unanswered above (a message, or the notification of a background "
        "task) as you would have, and do not start work again that still runs or is done."
    )
