# SPDX-License-Identifier: Apache-2.0
"""Tests for migrations_lint: numbering, locked checksums and the SQL rules of ADR 0007 and
chapter 14, on throwaway migration directories."""

from __future__ import annotations

import hashlib
from pathlib import Path

import migrations_lint as ml
import pytest

UP = "-- +goose Up\n"


@pytest.fixture
def mig(tmp_path: Path) -> Path:
    d = tmp_path / "migrations"
    d.mkdir()
    return d


@pytest.fixture
def sums(tmp_path: Path) -> Path:
    return tmp_path / "migrations.sha256"


def write(d: Path, name: str, body: str = "CREATE TABLE t (id int);\n") -> Path:
    path = d / name
    path.write_text(body if body.startswith("-- +goose") else UP + body, encoding="utf-8")
    return path


def digest(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def lock_all(d: Path, sum_file: Path) -> None:
    """Hand-written sum file: what `shasum -a 256 *.sql` prints, sorted by name."""
    lines = [f"{digest(p)}  {p.name}\n" for p in sorted(d.glob("*.sql"))]
    sum_file.write_text("".join(lines), encoding="utf-8")


def found(findings: list[ml.Finding]) -> list[tuple[str, str]]:
    return [(Path(f.file).name, f.rule) for f in findings]


def test_clean_sequence_passes(mig: Path, sums: Path) -> None:
    write(
        mig,
        "00001_init.sql",
        "CREATE TABLE spaces (\n"
        "  id uuid PRIMARY KEY,\n"
        "  name text NOT NULL,\n"
        "  search_vector tsvector generated always as (\n"
        "    setweight(to_tsvector('simple'::regconfig, coalesce(name, '')), 'A') ||\n"
        "    setweight(to_tsvector('simple'::regconfig, coalesce(name, '')), 'D')\n"
        "  ) stored\n"
        ");\n"
        "-- +goose Down\n"
        "DROP TABLE spaces;\n",
    )
    write(mig, "00002_users.sql", "CREATE TABLE users (id uuid PRIMARY KEY);\n")
    lock_all(mig, sums)
    assert ml.lint(mig, sums) == []


def test_finding_str_with_and_without_line() -> None:
    assert str(ml.Finding("m/00001_a.sql", 3, "goose-up", "x")) == "m/00001_a.sql:3: goose-up: x"
    assert str(ml.Finding("m/00001_a.sql", None, "name", "y")) == "m/00001_a.sql: name: y"


def test_timestamp_version_fails(mig: Path, sums: Path) -> None:
    write(mig, "00001_init.sql")
    write(mig, "20261010120000_users.sql")
    lock_all(mig, sums)
    findings = ml.lint(mig, sums)
    assert found(findings) == [("20261010120000_users.sql", "timestamp-version")]
    assert "make check" in findings[0].message
    assert "ADR 0007" in findings[0].message


@pytest.mark.parametrize(
    "name", ["1_init.sql", "0001_init.sql", "000001_init.sql", "123456789012345_init.sql"]
)
def test_five_digit_version_required(mig: Path, sums: Path, name: str) -> None:
    write(mig, name)
    lock_all(mig, sums)
    assert found(ml.lint(mig, sums)) == [(name, "version-format")]


@pytest.mark.parametrize(
    "name",
    ["00001_Init.sql", "00001-init.sql", "00001_.sql", "00001_init__x.sql", "init.sql", "x.sql"],
)
def test_bad_name_fails(mig: Path, sums: Path, name: str) -> None:
    write(mig, name)
    lock_all(mig, sums)
    assert found(ml.lint(mig, sums)) == [(name, "name")]


@pytest.mark.parametrize(
    "name", ["\u0660\u0660\u0660\u0660\u0661_init.sql", "\uff10" * 4 + "\uff11_init.sql"]
)
def test_non_ascii_digits_are_not_a_version(mig: Path, sums: Path, name: str) -> None:
    write(mig, name)
    lock_all(mig, sums)
    assert found(ml.lint(mig, sums)) == [(name, "name")]


def test_duplicate_version_fails(mig: Path, sums: Path) -> None:
    write(mig, "00001_init.sql")
    write(mig, "00001_users.sql")
    write(mig, "00002_more.sql")
    lock_all(mig, sums)
    findings = ml.lint(mig, sums)
    assert found(findings) == [("00001_users.sql", "duplicate-version")]
    assert "00001_init.sql" in findings[0].message


@pytest.mark.parametrize(
    ("names", "where", "missing"),
    [
        (["00001_a.sql", "00002_b.sql", "00004_d.sql"], "00004_d.sql", "00003"),
        (["00002_b.sql", "00003_c.sql"], "00002_b.sql", "00001"),
        (["00000_zero.sql", "00001_a.sql"], "00000_zero.sql", "00001"),
    ],
)
def test_version_gap_fails(
    mig: Path, sums: Path, names: list[str], where: str, missing: str
) -> None:
    for name in names:
        write(mig, name)
    lock_all(mig, sums)
    findings = ml.lint(mig, sums)
    assert found(findings) == [(where, "version-gap")]
    assert missing in findings[0].message


def test_changed_locked_migration_fails(mig: Path, sums: Path) -> None:
    write(mig, "00001_init.sql")
    path = write(mig, "00002_users.sql")
    lock_all(mig, sums)
    path.write_text(UP + "CREATE TABLE users (id bigint);\n", encoding="utf-8")
    findings = ml.lint(mig, sums)
    assert found(findings) == [("00002_users.sql", "changed-migration")]
    assert "ADR 0007" in findings[0].message


def test_removed_locked_migration_fails(mig: Path, sums: Path) -> None:
    write(mig, "00001_init.sql")
    path = write(mig, "00002_users.sql")
    lock_all(mig, sums)
    path.unlink()
    findings = ml.lint(mig, sums)
    assert [(f.file, f.line, f.rule) for f in findings] == [(str(sums), 2, "missing-migration")]
    assert "00002_users.sql" in findings[0].message


def test_unlocked_migration_fails(mig: Path, sums: Path) -> None:
    write(mig, "00001_init.sql")
    lock_all(mig, sums)
    write(mig, "00002_users.sql")
    findings = ml.lint(mig, sums)
    assert found(findings) == [("00002_users.sql", "unlocked-migration")]
    assert "make migrations-lock" in findings[0].message


def test_missing_sum_file_leaves_every_migration_unlocked(mig: Path, sums: Path) -> None:
    write(mig, "00001_init.sql")
    assert found(ml.lint(mig, sums)) == [("00001_init.sql", "unlocked-migration")]


def test_empty_directory_without_sum_file_passes(mig: Path, sums: Path) -> None:
    assert ml.lint(mig, sums) == []
    sums.write_text("", encoding="utf-8")
    assert ml.lint(mig, sums) == []


H1 = "a" * 64
H2 = "b" * 64


@pytest.mark.parametrize(
    "text",
    [
        f"{H1} 00001_init.sql\n",  # one space
        f"{H1.upper()}  00001_init.sql\n",  # uppercase hex
        f"{H1[:63]}  00001_init.sql\n",  # short digest
        f"{H1}  00001_init.sql\n{H2}  00001_init.sql\n",  # repeated name
        f"{H2}  00002_users.sql\n{H1}  00001_init.sql\n",  # unsorted
        f"{H1}  00001_init.sql",  # no final newline
        f"{H1}  00001_init.sql\n\n",  # blank line
        f"{H1}  00001_init.sql\r\n",  # CRLF
        f"{H1}  sub/00001_init.sql\n",  # path instead of a file name
    ],
)
def test_malformed_or_unsorted_sum_fails(mig: Path, sums: Path, text: str) -> None:
    sums.write_text(text, encoding="utf-8")
    findings = ml.lint(mig, sums)
    assert "sum-format" in [f.rule for f in findings]
    assert all(f.file == str(sums) for f in findings if f.rule == "sum-format")


def test_generated_without_stored_fails(mig: Path, sums: Path) -> None:
    write(
        mig,
        "00001_init.sql",
        "CREATE TABLE t (\n  a int,\n  b int GENERATED ALWAYS AS (\n    (a + 1) * 2\n  )\n);\n",
    )
    lock_all(mig, sums)
    findings = ml.lint(mig, sums)
    assert [(f.line, f.rule) for f in findings] == [(4, "generated-not-stored")]


@pytest.mark.parametrize(
    "column",
    [
        "b int GENERATED ALWAYS AS (a + 1) VIRTUAL",
        "b int generated always as (a + 1)",
        "b int GENERATED ALWAYS AS (a + 1) STOREDX",
        "b int GENERATED ALWAYS AS (a + 1) STORED$x",
        "b int GENERATED\n  ALWAYS\n  AS(a + 1) NOT NULL",
        "b int GENERATED ALWAYS AS (coalesce(a, 1)",  # closed only by the table's parenthesis
    ],
)
def test_generated_virtual_fails(mig: Path, sums: Path, column: str) -> None:
    write(mig, "00001_init.sql", f"CREATE TABLE t (a int, {column});\n")
    lock_all(mig, sums)
    assert [f.rule for f in ml.lint(mig, sums)] == ["generated-not-stored"]


def test_generated_open_at_end_of_file_fails(mig: Path, sums: Path) -> None:
    write(mig, "00001_init.sql", "CREATE TABLE t (b int GENERATED ALWAYS AS (1 +\n")
    lock_all(mig, sums)
    assert [(f.line, f.rule) for f in ml.lint(mig, sums)] == [(2, "generated-not-stored")]


def test_unterminated_literal_blanks_to_the_end(mig: Path, sums: Path) -> None:
    write(mig, "00001_init.sql", "COMMENT ON TABLE t IS 'open\nGENERATED ALWAYS AS (x);\n")
    lock_all(mig, sums)
    assert ml.lint(mig, sums) == []


def test_generated_stored_after_comment_passes(mig: Path, sums: Path) -> None:
    write(
        mig,
        "00001_init.sql",
        "CREATE TABLE t (a int, b int GENERATED ALWAYS AS (a + 1) /* why */ -- note\n STORED);\n",
    )
    lock_all(mig, sums)
    assert ml.lint(mig, sums) == []


def test_generated_identity_passes(mig: Path, sums: Path) -> None:
    write(
        mig,
        "00001_init.sql",
        "CREATE TABLE t (\n"
        "  id bigint GENERATED ALWAYS AS IDENTITY,\n"
        "  n bigint GENERATED BY DEFAULT AS IDENTITY (START WITH 10)\n"
        ");\n",
    )
    lock_all(mig, sums)
    assert ml.lint(mig, sums) == []


def test_generated_in_comment_or_string_ignored(mig: Path, sums: Path) -> None:
    write(
        mig,
        "00001_init.sql",
        "-- GENERATED ALWAYS AS (x)\n"
        "/* outer /* GENERATED ALWAYS AS (x) */ still comment GENERATED ALWAYS AS (y) */\n"
        "COMMENT ON TABLE t IS 'GENERATED ALWAYS AS (x) it''s';\n"
        "CREATE FUNCTION f() RETURNS text LANGUAGE sql AS $$ SELECT 'GENERATED ALWAYS AS (x)' $$;\n"
        "CREATE FUNCTION g() RETURNS int LANGUAGE sql AS $body$\n"
        "  SELECT 1; -- GENERATED ALWAYS AS (x) CONCURRENTLY\n"
        "$body$;\n"
        'CREATE TABLE "GENERATED ALWAYS AS (" (id int);\n',
    )
    lock_all(mig, sums)
    assert ml.lint(mig, sums) == []


def test_e_string_with_backslash_quote_ignored(mig: Path, sums: Path) -> None:
    write(
        mig,
        "00001_init.sql",
        "COMMENT ON TABLE t IS E'it\\'s GENERATED ALWAYS AS (x)';\n"
        "COMMENT ON COLUMN t.a IS E'a''b\\'c GENERATED ALWAYS AS (x)';\n"
        "CREATE TABLE u (v int GENERATED ALWAYS AS (1));\n",
    )
    lock_all(mig, sums)
    assert [(f.line, f.rule) for f in ml.lint(mig, sums)] == [(4, "generated-not-stored")]


def test_dollar_in_identifier_and_parameter_is_not_a_quote(mig: Path, sums: Path) -> None:
    write(
        mig,
        "00001_init.sql",
        "PREPARE p AS SELECT $1::int;\nCREATE TABLE a$b$c (x int GENERATED ALWAYS AS (1));\n",
    )
    lock_all(mig, sums)
    assert [(f.line, f.rule) for f in ml.lint(mig, sums)] == [(3, "generated-not-stored")]


@pytest.mark.parametrize(
    "body",
    [
        "CREATE TABLE t (id int);\n",  # Up missing (no annotation at all)
        "-- +goose Down\nDROP TABLE t;\n",
        "-- +goose Up\nSELECT 1;\n-- +goose Up\nSELECT 2;\n",
        "  -- +goose Up\nSELECT 1;\n",
    ],
)
def test_missing_goose_up_fails(mig: Path, sums: Path, body: str) -> None:
    (mig / "00001_init.sql").write_text(body, encoding="utf-8")
    lock_all(mig, sums)
    assert found(ml.lint(mig, sums)) == [("00001_init.sql", "goose-up")]


def test_goose_up_ignores_trailing_whitespace(mig: Path, sums: Path) -> None:
    (mig / "00001_init.sql").write_text("-- +goose Up  \t\nSELECT 1;\n", encoding="utf-8")
    lock_all(mig, sums)
    assert ml.lint(mig, sums) == []


@pytest.mark.parametrize(
    "statement",
    [
        "CREATE INDEX CONCURRENTLY idx ON t (a);",
        "CREATE UNIQUE INDEX concurrently IF NOT EXISTS idx ON t (a);",
        "DROP INDEX CONCURRENTLY idx;",
        "ALTER TABLE p DETACH PARTITION p1 CONCURRENTLY;",
        "REINDEX INDEX CONCURRENTLY idx;",
    ],
)
def test_concurrently_requires_no_transaction(mig: Path, sums: Path, statement: str) -> None:
    write(mig, "00001_init.sql", f"SELECT 1;\n{statement}\n")
    lock_all(mig, sums)
    findings = ml.lint(mig, sums)
    assert [(f.line, f.rule) for f in findings] == [(3, "no-transaction")]
    (mig / "00001_init.sql").write_text(
        f"-- +goose NO TRANSACTION\n-- +goose Up\n{statement}\n", encoding="utf-8"
    )
    lock_all(mig, sums)
    assert ml.lint(mig, sums) == []


def test_dollar_inside_an_identifier_is_not_a_keyword_boundary(mig: Path, sums: Path) -> None:
    write(mig, "00001_init.sql", "CREATE INDEX idx$concurrently ON t (a);\n")
    lock_all(mig, sums)
    assert ml.lint(mig, sums) == []


@pytest.mark.parametrize(
    "name", ["idx\u20acconcurrently", "idx\u00a0concurrently", "concurrently\u20ac"]
)
def test_non_ascii_character_continues_an_identifier(mig: Path, sums: Path, name: str) -> None:
    """PostgreSQL's lexer takes every non-ASCII character as part of an identifier."""
    write(mig, "00001_init.sql", f"CREATE INDEX {name} ON t (a);\n")
    lock_all(mig, sums)
    assert ml.lint(mig, sums) == []


def test_dollar_after_a_non_ascii_identifier_is_not_a_quote(mig: Path, sums: Path) -> None:
    write(mig, "00001_init.sql", "CREATE TABLE x\u20ac$$ (b int GENERATED ALWAYS AS (1));\n")
    lock_all(mig, sums)
    assert [(f.line, f.rule) for f in ml.lint(mig, sums)] == [(2, "generated-not-stored")]


def test_non_ascii_space_is_part_of_an_identifier(mig: Path, sums: Path) -> None:
    """PostgreSQL separates words only by ASCII white space; NBSP belongs to the identifier."""
    write(
        mig,
        "00001_init.sql",
        "CREATE VIEW v AS WITH generated\u00a0always AS (SELECT 1 AS a)\n"
        "  SELECT a FROM generated\u00a0always;\n",
    )
    lock_all(mig, sums)
    assert ml.lint(mig, sums) == []


def test_dollar_quote_with_a_non_ascii_tag(mig: Path, sums: Path) -> None:
    write(
        mig,
        "00001_init.sql",
        "SELECT $\u00e4$ concurrently $\u00e4$;\n"
        "COMMENT ON TABLE t IS $\u00e4$it's$\u00e4$;\n"
        "CREATE INDEX CONCURRENTLY i ON t (a);\n",
    )
    lock_all(mig, sums)
    assert [(f.line, f.rule) for f in ml.lint(mig, sums)] == [(4, "no-transaction")]


def test_refresh_concurrently_allowed_in_transaction(mig: Path, sums: Path) -> None:
    write(mig, "00001_init.sql", "REFRESH MATERIALIZED VIEW\n  CONCURRENTLY mv;\n")
    lock_all(mig, sums)
    assert ml.lint(mig, sums) == []


def test_other_files_are_ignored(mig: Path, sums: Path) -> None:
    write(mig, "00001_init.sql")
    (mig / "embed.go").write_text("package migrations\n", encoding="utf-8")
    (mig / "README.md").write_text("# x\n", encoding="utf-8")
    lock_all(mig, sums)
    assert ml.lint(mig, sums) == []


def test_subdirectory_is_layout_finding(mig: Path, sums: Path) -> None:
    write(mig, "00001_init.sql")
    (mig / "old").mkdir()
    lock_all(mig, sums)
    assert found(ml.lint(mig, sums)) == [("old", "layout")]


def test_lock_adds_only_new_entries(
    mig: Path, sums: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    first = write(mig, "00001_init.sql")
    lock_all(mig, sums)
    locked_line = sums.read_text(encoding="utf-8")
    first.write_text(UP + "CREATE TABLE t (id bigint);\n", encoding="utf-8")
    second = write(mig, "00002_users.sql")

    added, refused = ml.lock(mig, sums)

    assert (added, refused) == (["00002_users.sql"], [])
    assert sums.read_text(encoding="utf-8") == locked_line + f"{digest(second)}  00002_users.sql\n"
    assert found(ml.lint(mig, sums)) == [("00001_init.sql", "changed-migration")]


def test_lock_keeps_lines_of_missing_files(mig: Path, sums: Path) -> None:
    write(mig, "00001_init.sql")
    gone = write(mig, "00002_users.sql")
    lock_all(mig, sums)
    before = sums.read_text(encoding="utf-8")
    gone.unlink()
    assert ml.lock(mig, sums) == ([], [])
    assert sums.read_text(encoding="utf-8") == before


def test_lock_refuses_invalid_file(mig: Path, sums: Path) -> None:
    write(mig, "00001_init.sql")
    write(mig, "00003_gap.sql")
    added, refused = ml.lock(mig, sums)
    assert added == []
    assert found(refused) == [("00003_gap.sql", "version-gap")]
    assert not sums.exists()


@pytest.mark.parametrize(
    "text",
    [f"{H1}  00001_init.sql\nnot a sum line\n", f"{H1}  00001_init.sql\n{H2}  00001_init.sql\n"],
)
def test_lock_refuses_unreadable_sum_line(mig: Path, sums: Path, text: str) -> None:
    write(mig, "00001_init.sql")
    write(mig, "00002_users.sql")
    sums.write_text(text, encoding="utf-8")
    added, refused = ml.lock(mig, sums)
    assert added == []
    assert {f.rule for f in refused} == {"sum-format"}
    assert sums.read_text(encoding="utf-8") == text


def test_sum_file_that_is_not_utf8_fails_and_lock_leaves_it(mig: Path, sums: Path) -> None:
    write(mig, "00001_init.sql")
    data = f"{H1}  00001_init.sql\n".encode() + b"%s  00002_\xffusers.sql\n" % H2.encode()
    sums.write_bytes(data)
    assert "sum-format" in [f.rule for f in ml.lint(mig, sums)]
    added, refused = ml.lock(mig, sums)
    assert added == []
    assert [f.rule for f in refused] == ["sum-format"]
    assert sums.read_bytes() == data


def test_lock_normalizes_order_and_final_newline(mig: Path, sums: Path) -> None:
    a = write(mig, "00001_init.sql")
    b = write(mig, "00002_users.sql")
    sums.write_text(f"{digest(b)}  00002_users.sql\n{digest(a)}  00001_init.sql", encoding="utf-8")
    assert ml.lock(mig, sums) == ([], [])
    assert sums.read_text(encoding="utf-8") == (
        f"{digest(a)}  00001_init.sql\n{digest(b)}  00002_users.sql\n"
    )
    assert ml.lint(mig, sums) == []


def test_lock_without_migrations_writes_nothing(mig: Path, sums: Path) -> None:
    assert ml.lock(mig, sums) == ([], [])
    assert not sums.exists()


def test_cli_exit_codes(mig: Path, sums: Path, capsys: pytest.CaptureFixture[str]) -> None:
    args = ["--dir", str(mig), "--sum", str(sums)]
    write(mig, "00001_init.sql")

    assert ml.main(["check", *args]) == 1
    assert "00001_init.sql: unlocked-migration:" in capsys.readouterr().out

    assert ml.main(["lock", *args]) == 0
    assert "locked: 00001_init.sql" in capsys.readouterr().out
    assert ml.main(["check", *args]) == 0
    assert ml.main(["lock", *args]) == 0
    assert "no new migration" in capsys.readouterr().out

    write(mig, "00005_gap.sql")
    assert ml.main(["lock", *args]) == 1
    assert "version-gap" in capsys.readouterr().out

    assert ml.main(["verify", *args]) == 2
    assert ml.main(["check", "--dir", str(mig)]) == 2
    assert ml.main(["check", "--dir", str(mig / "absent"), "--sum", str(sums)]) == 2
    assert "not a directory" in capsys.readouterr().err
