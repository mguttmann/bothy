# SPDX-License-Identifier: Apache-2.0
"""The cause of a run that ends without a turn or with ``adapter_error`` (RUNNER-NO-GIT).

Every refusal of the runner names its cause in the ``summary`` of ``finishRun``, which the app
shows with the reason in the thread, so a person never sees a bare "Error in the adapter". A
text names the cause, what to do, and the log code in backticks at the end; it never carries a
value of the host (a path, a user or a model name), only codes and configuration key names.
The one value a text may carry is the reset time of a usage limit, which the CLI names and a
person needs (`turn_end_cause`).

A run its turn completed carries ``run_ended_idle``: its process ended, so nothing waits on the
thread any more and only a new message continues it; an orderly stop of the runner names the
background tasks it ended (FIX-runner-silent-stop, issue 553).

Examples:
    >>> cause("session_busy").endswith("(`session_busy`)")
    True
"""

from __future__ import annotations

from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from claude_code_runner.account import AccountEnd

__all__ = [
    "ADAPTER_STOPPED",
    "CAUSES",
    "RUN_ENDED_IDLE",
    "TOOL_CLASS_MISMATCH",
    "adapter_stopped_cause",
    "cause",
    "definition_refused",
    "turn_end_cause",
]

ADAPTER_STOPPED = "adapter_stopped"
"""The ``cancelled`` reason of a run whose turn an orderly stop of the runner ended (D-AR-1)."""

RUN_ENDED_IDLE = "run_ended_idle"
"""The summary code of a run its turn completed: nothing waits on the thread any more."""

_STOPPED = (
    "The runner was stopped (a restart or an update) while this run worked, so it ended the turn."
)
_SEND_AGAIN = "Send a message to continue in the same session"

CAUSES: dict[str, str] = {
    "project_not_bound": (
        "This forum has no project folder, so Claude Code has nowhere to work. Bind a folder "
        "to the forum in its settings, or set runner.default_workspace on the runner host "
        "for forums without a folder"
    ),
    "channel_not_served": (
        "This runner does not operate this channel, so it has no project folder here. Make "
        "the runner the operator of a forum to chat with Claude Code there"
    ),
    "trigger_kind_unsupported": (
        "Claude Code runs on messages only; this kind of trigger is not supported"
    ),
    "outside_root": (
        "The project folder of this forum is not a folder under runner.projects_root. "
        "Bind a folder name under the projects folder"
    ),
    "workspace_missing": (
        "The project folder of this forum does not exist on the runner host. Create it there "
        "or bind another folder"
    ),
    "not_writable": (
        "The runner may not read and write the project folder of this forum. Fix the "
        "folder's permissions for the runner user"
    ),
    "session_busy": (
        "Another run of this chat still holds its Claude Code session. Wait until it ends "
        "and send again"
    ),
    "invalid_argument": (
        "A value of the forum's runner settings (model or tools) cannot be passed to Claude "
        "Code. Fix the forum's runner settings"
    ),
    "session_ref_not_uuid": (
        "The session of this chat is not a Claude Code session ID. Start a new session in this chat"
    ),
    "unknown_tool_class": "The runner's runner.tool_class is not chat, scoped or host",
    "home_missing": "The runner user has no home folder, which Claude Code needs",
    "organization_needs_key": (
        "Runs billed to the organization need an API key, and the runner has none"
    ),
    "api_key_unavailable": (
        "The runner cannot read its API key (runner.anthropic_api_key_ref). Check the "
        "secret on the runner host"
    ),
    "billing_mode_not_allowed": (
        "This runner does not run in the requested billing mode. Change the agent's billing "
        "mode or set runner.allow_personal"
    ),
    "runtime_unavailable": (
        "Claude Code is not usable on the runner host right now (missing or too old). "
        "The agent's status names the reason"
    ),
    "runtime_unreachable": "The runner cannot find Claude Code (runner.claude_bin)",
    "runtime_not_authenticated": (
        "Claude Code has no valid login for this billing mode on the runner host"
    ),
    "billing_mode_mismatch": (
        "Claude Code on the runner host is logged in for another billing mode than this run needs"
    ),
    "adapter_stopped": f"{_STOPPED} {_SEND_AGAIN}",
    "run_ended_idle": (
        "Run ended; nothing is waiting, so this thread continues only when someone sends a message"
    ),
    "definition_missing": (
        "The runner has no agent definition it can run for this built-in agent yet, so the "
        "run ended without a turn. Set up the agent's definition in the app"
    ),
    "definition_unavailable": (
        "The runner could not read this built-in agent's definition from the server, so the "
        "run ended without a turn. Send again in a moment"
    ),
    "definition_unwritable": (
        "The runner could not write the files of this built-in agent's definition under "
        "runner.hosted_agents_root. The runner log names the problem"
    ),
    "mcp_reference_unavailable": (
        "The runner cannot read a secret that an MCP server of this built-in agent names, or "
        "the secret lies outside the agent's folder under runner.hosted_secrets_dir. Check the "
        "secret on the runner host or change the server in the agent's definition"
    ),
    "cost_cap_reached": (
        "The run reached the cost cap of this built-in agent's definition, so Claude Code "
        "stopped. Raise the cap in the agent's settings or ask for less in one message"
    ),
    "reply_refused": (
        "The server refused an answer of Claude Code, so the run could not deliver it. "
        "The runner log names the refusal"
    ),
}
"""The text per log code, without the code (`cause` appends it)."""

_DEFAULT_CAUSES: dict[str, str] = {
    "outside_root": (
        "This forum has no project folder, and runner.default_workspace is not a folder "
        "under runner.projects_root. Bind a folder to the forum or fix the key"
    ),
    "workspace_missing": (
        "This forum has no project folder, and the folder of runner.default_workspace does "
        "not exist on the runner host. Bind a folder to the forum or create that folder"
    ),
    "not_writable": (
        "This forum has no project folder, and the runner may not read and write the folder "
        "of runner.default_workspace. Bind a folder to the forum or fix the permissions"
    ),
}

_FALLBACK = "The runner could not start Claude Code; its log has the details"

TOOL_CLASS_MISMATCH = "tool_class_mismatch"
"""The reason of a run whose definition fails the runner's own check (26 5.6 item 1)."""

_MAX_CODES = 6
"""Most field codes one summary names; the summary stays short (the runner log has them all)."""


def cause(code: str, *, default_workspace: bool = False) -> str:
    """The sentence for the thread about log code ``code``.

    Args:
        code: The runner's log code of the refusal.
        default_workspace: True when the code is about ``runner.default_workspace`` (a forum
            without a binding), not the forum's own folder.

    Returns:
        The text with the code in backticks at the end.
    """
    table = _DEFAULT_CAUSES if default_workspace and code in _DEFAULT_CAUSES else CAUSES
    return f"{table.get(code, _FALLBACK)} (`{code}`)"


def definition_refused(codes: list[str]) -> str:
    """The summary of a run refused because its definition fails the check (26 5.6 item 1).

    It names the field codes, never the content of the definition.

    Args:
        codes: The field codes of the findings, in their order.

    Returns:
        The text with the reason ``tool_class_mismatch`` in backticks at the end.

    Examples:
        >>> definition_refused(["inline_shell"]).startswith("This built-in agent's definition")
        True
    """
    distinct = list(dict.fromkeys(codes))
    named = ", ".join(f"`{c}`" for c in distinct[:_MAX_CODES])
    if len(distinct) > _MAX_CODES:
        named += " and more"
    return (
        "This built-in agent's definition claims more than its tool class allows or breaks "
        f"a rule of the definition check ({named}), so the run ended without a turn. Fix the "
        f"definition in the app (`{TOOL_CLASS_MISMATCH}`)"
    )


def _lost_sentence(lost_tasks: int) -> str | None:
    """The sentence about ``lost_tasks`` background tasks that ended with a process, or None."""
    if lost_tasks == 1:
        return "1 background task was still running and was stopped; its result is lost."
    if lost_tasks > 1:
        return (
            f"{lost_tasks} background tasks were still running and were stopped; their results "
            "are lost."
        )
    return None


def adapter_stopped_cause(lost_tasks: int) -> str:
    """The summary of a run an orderly stop of the runner ended (D-AR-1, issue 553).

    Without ``runner.turn_handover`` a restart stops the process of every own turn, and the
    background tasks the process held end with it and never report back; the thread learns
    how many.

    Args:
        lost_tasks: The background tasks the process held when the stop came.

    Returns:
        The text with ``adapter_stopped`` in backticks at the end.

    Examples:
        >>> adapter_stopped_cause(0) == cause("adapter_stopped")
        True
        >>> "2 background tasks" in adapter_stopped_cause(2)
        True
    """
    lost = _lost_sentence(lost_tasks)
    if lost is None:
        return cause(ADAPTER_STOPPED)
    return f"{_STOPPED} {lost} {_SEND_AGAIN} (`{ADAPTER_STOPPED}`)"


_LOGIN_LOST = "Claude Code lost its login on the runner host during this run, so the run ended."
_NEXT = "your next message starts a new run that continues the same session"
_ACCOUNT = "Claude Code could not use its account on the runner host"
_NOTHING_KEPT = (
    "This session had no background work to keep, so the run ended at once instead of waiting "
    "and trying again."
)


def _retried_sentence(retries: int) -> str:
    """The sentence about the retries of a wait for the account that ended without an answer."""
    times = "1 time" if retries == 1 else f"{retries} times"
    return (
        f"The runner kept the session and tried the turn again {times} "
        "(runner.account_retry_max_wait bounds that wait) before the run ended."
    )


def _account_sentences(account: AccountEnd) -> list[str]:
    """What the account error was and what the runner did about it."""
    parts = [f"{_ACCOUNT}: {account.error.describe()}."]
    if account.retries is not None:
        parts.append(_retried_sentence(account.retries))
    elif not account.held:
        parts.append(_NOTHING_KEPT)
    return parts


def turn_end_cause(reason: str, lost_tasks: int, account: AccountEnd | None = None) -> str | None:
    """The summary of a run that a turn process ended with ``reason`` (FIX-runner-auth-failed-held).

    A lost login at a turn names what happened and what the next message does; background
    tasks that were still running when the process ended are named as lost, because they end
    with it and nothing reports their result. A turn that ended with an account error names
    the class of the error, the reset time when the CLI gave one, and whether the runner had
    tried the turn again (09 18.9, FIX-RUNNER-LIMIT-KEEPS-RUN); never a name of the account.

    Args:
        reason: The run's ``failed`` reason.
        lost_tasks: How many background tasks of the session were still running.
        account: The account error the failed turn ended with, if it was one.

    Returns:
        The text with the reason in backticks at the end, or ``None`` when there is nothing to
        add to the reason and its box note.

    Examples:
        >>> turn_end_cause("runtime_error", 0) is None
        True
        >>> turn_end_cause("runtime_error", 2).endswith("same session (`runtime_error`)")
        True
    """
    parts: list[str] = []
    if account is not None:
        parts += _account_sentences(account)
    elif reason == "runtime_not_authenticated":
        parts.append(_LOGIN_LOST)
    lost = _lost_sentence(lost_tasks)
    if lost is not None:
        parts.append(lost)
    if not parts:
        return None
    if reason == "runtime_not_authenticated":
        parts.append(f"Once the login works again, {_NEXT}")
    else:
        parts.append(_NEXT[0].upper() + _NEXT[1:])
    return f"{' '.join(parts)} (`{reason}`)"
