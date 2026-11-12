# SPDX-License-Identifier: Apache-2.0
"""The texts about the state of the backups follow the API and the Compose stack.

The runs of `bothy-backupd` are rows of the table `backup_runs`. A view of the admin console can
show them only through an operation of the API, and docs/api/openapi.yaml has none
(docs/decisions.d/AUD-docs-spec-01.md). While that is so:

- no current document names the console as a place of the backup state or of the warning
  without an off-site target (a sentence that names it there says "no" or "not");
- docs/spec/16-operations.md and docs/spec/03-data-model.md say that nothing of the API reads the
  table yet.

The change that adds such an operation fails here until the texts say what it shows. Until then
an operator reads the table with `psql` in the database container. The call has one home,
docs/operations/backup-restore.md section 1.1; it must be the one the image test of the database
image runs (same user, same database) and may name only columns the migration of `backup_runs`
creates. The runbooks that need the manifest of a run link that section.

The test lives with the tools tests because they run on every change, also on one that touches
only a document.
"""

from __future__ import annotations

import re
import subprocess
from pathlib import Path

import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]
OPENAPI = REPO / "docs" / "api" / "openapi.yaml"
COMPOSE = REPO / "infra" / "compose" / "compose.yaml"
MIGRATION = REPO / "services" / "server" / "migrations" / "00050_status_backup_runs.sql"
IMAGE_TEST = REPO / "services" / "server" / "test" / "postgresimage" / "postgresimage_test.go"
# Historical records: their text stays as written (docs/briefs, decision logs, change logs).
# The plan is no description of the product either: its rows say what a ticket builds.
HISTORY = re.compile(
    r"^docs/briefs/|^docs/decisions\.d/|^docs/DECISIONS[^/]*\.md$|^docs/adr/|^docs/PLAN\.md$"
    r"|(?:^|/)CHANGELOG\.md$|(?:^|/)changelog\.d/"
)
# The sentence of each text that says the API does not read the table, whitespace collapsed.
NOT_BUILT = {
    "docs/spec/16-operations.md": (
        "No operation of the API and no view of the admin console reads `backup_runs` yet"
    ),
    "docs/spec/03-data-model.md": "No operation of the API reads them yet",
}
# The guide that tells an operator how to read the table, and its section with the calls.
GUIDE = "docs/operations/backup-restore.md"
SECTION = "### 1.1 State of the runs"
ANCHOR = "backup-restore.md#11-state-of-the-runs"
# The runbooks whose steps need the manifest of a run.
MANIFEST_STEPS = ("docs/operations/disaster-recovery.md", "docs/operations/cloud-migration.md")

_OPERATION = re.compile(r"^ {4,}operationId:\s*(?P<id>\w+)\s*$")
_PATH = re.compile(r"^ {2}(?P<path>/[^\s:]*):\s*$")
# What marks a block as one about the state of the backups.
_BACKUP = re.compile(r"backup_runs|bothy-backupd|off-site target|restore test", re.IGNORECASE)
_CONSOLE = re.compile(r"\bconsole\b", re.IGNORECASE)
_NEGATION = re.compile(r"\b(?:no|not|neither|nor|never|without)\b", re.IGNORECASE)
_BLOCK_START = re.compile(r"\s*(?:\||#|\d+\.\s|[-*]\s)")
_SENTENCE_END = re.compile(r"(?<=[.;?!])\s+|\s\|\s")
_PSQL = re.compile(
    r"docker compose exec (?:-T )?(?P<service>[\w-]+) psql -U (?P<user>\w+) -d (?P<database>\w+)"
    r" -Atc\s+\"(?P<sql>[^\"]+)\""
)
_COLUMN = re.compile(r"^\s{4}(?P<name>[a-z_]+)\s+(?:uuid|text|timestamptz|bigint|jsonb)\b", re.M)
_WORD = re.compile(r"[a-z_]+")
# SQL words of the documented statements that are no column names.
_SQL_WORDS = frozenset(
    {"select", "from", "where", "order", "by", "desc", "asc", "limit", "and", "or", "in", "is"}
    | {"not", "null", "as", "backup_runs"}
)


def backup_operations(contract: str) -> list[str]:
    """Return the operations of the API contract whose ID or path names backups.

    Example: a path `/backup-runs` with `operationId: listBackupRuns` gives
    `["listBackupRuns"]`; a field `backup_eligible` of a schema gives nothing.
    """
    found, path = [], ""
    for line in contract.splitlines():
        if m := _PATH.match(line):
            path = m.group("path")
        elif (m := _OPERATION.match(line)) and "backup" in f"{path} {m.group('id')}".lower():
            found.append(m.group("id"))
    return found


def blocks(text: str) -> list[tuple[int, str]]:
    """Return the paragraphs, list items, headings and table rows of a Markdown text.

    Each block comes with the number of its first line and its text on one line.
    """
    out: list[tuple[int, str]] = []
    start, current = 1, []
    for number, line in enumerate(text.splitlines(), 1):
        if not line.strip() or _BLOCK_START.match(line):
            if current:
                out.append((start, " ".join(current)))
            start, current = number, [line.strip()] if line.strip() else []
        else:
            if not current:
                start = number
            current.append(line.strip())
    if current:
        out.append((start, " ".join(current)))
    return out


def console_claims(text: str) -> list[tuple[int, str]]:
    """Return the sentences that name the console as a place of the backup state.

    A sentence counts when its block is about the backups, it names the console and it does not
    negate. Example: the block "Every run writes a row into `backup_runs`. The admin console
    reads the table." gives its second sentence; "No view of the admin console reads
    `backup_runs` yet." gives nothing.
    """
    found = []
    for line, block in blocks(text):
        if not (_BACKUP.search(block) and _CONSOLE.search(block)):
            continue
        for sentence in _SENTENCE_END.split(block):
            if _CONSOLE.search(sentence) and not _NEGATION.search(sentence):
                found.append((line, sentence.strip()))
    return found


def says(text: str, sentence: str) -> bool:
    """Return whether `text` holds `sentence`, however its lines are wrapped."""
    return sentence in " ".join(text.split())


def psql_calls(text: str) -> list[dict[str, str]]:
    """Return the documented `psql` calls of a text, each with service, user, database, sql."""
    return [m.groupdict() for m in _PSQL.finditer(" ".join(text.split()))]


def table_columns(migration: str) -> set[str]:
    """Return the column names the migration of `backup_runs` creates."""
    return {m.group("name") for m in _COLUMN.finditer(migration)}


def documents() -> list[str]:
    """Return the tracked current Markdown files as repository paths."""
    out = subprocess.run(
        ["git", "ls-files", "-z", "--", "*.md"],
        cwd=REPO,
        capture_output=True,
        check=True,
        text=True,
    ).stdout
    return [p for p in out.split("\0") if p and not HISTORY.search(p)]


def test_backup_operations_are_read_from_the_contract() -> None:
    contract = (
        "paths:\n"
        "  /me/passkeys:\n    get:\n      operationId: listPasskeys\n"
        "  /backup-runs:\n    get:\n      operationId: listRuns\n"
        "  /status:\n    get:\n      operationId: getBackupState\n"
        "components:\n  schemas:\n    Passkey:\n      properties:\n        backup_eligible:\n"
    )
    assert backup_operations(contract) == ["listRuns", "getBackupState"]
    assert backup_operations("paths:\n  /me:\n    get:\n      operationId: getMe\n") == []


def test_blocks_are_paragraphs_items_and_rows() -> None:
    text = "One\ntwo.\n\n1. Item\n   more.\n2. Next.\n\n| a | b |\n| c | d |\n"
    assert blocks(text) == [
        (1, "One two."),
        (4, "1. Item more."),
        (6, "2. Next."),
        (8, "| a | b |"),
        (9, "| c | d |"),
    ]


@pytest.mark.parametrize(
    "text",
    [
        "4. Every run writes a row into `backup_runs`.\n   The admin console reads the table.\n",
        "Their state is in the admin console (`backup_runs`) and in the alerts.\n",
        "| `backup.offsite.target` | off-site target; empty: local only (warning in console) |",
        "| First backup | `bothy-backupd backup pg_full` | run `succeeded` in the admin console |",
        "### bothybackupstale\n`backup_runs` in the admin console; off-site target reachable?\n",
    ],
)
def test_a_console_claim_is_found(text: str) -> None:
    assert console_claims(text), text


@pytest.mark.parametrize(
    "text",
    [
        "No operation of the API and no view of the admin console reads `backup_runs` yet.\n",
        "The state of `backup_runs` is not shown in the admin console.\n",
        "The web console (M6) follows the desktop. Real data requires backup and restore.\n",
        "Every run writes a row into `backup_runs` and reports metrics.\n",
    ],
)
def test_a_negation_or_another_topic_is_no_claim(text: str) -> None:
    assert console_claims(text) == [], text


def test_a_wrapped_sentence_is_found() -> None:
    assert says("No operation of the API\n  reads them yet.", "the API reads them yet")
    assert not says("An operation reads them.", "No operation")


def test_psql_calls_and_columns_are_read() -> None:
    text = (
        "Run `docker compose exec -T postgres psql -U postgres -d bothy -Atc\n"
        '"SELECT state, error FROM backup_runs ORDER BY started_at DESC LIMIT 1"`.\n'
    )
    assert psql_calls(text) == [
        {
            "service": "postgres",
            "user": "postgres",
            "database": "bothy",
            "sql": "SELECT state, error FROM backup_runs ORDER BY started_at DESC LIMIT 1",
        }
    ]
    migration = "CREATE TABLE backup_runs (\n    id uuid NOT NULL,\n    kind text NOT NULL\n);\n"
    assert table_columns(migration) == {"id", "kind"}


def test_no_document_names_the_console_for_the_backup_state_without_an_operation() -> None:
    if backup_operations(OPENAPI.read_text(encoding="utf-8")):
        pytest.skip("the API reads the backup runs; the texts may name its view")
    found = [
        f"{path}:{line}: {sentence}"
        for path in documents()
        for line, sentence in console_claims((REPO / path).read_text(encoding="utf-8"))
    ]
    assert found == [], "\n".join(found)


@pytest.mark.parametrize("name", sorted(NOT_BUILT))
def test_the_chapters_follow_the_api(name: str) -> None:
    operations = backup_operations(OPENAPI.read_text(encoding="utf-8"))
    named = says((REPO / name).read_text(encoding="utf-8"), NOT_BUILT[name])
    assert named != bool(operations), (
        f"{name} says that the API does not read the backup runs: {named}; "
        f"operations of the API that name backups: {operations or 'none'}"
    )


def test_the_guide_reads_the_table_as_the_image_test_does() -> None:
    text = (REPO / GUIDE).read_text(encoding="utf-8")
    assert SECTION in text, f"{GUIDE} has no section {SECTION!r}"
    calls = psql_calls(text)
    assert len(calls) >= 2, f"{GUIDE} names no psql call for the runs and for the manifest"
    services = yaml.safe_load(COMPOSE.read_text(encoding="utf-8"))["services"]
    columns = table_columns(MIGRATION.read_text(encoding="utf-8"))
    assert {"kind", "state", "started_at", "manifest", "checks", "error"} <= columns
    image_test = IMAGE_TEST.read_text(encoding="utf-8")
    for call in calls:
        environment = services[call["service"]]["environment"]
        assert environment["POSTGRES_USER"] == call["user"], call
        assert environment["POSTGRES_DB"] == call["database"], call
        # The image test runs the same call in the database container of the real image.
        proven = f'"psql", "-v", "ON_ERROR_STOP=1", "-U", "{call["user"]}", "-d", '
        assert proven + f'"{call["database"]}", "-Atc"' in image_test, call
        assert "backup_runs" in call["sql"], call
        words = set(_WORD.findall(re.sub(r"'[^']*'", "", call["sql"]).lower()))
        assert words - _SQL_WORDS <= columns, sorted(words - _SQL_WORDS - columns)


@pytest.mark.parametrize("name", MANIFEST_STEPS)
def test_a_step_that_needs_the_manifest_links_the_call(name: str) -> None:
    text = (REPO / name).read_text(encoding="utf-8")
    assert "backup_runs.manifest" in text, f"{name} no longer needs the manifest of a run"
    assert ANCHOR in " ".join(text.split()), f"{name} does not link {ANCHOR}"
