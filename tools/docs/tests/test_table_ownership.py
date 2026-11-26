# SPDX-License-Identifier: Apache-2.0
"""Table ownership: a module's SQL names only its own tables; the register matches the schema.

docs/spec/03-data-model.md principle 12 and ADR 0050 say that only the module that owns a table
reads and writes it; other modules go through its port. The owner of every table is the key
`tables` of its module in `services/server/architecture/modules.yaml` (02 7.1). 02 7.3 lists the
check "Table ownership" as a pass over the query files; until this test nothing ran it, and the
boundary held only by review (audit finding architecture-3).

Two checks read the real files:

- Every name of a table of the register that a query file under
  `services/server/internal/<module>/adapters/postgres/queries/` names after `FROM`, `JOIN`,
  `INTO`, `UPDATE` or `TABLE` belongs to that module. Names that are no table of the register
  (functions, aliases, the system catalogue, the tables of River) are not judged; that every
  table of 03 is in the register is `make register-check`.
- Every table that a migration of `services/server/migrations/` creates is in the register, and
  every table of the register is created by a migration, except the tables of features that are
  not built (PLANNED, each with its reason).

A query that reads a foreign table today stands in KNOWN_FOREIGN with its reason (ratchet): the
list may only shrink, and an entry that no longer applies fails until it is removed.

The test lives with the tools tests because they run on every change, also on one that touches
only a document.
"""

from __future__ import annotations

import re
from pathlib import Path

import yaml

REPO = Path(__file__).resolve().parents[3]
SERVER = REPO / "services" / "server"
REGISTER = SERVER / "architecture" / "modules.yaml"
MIGRATIONS = SERVER / "migrations"

# Tables of the register without a migration: features that are not built yet.
PLANNED = {
    "dev_environments": "multi-user dev environment, V2 (03, 21-dev-isolation.md)",
    "dm_participants": "direct messages between people, 'later' in the feature list of 01",
}
# (module, query file, foreign table): queries that read a table of another module today.
KNOWN_FOREIGN = {
    (
        "runs",
        "team.sql",
        "messages",
    ): "TeamHistory reads the main thread of a team channel straight from the table of messaging "
    "(FEAT-TEAM-CONTEXT-ALL); a read port of messaging replaces it",
}

_COMMENT = re.compile(r"--[^\n]*")
# goose: what follows this line undoes the migration.
_DOWN = "-- +goose Down"
_NAME = re.compile(
    r"\b(?:FROM|JOIN|INTO|UPDATE|TABLE)\s+(?:ONLY\s+)?(?:IF\s+(?:NOT\s+)?EXISTS\s+)?"
    r"(?:public\.)?([a-z_][a-z0-9_]*)\b(?!\s*\()",
    re.IGNORECASE,
)
_CREATE = re.compile(
    r"\bCREATE\s+(?:UNLOGGED\s+)?TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:public\.)?([a-z_][a-z0-9_]*)\b(?!\s+PARTITION\s+OF)",
    re.IGNORECASE,
)
_DROP = re.compile(
    r"\bDROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?(?:public\.)?([a-z_][a-z0-9_]*)", re.IGNORECASE
)
_RENAME = re.compile(
    r"\bALTER\s+TABLE\s+(?:IF\s+EXISTS\s+)?(?:public\.)?([a-z_][a-z0-9_]*)\s+RENAME\s+TO\s+([a-z_][a-z0-9_]*)",
    re.IGNORECASE,
)


def owners() -> dict[str, str]:
    """Return the owning module of every table of the register."""
    data = yaml.safe_load(REGISTER.read_text(encoding="utf-8"))
    return {
        table: module
        for module, entry in data["modules"].items()
        for table in entry.get("tables") or []
    }


def tables_named(sql: str) -> set[str]:
    """Return the names a query file uses as tables, in lower case, comments left out.

    Example: `tables_named("SELECT * FROM messages m JOIN unnest($1) u ON true")` returns
    `{"messages"}`: a name followed by a parenthesis is a function.
    """
    return {name.lower() for name in _NAME.findall(_COMMENT.sub("", sql))}


def foreign_tables(module: str, sql: str, owner: dict[str, str]) -> set[str]:
    """Return the tables of the register of another module than `module` that `sql` names."""
    return {t for t in tables_named(sql) if t in owner and owner[t] != module}


def created_tables(sql_files: list[str]) -> set[str]:
    """Return the tables the migrations create, in their order, without dropped or renamed ones.

    Only the part before `-- +goose Down` of a migration counts.
    """
    tables: set[str] = set()
    for sql in sql_files:
        text = _COMMENT.sub("", sql.split(_DOWN, 1)[0])
        for old, new in _RENAME.findall(text):
            tables.discard(old.lower())
            tables.add(new.lower())
        tables |= {name.lower() for name in _CREATE.findall(text)}
        tables -= {name.lower() for name in _DROP.findall(text)}
    return tables


def query_files() -> list[Path]:
    """Return the query files of all modules, sorted."""
    return sorted(SERVER.glob("internal/*/adapters/postgres/queries/*.sql"))


def test_the_queries_of_a_module_name_only_its_own_tables() -> None:
    owner = owners()
    found = {
        (path.parts[-5], path.name, table)
        for path in query_files()
        for table in foreign_tables(path.parts[-5], path.read_text(encoding="utf-8"), owner)
    }
    assert query_files(), "no query files found"
    assert sorted(found - set(KNOWN_FOREIGN)) == [], "a query names a table of another module"
    assert sorted(set(KNOWN_FOREIGN) - found) == [], "an entry of KNOWN_FOREIGN no longer applies"


def test_the_register_and_the_migrations_name_the_same_tables() -> None:
    files = sorted(MIGRATIONS.glob("*.sql"))
    created = created_tables([f.read_text(encoding="utf-8") for f in files])
    registered = set(owners())
    assert sorted(created - registered) == [], "a migration creates a table the register lacks"
    missing = sorted(registered - created - set(PLANNED))
    assert missing == [], "a table of the register has no migration"
    assert sorted(set(PLANNED) & created) == [], "a table of PLANNED has a migration now"


def test_a_foreign_table_is_found_and_functions_are_not() -> None:
    owner = {"messages": "messaging", "runs": "runs"}
    sql = (
        "-- name: X :many\n-- FROM grants in a comment\n"
        "SELECT r.id FROM runs r JOIN messages m ON m.id = r.anchor_message_id\n"
        "CROSS JOIN LATERAL unnest($1::uuid[]) u FOR UPDATE OF r SKIP LOCKED;"
    )
    assert foreign_tables("runs", sql, owner) == {"messages"}
    assert foreign_tables("messaging", sql, owner) == {"runs"}


def test_created_tables_follow_drops_renames_and_partitions() -> None:
    migrations = [
        "CREATE TABLE a (id int); CREATE UNLOGGED TABLE b (id int);",
        "CREATE TABLE a_2026 PARTITION OF a FOR VALUES IN (1); ALTER TABLE b RENAME TO c;",
        "DROP TABLE IF EXISTS a;\n-- +goose Down\nDROP TABLE c;",
    ]
    assert created_tables(migrations) == {"c"}
