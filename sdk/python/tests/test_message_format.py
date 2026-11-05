# SPDX-License-Identifier: Apache-2.0
"""The format rules for agent replies (docs/spec/09-agents.md 4.11, F-AGENT-FORMAT).

The rules are the one source every agent adapter hands its runtime; the tests hold the rules
the clients rely on, so a reworded rule cannot drop one of them unnoticed.
"""

from __future__ import annotations

from bothy_sdk import MESSAGE_FORMAT_RULES as EXPORTED
from bothy_sdk.message_format import MESSAGE_FORMAT_RULES, MESSAGE_FORMAT_TITLE


def _flat(text: str) -> str:
    return " ".join(text.split())


def test_the_package_exports_the_same_rules() -> None:
    assert EXPORTED is MESSAGE_FORMAT_RULES
    assert MESSAGE_FORMAT_TITLE


def test_every_rule_the_clients_rely_on_is_stated() -> None:
    flat = _flat(MESSAGE_FORMAT_RULES)
    for phrase in (
        # Running text: no hard line breaks inside a paragraph (the live case of the ticket).
        "no line breaks inside a paragraph",
        # Code only fenced, with a language tag.
        "fenced code block with a language tag",
        # Tables as GFM tables.
        "GitHub Flavored Markdown table",
        # Text to copy or forward: a `text` block, one line per paragraph.
        "language tag `text`",
        "each paragraph on one line",
        # No block quote for text to copy.
        "never in a block quote",
    ):
        assert phrase in flat, phrase


def test_rules_are_a_markdown_list_without_dashes_or_wrapped_items() -> None:
    lines = MESSAGE_FORMAT_RULES.splitlines()
    assert lines
    # One rule per line: the source follows its own rule about hard line breaks.
    assert all(line.startswith("- ") for line in lines)
    assert chr(0x2013) not in MESSAGE_FORMAT_RULES
    assert chr(0x2014) not in MESSAGE_FORMAT_RULES
