# SPDX-License-Identifier: Apache-2.0
"""The search assistant of the runner (docs/spec/09-agents.md 18.16, 14-search.md 4.10).

When the runner is the agent of ``search.assist.agent_id``, each question of the search assistant
becomes one ``claude -p`` call without tools: the fixed instruction ``prompts/search_assist.md``
as the system prompt, the question and the candidates as a data block on stdin (marked as
untrusted, ADR 0054), ``--output-format json``, the model ``runner.search_assist_model``, no
session on disk, a private working folder below ``runner.state_dir`` and the time limit
``runner.search_assist_timeout``, never past the request's ``expires_at``. The reply is parsed
into picks of references the request named and terms of letters and digits; the usage of the call
becomes ``costs`` like a run's (18.7). A failing call, an unreadable reply or the time limit
answers nothing. Neither the question nor a candidate nor the reply is logged.

Examples:
    >>> reply = '{"picks": [{"ref": "c1", "reason": "Moved."}], "terms": ["Move"]}'
    >>> parse_reply(reply, {"c1"}, 1, 1)
    ([('c1', 'Moved.')], ['move'])
"""

from __future__ import annotations

import asyncio
import contextlib
import json
import logging
import os
import re
import shutil
import signal
import uuid
from collections.abc import Callable, Mapping, Sequence
from dataclasses import dataclass
from datetime import datetime
from importlib import resources
from pathlib import Path
from typing import Any

from bothy_sdk.search_assist import AssistChoice, AssistQuestion, AssistReply

from claude_code_runner.commandline import class_flags
from claude_code_runner.statedir import ensure_private_dir
from claude_code_runner.usage import cost_reports, parse_model_usage

__all__ = ["REASON_MAX_CHARS", "SearchAssistant", "data_block", "parse_reply"]

_LOG = logging.getLogger("claude_code_runner")

REASON_MAX_CHARS = 200
"""Longest reason of a pick; equal to the default of ``search.assist.reason_max_chars``."""

_TERM_MAX_CHARS = 64
_TERM = re.compile(r"^[^\W_]+$")
_MAX_PARALLEL = 2


def _instruction() -> str:
    return (
        resources.files("claude_code_runner")
        .joinpath("prompts/search_assist.md")
        .read_text(encoding="utf-8")
    )


def data_block(request: AssistQuestion) -> str:
    r"""Return the stdin of the call: the question and the candidates as one marked JSON block.

    ``</`` is written as ``<\\/`` (valid JSON), so no text of the chat can close the block.

    Args:
        request: The question.

    Returns:
        The text between ``<data>`` and ``</data>``.
    """
    data = {
        "question": request.q,
        "earlier_questions": list(request.history),
        "max_picks": request.max_picks,
        "max_terms": request.max_terms,
        "candidates": [
            {
                "ref": c.ref,
                "title": c.title or "",
                "place": " / ".join(p for p in (c.space_name, c.channel_name) if p),
                "time": c.at.isoformat(),
                "excerpt": c.excerpt,
                "shown_before": c.shown_before,
            }
            for c in request.candidates
        ],
    }
    text = json.dumps(data, ensure_ascii=False, indent=1).replace("</", "<\\/")
    return f"<data>\n{text}\n</data>\n"


def _json_object(text: str) -> Mapping[str, Any] | None:
    """Return the JSON object of a reply.

    The whole text, else the span from the first ``{`` to the last ``}`` (a model may wrap it in a
    code fence).
    """
    for candidate in (text.strip(), text[text.find("{") : text.rfind("}") + 1]):
        with contextlib.suppress(ValueError):
            value = json.loads(candidate)
            if isinstance(value, dict):
                return value
    return None


def _one_line(text: str) -> str:
    return " ".join("".join(ch if ch.isprintable() else " " for ch in text).split())


def parse_reply(
    text: str, refs: set[str], max_picks: int, max_terms: int
) -> tuple[list[tuple[str, str]], list[str]] | None:
    """Parse the model's reply into picks and terms the server accepts.

    Picks keep only references of ``refs``, each once, at most ``max_picks``, with a non-empty
    reason on one line cut to `REASON_MAX_CHARS`; terms are lower-case words of letters and
    digits, at most 64 characters, each once, at most ``max_terms``.

    Args:
        text: The ``result`` text of the call.
        refs: The references of the request.
        max_picks: ``max_picks`` of the request.
        max_terms: ``max_terms`` of the request.

    Returns:
        The picks as ``(ref, reason)`` and the terms, or ``None`` for an unreadable reply.
    """
    obj = _json_object(text)
    if obj is None:
        return None
    picks: list[tuple[str, str]] = []
    raw_picks = obj.get("picks")
    for item in raw_picks if isinstance(raw_picks, list) else []:
        if not isinstance(item, dict) or len(picks) == max_picks:
            continue
        ref, reason = item.get("ref"), _one_line(str(item.get("reason") or ""))
        if isinstance(ref, str) and ref in refs and reason and ref not in {p[0] for p in picks}:
            picks.append((ref, reason[:REASON_MAX_CHARS].rstrip()))
    terms: list[str] = []
    raw_terms = obj.get("terms")
    for term in raw_terms if isinstance(raw_terms, list) else []:
        word = str(term).strip().lower()
        if (
            len(terms) < max_terms
            and 0 < len(word) <= _TERM_MAX_CHARS
            and _TERM.match(word)
            and word not in terms
        ):
            terms.append(word)
    return picks, terms


@dataclass
class SearchAssistant:
    """One ``claude -p`` call per question.

    Attributes:
        claude_bin: The CLI found in the cleaned ``PATH``; ``None`` answers nothing.
        env: Builds the environment of a call (the allow list of 18.2 in the runner's mode), or
            raises to answer nothing.
        state_dir: ``runner.state_dir``; the private working folders go below ``search-assist``.
        model: ``runner.search_assist_model``.
        timeout: ``runner.search_assist_timeout`` in seconds.
        now: The time source of the deadline (the adapter's clock).
    """

    claude_bin: str | None
    env: Callable[[], Mapping[str, str]]
    state_dir: Path
    model: str
    timeout: float
    now: Callable[[], datetime]

    def __post_init__(self) -> None:
        """Create the slot of parallel calls."""
        self._slots = asyncio.Semaphore(_MAX_PARALLEL)

    async def answer(self, request: AssistQuestion) -> AssistReply | None:
        """Answer one question, or ``None`` (nothing is sent).

        Args:
            request: The question.

        Returns:
            The reply with the usage of the call.
        """
        if self.claude_bin is None:
            _failed("runtime_unreachable")
            return None
        async with self._slots:
            left = (request.expires_at - self.now()).total_seconds()
            if left <= 0:
                return None
            try:
                env = dict(self.env())
            except Exception:  # an unusable mode answers nothing; the server falls back
                _failed("no_billing_mode")
                return None
            return await self._call(request, env, min(self.timeout, left))

    async def _call(
        self, request: AssistQuestion, env: dict[str, str], timeout: float
    ) -> AssistReply | None:
        folder = ensure_private_dir(self.state_dir / "search-assist" / uuid.uuid4().hex)
        try:
            prompt = folder / "instruction.md"
            prompt.write_text(_instruction(), encoding="utf-8")
            env["TMPDIR"] = str(folder)
            argv = [
                self.claude_bin or "",
                "-p",
                "--output-format",
                "json",
                "--model",
                self.model,
                "--no-session-persistence",
                *class_flags("chat", []),
                "--permission-prompts",
                "none",
                "--system-prompt-file",
                str(prompt),
            ]
            out = await self._run(argv, env, folder, data_block(request).encode("utf-8"), timeout)
        finally:
            shutil.rmtree(folder, ignore_errors=True)
        if out is None:
            return None
        return self._reply(out, request)

    async def _run(
        self, argv: Sequence[str], env: Mapping[str, str], cwd: Path, data: bytes, timeout: float
    ) -> bytes | None:
        try:
            proc = await asyncio.create_subprocess_exec(
                *argv,
                env=dict(env),
                cwd=str(cwd),
                stdin=asyncio.subprocess.PIPE,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.DEVNULL,
                start_new_session=True,
            )
        except OSError:
            _failed("runtime_unreachable")
            return None
        try:
            out, _ = await asyncio.wait_for(proc.communicate(data), timeout)
        except TimeoutError:
            _kill_group(proc.pid)
            await proc.wait()
            _failed("timeout")
            return None
        except asyncio.CancelledError:
            _kill_group(proc.pid)
            await proc.wait()
            raise
        if proc.returncode != 0:
            _failed("exit_" + str(proc.returncode))
            return None
        return out

    def _reply(self, out: bytes, request: AssistQuestion) -> AssistReply | None:
        try:
            result = json.loads(out.decode("utf-8", errors="replace"))
        except ValueError:
            result = None
        if (
            not isinstance(result, dict)
            or result.get("is_error")
            or not isinstance(result.get("result"), str)
        ):
            _failed("bad_result")
            return None
        parsed = parse_reply(
            result["result"],
            {c.ref for c in request.candidates},
            request.max_picks,
            request.max_terms,
        )
        if parsed is None:
            _failed("unreadable_reply")
            return None
        picks, terms = parsed
        return AssistReply(
            picks=[AssistChoice(ref, reason) for ref, reason in picks],
            terms=terms,
            costs=cost_reports(parse_model_usage(result)),
        )


def _failed(code: str) -> None:
    _LOG.info(json.dumps({"event": "search_assist_failed", "code": code}, separators=(",", ":")))


def _kill_group(pid: int) -> None:
    with contextlib.suppress(ProcessLookupError, PermissionError):
        os.killpg(pid, signal.SIGKILL)
