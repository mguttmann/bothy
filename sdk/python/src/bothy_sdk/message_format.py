# SPDX-License-Identifier: Apache-2.0
"""The format rules for agent replies (docs/spec/09-agents.md 4.11, ADR 0074).

The apps render an agent's reply as Markdown (docs/spec/01-product.md 6.14): they wrap running
text to the width of the view, show fenced code with its language, scroll a wide table or a
long code line sideways and copy a code block as it is. A reply that breaks lines by hand, draws
a table with spaces or puts a text to copy into a block quote looks broken there and arrives as
a bundle of short lines when copied. Every agent adapter therefore hands its runtime these rules
on every run as an instruction that reaches the model whole (the system prompt, or the user side
of the turn where the host may shorten its system text; never inside the context with its trust
marks), with `MESSAGE_FORMAT_TITLE` as the heading. This module is their only source:
an adapter never writes its own copy.

Examples:
    >>> MESSAGE_FORMAT_RULES.startswith("- ")
    True
"""

from __future__ import annotations

from typing import Final

__all__ = ["MESSAGE_FORMAT_RULES", "MESSAGE_FORMAT_TITLE"]

MESSAGE_FORMAT_TITLE: Final = "Message format"
"""The heading under which an adapter places `MESSAGE_FORMAT_RULES`."""

MESSAGE_FORMAT_RULES: Final = "\n".join(
    (
        "- Your reply is shown as Markdown in apps that wrap lines to the width of the screen. "
        "Write running text as paragraphs with no line breaks inside a paragraph: one paragraph "
        "is one line, and a blank line separates two paragraphs.",
        "- Put code, commands, logs and configuration only in a fenced code block with a "
        "language tag, for example ```python or ```bash.",
        "- Show tabular data as a GitHub Flavored Markdown table with a header row and a "
        "separator row; never line up columns with spaces, tabs or box drawing characters.",
        "- Use Markdown lists (`- ` or `1. `) for lists and Markdown headings (`#`) for headings.",
        "- Put a text that the reader should copy or forward (a message, a mail, a ticket text) "
        "in a fenced code block with the language tag `text`, each paragraph on one line and a "
        "blank line between paragraphs; never in a block quote.",
    )
)
"""The rules as a Markdown list, one rule per line."""
