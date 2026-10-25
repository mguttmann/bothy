# SPDX-License-Identifier: Apache-2.0
"""The example runtime: fixed rules on the trigger text stand in for a model.

A real adapter hands the rendered context to its model at this point. The guide's runtime only
looks for a few words in the trigger text, so following the guide costs nothing and its output
is the same on every run. It never repeats a text it reads.
"""

from __future__ import annotations

import asyncio
from dataclasses import dataclass, field
from enum import StrEnum
from uuid import UUID

from bothy_sdk.generated.models import CostReport

MODEL = "guide-rules"
"""The model name the example reports in ``startRun`` and in its usage."""

TOKENS_IN, TOKENS_OUT = 120, 40
"""The usage the example reports per run: fixed, because there is no model."""

ANSWER = "Thanks for the details. I have checked the ticket and will take care of it."
"""The answer when no other rule applies."""

FILE_NAME = "summary.txt"
FILE_TEXT = "Ticket summary: one open ticket, waiting for a reply.\n"
"""The text file the rule ``file`` answers with."""


class Rule(StrEnum):
    """What the runtime does with a run."""

    FOLLOWUP = "follow-up"
    QUESTION = "question"
    APPROVAL = "approval"
    LONG = "long"
    FILE = "file"
    SUMMARIZE = "summarize"
    ANSWER = "answer"


# Case-insensitive, first match wins.
_WORDS = (Rule.FOLLOWUP, Rule.QUESTION, Rule.APPROVAL, Rule.LONG, Rule.FILE)


def decide(trigger_text: str, command: str | None = None) -> Rule:
    """Pick the rule for a run.

    Args:
        trigger_text: The text of the triggering message (empty for a command).
        command: The name of the model command that started the run, if any.

    Returns:
        The rule.

    Examples:
        >>> decide("Please ask me a QUESTION first")
        <Rule.QUESTION: 'question'>
        >>> decide("", command="summarize")
        <Rule.SUMMARIZE: 'summarize'>
    """
    text = trigger_text.casefold()
    for rule in _WORDS:
        if rule.value in text:
            return rule
    if command == Rule.SUMMARIZE.value:
        return Rule.SUMMARIZE
    return Rule.ANSWER


def summary(topic: str | None) -> str:
    """The answer of the model command ``summarize``.

    Args:
        topic: The value of the option ``topic``, if given.

    Returns:
        A short summary that names the topic.
    """
    return f"Summary for topic {topic or 'all'}: one open ticket, waiting for a reply."


def usage() -> CostReport:
    """The usage of one run, reported with ``finishRun``.

    No ``amount_micros``: the server prices the tokens with its price table.

    Returns:
        One report for the example's runtime.
    """
    return CostReport(
        provider="example", model=MODEL, currency="EUR", tokens_in=TOKENS_IN, tokens_out=TOKENS_OUT
    )


@dataclass
class Work:
    """The runtime's state of one run.

    Attributes:
        rule: What the run does.
        channel_id: The run's channel (uploads belong to a channel).
        topic: The option ``topic`` of a model command.
        card_message_id: The message of a question or approval card.
        stopped: Set when the run is asked to stop.
        stop_reason: Why it stops.
        followup: Set when a follow-up was absorbed.
    """

    rule: Rule
    channel_id: UUID | None = None
    topic: str | None = None
    card_message_id: UUID | None = None
    stopped: asyncio.Event = field(default_factory=asyncio.Event)
    stop_reason: str = "user_cancelled"
    followup: asyncio.Event = field(default_factory=asyncio.Event)
