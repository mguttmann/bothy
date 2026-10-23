# SPDX-License-Identifier: Apache-2.0
"""Message components of ``component.v1`` (docs/spec/09-agents.md 11).

`button`, `select` and `form` return generated `ComponentCreate` objects for the ``components``
of a message with ``kind = card``; `approval_card` builds the approval request of 09 11.5 and
`question_card` a choice question of 09 11.6. The builders refuse, before anything is sent, what
the server refuses with ``400 validation_failed``: styles outside ``BUTTON_STYLES``, field types
outside ``FORM_FIELD_TYPES``, the limits ``component.custom_id_max_chars`` and
``component.form_max_fields``, the rules of each definition, and display texts with control or
bidi characters (SEC-IN-4). `check_components` holds a whole card against
``component.max_per_message`` and unique custom IDs; the outbox calls it before it stores a
message. The limits come from `SdkSettings`, whose defaults are the server's; the server stays
the authority. Errors are `ValueError` naming the field, never the text a caller passed.

Examples:
    >>> card = approval_card(with_always=False)
    >>> [c.custom_id for c in card]
    ['approval.once', 'approval.session', 'approval.deny']
    >>> button("refund", "Refund", "yes", style="danger").to_dict()["definition"]
    {'label': 'Refund', 'style': 'danger', 'value': 'yes'}
"""

from __future__ import annotations

import enum
import re
import unicodedata
from collections.abc import Sequence
from dataclasses import dataclass
from datetime import datetime
from typing import Any, TypeVar

from bothy_sdk.generated.models import (
    Capability,
    ComponentCreate,
    ComponentRestriction,
    ComponentType,
    JsonObject,
)
from bothy_sdk.settings import SdkSettings

__all__ = [
    "APPROVAL_VALUES",
    "BUTTON_STYLES",
    "FORM_FIELD_TYPES",
    "FormField",
    "SelectOption",
    "approval_card",
    "button",
    "check_components",
    "form",
    "question_card",
    "select",
]

BUTTON_STYLES = ("primary", "secondary", "success", "danger", "ghost")
"""Button styles: roles of the design system, never a free colour (09 11.1)."""

FORM_FIELD_TYPES = ("text", "textarea", "number", "boolean", "choice")
"""Types of form fields (09 11.1)."""

APPROVAL_VALUES = ("once", "session", "always", "deny")
"""Values of the approval buttons (09 11.5); their custom IDs are ``approval.<value>``."""

_DEFAULT_SETTINGS = SdkSettings()
_FIELD_NAME = re.compile(r"[a-z][a-z0-9_]*")
_LENGTH_TYPES = frozenset({"text", "textarea"})
# `terminal` exists only on the target `host` (07 2.4); a component cannot require it.
_CAPABILITIES = tuple(c for c in Capability if c is not Capability.TERMINAL)
# Bidi controls of SEC-IN-4, as kernel/textcheck of the server lists them.
_BIDI = frozenset("\u061c\u200e\u200f\u202a\u202b\u202c\u202d\u202e\u2066\u2067\u2068\u2069")

E = TypeVar("E", bound=enum.Enum)

# (label, style, required capability, restricted to) per approval value, the table of 09 11.5.
_APPROVAL: dict[str, tuple[str, str, Capability | None, ComponentRestriction]] = {
    "once": (
        "Allow once",
        "primary",
        Capability.APPROVE_ACTIONS,
        ComponentRestriction.RUN_STARTER,
    ),
    "session": (
        "Allow for this session",
        "secondary",
        Capability.APPROVE_ACTIONS,
        ComponentRestriction.RUN_STARTER,
    ),
    "always": (
        "Always allow",
        "secondary",
        Capability.APPROVE_ACTIONS,
        ComponentRestriction.OWNER,
    ),
    "deny": ("Deny", "danger", None, ComponentRestriction.RUN_STARTER),
}


@dataclass(frozen=True)
class SelectOption:
    """One option of a `select`, or one answer of a `question_card`.

    Attributes:
        label: Display text.
        value: Value the interaction reports; unique within the select.
        description: Optional display text under the label.
    """

    label: str
    value: str
    description: str | None = None


@dataclass(frozen=True)
class FormField:
    """One field of a `form`.

    Attributes:
        name: Key of the field in the submitted values, ``[a-z][a-z0-9_]*``, unique in the form.
        label: Display text.
        type: One of ``FORM_FIELD_TYPES``.
        required: Whether the field must be filled.
        max_length: Longest text, at least 1; only for ``text`` and ``textarea``.
        choices: Distinct answers; required for ``choice`` and only there.
    """

    name: str
    label: str
    type: str
    required: bool = False
    max_length: int | None = None
    choices: tuple[str, ...] = ()


def button(  # noqa: PLR0913 - keywords mirror the members of ComponentCreate
    custom_id: str,
    label: str,
    value: str,
    *,
    style: str = "secondary",
    icon: str | None = None,
    single_use: bool | None = None,
    required_capability: str | None = None,
    restricted_to: str | None = None,
    expires_at: datetime | None = None,
    settings: SdkSettings = _DEFAULT_SETTINGS,
) -> ComponentCreate:
    """Build a button.

    Args:
        custom_id: ID of the component in its message, at most
            ``component.custom_id_max_chars`` characters.
        label: Display text.
        value: Value the interaction reports.
        style: One of ``BUTTON_STYLES``.
        icon: Optional semantic icon name of the design system; the server checks the name.
        single_use: Whether using it disables the other single-use components of the message
            (09 11.3); the server's default applies when left out.
        required_capability: A capability the person needs, for example ``approve_actions``.
        restricted_to: ``any``, ``run_starter`` or ``owner`` (09 11.2).
        expires_at: Time zone aware expiry; the server's default applies when left out.
        settings: Source of the limits.

    Returns:
        The generated `ComponentCreate`.

    Raises:
        ValueError: A value breaks a rule of ``component.v1``; the text names the field.
    """
    definition: dict[str, Any] = {"label": _text(label, "label"), "style": _style(style)}
    if icon is not None:
        definition["icon"] = _text(icon, "icon")
    definition["value"] = _text(value, "value")
    return _component(
        ComponentType.BUTTON,
        custom_id,
        definition,
        single_use=single_use,
        required_capability=required_capability,
        restricted_to=restricted_to,
        expires_at=expires_at,
        settings=settings,
    )


def select(  # noqa: PLR0913 - keywords mirror the members of ComponentCreate
    custom_id: str,
    options: Sequence[SelectOption],
    *,
    placeholder: str | None = None,
    min_values: int | None = None,
    max_values: int | None = None,
    single_use: bool | None = None,
    required_capability: str | None = None,
    restricted_to: str | None = None,
    expires_at: datetime | None = None,
    settings: SdkSettings = _DEFAULT_SETTINGS,
) -> ComponentCreate:
    """Build a select.

    Args:
        custom_id: ID of the component in its message.
        options: At least one option with distinct values.
        placeholder: Optional display text.
        min_values: Fewest selected options, the server's default 1 when left out.
        max_values: Most selected options, the server's default 1 when left out; at least 1,
            at least `min_values` and at most the number of options.
        single_use: As for `button`.
        required_capability: As for `button`.
        restricted_to: As for `button`.
        expires_at: As for `button`.
        settings: Source of the limits.

    Returns:
        The generated `ComponentCreate`.

    Raises:
        ValueError: A value breaks a rule of ``component.v1``; the text names the field.
    """
    definition: dict[str, Any] = {}
    if placeholder is not None:
        definition["placeholder"] = _text(placeholder, "placeholder")
    definition["options"] = _options(options, "options")
    low = 1 if min_values is None else _integer(min_values, "min_values")
    high = 1 if max_values is None else _integer(max_values, "max_values")
    if high < 1 or high > len(options):
        raise ValueError("max_values: must be at least 1 and at most the number of options")
    if not 0 <= low <= high:
        raise ValueError("min_values: must be between 0 and max_values")
    if min_values is not None:
        definition["min_values"] = low
    if max_values is not None:
        definition["max_values"] = high
    return _component(
        ComponentType.SELECT,
        custom_id,
        definition,
        single_use=single_use,
        required_capability=required_capability,
        restricted_to=restricted_to,
        expires_at=expires_at,
        settings=settings,
    )


def form(  # noqa: PLR0913 - keywords mirror the members of ComponentCreate
    custom_id: str,
    title: str,
    fields: Sequence[FormField],
    *,
    submit_label: str | None = None,
    single_use: bool | None = None,
    required_capability: str | None = None,
    restricted_to: str | None = None,
    expires_at: datetime | None = None,
    settings: SdkSettings = _DEFAULT_SETTINGS,
) -> ComponentCreate:
    """Build a form.

    Args:
        custom_id: ID of the component in its message.
        title: Display text.
        fields: At least one and at most ``component.form_max_fields`` fields with distinct
            names.
        submit_label: Optional display text of the submit button.
        single_use: As for `button`.
        required_capability: As for `button`.
        restricted_to: As for `button`.
        expires_at: As for `button`.
        settings: Source of the limits.

    Returns:
        The generated `ComponentCreate`.

    Raises:
        ValueError: A value breaks a rule of ``component.v1``; the text names the field.
    """
    if not fields or len(fields) > settings.component_form_max_fields:
        raise ValueError("fields: between one field and component.form_max_fields fields")
    definition: dict[str, Any] = {"title": _text(title, "title")}
    if submit_label is not None:
        definition["submit_label"] = _text(submit_label, "submit_label")
    names: set[str] = set()
    rendered = []
    for index, item in enumerate(fields):
        rendered.append(_field(item, f"fields[{index}]"))
        if item.name in names:
            raise ValueError(f"fields[{index}].name: repeated")
        names.add(item.name)
    definition["fields"] = rendered
    return _component(
        ComponentType.FORM,
        custom_id,
        definition,
        single_use=single_use,
        required_capability=required_capability,
        restricted_to=restricted_to,
        expires_at=expires_at,
        settings=settings,
    )


def approval_card(
    *, with_always: bool = True, settings: SdkSettings = _DEFAULT_SETTINGS
) -> list[ComponentCreate]:
    """Build the approval buttons of 09 11.5.

    The buttons are ``approval.once``, ``approval.session``, ``approval.always`` and
    ``approval.deny``, all single use, so one decision disables the rest (09 11.3). Every
    button but ``deny`` requires ``approve_actions``; ``always`` is for owners only, because it
    changes the runtime's lasting allow list for every future trigger; the others are for the
    run's starter. Send them through the outbox as a message with ``kind = card`` and the run's
    ``run_id``.

    Args:
        with_always: False leaves out ``approval.always`` (an adapter setting of 09 11.5).
        settings: Source of the limits.

    Returns:
        The buttons in the order of the table.
    """
    card = [
        button(
            f"approval.{value}",
            label,
            value,
            style=style,
            single_use=True,
            required_capability=None if capability is None else capability.value,
            restricted_to=restricted.value,
            settings=settings,
        )
        for value, (label, style, capability, restricted) in _APPROVAL.items()
        if with_always or value != "always"
    ]
    check_components(card, settings)
    return card


def question_card(  # noqa: PLR0913 - every option of the card is explicit
    custom_id: str,
    answers: Sequence[SelectOption],
    *,
    as_select: bool = False,
    placeholder: str | None = None,
    restricted_to: str = "run_starter",
    settings: SdkSettings = _DEFAULT_SETTINGS,
) -> list[ComponentCreate]:
    """Build a choice question of 09 11.6.

    The run waits for the answer; whichever comes first, a use of the card or a follow-up,
    counts, so every component is single use. With buttons, answer ``i`` gets the custom ID
    ``<custom_id>.<i>`` and its value; with `as_select` one select of exactly one choice.

    Args:
        custom_id: ID of the select, or the prefix of the button IDs.
        answers: At least one answer with distinct values.
        as_select: True builds one select instead of one button per answer.
        placeholder: Display text of the select.
        restricted_to: Who may answer; the run's starter (and owners) by default.
        settings: Source of the limits.

    Returns:
        The components of the card.

    Raises:
        ValueError: No answers, a broken rule of ``component.v1`` or more components than
            ``component.max_per_message``.
    """
    if not answers:
        raise ValueError("answers: at least one answer is required")
    _options(answers, "answers")
    if as_select:
        card = [
            select(
                custom_id,
                answers,
                placeholder=placeholder,
                min_values=1,
                max_values=1,
                single_use=True,
                restricted_to=restricted_to,
                settings=settings,
            )
        ]
    else:
        if len(answers) > settings.component_max_per_message:
            raise ValueError("components: more than component.max_per_message components")
        card = [
            button(
                f"{custom_id}.{index}",
                answer.label,
                answer.value,
                single_use=True,
                restricted_to=restricted_to,
                settings=settings,
            )
            for index, answer in enumerate(answers)
        ]
    check_components(card, settings)
    return card


def check_components(
    components: Sequence[ComponentCreate], settings: SdkSettings = _DEFAULT_SETTINGS
) -> None:
    """Hold the components of one message against the limits.

    Args:
        components: The components of one message.
        settings: Source of ``component.max_per_message`` and ``component.custom_id_max_chars``.

    Raises:
        ValueError: More than ``component.max_per_message`` components, a repeated custom ID,
            or a custom ID outside its rules.
    """
    if len(components) > settings.component_max_per_message:
        raise ValueError("components: more than component.max_per_message components")
    seen: set[str] = set()
    for index, component in enumerate(components):
        _custom_id(component.custom_id, settings, f"components[{index}].custom_id")
        if component.custom_id in seen:
            raise ValueError(f"components: custom ID of components[{index}] repeated")
        seen.add(component.custom_id)


def _component(  # noqa: PLR0913 - keywords mirror the members of ComponentCreate
    kind: ComponentType,
    custom_id: str,
    definition: dict[str, Any],
    *,
    single_use: bool | None,
    required_capability: str | None,
    restricted_to: str | None,
    expires_at: datetime | None,
    settings: SdkSettings,
) -> ComponentCreate:
    """The generated component with the optional members that were given."""
    component = ComponentCreate(
        custom_id=_custom_id(custom_id, settings, "custom_id"),
        type_=kind,
        definition=JsonObject.from_dict(definition),
    )
    if single_use is not None:
        if not isinstance(single_use, bool):
            raise ValueError("single_use: must be a boolean")
        component.single_use = single_use
    if required_capability is not None:
        component.required_capability = _member(
            _CAPABILITIES, required_capability, "required_capability"
        )
    if restricted_to is not None:
        component.restricted_to = _member(
            tuple(ComponentRestriction), restricted_to, "restricted_to"
        )
    if expires_at is not None:
        if not isinstance(expires_at, datetime) or expires_at.utcoffset() is None:
            raise ValueError("expires_at: must be a time zone aware datetime")
        component.expires_at = expires_at
    return component


def _member(members: Sequence[E], value: object, field: str) -> E:
    """A declared member of a generated enum (the core's enums accept unknown values)."""
    for member in members:
        if value is member or (isinstance(value, str) and value == member.value):
            return member
    raise ValueError(f"{field}: not one of the allowed values")


def _custom_id(value: object, settings: SdkSettings, field: str) -> str:
    """A custom ID: a display-safe text of at most ``component.custom_id_max_chars``."""
    text = _text(value, field)
    if len(text) > settings.component_custom_id_max_chars:
        raise ValueError(f"{field}: longer than component.custom_id_max_chars")
    return text


def _text(value: object, field: str) -> str:
    """A non-empty display text without control or bidi characters (SEC-IN-4)."""
    if not isinstance(value, str) or not value:
        raise ValueError(f"{field}: must be a non-empty string")
    for char in value:
        if unicodedata.category(char) == "Cc" or char in _BIDI:
            raise ValueError(f"{field}: contains a control or bidi character")
    return value


def _style(value: object) -> str:
    """A style of ``BUTTON_STYLES``."""
    if value not in BUTTON_STYLES:
        raise ValueError("style: not one of the design system roles")
    return str(value)


def _integer(value: object, field: str) -> int:
    """An integer that is not a boolean."""
    if isinstance(value, bool) or not isinstance(value, int):
        raise ValueError(f"{field}: must be an integer")
    return value


def _options(options: Sequence[SelectOption], field: str) -> list[dict[str, str]]:
    """Options as JSON: at least one, distinct values."""
    if not options:
        raise ValueError(f"{field}: at least one option is required")
    out: list[dict[str, str]] = []
    values: set[str] = set()
    for index, option in enumerate(options):
        where = f"{field}[{index}]"
        item = {
            "label": _text(option.label, f"{where}.label"),
            "value": _text(option.value, f"{where}.value"),
        }
        if option.description is not None:
            item["description"] = _text(option.description, f"{where}.description")
        if option.value in values:
            raise ValueError(f"{where}.value: repeated")
        values.add(option.value)
        out.append(item)
    return out


def _field(item: FormField, where: str) -> dict[str, Any]:
    """One form field as JSON."""
    if not isinstance(item.name, str) or _FIELD_NAME.fullmatch(item.name) is None:
        raise ValueError(f"{where}.name: must match [a-z][a-z0-9_]*")
    if item.type not in FORM_FIELD_TYPES:
        raise ValueError(f"{where}.type: not one of the field types")
    out: dict[str, Any] = {
        "name": item.name,
        "label": _text(item.label, f"{where}.label"),
        "type": item.type,
    }
    if not isinstance(item.required, bool):
        raise ValueError(f"{where}.required: must be a boolean")
    out["required"] = item.required
    if item.max_length is not None:
        if item.type not in _LENGTH_TYPES:
            raise ValueError(f"{where}.max_length: only for text and textarea")
        if _integer(item.max_length, f"{where}.max_length") < 1:
            raise ValueError(f"{where}.max_length: must be at least 1")
        out["max_length"] = item.max_length
    if item.type == "choice":
        if not item.choices:
            raise ValueError(f"{where}.choices: required for a choice field")
        choices = [_text(c, f"{where}.choices") for c in item.choices]
        if len(set(choices)) != len(choices):
            raise ValueError(f"{where}.choices: entries must be distinct")
        out["choices"] = choices
    elif item.choices:
        raise ValueError(f"{where}.choices: only for a choice field")
    return out
