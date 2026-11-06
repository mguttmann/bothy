# SPDX-License-Identifier: Apache-2.0
"""The builder of the interview of "New agent" (docs/spec/26-builtin-agents.md 6.2).

With ``runner.agent_builder`` on, and when this runner is the agent of the server's
``agents.builder_agent_id`` (only that agent receives ``agent.interview_requested``), each
interview request becomes one ``claude -p`` call without tools: the fixed instruction
``prompts/agent_builder.md`` as the system prompt, the interview so far, the template catalogue
and the limits as a data block on stdin (marked as untrusted, ADR 0054), ``--output-format
json``, the model ``runner.agent_builder_model``, no session on disk, a private working folder
below ``runner.state_dir`` that is removed afterwards, the billing rule of the search assistant
(09 18.17 item 1) and the time limit ``runner.agent_builder_timeout``, never past the request's
``expires_at``.

The runner parses the last JSON object of the reply and checks it against the answer schema and
its own copy of the definition rules (`claude_code_runner.definition`) before it sends anything;
an answer that fails sends nothing, and the server then falls back with ``builder_timeout``. The
builder holds no tool and writes no file (D-A1-13). Neither a turn nor the reply is logged.

Examples:
    >>> lim = InterviewLimits(64, 4, 300, 64, 2, 2, 8, 50.0, ("npx",), 200)
    >>> check_answer({"kind": "questions", "questions": ["Who reads it?"]}, lim) is None
    True
"""

from __future__ import annotations

import asyncio
import json
import logging
import shutil
import uuid
from collections.abc import Callable, Mapping
from dataclasses import dataclass
from datetime import datetime
from importlib import resources
from pathlib import Path
from typing import Any

from bothy_sdk.agent_interview import InterviewLimits, InterviewReply, InterviewRequest

from claude_code_runner.commandline import class_flags
from claude_code_runner.definition import Limits, text_rule, validate_value
from claude_code_runner.oneshot import run_once
from claude_code_runner.statedir import ensure_private_dir
from claude_code_runner.usage import cost_reports, parse_model_usage

__all__ = ["PROMPT_FILE", "AgentBuilder", "check_answer", "data_block", "parse_reply"]

_LOG = logging.getLogger("claude_code_runner")

PROMPT_FILE = "builder.md"
"""The name of the instruction in the private folder of a call."""

_ANSWER_MEMBERS = frozenset({"kind", "questions", "draft"})
_DRAFT_EXTRAS = ("name", "handle", "description", "channel_suggestion")
_CHANNEL_MEMBERS = frozenset({"space_name", "channel_name"})
_MAX_QUESTIONS = 2
_MAX_PARALLEL = 2


def _instruction() -> str:
    return (
        resources.files("claude_code_runner")
        .joinpath("prompts/agent_builder.md")
        .read_text(encoding="utf-8")
    )


def data_block(request: InterviewRequest) -> str:
    r"""Return the stdin of the call: the interview, the catalogue and the limits as one block.

    ``</`` is written as ``<\\/`` (valid JSON), so no text of the interview can close the block.

    Args:
        request: The interview request.

    Returns:
        The text between ``<data>`` and ``</data>``.
    """
    lim = request.limits
    data = {
        "turns": [{"role": t.role, "text": t.text} for t in request.turns],
        "want_draft": request.want_draft,
        "catalogue": [dict(entry) for entry in request.catalogue],
        "limits": {
            "instructions_max_bytes": lim.instructions_max_bytes,
            "max_skills": lim.max_skills,
            "skill_description_max_chars": lim.skill_description_max_chars,
            "skill_body_max_bytes": lim.skill_body_max_bytes,
            "max_skill_files": lim.max_skill_files,
            "max_mcp_servers": lim.max_mcp_servers,
            "max_allowed_tools": lim.max_allowed_tools,
            "max_cost_usd_per_run": lim.max_cost_usd_per_run,
            "mcp_stdio_runners": list(lim.mcp_stdio_runners),
            "question_max_chars": lim.question_max_chars,
        },
    }
    text = json.dumps(data, ensure_ascii=False, indent=1).replace("</", "<\\/")
    return f"<data>\n{text}\n</data>\n"


def _last_json_object(text: str) -> dict[str, Any] | None:
    """Return the last top-level JSON object of ``text``, or ``None``.

    A model may write prose or a code fence around its answer, or think aloud with an earlier
    object first; the last complete object is the answer.
    """
    decoder = json.JSONDecoder()
    found: dict[str, Any] | None = None
    i = text.find("{")
    while i >= 0:
        try:
            value, end = decoder.raw_decode(text, i)
        except ValueError:
            i = text.find("{", i + 1)
            continue
        if isinstance(value, dict):
            found = value
        i = text.find("{", end)
    return found


def _limits(lim: InterviewLimits) -> Limits:
    return Limits(
        instructions_max_bytes=lim.instructions_max_bytes,
        max_skills=lim.max_skills,
        skill_description_max_chars=lim.skill_description_max_chars,
        skill_body_max_bytes=lim.skill_body_max_bytes,
        max_skill_files=lim.max_skill_files,
        max_mcp_servers=lim.max_mcp_servers,
        max_allowed_tools=lim.max_allowed_tools,
        max_cost_usd_per_run=lim.max_cost_usd_per_run,
        stdio_runners=tuple(lim.mcp_stdio_runners),
    )


def _line(value: Any, *, blank_ok: bool = False) -> bool:
    """Tell whether ``value`` is a one-line display text, not blank unless ``blank_ok``."""
    if not isinstance(value, str) or "\n" in value or "\r" in value or text_rule(value):
        return False
    return blank_ok or value.strip() != ""


def _questions_problem(answer: Mapping[str, Any], lim: InterviewLimits) -> str | None:
    questions = answer.get("questions")
    if "draft" in answer or not isinstance(questions, list):
        return "answer_shape"
    if not 1 <= len(questions) <= _MAX_QUESTIONS:
        return "question_count"
    for q in questions:
        if not isinstance(q, str) or not q.strip() or len(q) > lim.question_max_chars:
            return "question_text"
        if text_rule(q.replace("\n", " ").replace("\r", " ")):
            return "question_text"
    return None


def _draft_problem(answer: Mapping[str, Any], lim: InterviewLimits) -> str | None:
    draft = answer.get("draft")
    if "questions" in answer or not isinstance(draft, dict):
        return "answer_shape"
    definition = {k: v for k, v in draft.items() if k not in _DRAFT_EXTRAS}
    settings = definition.get("settings")
    wish = settings.get("tool_class_wish") if isinstance(settings, dict) else None
    if validate_value(definition, _limits(lim), wish if isinstance(wish, str) else ""):
        return "definition_rules"
    if not (
        _line(draft.get("name"))
        and _line(draft.get("handle"))
        and _line(draft.get("description"), blank_ok=True)
    ):
        return "draft_names"
    channel = draft.get("channel_suggestion")
    if channel is not None and not (
        isinstance(channel, dict)
        and set(channel) == _CHANNEL_MEMBERS
        and all(_line(channel[k]) for k in _CHANNEL_MEMBERS)
    ):
        return "draft_channel"
    return None


def check_answer(answer: Mapping[str, Any], limits: InterviewLimits) -> str | None:
    """Check an answer against the answer schema of 26 6.1 item 2 and the rules of 26 4.3.

    The answer schema has no member for settings files, hooks, permission rules, commands,
    subagents or plugin parts: a member outside it fails, and the draft must pass the definition
    rules for the class of its ``settings.tool_class_wish``, every finding included.

    Args:
        answer: The decoded answer object.
        limits: The limits of the request.

    Returns:
        ``None`` when the answer may be sent, else a code that names the failed rule (never a
        value).
    """
    if set(answer) - _ANSWER_MEMBERS:
        return "unknown_member"
    kind = answer.get("kind")
    if kind == "questions":
        return _questions_problem(answer, limits)
    if kind == "draft":
        return _draft_problem(answer, limits)
    return "answer_kind"


def parse_reply(text: str, limits: InterviewLimits) -> dict[str, Any] | None:
    """Return the last JSON object of the reply when it passes `check_answer`, else ``None``.

    Args:
        text: The ``result`` text of the call.
        limits: The limits of the request.

    Returns:
        The answer object, or ``None``.
    """
    obj = _last_json_object(text)
    if obj is None or check_answer(obj, limits) is not None:
        return None
    return obj


@dataclass
class AgentBuilder:
    """One ``claude -p`` call per interview request.

    Attributes:
        claude_bin: The CLI found in the cleaned ``PATH``; ``None`` answers nothing.
        env: Builds the environment of a call (the allow list of 18.2 in the runner's mode, the
            billing rule of 09 18.17 item 1), or raises to answer nothing.
        state_dir: ``runner.state_dir``; the private working folders go below ``agent-builder``.
        model: ``runner.agent_builder_model``.
        timeout: ``runner.agent_builder_timeout`` in seconds.
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

    async def answer(self, request: InterviewRequest) -> InterviewReply | None:
        """Answer one interview request, or ``None`` (nothing is sent).

        Args:
            request: The interview request.

        Returns:
            Questions or a draft with the usage of the call.
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

    def argv(self, prompt: Path) -> list[str]:
        """Return the command line of a call.

        It holds the flags of ``chat`` (no tool), JSON output, no session, the builder model and
        the instruction as the system prompt.

        Args:
            prompt: The instruction file in the private folder.

        Returns:
            The argument list.
        """
        return [
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

    async def _call(
        self, request: InterviewRequest, env: dict[str, str], timeout: float
    ) -> InterviewReply | None:
        folder = ensure_private_dir(self.state_dir / "agent-builder" / uuid.uuid4().hex)
        try:
            prompt = folder / PROMPT_FILE
            prompt.write_text(_instruction(), encoding="utf-8")
            env["TMPDIR"] = str(folder)
            out = await run_once(
                self.argv(prompt),
                env,
                folder,
                data_block(request).encode("utf-8"),
                timeout,
                failed=_failed,
            )
        finally:
            shutil.rmtree(folder, ignore_errors=True)
        if out is None:
            return None
        return _reply(out, request.limits)


def _reply(out: bytes, limits: InterviewLimits) -> InterviewReply | None:
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
    obj = _last_json_object(result["result"])
    if obj is None:
        _failed("unreadable_reply")
        return None
    problem = check_answer(obj, limits)
    if problem is not None:
        _failed("invalid_answer", problem)
        return None
    costs = cost_reports(parse_model_usage(result))
    if obj["kind"] == "questions":
        return InterviewReply.ask(*obj["questions"], costs=costs)
    return InterviewReply.propose(obj["draft"], costs)


def _failed(code: str, rule: str = "") -> None:
    line = {"event": "agent_builder_failed", "code": code}
    if rule:
        line["rule"] = rule
    _LOG.info(json.dumps(line, separators=(",", ":")))
