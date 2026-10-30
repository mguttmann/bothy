# SPDX-License-Identifier: Apache-2.0
"""The cause of a run that ends without a turn or with ``adapter_error`` (RUNNER-NO-GIT).

Every refusal of the runner names its cause in the ``summary`` of ``finishRun``, which the app
shows with the reason in the thread, so a person never sees a bare "Error in the adapter". A
text names the cause, what to do, and the log code in backticks at the end; it never carries a
value of the host (a path, a user or a model name), only codes and configuration key names.

Examples:
    >>> cause("session_busy").endswith("(`session_busy`)")
    True
"""

from __future__ import annotations

__all__ = ["CAUSES", "cause"]

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
