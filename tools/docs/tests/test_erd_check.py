# SPDX-License-Identifier: Apache-2.0
"""Tests for the ERD check against the data model."""

from __future__ import annotations

from pathlib import Path

import erd_check as ec

MODEL = """# 03 Datenmodell

## 4. Tabellen

### 4.1 Teil

#### `users`

Zweck: Menschen.

PK: `id`. FK: `created_by` -> `principals(id)` ON DELETE RESTRICT.

#### `principals`

PK: `id`. FK: keine.

#### `devices`

PK: `id`. FK: `user_id` -> `users(id)` ON DELETE CASCADE; `approved_by_device_id`,
`paired_device_id` -> `devices(id)` ON DELETE SET NULL. Checks: `user_id` ist gesetzt.

### 4.2 Nichts

## 5. Suchvektoren

#### `not_a_table`
"""

GOOD = """erDiagram
  users {
    uuid id PK
    uuid created_by
  }
  principals {
    uuid id PK
    text kind
  }
  devices {
    uuid id PK
    uuid user_id FK
  }
  users ||--o{ devices : "hat"
  devices |o--o{ devices : "bestätigt"
  devices |o--o{ devices : "gekoppelt"
"""


def test_model_tables_only_in_section_4() -> None:
    assert sorted(ec.model_tables(MODEL)) == ["devices", "principals", "users"]


def test_foreign_keys_with_column_lists_and_line_breaks() -> None:
    tables = ec.model_tables(MODEL)
    assert ec.foreign_keys("devices", tables["devices"]) == [
        ("devices", "user_id", "users"),
        ("devices", "approved_by_device_id", "devices"),
        ("devices", "paired_device_id", "devices"),
    ]


def test_protocol_columns_are_exempt() -> None:
    assert ec.is_protocol_column("created_by", "principals")
    assert not ec.is_protocol_column("approved_by_device_id", "devices")
    assert not ec.is_protocol_column("owner_id", "principals")


def test_matching_erd_has_no_problems() -> None:
    assert ec.check(MODEL, {"erd-1.mmd": GOOD}) == []


def test_missing_table_and_relationship_are_reported() -> None:
    erd = GOOD.replace('  devices |o--o{ devices : "gekoppelt"\n', "").replace(
        "  principals {\n    uuid id PK\n    text kind\n  }\n", ""
    )
    assert ec.check(MODEL, {"erd-1.mmd": erd}) == [
        "docs/spec/03-datenmodell.md: table principals has no full entity in any ERD",
        "ERD: 1 of 2 relationship(s) missing for devices.approved_by_device_id -> devices; "
        "devices.paired_device_id -> devices",
    ]


def test_stub_is_not_full_but_a_second_full_entity_is_reported() -> None:
    stub = "erDiagram\n  users {\n    uuid id PK\n  }\n"
    assert ec.check(MODEL, {"erd-1.mmd": GOOD, "erd-2.mmd": stub}) == []
    twice = "erDiagram\n  users {\n    uuid id PK\n    text name\n  }\n"
    assert ec.check(MODEL, {"erd-1.mmd": GOOD, "erd-2.mmd": twice}) == [
        "docs/spec/03-datenmodell.md: table users has a full entity in erd-1.mmd, erd-2.mmd"
    ]


def test_all_pk_table_with_foreign_keys_counts_as_full() -> None:
    assert ec._is_full(["uuid a PK, FK", "uuid b PK, FK"])
    assert not ec._is_full(["uuid a PK"])


def test_unknown_entity_is_reported() -> None:
    erd = GOOD + "  ghosts {\n    uuid id PK\n  }\n"
    assert ec.check(MODEL, {"erd-1.mmd": erd}) == [
        "erd-1.mmd: entity ghosts is no table of docs/spec/03-datenmodell.md"
    ]


def test_repository_erds_match_the_data_model() -> None:
    assert ec.problems(Path(__file__).resolve().parents[3]) == []
