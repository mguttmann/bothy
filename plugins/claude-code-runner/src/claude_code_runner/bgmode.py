# SPDX-License-Identifier: Apache-2.0
"""Background mode of the runner process (09 18.2 "Background mode", 18.9).

`BackgroundMode` is what every background turn of one runner process shares: the one fixed
environment of every launch and control command, the one reader of the session list, the
channels the mode is switched on for, the lock that lets one run at a time launch, and the
heartbeat reason of a fallback.

The environment is built once, from the allow list of 18.2 without the SSH agent and without
the temporary folder of any run. The first launch starts the CLI's supervisor with it, the
supervisor lives across runs, and every session inherits what it carries; so nothing of a run
may be in it. What belongs to a run reaches its session through the settings file of its
launch (`claude_code_runner.bgsession.settings_document`).

A run that should use background mode and cannot ends ``failed`` with its cause in the thread;
it never uses print mode (ruling D-FIX-RUNNER-CLI-PIN-326-4). When the cause lies
with the host (the CLI is not the pinned version, a supervisor the runner did
not start, bypass mode without a recorded acceptance, a launch that fails), the runner reports
`UNAVAILABLE` in ``details.degraded_reasons`` until the next background launch starts; the
reason blocks no run.

Examples:
    >>> UNAVAILABLE
    'background_unavailable'
"""

from __future__ import annotations

import asyncio
from collections.abc import Callable, Mapping
from pathlib import Path
from uuid import UUID

from claude_code_runner.adoption import read_identity
from claude_code_runner.bgsession import SessionCli, SessionList
from claude_code_runner.config import RunnerConfig
from claude_code_runner.environment import RunEnvironmentError, build_environment, clean_path
from claude_code_runner.heartbeat import PINNED_CLAUDE_CODE_VERSION
from claude_code_runner.usage import claude_config_dir

__all__ = ["FALLBACK_CODES", "UNAVAILABLE", "BackgroundMode", "launch_environment"]

UNAVAILABLE = "background_unavailable"
"""The degraded reason while the last launch attempt failed for a cause of the host."""

FALLBACK_CODES: dict[str, bool] = {
    "background_version_untested": True,
    "background_supervisor_foreign": True,
    "background_supervisor_version": True,
    "background_bypass_not_accepted": True,
    "background_workspace_untrusted": False,
    "background_files_untrusted": True,
    "background_session_foreign": False,
    "background_launch_failed": True,
    "background_api_key": False,
    "background_compaction": False,
    "background_restricted": False,
    "background_hosted_bypass": False,
}
"""Why a run of background mode ends without a session (18.9; it never uses print mode,
FIX-RUNNER-CLI-PIN-326), and whether the cause lies with the host (then the heartbeat reports
`UNAVAILABLE`)."""

_NO_UPDATE = ("DISABLE_AUTOUPDATER", "1")


def launch_environment(env: Mapping[str, str]) -> dict[str, str]:
    """Return the environment of a control command or a launch: the allow list plus one name.

    ``DISABLE_AUTOUPDATER`` keeps the CLI and a supervisor a launch starts on the installed
    version: a supervisor restarts itself onto a version an update installed.

    Args:
        env: The allow-list environment of 18.2.

    Returns:
        The environment.
    """
    return {**env, _NO_UPDATE[0]: _NO_UPDATE[1]}


class BackgroundMode:
    """What the background turns of one runner process share; see the module docstring.

    Args:
        config: The runner configuration.
        claude_bin: The CLI found in the cleaned ``PATH``; the configured name without one.
        environ: The runner's own environment.
        on_change: Called when the heartbeat reason appears or goes (a heartbeat goes out).
        version: Returns the version of the installed CLI as the last probe read it.

    Attributes:
        env: The fixed environment of every launch and control command; no value of a run.
        cli: The control commands.
        sessions: The reader of the session list.
        launching: Held by a run from its check of the supervisor until its launch command
            ended and the supervisor that command started is recorded (18.2 "Before a
            launch"). Runs of one runner process launch one after the other, so a run never
            reads the supervisor between the launch of another run that starts it and its
            record, which would make the runner's own supervisor look like another program's.
        unavailable: The code of the last fallback for a cause of the host, ``None`` once a
            background launch started after it.
    """

    def __init__(
        self,
        config: RunnerConfig,
        claude_bin: str | None,
        environ: Mapping[str, str],
        on_change: Callable[[], None] | None = None,
        version: Callable[[], str | None] | None = None,
    ) -> None:
        """Create the shared parts; no command runs before a turn needs one."""
        self.config = config
        self._version = version
        try:
            env = build_environment(
                environ, mode="personal", api_key=None, config_dir=None, forward_ssh_agent=False
            )
        except RunEnvironmentError:
            env = {"PATH": ":".join(clean_path(environ.get("PATH", "")))}
        self.env = launch_environment(env)
        self.cli = SessionCli(
            claude_bin or config.claude_bin, self.env, config.state_dir, config.cli_probe_timeout
        )
        self.sessions = SessionList(self.cli, config.background_status_interval)
        self.launching = asyncio.Lock()
        self.unavailable: str | None = None
        self._on_change = on_change

    @property
    def config_dir(self) -> Path | None:
        """The CLI's configuration folder of a ``personal`` turn (its transcripts live there)."""
        return claude_config_dir(self.env)

    def wants(self, channel_id: UUID) -> bool:
        """Whether a run in this channel should use background mode.

        Args:
            channel_id: The channel of the run.

        Returns:
            True with ``runner.launch_mode`` ``background`` and, when
            ``runner.background_channels`` is not empty, only for a listed channel.
        """
        if self.config.launch_mode != "background":
            return False
        listed = self.config.background_channels
        return not listed or channel_id in listed

    def pinned(self) -> bool:
        """Whether the installed CLI is the pinned version: the mode runs on no other (18.2)."""
        return self._version is not None and self._version() == PINNED_CLAUDE_CODE_VERSION

    def lstart(self, pid: int) -> str | None:
        """The start time of a process as ``ps`` prints it; ``None`` when it does not exist."""
        found = read_identity(pid, self.config.cli_probe_timeout)
        return found.lstart if found is not None else None

    def fell_back(self, code: str) -> None:
        """A run ended without a session for the cause ``code``; a cause of the host is reported.

        Args:
            code: One of `FALLBACK_CODES`.
        """
        if FALLBACK_CODES.get(code, True):
            self._set(code)

    def launched(self) -> None:
        """A background launch started: the mode works on this host again."""
        self._set(None)

    def _set(self, code: str | None) -> None:
        changed = (self.unavailable is None) != (code is None)
        self.unavailable = code
        if changed and self._on_change is not None:
            self._on_change()
