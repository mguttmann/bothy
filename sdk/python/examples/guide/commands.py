# SPDX-License-Identifier: Apache-2.0
"""Step 6: the adapter's slash commands.

The SDK makes the agent's registered commands equal to this list after every connect: it lists
the commands in the channels of ``listServedChannels`` and registers, updates or removes the
agent's own ones. Suggestions come from a fixed list: typing must never cost a model call.
"""

from __future__ import annotations

from bothy_sdk.commands import AutocompleteRequest, CommandSpec
from bothy_sdk.generated.models import CommandChoice, CommandOption, CommandOptionType

TOPICS = ("billing", "login", "shipping")
"""The suggestions for the option ``topic``."""

COMMANDS = (
    CommandSpec(
        "summarize",
        "Summarize the thread",
        options=(
            CommandOption(
                name="topic",
                description="What the summary is about",
                type_=CommandOptionType.STRING,
                required=False,
                autocomplete=True,
            ),
        ),
    ),
    CommandSpec("version", "Show the adapter version", control=True),
)
"""``summarize`` starts a run (model command); ``version`` is answered at once (control)."""


def suggest(request: AutocompleteRequest) -> list[CommandChoice]:
    """Return the topics that start with what the person typed.

    Args:
        request: The autocomplete request.

    Returns:
        The matching topics.
    """
    typed = request.value.casefold()
    return [CommandChoice(name=t, value=t) for t in TOPICS if t.startswith(typed)]
