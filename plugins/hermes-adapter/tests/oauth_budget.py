# SPDX-License-Identifier: Apache-2.0
"""A reproduction of a host's system text budget on the Anthropic OAuth route (D-FHUSC-6).

Some Hermes installations carry a patch (after upstream PR NousResearch/hermes-agent#72173) that
keeps the Claude Code identity block of the system text and caps everything after it at
``providers.anthropic.oauth_system_budget_chars`` characters, 3000 when unset. The pinned public
release has no such budget, so the Fake-Hermes test applies this reproduction to the request the
pinned ``build_anthropic_kwargs(..., is_oauth=True)`` built. It follows the patch's rules: text
blocks after the first stay whole while they fit, the first one that does not is cut at the last
line break before the remaining room (a hard cut when that keeps less than half), every later
text block is dropped, other blocks pass through. Cache markers are not reproduced; they do not
change which text reaches the model.

Examples:
    >>> trim([{"type": "text", "text": "id"}, {"type": "text", "text": "abc"}], 2)
    [{'type': 'text', 'text': 'id'}, {'type': 'text', 'text': 'ab'}]
"""

from __future__ import annotations

from typing import Any, Final

DEFAULT_BUDGET: Final = 3000
"""The patch's default of ``oauth_system_budget_chars`` (the profiles do not set it)."""


def trim(system: Any, budget: int = DEFAULT_BUDGET) -> Any:
    """``system`` as the host sends it on the OAuth route.

    Args:
        system: The ``system`` value of the request (a list of blocks, or anything else).
        budget: The characters allowed after the identity block; ``0`` disables the trim.

    Returns:
        The trimmed blocks (new dicts), or ``system`` unchanged.
    """
    if budget <= 0 or not isinstance(system, list) or len(system) < 2:
        return system
    remaining, kept = budget, [system[0]]
    for block in system[1:]:
        if not (isinstance(block, dict) and block.get("type") == "text"):
            kept.append(block)
            continue
        text = str(block.get("text", ""))
        if len(text) <= remaining:
            remaining -= len(text)
            kept.append(block)
            continue
        cut = _cut(text, remaining)
        remaining = 0
        if cut:
            kept.append({**block, "text": cut})
    return kept


def _cut(text: str, limit: int) -> str:
    cut = text.rfind("\n", 0, limit)
    if cut < limit // 2:
        cut = limit
    return text[:cut].rstrip()
