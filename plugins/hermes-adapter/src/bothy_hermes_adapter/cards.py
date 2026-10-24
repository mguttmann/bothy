# SPDX-License-Identifier: Apache-2.0
"""Cards for the Hermes prompts that wait for a person (09 11, 17.3; SEC-AI-4).

Every card is a message ``kind = card`` of the run with ``single_use`` components whose
``custom_id`` is Hermes's callback ID (docs/DECISIONS.md D-M2-33-9):

* clarify: one button per answer ``cl:<id>:<idx>`` (a select ``cl:<id>:select`` with the index as
  value above `MAX_BUTTONS` answers), ``restricted_to: run_starter``; an open question has no
  component and is answered with a follow-up;
* approval: the buttons of 09 11.5 ``appr:<id>:<choice>`` with ``once``, ``session``,
  ``always`` and ``deny`` (only the choices Hermes offers), ``required_capability:
  approve_actions`` except ``deny``, ``restricted_to: run_starter`` and ``owner`` for ``always``;
* slash confirmation: "Once", "Always", "Cancel" as ``sc:<choice>:<id>``, "Always" restricted to
  ``owner`` (it changes the runtime's permanent allow list, R-AG-12), the others to
  ``run_starter``.

The server disables the other single-use components of the card in the transaction of the first
interaction (09 11.3), so the adapter sends no ``updateComponent`` after an interaction.

Examples:
    >>> parse_custom_id("appr:7:once")
    ('appr', '7', 'once')
    >>> parse_custom_id("sc:always:c1")
    ('sc', 'c1', 'always')
"""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Final

from bothy_sdk.components import SelectOption, button, select
from bothy_sdk.generated.models import ComponentCreate
from bothy_sdk.settings import SdkSettings

from bothy_hermes_adapter.runtime import CardKind, CardRequest, OpenCard

__all__ = [
    "APPROVAL_BUTTONS",
    "MAX_BUTTONS",
    "MAX_RESPONSE_CHARS",
    "build_components",
    "card_text",
    "chosen_label",
    "parse_custom_id",
    "status_line",
]

MAX_BUTTONS: Final = 5
"""Clarify answers shown as buttons; more become one select."""

MAX_RESPONSE_CHARS: Final = 200
"""Longest ``response.text`` of an answered interaction (the chosen label)."""

_LABEL_CHARS: Final = 80
_STATUS_CHARS: Final = 200

APPROVAL_BUTTONS: Final[Mapping[str, tuple[str, str, str | None, str]]] = {
    "once": ("Allow once", "primary", "approve_actions", "run_starter"),
    "session": ("Allow for this session", "secondary", "approve_actions", "run_starter"),
    "always": ("Always allow", "secondary", "approve_actions", "owner"),
    "deny": ("Deny", "danger", None, "run_starter"),
}
"""Approval buttons of 09 11.5: choice to (label, style, capability, restriction)."""

_SLASH_BUTTONS: Final = (
    ("once", "Once", "primary", "run_starter"),
    ("always", "Always", "secondary", "owner"),
    ("cancel", "Cancel", "danger", "run_starter"),
)


def _label(text: str) -> str:
    """A display label: one line, at most `_LABEL_CHARS` characters."""
    line = " ".join(str(text).split())
    return line if len(line) <= _LABEL_CHARS else line[: _LABEL_CHARS - 1] + "…"


def card_text(request: CardRequest) -> str:
    """The Markdown of the card message.

    Args:
        request: The Hermes prompt.

    Returns:
        The title and question.
    """
    if request.kind is CardKind.SLASH_CONFIRM and request.title:
        return f"**{_label(request.title)}**\n\n{request.question}"
    return request.question


def status_line(request: CardRequest) -> str:
    """The run's status line while the card waits: the question's first line.

    Args:
        request: The Hermes prompt.

    Returns:
        A short line.
    """
    first = next((line.strip() for line in request.question.splitlines() if line.strip()), "")
    first = first or _label(request.title) or "Waiting for an answer"
    return first[:_STATUS_CHARS]


def build_components(request: CardRequest, settings: SdkSettings) -> list[ComponentCreate]:
    """The components of a card.

    Args:
        request: The Hermes prompt.
        settings: The SDK settings (``component.custom_id_max_chars`` and friends).

    Returns:
        The components; empty for an open clarify question.

    Raises:
        ValueError: A ``custom_id`` would exceed ``component.custom_id_max_chars``.
    """
    rid = request.hermes_id
    if request.kind is CardKind.CLARIFY:
        if not request.choices:
            return []
        if len(request.choices) > MAX_BUTTONS:
            options = [
                SelectOption(_label(choice), str(i)) for i, choice in enumerate(request.choices)
            ]
            return [
                select(
                    f"cl:{rid}:select",
                    options,
                    single_use=True,
                    restricted_to="run_starter",
                    settings=settings,
                )
            ]
        return [
            button(
                f"cl:{rid}:{i}",
                _label(choice),
                str(i),
                single_use=True,
                restricted_to="run_starter",
                settings=settings,
            )
            for i, choice in enumerate(request.choices)
        ]
    if request.kind is CardKind.APPROVAL:
        offered = [c for c in request.choices if c in APPROVAL_BUTTONS] or list(APPROVAL_BUTTONS)
        out = []
        for choice in APPROVAL_BUTTONS:
            if choice not in offered:
                continue
            label, style, capability, restriction = APPROVAL_BUTTONS[choice]
            out.append(
                button(
                    f"appr:{rid}:{choice}",
                    label,
                    choice,
                    style=style,
                    single_use=True,
                    required_capability=capability,
                    restricted_to=restriction,
                    settings=settings,
                )
            )
        return out
    return [
        button(
            f"sc:{choice}:{rid}",
            label,
            choice,
            style=style,
            single_use=True,
            restricted_to=restriction,
            settings=settings,
        )
        for choice, label, style, restriction in _SLASH_BUTTONS
    ]


def parse_custom_id(custom_id: str) -> tuple[str, str, str] | None:
    """Split a callback ID into prefix, Hermes ID and choice.

    Args:
        custom_id: ``cl:<id>:<idx>``, ``appr:<id>:<choice>`` or ``sc:<choice>:<id>``.

    Returns:
        ``(prefix, hermes_id, choice)``, or ``None`` for another form.
    """
    parts = custom_id.split(":")
    if len(parts) != 3 or not all(parts):
        return None
    prefix = parts[0]
    if prefix in ("cl", "appr"):
        return prefix, parts[1], parts[2]
    if prefix == "sc":
        return prefix, parts[2], parts[1]
    return None


def chosen_label(card: OpenCard, custom_id: str, values: Mapping[str, Any]) -> str | None:
    """The answer a component stands for: the clarify text or the approval choice.

    Args:
        card: The open card.
        custom_id: The chosen component.
        values: The submitted values (a select reports the chosen index).

    Returns:
        The answer, or ``None`` when the component does not belong to the card.
    """
    parsed = parse_custom_id(custom_id)
    if parsed is None or parsed[1] != card.request.hermes_id:
        return None
    prefix, _, choice = parsed
    if prefix == "cl":
        if choice == "select":
            raw = values.get("values", values.get("value"))
            choice = str(raw[0]) if isinstance(raw, list) and raw else str(raw)
        if not choice.isdigit() or int(choice) >= len(card.request.choices):
            return None
        return card.request.choices[int(choice)]
    return choice
