# SPDX-License-Identifier: Apache-2.0
"""Order of the tool classes (docs/spec/09-agents.md 2.4 and 4.5 point 10, 07 2.11).

Examples:
    >>> exceeds("host", "scoped"), exceeds("chat", "scoped")
    (True, False)
"""

from __future__ import annotations

__all__ = ["ORDER", "exceeds", "rank"]

ORDER = ("chat", "scoped", "host")
"""The tool classes from the lowest to the highest."""


def rank(tool_class: str) -> int:
    """Return the position of a tool class in `ORDER`.

    Args:
        tool_class: ``chat``, ``scoped`` or ``host``.

    Returns:
        0, 1 or 2.

    Raises:
        ValueError: Not a tool class.
    """
    try:
        return ORDER.index(tool_class)
    except ValueError:
        raise ValueError("not a tool class") from None


def exceeds(configured: str, token: str) -> bool:
    """Tell whether the configured class is above the token's ``tool_class`` (AG-30).

    Args:
        configured: The class the runner loaded (``runner.tool_class``).
        token: The claim ``tool_class`` of the invocation token.

    Returns:
        True when no turn may start.
    """
    return rank(configured) > rank(token)
