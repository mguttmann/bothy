# SPDX-License-Identifier: Apache-2.0
"""Run patches of the FakeServer: the path table of docs/spec/09-agents.md 5.3 (private).

Static checks run over every op first (all field errors in one ``400 validation_failed``), then
the ops apply in order on a copy of the live box (the first failure is the only error). Set
semantics: ``add`` and ``replace`` set a member also when it is absent, ``remove`` of an absent
member changes nothing. Follow-up ops always miss their target, as in the server until M2-11.
"""

from __future__ import annotations

import copy
import math
import re
from collections.abc import Callable
from datetime import datetime
from typing import Any
from uuid import UUID

from bothy_sdk.testing._fake_base import FakeProblem

_FORBIDDEN = "this path is not patchable with this operation"
_INT_METRICS = frozenset(
    {
        "tokens_in",
        "tokens_out",
        "tokens_cache_read",
        "tokens_cache_write",
        "tokens_cache_write_1h",
        "subagents",
        "background_jobs",
        "api_retries",
    }
)
_STEP_KINDS = frozenset({"tool_call", "status", "subagent", "workflow", "background_job"})
_STEP_STATUS = frozenset({"pending", "running", "completed", "failed", "cancelled"})
_PLAN_STATUS = frozenset({"pending", "in_progress", "completed"})
_STEP_MEMBERS = frozenset(
    {
        "id",
        "kind",
        "name",
        "title",
        "status",
        "detail",
        "parent_step_id",
        "started_at",
        "finished_at",
    }
)
_MAX_SAFE = 2**53 - 1
_BIDI = frozenset("\u061c\u200e\u200f\u202a\u202b\u202c\u202d\u202e\u2066\u2067\u2068\u2069")
_CURRENCY = re.compile(r"^[A-Z]{3}$")
_ABSENT = object()


def text_error(value: object) -> str | None:
    """The code of a display text violation, or ``None``."""
    if not isinstance(value, str):
        return "invalid_type"
    for ch in value:
        code = ord(ch)
        if code < 0x20 or 0x7F <= code <= 0x9F:
            return "control_character"
        if ch in _BIDI:
            return "bidi_control"
    return None


def is_uuid7(text: str) -> bool:
    """Whether ``text`` is a UUIDv7 (version 7, RFC 4122 variant)."""
    try:
        parsed = UUID(text)
    except ValueError:
        return False
    return parsed.version == 7 and parsed.variant == "specified in RFC 4122"


def _pointer(path: str) -> list[str] | None:
    if path == "":
        return []
    if not path.startswith("/"):
        return None
    tokens = []
    for raw in path[1:].split("/"):
        if re.search(r"~(?![01])", raw):
            return None
        tokens.append(raw.replace("~1", "/").replace("~0", "~"))
    return tokens


def _route(op: str, tokens: list[str]) -> str | None:  # noqa: PLR0911 - one answer per row
    """The path kind for ``op`` on ``tokens``, or ``None`` when forbidden."""
    simple = {
        ("status_line",): ("text", {"add", "replace", "remove"}),
        ("last_text_excerpt",): ("excerpt", {"add", "replace"}),
        ("model",): ("text", {"replace"}),
        ("cost",): ("money", {"replace"}),
        ("plan",): ("plan", {"add", "replace", "remove"}),
        ("metrics", "last_api_retry"): ("retry", {"add", "replace", "remove"}),
    }
    found = simple.get(tuple(tokens))
    if found is not None:
        return found[0] if op in found[1] else None
    if len(tokens) == 2 and tokens[0] == "metrics":
        if tokens[1] in _INT_METRICS or tokens[1] == "context_pct":
            return "metric" if op in ("add", "replace") else None
        return None
    if tokens and tokens[0] == "steps":
        if len(tokens) == 2 and op == "add":
            return "step"
        if (
            len(tokens) == 3
            and op == "replace"
            and tokens[2]
            in (
                "status",
                "title",
                "detail",
                "finished_at",
            )
        ):
            return "step_member"
        return None
    if len(tokens) == 3 and tokens[0] == "followups" and tokens[2] == "state":
        return "followup" if op == "replace" else None
    return None


def _first_null(value: Any, at: str) -> str | None:
    if value is None:
        return at
    if isinstance(value, dict):
        for key in sorted(value):
            found = _first_null(value[key], f"{at}/{key}")
            if found:
                return found
    if isinstance(value, list):
        for i, item in enumerate(value):
            found = _first_null(item, f"{at}/{i}")
            if found:
                return found
    return None


def _rfc3339(value: object) -> bool:
    if not isinstance(value, str):
        return False
    try:
        datetime.fromisoformat(value)
    except ValueError:
        return False
    return "T" in value


def _integral(value: object) -> bool:
    return (
        isinstance(value, int | float)
        and not isinstance(value, bool)
        and math.isfinite(value)
        and float(value).is_integer()
    )


def _check_value(  # noqa: PLR0911, PLR0912, PLR0915 - one branch per value kind
    kind: str, member: str | None, value: Any, at: str
) -> list[dict[str, str]]:
    errs: list[dict[str, str]] = []

    def err(pointer: str, code: str) -> list[dict[str, str]]:
        errs.append({"pointer": pointer, "code": code})
        return errs

    if kind == "text":
        code = text_error(value)
        return err(at, code) if code else errs
    if kind == "excerpt":
        return errs if isinstance(value, str) else err(at, "invalid_type")
    if kind == "metric":
        if not isinstance(value, int | float) or isinstance(value, bool):
            return err(at, "invalid_type")
        if member == "context_pct":
            if value < 0:
                return err(at, "too_small")
            return err(at, "too_large") if value > 100 else errs
        if not _integral(value):
            return err(at, "invalid_type")
        if value < 0:
            return err(at, "too_small")
        return err(at, "too_large") if value > _MAX_SAFE else errs
    if kind == "retry":
        if not isinstance(value, dict):
            return err(at, "invalid_type")
        for name in sorted(set(value) - {"attempt", "max_retries", "retry_delay_ms", "error"}):
            err(f"{at}/{name}", "unknown_member")
        if "attempt" not in value:
            err(f"{at}/attempt", "required")
        for name in ("attempt", "max_retries", "retry_delay_ms"):
            if name in value and not _integral(value[name]):
                err(f"{at}/{name}", "invalid_type")
        if "error" in value and (code := text_error(value["error"])):
            err(f"{at}/error", code)
        return errs
    if kind == "money":
        if not isinstance(value, dict):
            return err(at, "invalid_type")
        for name in sorted(set(value) - {"amount_micros", "currency", "is_estimate"}):
            err(f"{at}/{name}", "unknown_member")
        for name in ("amount_micros", "currency", "is_estimate"):
            if name not in value:
                err(f"{at}/{name}", "required")
        if "amount_micros" in value and (
            not _integral(value["amount_micros"]) or value["amount_micros"] < 0
        ):
            err(f"{at}/amount_micros", "invalid_type")
        if "currency" in value and not _CURRENCY.match(str(value["currency"])):
            err(f"{at}/currency", "pattern_mismatch")
        if "is_estimate" in value and value["is_estimate"] is not True:
            err(f"{at}/is_estimate", "invalid")
        return errs
    if kind == "plan":
        items = value.get("items") if isinstance(value, dict) else None
        if not isinstance(items, list):
            return err(f"{at}/items", "invalid_type")
        seen: set[object] = set()
        for j, item in enumerate(items):
            ip = f"{at}/items/{j}"
            if not isinstance(item, dict):
                err(ip, "invalid_type")
                continue
            for name in ("id", "text", "status"):
                if name not in item:
                    err(f"{ip}/{name}", "required")
            if item.get("id") in seen:
                err(f"{ip}/id", "invalid")
            seen.add(item.get("id"))
            if "text" in item and (code := text_error(item["text"])):
                err(f"{ip}/text", code)
            if "status" in item and item["status"] not in _PLAN_STATUS:
                err(f"{ip}/status", "invalid_enum")
        return errs
    if kind == "step":
        return _check_step(value, at, member)
    if kind == "step_member":
        if member == "status" and value not in _STEP_STATUS:
            return err(at, "invalid_enum")
        if member == "title" and (code := text_error(value)):
            return err(at, code)
        if member == "detail" and not isinstance(value, dict):
            return err(at, "invalid_type")
        if member == "finished_at" and not _rfc3339(value):
            return err(at, "invalid_format")
        return errs
    if kind == "followup" and value not in ("delivered", "absorbed"):
        return err(at, "invalid_enum")
    return errs


def _check_step(  # noqa: PLR0912 - one rule per member
    value: Any, at: str, step_id: str | None
) -> list[dict[str, str]]:
    errs: list[dict[str, str]] = []
    if not isinstance(value, dict):
        return [{"pointer": at, "code": "invalid_type"}]
    if "ordinal" in value:
        errs.append({"pointer": f"{at}/ordinal", "code": "patch_path_forbidden"})
    for name in sorted(set(value) - _STEP_MEMBERS - {"ordinal"}):
        errs.append({"pointer": f"{at}/{name}", "code": "unknown_member"})
    if "id" in value and value["id"] != step_id:
        errs.append({"pointer": f"{at}/id", "code": "invalid"})
    if value.get("kind") not in _STEP_KINDS:
        errs.append(
            {"pointer": f"{at}/kind", "code": "required" if "kind" not in value else "invalid_enum"}
        )
    if "name" not in value:
        errs.append({"pointer": f"{at}/name", "code": "required"})
    for name in ("name", "title"):
        if name in value and (code := text_error(value[name])):
            errs.append({"pointer": f"{at}/{name}", "code": code})
    if "status" in value and value["status"] not in _STEP_STATUS:
        errs.append({"pointer": f"{at}/status", "code": "invalid_enum"})
    if "detail" in value and not isinstance(value["detail"], dict):
        errs.append({"pointer": f"{at}/detail", "code": "invalid_type"})
    parent = value.get("parent_step_id")
    if parent is not None and not (isinstance(parent, str) and is_uuid7(parent)):
        errs.append({"pointer": f"{at}/parent_step_id", "code": "invalid_format"})
    for name in ("started_at", "finished_at"):
        if name in value and not _rfc3339(value[name]):
            errs.append({"pointer": f"{at}/{name}", "code": "invalid_format"})
    return errs


def parse_ops(ops: Any) -> list[tuple[str, list[str], str, Any]]:
    """The static checks of every op.

    Returns:
        Per op: the op, the pointer tokens, the path kind and the value (``_ABSENT`` if none).

    Raises:
        FakeProblem: ``400 validation_failed`` with every field error in op order.
    """
    if not isinstance(ops, list):
        raise FakeProblem("validation_failed", errors=[{"pointer": "/ops", "code": "invalid_type"}])
    errors: list[dict[str, str]] = []
    parsed: list[tuple[str, list[str], str, Any]] = []
    for i, raw in enumerate(ops):
        at = f"/ops/{i}"
        if not isinstance(raw, dict) or not isinstance(raw.get("path"), str):
            errors.append({"pointer": at, "code": "invalid_type"})
            continue
        op = raw.get("op")
        if op not in ("add", "replace", "remove"):
            errors.append({"pointer": f"{at}/op", "code": "invalid_enum"})
            continue
        tokens = _pointer(raw["path"])
        if tokens is None:
            errors.append({"pointer": f"{at}/path", "code": "invalid_format"})
            continue
        if len(tokens) >= 2 and tokens[0] in ("steps", "followups") and not is_uuid7(tokens[1]):
            errors.append({"pointer": f"{at}/path", "code": "invalid_format"})
            continue
        kind = _route(op, tokens)
        if kind is None:
            errors.append({"pointer": at, "code": "patch_path_forbidden", "detail": _FORBIDDEN})
            continue
        value = raw.get("value", _ABSENT)
        if op == "remove":
            parsed.append((op, tokens, kind, _ABSENT))
            continue
        if value is _ABSENT:
            errors.append({"pointer": f"{at}/value", "code": "required"})
            continue
        null_at = _first_null(value, f"{at}/value")
        if null_at:
            errors.append({"pointer": null_at, "code": "invalid_type"})
            continue
        member = tokens[1] if kind in ("metric", "step") else tokens[-1]
        found = _check_value(kind, member, value, f"{at}/value")
        if found:
            errors.extend(found)
            continue
        parsed.append((op, tokens, kind, value))
    if errors:
        raise FakeProblem(
            "validation_failed", detail="the patch has invalid operations", errors=errors
        )
    return parsed


_FOLLOWUP_ORDER = {"queued": 0, "delivered": 1, "absorbed": 2, "deferred": 3, "discarded": 3}


def _move_followup(rows: dict[str, dict[str, Any]], message_id: str, state: str) -> str | None:
    """Move a follow-up row forward; an error code when the row is missing or further."""
    row = rows.get(message_id)
    if row is None:
        return "patch_target_missing"
    if _FOLLOWUP_ORDER[state] <= _FOLLOWUP_ORDER.get(row["state"], 0):
        return "invalid_transition"
    row["state"] = state
    return None


def apply_ops(  # noqa: PLR0912 - one branch per path kind
    live: dict[str, Any],
    parsed: list[tuple[str, list[str], str, Any]],
    *,
    now_iso: Callable[[], str],
    excerpt_max: int,
) -> dict[str, Any]:
    """Apply checked ops to a copy of the live box.

    Returns:
        The new live box.

    Raises:
        FakeProblem: ``400 validation_failed`` with the first dynamic error.
    """
    box = copy.deepcopy(live)
    steps: dict[str, dict[str, Any]] = box.setdefault("steps", {})
    for i, (op, tokens, kind, value) in enumerate(parsed):

        def fail(pointer: str, code: str, index: int = i) -> FakeProblem:
            return FakeProblem(
                "validation_failed",
                detail="the patch does not apply to the run",
                errors=[{"pointer": pointer.format(i=index), "code": code}],
            )

        if kind in ("text", "excerpt"):
            text = "" if op == "remove" else value
            if kind == "excerpt" and text:
                text = text[-excerpt_max:] if excerpt_max > 0 else ""
            if text:
                box[tokens[0]] = text
            else:
                box.pop(tokens[0], None)
        elif kind == "money":
            box["cost"] = {
                "amount_micros": int(value["amount_micros"]),
                "currency": value["currency"],
                "is_estimate": True,
            }
        elif kind == "plan":
            if op == "remove" or not value["items"]:
                box.pop("plan", None)
            else:
                box["plan"] = {"items": value["items"]}
        elif kind in ("metric", "retry"):
            metrics = box.setdefault("metrics", {})
            if op == "remove":
                metrics.pop(tokens[1], None)
            else:
                metrics[tokens[1]] = value
        elif kind == "step":
            step_id = str(UUID(tokens[1]))
            if step_id in steps:
                raise fail("/ops/{i}", "patch_target_exists")
            parent = value.get("parent_step_id")
            if parent is not None and str(UUID(parent)) not in steps:
                raise fail("/ops/{i}/value/parent_step_id", "patch_target_missing")
            step = {
                "id": step_id,
                "ordinal": max((s["ordinal"] for s in steps.values()), default=0) + 1,
                "kind": value["kind"],
                "name": value["name"],
                "title": value.get("title", ""),
                "status": value.get("status", "running"),
                "detail": value.get("detail", {}),
                "started_at": value.get("started_at", now_iso()),
            }
            if parent is not None:
                step["parent_step_id"] = str(UUID(parent))
            if "finished_at" in value:
                step["finished_at"] = value["finished_at"]
            steps[step_id] = step
        elif kind == "step_member":
            step_id = str(UUID(tokens[1]))
            if step_id not in steps:
                raise fail("/ops/{i}", "patch_target_missing")
            steps[step_id][tokens[2]] = value
        else:  # followup: only a row the run has (M2-10 note 4), forward only
            code = _move_followup(box.setdefault("followups", {}), tokens[1], value)
            if code is not None:
                raise fail("/ops/{i}", code)
    return box
