# SPDX-License-Identifier: Apache-2.0
"""The heartbeat building block for ``agent_heartbeat.v1`` (docs/spec/09-agents.md 4.8).

`build_heartbeat` returns the generated `Heartbeat` with exactly the members and ``details``
keys the server's decoder accepts (M2-20, docs/DECISIONS.md D-M2-20-2) and refuses, before
anything is sent, every value outside its form: names ``NAME_FORM``, versions
``VERSION_FORM``, codes ``CODE_FORM``, no duplicates in lists. These forms make paths,
whitespace, control and bidi characters and most key formats unrepresentable, so a heartbeat
cannot carry a secret, content or a path. A refusal is a `ValueError` that names the field and
never the value. The adapter fills ``details.active_runs`` and sends the result as member
``agent`` of every gateway heartbeat, or through ``sendHeartbeat`` with the polling source.

Examples:
    >>> beat = build_heartbeat(
    ...     "online",
    ...     runtime_version="2.3.1",
    ...     billing_evidence={"key_type": "api_key", "provider": "anthropic"},
    ...     details={"adapter": {"name": "example-adapter"}, "protocol_version": 1},
    ... )
    >>> beat.to_dict()["details"]
    {'adapter': {'name': 'example-adapter'}, 'protocol_version': 1}
    >>> build_heartbeat(runtime_version="/opt/runtime")
    Traceback (most recent call last):
    ...
    ValueError: runtime_version: does not match the version form
"""

from __future__ import annotations

import enum
import re
from collections.abc import Iterable, Mapping
from datetime import UTC, datetime
from typing import Any, TypeVar
from uuid import UUID

from bothy_sdk.generated.models import (
    AgentRateLimitState,
    AgentRateLimitStateState,
    BillingEvidence,
    BillingEvidenceKeyType,
    BillingMode,
    Heartbeat,
    HeartbeatState,
    JsonObject,
    ProjectReadiness,
    ToolClass,
)

__all__ = [
    "CAPABILITY_KEYS",
    "CODE_FORM",
    "DETAILS_KEYS",
    "HEARTBEAT_MEMBERS",
    "NAME_FORM",
    "VERSION_FORM",
    "build_heartbeat",
]

NAME_FORM = "^[a-z0-9][a-z0-9._-]*$"
"""Form of ``details.adapter.name`` and ``details.runtime.name``."""

VERSION_FORM = "^[0-9A-Za-z][0-9A-Za-z.+_-]*$"
"""Form of ``runtime_version`` and ``details.adapter.version``."""

CODE_FORM = "^[a-z0-9][a-z0-9_.:-]*$"
"""Form of ``billing_evidence.provider``, ``billing_evidence.method`` and each degraded reason."""

DETAILS_KEYS = (
    "capabilities",
    "active_runs",
    "degraded_reasons",
    "adapter",
    "runtime",
    "protocol_version",
)
"""The closed key set of ``details``."""

CAPABILITY_KEYS = ("followups", "components", "voice_input", "title", "files_out")
"""The closed key set of ``details.capabilities`` (09 2.2)."""

HEARTBEAT_MEMBERS = (
    "state",
    "runtime_version",
    "reported_billing_mode",
    "billing_modes_supported",
    "billing_evidence",
    "reported_tool_class",
    "rate_limit",
    "details",
    "project_readiness",
)
"""The top-level members `build_heartbeat` sets."""

_FOLLOWUP_MODES = ("steer", "queue")
_EVIDENCE_KEYS = ("provider", "key_type", "method")
_RATE_LIMIT_KEYS = ("state", "resets_at")
# `protocol_version` is a 32-bit integer on the server; a larger one is refused there.
_PROTOCOL_VERSION_MAX = 2**31 - 1

# The forms of the specification end with `$`, which in Python also matches before a final
# line break; `fullmatch` on the pattern without the anchors matches exactly the whole value.
_NAME = re.compile(NAME_FORM[1:-1])
_VERSION = re.compile(VERSION_FORM[1:-1])
_CODE = re.compile(CODE_FORM[1:-1])

E = TypeVar("E", bound=enum.Enum)


def build_heartbeat(  # noqa: PLR0913 - keywords mirror the members of Heartbeat
    state: str = "online",
    *,
    runtime_version: str | None = None,
    reported_billing_mode: str | None = None,
    billing_modes_supported: Iterable[str] | None = None,
    billing_evidence: BillingEvidence | Mapping[str, Any] | None = None,
    reported_tool_class: str | None = None,
    rate_limit: AgentRateLimitState | Mapping[str, Any] | None = None,
    details: Mapping[str, Any] | None = None,
    project_readiness: Iterable[ProjectReadiness] | None = None,
) -> Heartbeat:
    """Build a heartbeat of schema ``agent_heartbeat.v1``.

    Every heartbeat is the whole reported state: an argument left at ``None`` is "not
    reported" (09 4.8 "Full state"), except ``project_readiness``, where absent means
    "unchanged".

    Args:
        state: ``online`` or ``degraded``.
        runtime_version: Version of the agent runtime, version form.
        reported_billing_mode: ``personal`` or ``organization``; leave it out when the adapter
            follows the configured mode per run.
        billing_modes_supported: Distinct modes the adapter can prove.
        billing_evidence: ``key_type`` (required), ``provider`` and ``method`` (codes); never a
            key value or prefix.
        reported_tool_class: The tool class the runtime actually loaded.
        rate_limit: The rate-limit state of the runtime at its provider (M6-14): ``state``
            (``ok``, ``near_limit``, ``limited`` or ``unknown``) and optionally ``resets_at``, a
            `datetime` with a time zone; never a quota, key or subscription name. Leaving it out
            clears the state the server stored from an earlier heartbeat.
        details: The closed object ``details``: ``capabilities`` (``followups`` ``steer`` or
            ``queue``; ``components``, ``voice_input``, ``title``, ``files_out`` booleans),
            ``active_runs`` (distinct run IDs as `UUID` or string), ``degraded_reasons``
            (distinct codes), ``adapter`` (``name`` required, ``version`` optional),
            ``runtime`` (``name`` required) and ``protocol_version`` (integer, at least 1).
        project_readiness: Runners only: one entry per bound forum, distinct channels.

    Returns:
        The generated `Heartbeat`.

    Raises:
        ValueError: An unknown key, a missing required member, a value of the wrong type,
            outside its vocabulary or form, or a duplicate. The text names the field, never the
            value.
    """
    beat = Heartbeat(state=_member(HeartbeatState, state, "state"))
    if runtime_version is not None:
        beat.runtime_version = _form(runtime_version, _VERSION, "version", "runtime_version")
    if reported_billing_mode is not None:
        beat.reported_billing_mode = _member(
            BillingMode, reported_billing_mode, "reported_billing_mode"
        )
    if billing_modes_supported is not None:
        modes = [
            _member(BillingMode, m, "billing_modes_supported") for m in billing_modes_supported
        ]
        _distinct(modes, "billing_modes_supported")
        beat.billing_modes_supported = modes
    if billing_evidence is not None:
        beat.billing_evidence = _evidence(billing_evidence)
    if reported_tool_class is not None:
        beat.reported_tool_class = _member(ToolClass, reported_tool_class, "reported_tool_class")
    if rate_limit is not None:
        beat.rate_limit = _rate_limit(rate_limit)
    if details is not None:
        beat.details = JsonObject.from_dict(_details(details))
    if project_readiness is not None:
        readiness = list(project_readiness)
        for entry in readiness:
            if not isinstance(entry, ProjectReadiness):
                raise ValueError("project_readiness: entries must be ProjectReadiness")
        _distinct([r.channel_id for r in readiness], "project_readiness")
        beat.project_readiness = readiness
    return beat


def _member(kind: type[E], value: object, field: str) -> E:
    """A declared member of a generated enum.

    The tolerant enums of the core accept any string as a pseudo member, so the declared
    members are compared explicitly.
    """
    for member in kind:
        if value is member or (isinstance(value, str) and value == member.value):
            return member
    raise ValueError(f"{field}: not one of the allowed values")


def _form(value: object, form: re.Pattern[str], name: str, field: str) -> str:
    """`value` when it is a string of `form`."""
    if not isinstance(value, str):
        raise ValueError(f"{field}: must be a string")
    if form.fullmatch(value) is None:
        raise ValueError(f"{field}: does not match the {name} form")
    return value


def _distinct(values: list[Any], field: str) -> None:
    """Refuse a list with a repeated entry."""
    if len(set(values)) != len(values):
        raise ValueError(f"{field}: entries must be distinct")


def _closed(value: object, keys: Iterable[str], field: str) -> Mapping[str, Any]:
    """`value` as an object with no key outside `keys`.

    An unknown key is named only when it has the name form, because anything else could be
    the very text a heartbeat must not carry.
    """
    if not isinstance(value, Mapping):
        raise ValueError(f"{field}: must be an object")
    known = set(keys)
    for key in value:
        if key not in known:
            if isinstance(key, str) and _NAME.fullmatch(key):
                raise ValueError(f"{field}.{key}: unknown key")
            raise ValueError(f"{field}: unknown key")
    return value


def _evidence(value: BillingEvidence | Mapping[str, Any]) -> BillingEvidence:
    """``billing_evidence`` with ``key_type`` from its vocabulary and codes."""
    raw = value.to_dict() if isinstance(value, BillingEvidence) else value
    raw = _closed(raw, _EVIDENCE_KEYS, "billing_evidence")
    if "key_type" not in raw:
        raise ValueError("billing_evidence.key_type: required")
    evidence = BillingEvidence(
        key_type=_member(BillingEvidenceKeyType, raw["key_type"], "billing_evidence.key_type")
    )
    if "provider" in raw:
        evidence.provider = _form(raw["provider"], _CODE, "code", "billing_evidence.provider")
    if "method" in raw:
        evidence.method = _form(raw["method"], _CODE, "code", "billing_evidence.method")
    return evidence


def _rate_limit(value: AgentRateLimitState | Mapping[str, Any]) -> AgentRateLimitState:
    """``rate_limit``: ``state`` from its vocabulary and an aware ``resets_at`` in UTC."""
    if isinstance(value, AgentRateLimitState):
        raw: Mapping[str, Any] = {"state": value.state}
        if isinstance(value.resets_at, datetime):
            raw = {**raw, "resets_at": value.resets_at}
    else:
        raw = _closed(value, _RATE_LIMIT_KEYS, "rate_limit")
    if "state" not in raw:
        raise ValueError("rate_limit.state: required")
    out = AgentRateLimitState(
        state=_member(AgentRateLimitStateState, raw["state"], "rate_limit.state")
    )
    if "resets_at" in raw:
        at = raw["resets_at"]
        if not isinstance(at, datetime) or at.utcoffset() is None:
            raise ValueError("rate_limit.resets_at: must be a datetime with a time zone")
        out.resets_at = at.astimezone(UTC)
    return out


def _details(value: Mapping[str, Any]) -> dict[str, Any]:
    """The closed object ``details`` as JSON."""
    raw = _closed(value, DETAILS_KEYS, "details")
    out: dict[str, Any] = {}
    if "capabilities" in raw:
        out["capabilities"] = _capabilities(raw["capabilities"])
    if "active_runs" in raw:
        out["active_runs"] = _runs(raw["active_runs"])
    if "degraded_reasons" in raw:
        reasons = [
            _form(r, _CODE, "code", "details.degraded_reasons")
            for r in _list(raw["degraded_reasons"], "details.degraded_reasons")
        ]
        _distinct(reasons, "details.degraded_reasons")
        out["degraded_reasons"] = reasons
    if "adapter" in raw:
        adapter = _closed(raw["adapter"], ("name", "version"), "details.adapter")
        out["adapter"] = {"name": _name(adapter, "details.adapter.name")}
        if "version" in adapter:
            out["adapter"]["version"] = _form(
                adapter["version"], _VERSION, "version", "details.adapter.version"
            )
    if "runtime" in raw:
        runtime = _closed(raw["runtime"], ("name",), "details.runtime")
        out["runtime"] = {"name": _name(runtime, "details.runtime.name")}
    if "protocol_version" in raw:
        out["protocol_version"] = _protocol_version(raw["protocol_version"])
    return out


def _name(value: Mapping[str, Any], field: str) -> str:
    """The required member ``name`` in the name form."""
    if "name" not in value:
        raise ValueError(f"{field}: required")
    return _form(value["name"], _NAME, "name", field)


def _list(value: object, field: str) -> list[Any]:
    """`value` as a list; a string is no list of entries."""
    if isinstance(value, str | bytes | Mapping) or not isinstance(value, Iterable):
        raise ValueError(f"{field}: must be a list")
    return list(value)


def _capabilities(value: object) -> dict[str, Any]:
    """``details.capabilities`` (09 2.2)."""
    raw = _closed(value, CAPABILITY_KEYS, "details.capabilities")
    out: dict[str, Any] = {}
    for key in CAPABILITY_KEYS:
        if key not in raw:
            continue
        field = f"details.capabilities.{key}"
        if key == "followups":
            if not isinstance(raw[key], str) or raw[key] not in _FOLLOWUP_MODES:
                raise ValueError(f"{field}: not one of the allowed values")
        elif not isinstance(raw[key], bool):
            raise ValueError(f"{field}: must be a boolean")
        out[key] = raw[key]
    return out


def _runs(value: object) -> list[str]:
    """``details.active_runs``: distinct run IDs in canonical form."""
    runs: list[str] = []
    for entry in _list(value, "details.active_runs"):
        if isinstance(entry, UUID):
            runs.append(str(entry))
            continue
        try:
            runs.append(str(UUID(entry)))
        except (TypeError, ValueError, AttributeError):
            raise ValueError("details.active_runs: entries must be run IDs") from None
    _distinct(runs, "details.active_runs")
    return runs


def _protocol_version(value: object) -> int:
    """``details.protocol_version``: an integer of at least 1."""
    if isinstance(value, bool) or not isinstance(value, int):
        raise ValueError("details.protocol_version: must be an integer")
    if not 1 <= value <= _PROTOCOL_VERSION_MAX:
        raise ValueError("details.protocol_version: out of range")
    return value
