# SPDX-License-Identifier: Apache-2.0
"""Build the FakeServer's events and bodies from the recorded fixtures (private).

Every fixture in ``testing/fixtures/`` is a normalized object recorded from the real server
(fixed IDs and times, ``seq`` renumbered, no tokens; docs/DECISIONS.md, recorded fixtures). The
FakeServer loads one by file name, overrides the members it knows and substitutes every other
occurrence of a replaced fixture value generically, so a fixture recorded again with more
members still yields consistent IDs.
"""

from __future__ import annotations

import copy
import json
from collections.abc import Mapping
from functools import cache
from pathlib import Path
from typing import Any
from uuid import UUID

_DIR = Path(__file__).parent / "fixtures"


@cache
def _raw(name: str) -> str:
    return (_DIR / f"{name}.json").read_text(encoding="utf-8")


def load(name: str) -> dict[str, Any]:
    """A fresh copy of the fixture ``name`` (file ``<name>.json``)."""
    loaded: dict[str, Any] = json.loads(_raw(name))
    return loaded


def _collect(fixture: object, override: object, pairs: dict[str, str]) -> None:
    # Pair every fixture string with the override value at the same place.
    if isinstance(fixture, Mapping) and isinstance(override, Mapping):
        for key, value in override.items():
            if key in fixture:
                _collect(fixture[key], value, pairs)
    elif (
        isinstance(fixture, str)
        and isinstance(override, str)
        and fixture != override
        and _is_identity(fixture)
    ):
        pairs.setdefault(fixture, override)


def _is_identity(text: str) -> bool:
    # Only IDs and times are substituted; enum values such as a state stay where they are.
    try:
        UUID(text)
    except ValueError:
        return text.endswith("Z") and "T" in text
    return True


def _merge(base: dict[str, Any], override: Mapping[str, Any]) -> None:
    for key, value in override.items():
        if isinstance(value, Mapping) and isinstance(base.get(key), dict):
            _merge(base[key], value)
        else:
            base[key] = copy.deepcopy(value)


def _substitute(value: Any, pairs: Mapping[str, str]) -> Any:
    if isinstance(value, dict):
        return {k: _substitute(v, pairs) for k, v in value.items()}
    if isinstance(value, list):
        return [_substitute(v, pairs) for v in value]
    if isinstance(value, str):
        return pairs.get(value, value)
    return value


def _drop(doc: dict[str, Any], paths: tuple[str, ...]) -> None:
    for path in paths:
        *parents, last = path.split("/")
        node: Any = doc
        for part in parents:
            node = node.get(part, {}) if isinstance(node, dict) else {}
        if isinstance(node, dict):
            node.pop(last, None)


def build(
    name: str,
    override: Mapping[str, Any],
    *,
    replace: tuple[str, ...] = (),
    drop: tuple[str, ...] = (),
) -> dict[str, Any]:
    """The fixture ``name`` with ``override`` applied.

    Args:
        name: The fixture file name without ``.json``.
        override: Members to set (deep merge); fixture strings they replace are substituted
            everywhere else in the document too.
        replace: Top-level paths (``"payload"``) whose fixture value is replaced as a whole by
            the override instead of merged (members the FakeServer does not produce vanish).
        drop: Slash paths (``"payload/message_id"``) removed after the merge.

    Returns:
        The document.
    """
    doc = load(name)
    pairs: dict[str, str] = {}
    _collect(doc, override, pairs)
    for path in replace:
        doc.pop(path, None)
    _merge(doc, override)
    merged: dict[str, Any] = _substitute(doc, pairs)
    _drop(merged, drop)
    return merged
