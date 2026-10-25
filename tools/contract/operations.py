# SPDX-License-Identifier: Apache-2.0
"""The enabled operations of the server (enable list ``services/server/api/operations.yaml``)."""

from __future__ import annotations

from collections.abc import Callable, Mapping
from pathlib import Path
from typing import Any

import yaml

HTTP_METHODS = frozenset({"get", "put", "post", "delete", "options", "head", "patch", "trace"})

# Actions that end what the credential broker needs of a principal: a disabled person gets no
# session, and the owner's recovery code needs an active owner. A generated call may perform them
# on any person except the principals of the passes (see shield_principals).
PRINCIPAL_ENDING_ACTIONS = frozenset({"user.disable", "owner.remove"})


class UnknownOperationError(ValueError):
    """An enabled operation ID that the OpenAPI document does not define."""


def spec_operation_ids(spec: Mapping[str, Any]) -> set[str]:
    """Return every ``operationId`` under ``paths`` of the OpenAPI document ``spec``."""
    ids: set[str] = set()
    for item in (spec.get("paths") or {}).values():
        for method, operation in item.items():
            if (
                method in HTTP_METHODS
                and isinstance(operation, dict)
                and "operationId" in operation
            ):
                ids.add(str(operation["operationId"]))
    return ids


def enabled_operations(enable_list: Path, spec: Mapping[str, Any]) -> list[str]:
    """Return the IDs under ``enabled`` of ``enable_list``, sorted.

    Raises:
        UnknownOperationError: an enabled ID is not an operation of ``spec`` (listed in the
            message).
        ValueError: the file has no list ``enabled``.
    """
    data = yaml.safe_load(enable_list.read_text(encoding="utf-8")) or {}
    enabled = data.get("enabled")
    if not isinstance(enabled, list) or not all(isinstance(i, str) for i in enabled):
        raise ValueError(f"{enable_list}: no list 'enabled'")
    missing = sorted(set(enabled) - spec_operation_ids(spec))
    if missing:
        raise UnknownOperationError(
            "enabled operations missing from openapi.yaml: " + ", ".join(missing)
        )
    return sorted(enabled)


def load_spec(path: Path) -> dict[str, Any]:
    """Load the OpenAPI document at ``path``."""
    data = yaml.safe_load(path.read_text(encoding="utf-8"))
    if not isinstance(data, dict):
        raise ValueError(f"{path}: not an OpenAPI document")
    return data


def principal_ending_operations(spec: Mapping[str, Any]) -> dict[str, str]:
    """Return the operations of ``spec`` that end a person, with the parameter naming the person.

    These are the operations whose ``x-authz-action`` is in :data:`PRINCIPAL_ENDING_ACTIONS` and
    whose path has a ``user_id``.
    """
    out: dict[str, str] = {}
    for path, item in (spec.get("paths") or {}).items():
        for method, operation in item.items():
            if (
                method in HTTP_METHODS
                and isinstance(operation, dict)
                and operation.get("x-authz-action") in PRINCIPAL_ENDING_ACTIONS
                and "{user_id}" in str(path)
            ):
                out[str(operation["operationId"])] = "user_id"
    return out


def shield_principals(
    op_id: str,
    path_parameters: Mapping[str, Any] | None,
    ending: Mapping[str, str],
    protected: set[str],
    fresh: Callable[[], str],
) -> dict[str, Any] | None:
    """Return path parameters that point an ending operation away from a protected principal.

    A generated call of an operation in ``ending`` whose person is in ``protected`` (the
    principals the broker issues sessions for) would disable or demote the pass itself, and every
    later session of the run would fail. The person is replaced by ``fresh()``, an unknown ID that
    answers the documented 404. Returns None when nothing is replaced.
    """
    name = ending.get(op_id)
    if name is None or not path_parameters or str(path_parameters.get(name)) not in protected:
        return None
    params = dict(path_parameters)
    params[name] = fresh()
    return params
