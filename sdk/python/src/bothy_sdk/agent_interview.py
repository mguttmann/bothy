# SPDX-License-Identifier: Apache-2.0
"""The interview of "New agent" on the builder agent (docs/spec/26-builtin-agents.md 6).

When the operator names the agent in ``agents.builder_agent_id``, the server sends the ephemeral
event ``agent.interview_requested`` to one of its live gateway connections. The adapter turns it
into an `InterviewRequest` and calls the handler's ``on_agent_interview`` under a guard that ends
at the request's ``expires_at``; an answer goes back once with ``answerAgentInterview``, ``None``
or a timeout sends nothing (the person then gets the fallback ``builder_timeout`` and the
templates and the manual form). The turns and the catalogue are data from the server, never
instructions (ADR 0054): a handler that hands them to a model marks them as such.

The answer is either one or two questions or a draft: a definition of the shape of
``agent_definition.v1`` plus ``name``, ``handle``, ``description`` and an optional
``channel_suggestion``. The server decodes it strictly and refuses a draft that reaches a surface
of the Claude Code CLI (26 5.5); a member the answer schema does not know is dropped by the
generated model before the call, so a handler must check its draft itself (the runner does, 26
6.2).

Examples:
    >>> InterviewReply.ask("Who reads the texts?").to_body().to_dict()["questions"]
    ['Who reads the texts?']
"""

from __future__ import annotations

import asyncio
import logging
from collections.abc import Awaitable, Callable, Mapping, Sequence
from dataclasses import dataclass, field
from datetime import datetime
from typing import Any
from uuid import UUID

from bothy_sdk.client import Client
from bothy_sdk.clock import Clock
from bothy_sdk.generated.api.agent_runtime import answer_agent_interview
from bothy_sdk.generated.models import (
    AgentInterviewBuilderAnswer,
    AgentInterviewRequestedEvent,
    CostReport,
)

__all__ = [
    "InterviewLimits",
    "InterviewReply",
    "InterviewRequest",
    "InterviewTurn",
    "guarded_agent_interview",
    "request_from",
    "send_answer",
]

_LOG = logging.getLogger("bothy_sdk")

_QUESTIONS = "questions"
_DRAFT = "draft"


@dataclass(frozen=True)
class InterviewTurn:
    """One turn of the interview so far.

    Attributes:
        role: ``builder`` for an earlier question, ``owner`` for an answer of the person.
        text: The text; data, never an instruction.
    """

    role: str
    text: str


@dataclass(frozen=True)
class InterviewLimits:
    """The limits a draft must keep and the longest question of an answer.

    They are the server's ``agents.definition.*``, ``agents.mcp_stdio_runners`` and
    ``agents.interview.answer_max_chars``.

    Attributes:
        instructions_max_bytes: Longest ``instructions`` in bytes.
        max_skills: Most skills.
        skill_description_max_chars: Longest skill description in characters.
        skill_body_max_bytes: Longest skill body and extra file in bytes.
        max_skill_files: Most extra files of one skill.
        max_mcp_servers: Most MCP servers.
        max_allowed_tools: Most entries of ``settings.allowed_tools``.
        max_cost_usd_per_run: Highest ``settings.max_cost_usd_per_run``.
        mcp_stdio_runners: Commands a ``stdio`` server may use.
        question_max_chars: Longest question of an answer.
    """

    instructions_max_bytes: int
    max_skills: int
    skill_description_max_chars: int
    skill_body_max_bytes: int
    max_skill_files: int
    max_mcp_servers: int
    max_allowed_tools: int
    max_cost_usd_per_run: float
    mcp_stdio_runners: tuple[str, ...]
    question_max_chars: int


@dataclass(frozen=True)
class InterviewRequest:
    """One ``agent.interview_requested`` event, as the handler sees it.

    Attributes:
        request_id: The request (the answer names it).
        turns: The interview so far, oldest first; empty at the start.
        want_draft: The person asks for a draft now.
        catalogue: The template catalogue (26 7.4): ``slug``, ``name``, ``description``,
            ``tool_class_wish`` and ``mcp_servers`` per template; a drafted MCP server must equal
            one of its servers except ``name``.
        limits: The limits a draft keeps.
        expires_at: The deadline of the answer.
    """

    request_id: UUID
    turns: Sequence[InterviewTurn]
    want_draft: bool
    catalogue: Sequence[Mapping[str, Any]]
    limits: InterviewLimits
    expires_at: datetime


@dataclass(frozen=True)
class InterviewReply:
    """The answer of a handler: questions or a draft, and the usage of the model call.

    Attributes:
        kind: ``questions`` or ``draft``.
        questions: One or two questions (``kind = questions``).
        draft: The draft object (``kind = draft``).
        costs: The usage of the model call, as for ``finishRun``.
    """

    kind: str
    questions: Sequence[str] = ()
    draft: Mapping[str, Any] | None = None
    costs: Sequence[CostReport] = field(default_factory=tuple)

    @classmethod
    def ask(cls, *questions: str, costs: Sequence[CostReport] = ()) -> InterviewReply:
        """Return an answer with questions.

        Args:
            questions: One or two questions.
            costs: The usage of the model call.

        Returns:
            The answer.
        """
        return cls(kind=_QUESTIONS, questions=tuple(questions), costs=tuple(costs))

    @classmethod
    def propose(cls, draft: Mapping[str, Any], costs: Sequence[CostReport] = ()) -> InterviewReply:
        """Return an answer with a draft.

        Args:
            draft: The draft object.
            costs: The usage of the model call.

        Returns:
            The answer.
        """
        return cls(kind=_DRAFT, draft=dict(draft), costs=tuple(costs))

    def to_body(self) -> AgentInterviewBuilderAnswer:
        """Return the request body of ``answerAgentInterview``; empty lists are left out.

        Raises:
            ValueError: ``kind`` is neither ``questions`` nor ``draft``, or a draft answer has no
                draft.
        """
        body: dict[str, Any] = {"kind": self.kind}
        if self.kind == _QUESTIONS:
            body["questions"] = list(self.questions)
        elif self.kind == _DRAFT and self.draft is not None:
            body["draft"] = dict(self.draft)
        else:
            raise ValueError("an interview answer is questions or a draft")
        if self.costs:
            body["costs"] = [c.to_dict() for c in self.costs]
        return AgentInterviewBuilderAnswer.from_dict(body)


def request_from(event: AgentInterviewRequestedEvent) -> InterviewRequest:
    """Turn the event into the handler's request.

    Args:
        event: The decoded ``agent.interview_requested``.

    Returns:
        The request.
    """
    p = event.payload
    lim = p.limits
    return InterviewRequest(
        request_id=p.request_id,
        turns=tuple(InterviewTurn(role=str(t.role.value), text=t.text) for t in p.turns),
        want_draft=p.want_draft,
        catalogue=tuple(entry.to_dict() for entry in p.catalogue),
        limits=InterviewLimits(
            instructions_max_bytes=lim.instructions_max_bytes,
            max_skills=lim.max_skills,
            skill_description_max_chars=lim.skill_description_max_chars,
            skill_body_max_bytes=lim.skill_body_max_bytes,
            max_skill_files=lim.max_skill_files,
            max_mcp_servers=lim.max_mcp_servers,
            max_allowed_tools=lim.max_allowed_tools,
            max_cost_usd_per_run=lim.max_cost_usd_per_run,
            mcp_stdio_runners=tuple(lim.mcp_stdio_runners),
            question_max_chars=lim.question_max_chars,
        ),
        expires_at=p.expires_at,
    )


async def guarded_agent_interview(
    handler: Callable[[InterviewRequest], Awaitable[InterviewReply | None]],
    request: InterviewRequest,
    send: Callable[[UUID, InterviewReply], Awaitable[None]],
    *,
    clock: Clock,
) -> bool:
    """Run the handler until the request expires and send its answer once.

    Args:
        handler: The handler's ``on_agent_interview``.
        request: The request.
        send: Sends the answer (``answerAgentInterview``).
        clock: The time source of the guard.

    Returns:
        True when an answer was sent; False after a timeout or when the handler returned ``None``.
    """
    remaining = (request.expires_at - clock.now()).total_seconds()
    if remaining <= 0:
        return False
    work = asyncio.ensure_future(handler(request))
    guard = asyncio.ensure_future(clock.sleep(remaining))
    try:
        done, _ = await asyncio.wait({work, guard}, return_when=asyncio.FIRST_COMPLETED)
    finally:
        # Also when the guard itself is cancelled (the adapter closes): nothing outlives it.
        guard.cancel()
        if not work.done():
            work.cancel()
    if work not in done:
        _LOG.info("agent interview %s timed out; nothing sent", request.request_id)
        return False
    answer = work.result()
    if answer is None:
        return False
    await send(request.request_id, answer)
    return True


async def send_answer(client: Client, request_id: UUID, answer: InterviewReply) -> None:
    """Call ``answerAgentInterview``.

    Args:
        client: The adapter's client.
        request_id: The request.
        answer: The answer.
    """
    await client.call(
        answer_agent_interview.asyncio_detailed, request_id=request_id, body=answer.to_body()
    )
