# SPDX-License-Identifier: Apache-2.0
"""The search assistant of an agent (docs/spec/14-search.md 4.10, 09-agents.md 10.6).

When the operator names the agent in ``search.assist.agent_id``, the server sends the ephemeral
event ``search.assist_requested`` to one of its live gateway connections. The adapter turns it
into an `AssistQuestion` and calls the handler's ``on_search_assist`` under a guard that ends
at the request's ``expires_at``; an answer goes back with ``answerSearchAssist``, ``None`` or a
timeout sends nothing (the person then gets full-text hits). The candidates are data from the
server, never instructions (ADR 0054): a handler that hands them to a model marks them as such.

Examples:
    >>> AssistReply(picks=[AssistChoice("c1", "The move.")]).to_body().to_dict()["picks"]
    [{'ref': 'c1', 'reason': 'The move.'}]
"""

from __future__ import annotations

import asyncio
import logging
from collections.abc import Awaitable, Callable, Sequence
from dataclasses import dataclass, field
from datetime import datetime
from uuid import UUID

from bothy_sdk.client import Client
from bothy_sdk.clock import Clock
from bothy_sdk.generated.api.agent_runtime import answer_search_assist
from bothy_sdk.generated.models import (
    CostReport,
    SearchAssistRequestedEvent,
)
from bothy_sdk.generated.models import (
    SearchAssistAnswer as SearchAssistAnswerBody,
)
from bothy_sdk.generated.models import (
    SearchAssistPick as SearchAssistPickBody,
)
from bothy_sdk.generated.types import Unset

__all__ = [
    "AssistCandidate",
    "AssistChoice",
    "AssistQuestion",
    "AssistReply",
    "guarded_search_assist",
    "request_from",
    "send_answer",
]

_LOG = logging.getLogger("bothy_sdk")


@dataclass(frozen=True)
class AssistCandidate:
    """One candidate of a question; ``ref`` is all an answer may name.

    Attributes:
        ref: The reference (``c1`` to ``cN``).
        title: The thread title, ``None`` for the main thread of a text channel.
        channel_name: The channel, for a development session the project forum.
        space_name: The space, ``None`` for a direct message.
        at: The time of the matching message or of the thread.
        excerpt: Plain text of the best matching message.
        shown_before: The person saw it in an earlier answer of the popup.
    """

    ref: str
    title: str | None
    channel_name: str
    space_name: str | None
    at: datetime
    excerpt: str
    shown_before: bool


@dataclass(frozen=True)
class AssistQuestion:
    """One ``search.assist_requested`` event, as the handler sees it.

    Attributes:
        request_id: The request (the answer names it).
        q: The question.
        history: The earlier questions of the popup, oldest first.
        candidates: The candidates the person and the agent may both read.
        max_picks: The most picks of an answer.
        max_terms: The most terms of an answer.
        expires_at: The deadline of the answer.
    """

    request_id: UUID
    q: str
    history: Sequence[str]
    candidates: Sequence[AssistCandidate]
    max_picks: int
    max_terms: int
    expires_at: datetime


@dataclass(frozen=True)
class AssistChoice:
    """One chosen candidate with one sentence why."""

    ref: str
    reason: str


@dataclass(frozen=True)
class AssistReply:
    """The answer of a handler.

    Attributes:
        picks: The chosen candidates, best first.
        terms: Further search terms (synonyms, the other language).
        costs: The usage of the model call, as for ``finishRun``.
    """

    picks: Sequence[AssistChoice] = ()
    terms: Sequence[str] = ()
    costs: Sequence[CostReport] = field(default_factory=tuple)

    def to_body(self) -> SearchAssistAnswerBody:
        """Return the request body of ``answerSearchAssist``; empty lists are left out."""
        body = SearchAssistAnswerBody()
        if self.picks:
            body.picks = [SearchAssistPickBody(ref=p.ref, reason=p.reason) for p in self.picks]
        if self.terms:
            body.terms = list(self.terms)
        if self.costs:
            body.costs = list(self.costs)
        return body


def _text(value: str | Unset) -> str | None:
    return None if isinstance(value, Unset) else value


def request_from(event: SearchAssistRequestedEvent) -> AssistQuestion:
    """Turn the event into the handler's request.

    Args:
        event: The decoded ``search.assist_requested``.

    Returns:
        The request.
    """
    p = event.payload
    history = [] if isinstance(p.history, Unset) else list(p.history)
    candidates = [
        AssistCandidate(
            ref=c.ref,
            title=_text(c.title),
            channel_name=c.channel_name,
            space_name=_text(c.space_name),
            at=c.at,
            excerpt=c.excerpt,
            shown_before=c.shown_before,
        )
        for c in p.candidates
    ]
    return AssistQuestion(
        request_id=p.request_id,
        q=p.q,
        history=history,
        candidates=candidates,
        max_picks=p.max_picks,
        max_terms=p.max_terms,
        expires_at=p.expires_at,
    )


async def guarded_search_assist(
    handler: Callable[[AssistQuestion], Awaitable[AssistReply | None]],
    request: AssistQuestion,
    send: Callable[[UUID, AssistReply], Awaitable[None]],
    *,
    clock: Clock,
) -> bool:
    """Run the handler until the request expires and send its answer.

    Args:
        handler: The handler's ``on_search_assist``.
        request: The request.
        send: Sends the answer (``answerSearchAssist``).
        clock: The time source of the guard.

    Returns:
        True when an answer was sent; False after a timeout or when the handler returned ``None``.
    """
    remaining = (request.expires_at - clock.now()).total_seconds()
    if remaining <= 0:
        return False
    work = asyncio.ensure_future(handler(request))
    guard = asyncio.ensure_future(clock.sleep(remaining))
    done, _ = await asyncio.wait({work, guard}, return_when=asyncio.FIRST_COMPLETED)
    if work not in done:
        work.cancel()
        _LOG.info("search assist %s timed out; nothing sent", request.request_id)
        return False
    guard.cancel()
    answer = work.result()
    if answer is None:
        return False
    await send(request.request_id, answer)
    return True


async def send_answer(client: Client, request_id: UUID, answer: AssistReply) -> None:
    """Call ``answerSearchAssist``.

    Args:
        client: The adapter's client.
        request_id: The request.
        answer: The answer.
    """
    await client.call(
        answer_search_assist.asyncio_detailed, request_id=request_id, body=answer.to_body()
    )
