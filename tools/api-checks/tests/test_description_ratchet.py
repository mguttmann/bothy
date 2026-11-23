# SPDX-License-Identifier: Apache-2.0
"""The number of undescribed parts of the API contract never grows (issue #631).

Descriptions in docs/api/openapi.yaml become the comments of the generated clients (06 R-API-15).
Many parts have none: an operation with only a `summary`, a schema without a text, a field
without one. Filling them is work for the runs in each area; this ratchet keeps the gaps from
growing meanwhile. Each count must equal its ceiling: a new part without a description raises the
count and fails, and a filled gap lowers it, so the ceiling is lowered in the same change.

Counted are the operations under `paths`, the schemas under `components.schemas`, and the
properties of those schemas (also inside `allOf` parts) that are no bare `$ref`; a `$ref` takes the
text of its target. The webhook entries are not counted.
"""

from __future__ import annotations

from pathlib import Path
from typing import Any

import yaml

REPO = Path(__file__).resolve().parents[3]
OPENAPI = REPO / "docs" / "api" / "openapi.yaml"
METHODS = ("get", "put", "post", "patch", "delete", "head", "options")

# Ceilings measured on 2026-11-23; lower them when a change fills gaps, never raise them.
CEILINGS = {
    "operations": 159,
    "schemas": 388,
    "properties": 980,
}


def _described(node: dict[str, Any]) -> bool:
    return bool(str(node.get("description", "")).strip())


def count_undescribed(doc: dict[str, Any]) -> dict[str, int]:
    """Count the operations, schemas and inline properties without a description."""
    operations = sum(
        1
        for item in doc.get("paths", {}).values()
        for method, op in item.items()
        if method in METHODS and not _described(op)
    )
    schemas = doc.get("components", {}).get("schemas", {})
    properties = 0
    for schema in schemas.values():
        for part in [schema, *schema.get("allOf", [])]:
            for prop in (part.get("properties") or {}).values():
                if "$ref" not in prop and not _described(prop):
                    properties += 1
    return {
        "operations": operations,
        "schemas": sum(1 for s in schemas.values() if not _described(s)),
        "properties": properties,
    }


def test_counter_sees_a_missing_and_a_present_description() -> None:
    doc = {
        "paths": {"/a": {"get": {"summary": "x"}, "post": {"description": "y"}, "parameters": []}},
        "components": {
            "schemas": {
                "A": {
                    "properties": {
                        "f": {"type": "string"},
                        "g": {"type": "string", "description": "g"},
                        "r": {"$ref": "#/components/schemas/B"},
                    }
                },
                "B": {"description": "b", "allOf": [{"properties": {"h": {"type": "integer"}}}]},
            }
        },
    }
    assert count_undescribed(doc) == {"operations": 1, "schemas": 1, "properties": 2}


def test_undescribed_parts_of_the_contract_do_not_grow() -> None:
    counts = count_undescribed(yaml.safe_load(OPENAPI.read_text(encoding="utf-8")))
    grown = {k: (counts[k], CEILINGS[k]) for k in CEILINGS if counts[k] > CEILINGS[k]}
    assert not grown, (
        f"parts of docs/api/openapi.yaml without a description grew (count, ceiling): {grown}; "
        "give the new parts a description"
    )
    shrunk = {k: counts[k] for k in CEILINGS if counts[k] < CEILINGS[k]}
    assert not shrunk, f"gaps were filled; lower CEILINGS in this test to {shrunk}"
