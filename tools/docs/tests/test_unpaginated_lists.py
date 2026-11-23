# SPDX-License-Identifier: Apache-2.0
"""Every list without a cursor is named in the table of 06 section 6 item 6 (issue #708).

06 section 6 lets only small sets go without pagination and names, per operation, what bounds
the set. The table must name exactly the `GET` operations of docs/api/openapi.yaml whose `200`
answer has `items` but no `next_cursor`: a new unpaginated list must be named with its bound, and
a list that gets a cursor leaves the table.
"""

from __future__ import annotations

import re
from pathlib import Path
from typing import Any

import yaml

REPO = Path(__file__).resolve().parents[3]
OPENAPI = REPO / "docs" / "api" / "openapi.yaml"
CHAPTER = REPO / "docs" / "spec" / "06-api.md"
SECTION = re.compile(r"^## 6\. Pagination\n(.*?)^## 7\. ", re.S | re.M)
ROW = re.compile(r"^\s*\| (`[^|]+`) \| [^|]+ \|$", re.M)


def _properties(schema: dict[str, Any], schemas: dict[str, Any]) -> set[str]:
    while "$ref" in schema:
        schema = schemas[schema["$ref"].rsplit("/", 1)[-1]]
    names = set(schema.get("properties") or {})
    for part in schema.get("allOf", []):
        names |= _properties(part, schemas)
    return names


def unpaginated_lists(doc: dict[str, Any]) -> set[str]:
    """The `GET` operations whose `200` JSON answer has `items` and no `next_cursor`."""
    schemas = doc["components"]["schemas"]
    found = set()
    for item in doc["paths"].values():
        op = item.get("get")
        if not op:
            continue
        content = op.get("responses", {}).get("200", {}).get("content", {})
        schema = content.get("application/json", {}).get("schema")
        if schema is None:
            continue
        names = _properties(schema, schemas)
        if "items" in names and "next_cursor" not in names:
            found.add(op["operationId"])
    return found


def table_operations(text: str) -> set[str]:
    """The operation names in the first column of the table of section 6 item 6."""
    section = SECTION.search(text)
    assert section, "06-api.md has no section 6 Pagination"
    names = set()
    for cell in ROW.findall(section.group(1)):
        names |= set(re.findall(r"`([a-zA-Z]+)`", cell))
    return names


def test_finder_sees_a_list_without_a_cursor() -> None:
    doc = {
        "paths": {
            "/a": {
                "get": {
                    "operationId": "listA",
                    "responses": {
                        "200": {
                            "content": {
                                "application/json": {
                                    "schema": {"$ref": "#/components/schemas/AList"}
                                }
                            }
                        }
                    },
                }
            },
            "/b": {
                "get": {
                    "operationId": "listB",
                    "responses": {
                        "200": {
                            "content": {
                                "application/json": {
                                    "schema": {"$ref": "#/components/schemas/BPage"}
                                }
                            }
                        }
                    },
                }
            },
        },
        "components": {
            "schemas": {
                "AList": {"properties": {"items": {"type": "array"}}},
                "BPage": {
                    "allOf": [
                        {"properties": {"items": {"type": "array"}}},
                        {"properties": {"next_cursor": {"type": "string"}}},
                    ]
                },
            }
        },
    }
    assert unpaginated_lists(doc) == {"listA"}


def test_table_names_every_unpaginated_list() -> None:
    built = unpaginated_lists(yaml.safe_load(OPENAPI.read_text(encoding="utf-8")))
    named = table_operations(CHAPTER.read_text(encoding="utf-8"))
    assert built - named == set(), "lists without a cursor that 06 section 6 item 6 does not name"
    assert named - built == set(), (
        "operations in 06 section 6 item 6 that are no list without a cursor"
    )
