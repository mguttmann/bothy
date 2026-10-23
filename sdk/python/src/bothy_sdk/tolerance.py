# SPDX-License-Identifier: Apache-2.0
"""Tolerance of the generated core for what a newer server may send (gate G5).

A new response field, enum value or event type is an additive change of the API
(docs/spec/06-api.md 13), so every client must accept it. The generated core of
openapi-python-client raises on an unknown enum value and on an event of an unknown type, and
it would take a JSON ``null`` for a value. This module fixes that from the outside: `install`
adds hooks to the generated classes, the generator keeps its standard settings and its output is
never edited (docs/spec/11-extensions.md 11.1, ADR 0026).

What `install` establishes:

* Unknown field: nothing to do; the generated models keep unknown members in
  ``additional_properties`` and ignore them otherwise.
* Unknown enum value: every generated enum returns a pseudo member that carries the raw
  value (`is_unknown` tells it apart). Exception: the one-member ``type`` enum of every event
  schema composed from ``EventEnvelope`` stays strict, because the union parser of the core
  tries the event models in order and a tolerant ``type`` would let it accept an unknown event
  as the first model whose payload happens to fit.
* JSON ``null``: ``from_dict`` of every generated model drops members whose value is ``None``
  before the generated code runs, so a received ``null`` counts as an absent member
  (docs/spec/06-api.md 2 "Kein null", ADR 0053).
* Unknown event type: ``EventPage.from_dict`` drops every item of ``events`` whose ``type`` is
  not in `KNOWN_EVENT_TYPES` before the generated code runs; the caller still advances with
  ``next_cursor`` (docs/spec/04-sync-protocol.md 7.4).

Not covered here: unknown gateway ops and close codes (the gateway module of M2-29). Known
values decode exactly as without the hooks.

Examples:
    >>> import bothy_sdk  # importing any part of the SDK installs the hooks
    >>> from bothy_sdk.generated.models import ThreadState
    >>> state = ThreadState("archived_example")
    >>> (state.value, is_unknown(state), is_unknown(ThreadState.OPEN))
    ('archived_example', True, False)
"""

from __future__ import annotations

import enum
import functools
import re
from collections.abc import Callable, Iterator, Mapping
from dataclasses import dataclass, field
from types import MappingProxyType
from typing import Any, Protocol, TypeVar, cast

import attrs

from bothy_sdk.generated import models

__all__ = [
    "KNOWN_EVENT_TYPES",
    "KnownEvent",
    "UnknownEvent",
    "decode_event",
    "generated_from_dict",
    "install",
    "is_unknown",
]

T = TypeVar("T")

# Marks a hook installed by this module, so that `install` stays idempotent.
_MARK = "__bothy_tolerance__"
_NAME = re.compile(r"[A-Za-z_]\w*")


class KnownEvent(Protocol):
    """What every generated durable event model offers (a structural type).

    The core has no common base class for its event models; this protocol names the members
    the hand layer relies on. ``isinstance`` against a concrete generated model narrows it.
    """

    @property
    def type_(self) -> enum.Enum:
        """The one-member ``type`` enum of the event schema."""

    def to_dict(self) -> dict[str, Any]:
        """The event as a JSON object."""


@dataclass(frozen=True)
class UnknownEvent:
    """An event whose ``type`` this SDK version does not know (or that has no ``type``).

    The core has no model for it (ADR 0026 "Confirmation"); callers skip it and keep advancing
    their cursor (docs/spec/04-sync-protocol.md 7.1).

    Attributes:
        type: The received ``type``, or ``""`` when it is missing or not a string.
        seq: The received ``seq`` when it is an integer, else ``None`` (ephemeral events have
            none).
        raw: The received object, read-only.
    """

    type: str
    seq: int | None
    raw: Mapping[str, Any] = field(hash=False)


def _models() -> Iterator[type[Any]]:
    """Every class the generated ``models`` package exports (it exports nothing else)."""
    for name in models.__all__:
        yield getattr(models, name)


def _resolve(annotation: object) -> list[type[Any]]:
    """Generated classes named in a field annotation (attrs keeps it as a string)."""
    names = _NAME.findall(annotation if isinstance(annotation, str) else "")
    return [c for n in names if isinstance(c := getattr(models, n, None), type)]


def _event_type_enums() -> dict[type[Any], type[enum.Enum]]:
    """Event models composed from ``EventEnvelope``, with the enum of their ``type``.

    Durable and ephemeral events alike; each ``type`` enum has exactly one member.
    """
    envelope = {a.name for a in attrs.fields(models.EventEnvelope)}
    found: dict[type[Any], type[enum.Enum]] = {}
    for cls in _models():
        if cls is models.EventEnvelope or not attrs.has(cls):
            continue
        fields = {a.name: a for a in attrs.fields(cls)}
        if not envelope <= set(fields) or "type_" not in fields:
            continue
        [kind] = _resolve(fields["type_"].type)
        found[cls] = cast("type[enum.Enum]", kind)
    return found


def _event_models() -> dict[str, type[Any]]:
    """The models of the ``Event`` union (the items of ``EventPage.events``) by ``type``."""
    union = _resolve(attrs.fields(models.EventPage).events.type)
    enums = _event_type_enums()
    by_type: dict[str, type[Any]] = {}
    for cls in union:
        [member] = list(enums[cls])
        by_type[str(member.value)] = cls
    return by_type


_EVENT_MODELS = MappingProxyType(_event_models())
_STRICT_ENUMS = frozenset(_event_type_enums().values())

KNOWN_EVENT_TYPES: frozenset[str] = frozenset(_EVENT_MODELS)
"""The discriminator values of the ``Event`` union of this SDK version, read from the
generated core. A test holds it equal to ``components.schemas.Event.discriminator.mapping``
of docs/api/openapi.yaml."""


def is_unknown(value: enum.Enum) -> bool:
    """Tell whether an enum value is one that the generated enum did not know.

    Args:
        value: A value of a generated enum, as the core decoded it.

    Returns:
        True for a pseudo member that carries a value received from a newer server; False for
        every member the enum declares.

    Examples:
        >>> from bothy_sdk.generated.models import ActorKind
        >>> is_unknown(ActorKind("robot_example")), is_unknown(ActorKind.USER)
        (True, False)
    """
    return all(value is not member for member in type(value))


def _missing(cls: type[enum.Enum], value: object) -> enum.Enum | None:
    """``_missing_`` hook of a tolerant enum: a pseudo member that carries ``value``.

    Only a value of the enum's data type (a string for every enum of this API) is kept; any
    other value still raises ``ValueError``, because it is malformed rather than new.
    """
    member_type: type[Any] = cls._member_type_  # type: ignore[attr-defined]
    if member_type is object or not isinstance(value, member_type):
        return None
    pseudo = cast("enum.Enum", cast("Any", member_type).__new__(cls, value))
    pseudo._value_ = value
    pseudo._name_ = None  # type: ignore[assignment]
    return pseudo


def _tolerant_missing(cls: type[enum.Enum], value: object) -> enum.Enum | None:
    return _missing(cls, value)


setattr(_tolerant_missing, _MARK, True)
# One hook object for every tolerant enum; `install` recognises it by the mark.
_MISSING_HOOK = classmethod(_tolerant_missing)


def _marked(attribute: object) -> bool:
    """Whether a class attribute (a classmethod) is a hook installed by this module."""
    return bool(getattr(getattr(attribute, "__func__", None), _MARK, False))


def _without_null(src: object) -> object:
    if isinstance(src, Mapping):
        return {k: v for k, v in src.items() if v is not None}
    return src


def _is_known_event(item: object) -> bool:
    # A `type` that is not a string (possibly unhashable) is never a known event type.
    if not isinstance(item, Mapping):
        return False
    kind = item.get("type")
    return isinstance(kind, str) and kind in KNOWN_EVENT_TYPES


def _hook_from_dict(cls: type[Any], page: bool) -> None:
    """Wrap the generated ``from_dict`` of ``cls`` with the tolerance steps.

    The generated function stays reachable through ``__wrapped__`` (`generated_from_dict`).
    """
    generated = cls.__dict__["from_dict"].__func__

    @functools.wraps(generated)
    def from_dict(model: type[Any], src_dict: Mapping[str, Any]) -> Any:
        data = _without_null(src_dict)
        if page and isinstance(data, dict) and isinstance(data.get("events"), list):
            data["events"] = [item for item in data["events"] if _is_known_event(item)]
        return generated(model, data)

    setattr(from_dict, _MARK, True)
    cls.from_dict = classmethod(from_dict)


def install() -> None:
    """Make the generated core tolerant (see the module documentation).

    Idempotent: a second call changes nothing. ``bothy_sdk/__init__.py`` calls it, so importing
    any part of the SDK installs the hooks before the core decodes anything.
    """
    for cls in _models():
        if issubclass(cls, enum.Enum):
            if cls not in _STRICT_ENUMS and not _marked(cls.__dict__.get("_missing_")):
                cls._missing_ = _MISSING_HOOK  # type: ignore[assignment]
        elif (
            attrs.has(cls)
            and "from_dict" in cls.__dict__
            and not _marked(cls.__dict__["from_dict"])
        ):
            _hook_from_dict(cls, page=cls is models.EventPage)


def generated_from_dict(cls: type[T]) -> Callable[[Mapping[str, Any]], T]:
    """The generated ``from_dict`` of a model without the tolerance hook.

    For tests and diagnostics that compare the hooked core with the plain one.

    Args:
        cls: A generated model class.

    Returns:
        A function that decodes a JSON object exactly as the generated code does.
    """
    current = cls.__dict__["from_dict"].__func__
    generated = getattr(current, "__wrapped__", current)
    return functools.partial(generated, cls)


def decode_event(data: Mapping[str, Any]) -> KnownEvent | UnknownEvent:
    """Decode one event object, classified by its ``type`` before any model is tried.

    Args:
        data: One event as received (an item of ``EventPage.events`` or a gateway dispatch).

    Returns:
        The generated model of exactly that type for a type in `KNOWN_EVENT_TYPES`; an
        `UnknownEvent` for any other or a missing ``type``. Never a model of another type,
        even when the payload would fit it.

    Raises:
        TypeError: ``data`` is not a JSON object.
        KeyError: A known event lacks a required member.
        ValueError: A known event carries a value of the wrong type.

    Examples:
        >>> event = decode_event({"type": "future.example", "seq": 9})
        >>> isinstance(event, UnknownEvent), event.type, event.seq
        (True, 'future.example', 9)
    """
    received: object = data  # callers may pass any decoded JSON value
    if not isinstance(received, Mapping):
        msg = f"decode_event needs an event object, got {type(received).__name__}"
        raise TypeError(msg)
    kind = data.get("type")
    model = _EVENT_MODELS.get(kind) if isinstance(kind, str) else None
    if model is not None:
        return cast("KnownEvent", model.from_dict(data))
    seq = data.get("seq")
    return UnknownEvent(
        type=kind if isinstance(kind, str) else "",
        seq=seq if isinstance(seq, int) and not isinstance(seq, bool) else None,
        raw=MappingProxyType(dict(data)),
    )
