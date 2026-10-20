# SPDX-License-Identifier: Apache-2.0
"""Tests for sql_visibility (AZ-P12): reading sqlc queries on content tables carry the
visibility predicate or a reviewed exception, on throwaway repositories built from the
fixtures, and the real tree is clean."""

from __future__ import annotations

import shutil
from pathlib import Path
from typing import Any

import pytest
import sql_visibility as sv
import yaml

FIXTURES = Path(__file__).parent / "fixtures"
REPO = Path(__file__).resolve().parents[3]
TABLES = ["spaces", "channels", "threads", "messages", "mentions"]


def make_repo(
    root: Path,
    queries: dict[str, list[str]],
    exceptions: list[dict[str, Any]] | None = None,
    tables: list[str] | None = None,
) -> Path:
    """A repository with the fixture query files per module, the fixture data model and a
    configuration with the given content tables and exceptions."""
    for module, files in queries.items():
        qdir = root / "services/server/internal" / module / "adapters/postgres/queries"
        qdir.mkdir(parents=True, exist_ok=True)
        for name in files:
            shutil.copy(FIXTURES / name, qdir / name)
    model = root / "docs/spec/03-data-model.md"
    model.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy(FIXTURES / "data-model.md", model)
    config = {
        "data_model": "docs/spec/03-data-model.md",
        "content_tables": TABLES if tables is None else tables,
        "exceptions": exceptions or [],
    }
    cfg = root / sv.CONFIG
    cfg.parent.mkdir(parents=True, exist_ok=True)
    cfg.write_text(yaml.safe_dump(config, sort_keys=False), encoding="utf-8")
    return root


def rules(findings: list[sv.Finding]) -> list[tuple[str, str]]:
    return [(f.query, f.rule) for f in findings]


def test_az_p12_read_without_predicate_fails(tmp_path: Path) -> None:
    repo = make_repo(tmp_path, {"messaging": ["read_without_predicate.sql"]})
    findings = sv.check(repo)
    assert rules(findings) == [("messaging.ListMessagesOfThread", "missing-predicate")]
    assert findings[0].line == 4
    assert findings[0].file.endswith(
        "messaging/adapters/postgres/queries/read_without_predicate.sql"
    )
    assert str(findings[0]).startswith(findings[0].file + ":4: messaging.ListMessagesOfThread: ")


@pytest.mark.parametrize(
    "name",
    ["ByPositional", "ByAtName", "BySqlcArg", "BySqlcNarg", "ByPositionalNoCast", "SpacesByKey"],
)
def test_az_p12_predicate_with_param_passes(tmp_path: Path, name: str) -> None:
    repo = make_repo(tmp_path, {"chat": ["predicate_params.sql"]})
    queries = {q.ident: q for q in sv.load_queries(repo)}
    assert f"chat.{name}" in queries
    assert sv.check(repo) == []


def test_az_p12_any_with_subquery_fails(tmp_path: Path) -> None:
    repo = make_repo(tmp_path, {"messaging": ["any_subquery.sql"]})
    assert rules(sv.check(repo)) == [("messaging.MessagesOfGrantedChannels", "missing-predicate")]


def test_az_p12_join_with_alias_predicate_passes(tmp_path: Path) -> None:
    repo = make_repo(tmp_path, {"chat": ["join_alias.sql"]})
    q = sv.load_queries(repo)[0]
    assert sv.content_tables_of(q.sql, set(TABLES)) == {"threads", "channels"}
    assert sv.check(repo) == []


def test_az_p12_write_statement_ignored(tmp_path: Path) -> None:
    repo = make_repo(tmp_path, {"messaging": ["write_statement.sql"]})
    assert [sv.is_read(q.sql) for q in sv.load_queries(repo)] == [False, False, False]
    assert sv.check(repo) == []


def test_az_p12_select_for_update_is_a_read(tmp_path: Path) -> None:
    repo = make_repo(tmp_path, {"messaging": ["exception.sql"]})
    assert rules(sv.check(repo)) == [
        ("messaging.GetMessage", "missing-predicate"),
        ("messaging.LockMessage", "missing-predicate"),
    ]


def test_az_p12_exception_passes(tmp_path: Path) -> None:
    exc = [
        {"query": "messaging.GetMessage", "category": "after_authorize", "reason": "single row"},
        {"query": "messaging.LockMessage", "category": "after_authorize", "reason": "lock"},
    ]
    repo = make_repo(tmp_path, {"messaging": ["exception.sql"]}, exc)
    assert sv.check(repo) == []


def test_az_p12_thread_scope_exception_passes(tmp_path: Path) -> None:
    # thread_scope (D-M2-05-9): the active-run lists read rows of tables without channel_id,
    # resolve their threads through the chat port and keep only threads in Scope.
    exc = [
        {"query": "messaging.GetMessage", "category": "thread_scope", "reason": "active runs"},
        {"query": "messaging.LockMessage", "category": "after_authorize", "reason": "lock"},
    ]
    repo = make_repo(tmp_path, {"messaging": ["exception.sql"]}, exc)
    assert "thread_scope" in sv.CATEGORIES
    assert sv.check(repo) == []


def test_az_p12_unknown_category_near_thread_scope_fails(tmp_path: Path) -> None:
    exc = [
        {"query": "messaging.GetMessage", "category": "thread-scope", "reason": "x"},
        {"query": "messaging.LockMessage", "category": "thread_scope", "reason": "y"},
    ]
    repo = make_repo(tmp_path, {"messaging": ["exception.sql"]}, exc)
    assert rules(sv.check(repo)) == [("messaging.GetMessage", "unknown-category")]


def test_az_p12_exception_without_reason_fails(tmp_path: Path) -> None:
    exc = [
        {"query": "messaging.GetMessage", "category": "after_authorize", "reason": " "},
        {"query": "messaging.LockMessage", "category": "after_authorize", "reason": "lock"},
    ]
    repo = make_repo(tmp_path, {"messaging": ["exception.sql"]}, exc)
    assert rules(sv.check(repo)) == [("messaging.GetMessage", "exception-reason")]


def test_az_p12_stale_exception_fails(tmp_path: Path) -> None:
    exc = [{"query": "messaging.Gone", "category": "system_job", "reason": "retention"}]
    repo = make_repo(tmp_path, {"chat": ["join_alias.sql"]}, exc)
    findings = sv.check(repo)
    assert rules(findings) == [("messaging.Gone", "stale-exception")]
    assert findings[0].file == sv.CONFIG


def test_az_p12_unknown_category_fails(tmp_path: Path) -> None:
    exc = [
        {"query": "messaging.GetMessage", "category": "trusted", "reason": "x"},
        {"query": "messaging.LockMessage", "category": "log_tail", "reason": "y"},
    ]
    repo = make_repo(tmp_path, {"messaging": ["exception.sql"]}, exc)
    assert rules(sv.check(repo)) == [("messaging.GetMessage", "unknown-category")]


def test_az_p12_duplicate_exception_fails(tmp_path: Path) -> None:
    exc = [
        {"query": "messaging.GetMessage", "category": "after_authorize", "reason": "x"},
        {"query": "messaging.GetMessage", "category": "after_authorize", "reason": "x"},
        {"query": "messaging.LockMessage", "category": "after_authorize", "reason": "y"},
    ]
    repo = make_repo(tmp_path, {"messaging": ["exception.sql"]}, exc)
    assert rules(sv.check(repo)) == [("messaging.GetMessage", "duplicate-exception")]


def test_az_p12_unknown_content_table_fails(tmp_path: Path) -> None:
    repo = make_repo(tmp_path, {"chat": ["join_alias.sql"]}, tables=[*TABLES, "posts"])
    findings = sv.check(repo)
    assert rules(findings) == [("posts", "unknown-content-table")]


def test_az_p12_usage_errors(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    assert sv.main(["--repo", str(tmp_path / "missing")]) == 2
    assert sv.main(["--bogus"]) == 2
    bad = make_repo(tmp_path / "bad", {})
    (bad / sv.CONFIG).write_text("content_tables: 3\n", encoding="utf-8")
    assert sv.main(["--repo", str(bad)]) == 2
    assert "sql_visibility:" in capsys.readouterr().err


def test_az_p12_main_reports_findings(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    repo = make_repo(tmp_path, {"messaging": ["read_without_predicate.sql"]})
    assert sv.main(["--repo", str(repo)]) == 1
    out = capsys.readouterr().out
    assert "read_without_predicate.sql:4: messaging.ListMessagesOfThread: missing-predicate" in out
    clean = make_repo(tmp_path / "clean", {"chat": ["join_alias.sql"]})
    assert sv.main(["--repo", str(clean)]) == 0


def test_az_p12_comments_and_strings_do_not_count(tmp_path: Path) -> None:
    sql = (
        "-- name: Tricky :many\n"
        "-- channel_id = ANY($1) in a comment does not count\n"
        "SELECT id, 'channel_id = ANY($1)' AS s FROM messages /* space_id = ANY($2) */\n"
        "WHERE thread_id = $3;\n"
    )
    qdir = tmp_path / "services/server/internal/messaging/adapters/postgres/queries"
    make_repo(tmp_path, {})
    qdir.mkdir(parents=True)
    (qdir / "tricky.sql").write_text(sql, encoding="utf-8")
    assert rules(sv.check(tmp_path)) == [("messaging.Tricky", "missing-predicate")]


def test_az_p12_repository_clean() -> None:
    findings = sv.check(REPO)
    assert findings == [], "\n".join(str(f) for f in findings)
    config = sv.load_config(REPO)
    assert config.exceptions, "the reviewed exception list is not empty in M1"
    assert all(e.category in sv.CATEGORIES and e.reason.strip() for e in config.exceptions)


def test_az_p12_sql_text_forms() -> None:
    sql = (
        "SELECT x /* channel_id = ANY($1)\n */, $$ space_id = ANY($2) $$, $t$ a $t$, 'it''s'\n"
        "FROM ONLY public.messages AS m, unnest($3::uuid[]) u JOIN threads t USING (id)\n"
        "WHERE m.thread_id = $4"
    )
    stripped = sv.strip_sql(sql)
    assert "ANY($1)" not in stripped
    assert "ANY($2)" not in stripped
    assert stripped.count("\n") == sql.count("\n")
    assert sv.content_tables_of(sql, {"messages", "threads"}) == {"messages", "threads"}
    assert not sv.has_predicate(sql)
    assert sv.has_predicate("SELECT 1 FROM channels c WHERE c.id = ANY(@v::uuid[])")
    assert not sv.has_predicate("SELECT 1 FROM messages m WHERE m.id = ANY(@v::uuid[])")
    assert not sv.has_predicate("SELECT 1 FROM messages WHERE id = ANY(@v::uuid[])")
    assert sv.strip_sql("SELECT '/* x") == "SELECT ''"
    assert sv.strip_sql("SELECT 1 /* open") == "SELECT 1  "
    assert sv.strip_sql("SELECT $$ open") == "SELECT ''"
    assert not sv.is_read("")
    assert sv.is_read("SELECT * FROM t FOR NO KEY UPDATE")
    assert not sv.is_read("WITH x AS (SELECT 1) DELETE FROM t")


@pytest.mark.parametrize(
    "text",
    [
        "- 1\n",
        "data_model: ''\ncontent_tables: []\n",
        "data_model: m.md\ncontent_tables: [1]\n",
        "data_model: m.md\ncontent_tables: []\nexceptions: 3\n",
        "data_model: m.md\ncontent_tables: []\nexceptions: [{query: a}]\n",
        "data_model: m.md\ncontent_tables: [\n",
    ],
)
def test_az_p12_malformed_config(tmp_path: Path, text: str) -> None:
    cfg = tmp_path / sv.CONFIG
    cfg.parent.mkdir(parents=True)
    cfg.write_text(text, encoding="utf-8")
    with pytest.raises(sv.UsageError):
        sv.load_config(tmp_path)


def test_az_p12_missing_data_model(tmp_path: Path) -> None:
    repo = make_repo(tmp_path, {})
    (repo / "docs/spec/03-data-model.md").unlink()
    with pytest.raises(sv.UsageError):
        sv.check(repo)
    cfg = repo / sv.CONFIG
    cfg.write_text("data_model: docs/spec/03-data-model.md\ncontent_tables: []\n", encoding="utf-8")
    assert sv.load_config(repo).exceptions == ()
