# SPDX-License-Identifier: Apache-2.0
"""The enabled operations of the server (enable list ``services/server/api/operations.yaml``)."""

from __future__ import annotations

from collections.abc import Mapping
from pathlib import Path
from typing import Any

import yaml

HTTP_METHODS = frozenset({"get", "put", "post", "delete", "options", "head", "patch", "trace"})


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
