# SPDX-License-Identifier: Apache-2.0
"""Tests for tools/configdoc/configdoc.py: section 4 of the configuration reference and the
checks D1 to D10 of `make config-doc-check`. Fixture repositories are built in `tmp_path`."""

from __future__ import annotations

import importlib.util
import json
import os
import re
import shutil
import subprocess
import sys
from collections import Counter
from pathlib import Path

import pytest
from markdown_it import MarkdownIt

# pytest's importlib mode registers the directory tools/configdoc as the namespace package
# "configdoc" (the parent of this test module), which hides the tool module of the same name;
# the tests load the tool from its file instead.
_SPEC = importlib.util.spec_from_file_location(
    "configdoc_tool", Path(__file__).resolve().parents[1] / "configdoc.py"
)
assert _SPEC is not None
assert _SPEC.loader is not None
configdoc = importlib.util.module_from_spec(_SPEC)
sys.modules[_SPEC.name] = configdoc
_SPEC.loader.exec_module(configdoc)

REPO = Path(__file__).resolve().parents[3]

CHAPTER_02 = """# 02 Architecture

## 22. Configuration values of this chapter

Performance targets are no runtime configuration.

| Name | Place | Default | Meaning |
|---|---|---|---|
| `http.max_json_body_bytes` | Server | `1048576` | largest JSON body |
| `shutdown.total_timeout` | Server | `60s` | total deadline |
| `perf.message_create_p99` | Load test | `150ms` | Target |

## Sources
"""

CHAPTER_09 = """# 09 Agents

## 21. Configuration values of this chapter

| Name | Place | Default | Meaning |
|---|---|---|---|
| `agent.context_max_bytes` | Server | `262144` | Sum |
| `adapter.invocation_token_leeway` | Adapter | `10s` | Clock tolerance |
"""

CHAPTER_16 = """# 16 Operations

## 14. Configuration values of this chapter

| Name | Default | Meaning |
|---|---|---|
| `server.role` | `all` | `api`, `worker` or `all` |
| `server.public_url` | no default, required | Origin |
| `secrets.dir` | `/run/bothy/secrets` | Base |
| `database.sslrootcert` | empty | CA bundle |
"""

CHAPTER_04 = """# 04 Sync

## 18. Configuration values of this chapter

| Name | Default | Meaning |
|---|---|---|
| `sync.event_retention` | `30d` | how long events can be caught up |
| `ratelimit.rl.read.per_minute` / `.burst` | `600` / `100` | per principal |
| `file.s3.access_key_ref`, `file.s3.secret_key_ref` | empty | References |

## 19. After it

| `not.a_value` | `1` | not part of the table |
"""

PLANNED = """# SPDX-License-Identifier: Apache-2.0
rules:
  - match: "shutdown."
    milestone: M1
  - match: "agent."
    milestone: M2
  - match: "server."
    milestone: M1
  - match: secrets.dir
    milestone: M1
  - match: "database."
    milestone: M7
"""

RETIRED = """# SPDX-License-Identifier: Apache-2.0
retired:
  - {name: http.listen_addr, replaced_by: server.listen_addr}
  - {name: db.lock_timeout, replaced_by: database.lock_timeout}
"""

COMPOSE = """name: bothy
x-hardening: &hardening
  read_only: true
x-api: &api
  image: example/bothy-server:1.0.0
  command: ["serve", "--role=api"]
  stop_grace_period: 75s
  <<: *hardening
services:
  api:
    <<: *api
  worker:
    command: ["serve", "--role=worker"]
    stop_grace_period: 75s
  migrate:
    command: ["migrate", "up"]
"""

REFERENCE = """# Configuration reference

Intro stays.

## 4. Named values of all chapters

<!-- config-reference:start -->
old
<!-- config-reference:end -->

Footer stays.
"""


def key(
    name: str, chapter: str, default: str = "", kind: str = "int", **extra: object
) -> dict[str, object]:
    """One key of config-schema.json with every field, as services/server/tools/configschema
    writes it."""
    entry: dict[str, object] = {
        "name": name,
        "env": "BOTHY_" + name.upper().replace(".", "__"),
        "kind": kind,
        "default": default,
        "required": False,
        "enum": [],
        "min": "",
        "max": "",
        "chapter": chapter,
        "doc": "Doc.",
    }
    entry.update(extra)
    return entry


SCHEMA = [
    key("http.max_json_body_bytes", "02-architecture.md", "1048576", min="1"),
    key("server.role", "16-operations.md", "all", kind="enum", enum=["api", "worker", "all"]),
]


def write_repo(
    root: Path,
    *,
    chapters: dict[str, str] | None = None,
    schema: list[dict[str, object]] | None = None,
    planned: str = PLANNED,
    retired: str = RETIRED,
    compose: str = COMPOSE,
    reference: str = REFERENCE,
    extra: dict[str, str | bytes] | None = None,
) -> Path:
    """Write a fixture repository with the layout of the real one and track every file in git.

    The defaults form a repository whose check passes once section 4 is written."""
    if chapters is None:
        chapters = {
            "02-architecture": CHAPTER_02,
            "09-agents": CHAPTER_09,
            "16-operations": CHAPTER_16,
        }
    files: dict[str, str | bytes] = {f"docs/spec/{n}.md": t for n, t in chapters.items()}
    files["services/server/architecture/config-schema.json"] = (
        json.dumps({"version": 1, "keys": SCHEMA if schema is None else schema}, indent=2) + "\n"
    )
    files["tools/configdoc/planned.yaml"] = planned
    files["tools/configdoc/retired.yaml"] = retired
    files["infra/compose/compose.yaml"] = compose
    files["docs/operations/configuration.md"] = reference
    files.update(extra or {})
    for rel, content in files.items():
        path = root / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        if isinstance(content, bytes):
            path.write_bytes(content)
        else:
            path.write_text(content, encoding="utf-8")
    (root / "docs" / "spec").mkdir(parents=True, exist_ok=True)
    subprocess.run(["git", "init", "-q", str(root)], check=True)
    subprocess.run(["git", "-C", str(root), "add", "-A"], check=True)
    return root


def findings(root: Path) -> list[str]:
    """Every finding of the check in the fixture repository as printed."""
    return [str(f) for f in configdoc.check(configdoc.Paths(root))]


def rule_ids(root: Path) -> list[str]:
    return [f.rule for f in configdoc.check(configdoc.Paths(root))]


def written(root: Path) -> Path:
    """Write section 4 of the fixture repository and return the repository."""
    assert configdoc.main(["--root", str(root)]) == 0
    return root


def chapters_of(tmp_path: Path, **texts: str) -> list[configdoc.Chapter]:
    spec = tmp_path / "spec"
    spec.mkdir()
    for name, text in texts.items():
        (spec / f"{name}.md").write_text(text, encoding="utf-8")
    found, problems = configdoc.collect(spec)
    assert problems == []
    return found


# ---- chapter tables (rules kept from the earlier generator) ------------------------------


def test_parse_expands_shorthand_and_lists(tmp_path: Path) -> None:
    [chapter] = chapters_of(tmp_path, **{"04-sync-protocol": CHAPTER_04})
    names = [(v.name, v.default) for v in chapter.values]
    assert names == [
        ("sync.event_retention", "`30d`"),
        ("ratelimit.rl.read.per_minute", "`600`"),
        ("ratelimit.rl.read.burst", "`100`"),
        ("file.s3.access_key_ref", "empty"),
        ("file.s3.secret_key_ref", "empty"),
    ]
    assert chapter.section == "18"
    assert all(v.place == "Server" for v in chapter.values)
    assert [v.line for v in chapter.values] == [7, 8, 8, 9, 9]


def test_place_column_and_environment_name(tmp_path: Path) -> None:
    [chapter] = chapters_of(tmp_path, **{"09-agents": CHAPTER_09})
    server, adapter = chapter.values
    assert (server.place, server.env) == ("Server", "BOTHY_AGENT__CONTEXT_MAX_BYTES")
    assert (adapter.place, adapter.env) == ("Adapter", "")


def test_default_place_per_chapter(tmp_path: Path) -> None:
    text = CHAPTER_04.replace("# 04 Sync", "# 22 Import")
    [chapter] = chapters_of(tmp_path, **{"22-import-migration": text})
    assert {v.place for v in chapter.values} == {"Tool `bothy import`"}
    assert {v.env for v in chapter.values} == {""}


def test_excluded_chapter_is_skipped(tmp_path: Path) -> None:
    assert chapters_of(tmp_path, **{"23-quality": CHAPTER_04}) == []


def test_chapter_without_section_is_skipped(tmp_path: Path) -> None:
    assert chapters_of(tmp_path, **{"07-permissions": "# 07 Permissions\n\nNo section.\n"}) == []


def test_chapter_without_title_uses_file_name(tmp_path: Path) -> None:
    text = CHAPTER_04.replace("# 04 Sync\n\n", "")
    [chapter] = chapters_of(tmp_path, **{"04-sync-protocol": text})
    assert chapter.title == "04-sync-protocol"


def test_section_without_table_has_no_values(tmp_path: Path) -> None:
    text = "# 07 Permissions\n\n## 14. Configuration values of this chapter\n\nNone.\n"
    [chapter] = chapters_of(tmp_path, **{"07-permissions": text})
    assert (chapter.section, chapter.values) == ("14", ())


def test_bad_name_is_a_table_finding(tmp_path: Path) -> None:
    bad = CHAPTER_04.replace("`sync.event_retention`", "`Sync.Event`")
    root = written_or_not(tmp_path, {"04-sync-protocol": bad})
    assert "config-doc: docs/spec/04-sync-protocol.md:7: table: no value name in `Sync.Event`" in (
        findings(root)
    )


def test_cell_without_name_is_a_table_finding(tmp_path: Path) -> None:
    bad = CHAPTER_04.replace("`sync.event_retention`", "sync.event_retention")
    root = written_or_not(tmp_path, {"04-sync-protocol": bad})
    assert any(": table: no value name in 'sync.event_retention'" in f for f in findings(root))


def test_split_default_count_must_match(tmp_path: Path) -> None:
    bad = CHAPTER_04.replace("`600` / `100`", "`600` / `100` / `7`")
    root = written_or_not(tmp_path, {"04-sync-protocol": bad})
    assert "config-doc: docs/spec/04-sync-protocol.md:8: table: 2 names but 3 defaults" in findings(
        root
    )


def written_or_not(tmp_path: Path, chapters: dict[str, str]) -> Path:
    """A fixture repository with only `chapters`, every Server key planned by one rule."""
    planned = "rules:\n" + "".join(
        f'  - match: "{p}"\n    milestone: M3\n' for p in ("sync.", "ratelimit.", "file.")
    )
    return write_repo(tmp_path, chapters=chapters, schema=[], planned=planned)


def test_render_replaces_only_the_marked_block(tmp_path: Path) -> None:
    root = written(write_repo(tmp_path))
    out = (root / "docs/operations/configuration.md").read_text(encoding="utf-8")
    assert out.startswith("# Configuration reference\n\nIntro stays.\n")
    assert out.endswith("Footer stays.\n")
    assert "\nold\n" not in out
    assert "### 02 Architecture\n" in out
    assert "Source: [02-architecture.md](../spec/02-architecture.md), section 22." in out
    assert "| Key | Environment | Place | Type | Default | Meaning | Status |" in out
    assert (
        "| `http.max_json_body_bytes` | `BOTHY_HTTP__MAX_JSON_BODY_BYTES` | Server | int"
        " | `1048576` | largest JSON body | active |" in out
    )
    assert (
        "| `adapter.invocation_token_leeway` |  | Adapter |  | `10s` | Clock tolerance |  |" in out
    )


def test_render_rewrites_chapter_relative_links(tmp_path: Path) -> None:
    linked = CHAPTER_09.replace(
        "| Sum |",
        "| see [19](19-security.md#x), [ADR](../adr/README.md), [W](https://example.com) |",
    )
    root = written(
        write_repo(
            tmp_path,
            chapters={
                "09-agents": linked,
                "02-architecture": CHAPTER_02,
                "16-operations": CHAPTER_16,
            },
        )
    )
    out = (root / "docs/operations/configuration.md").read_text(encoding="utf-8")
    assert "[19](../spec/19-security.md#x)" in out
    assert "[ADR](../adr/README.md)" in out
    assert "[W](https://example.com)" in out


def test_render_needs_markers() -> None:
    with pytest.raises(configdoc.InputError, match="markers"):
        configdoc.render("no markers", [], {}, [])


def test_main_missing_marker_exits_2(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    root = write_repo(tmp_path, reference="# Configuration reference\n")
    assert configdoc.main(["--root", str(root), "--check"]) == 2
    assert "markers" in capsys.readouterr().err


def test_main_writes_then_check_passes(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    root = write_repo(tmp_path)
    args = ["--root", str(root)]
    assert configdoc.main([*args, "--check"]) == 1
    assert ": D10: section 4 is stale; run `make config-reference`" in capsys.readouterr().err
    assert configdoc.main(args) == 0
    assert configdoc.main([*args, "--check"]) == 0
    assert (
        capsys.readouterr().out.splitlines()[-1]
        == "config-doc: ok (9 keys, 2 in schema, 5 planned)"
    )


def test_write_mode_writes_and_reports_findings(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    root = write_repo(tmp_path, planned="rules: []\n")
    assert configdoc.main(["--root", str(root)]) == 1
    out = capsys.readouterr()
    assert ": D6: " in out.err
    assert "config-doc: wrote docs/operations/configuration.md" in out.out
    text = (root / "docs/operations/configuration.md").read_text(encoding="utf-8")
    assert (
        "| `agent.context_max_bytes` | `BOTHY_AGENT__CONTEXT_MAX_BYTES` | Server |  | `262144`"
        " | Sum | planned |" in text
    )
    assert [f.rule for f in configdoc.check(configdoc.Paths(root))] == ["D6"] * 5


# ---- table rules -------------------------------------------------------------------------


def table_findings(tmp_path: Path, chapter: str, name: str = "16-operations") -> list[str]:
    root = write_repo(
        tmp_path, chapters={"02-architecture": CHAPTER_02, "09-agents": CHAPTER_09, name: chapter}
    )
    return [f for f in findings(root) if ": table: " in f]


def test_escaped_pipe_stays_in_its_cell(tmp_path: Path) -> None:
    text = CHAPTER_16.replace("| `api`, `worker` or `all` |", "| `api` \\| `worker` \\| `all` |")
    root = written(
        write_repo(
            tmp_path,
            chapters={
                "02-architecture": CHAPTER_02,
                "09-agents": CHAPTER_09,
                "16-operations": text,
            },
        )
    )
    out = (root / "docs/operations/configuration.md").read_text(encoding="utf-8")
    assert "| `all` | `api` \\| `worker` \\| `all` | active |" in out


def test_row_with_extra_cell_is_a_table_finding(tmp_path: Path) -> None:
    text = CHAPTER_16.replace(
        "| `secrets.dir` | `/run/bothy/secrets` | Base |",
        "| `secrets.dir` | `/run/bothy/secrets` | Base | M1 |",
    )
    assert table_findings(tmp_path, text) == [
        "config-doc: docs/spec/16-operations.md:9: table: 4 cells, the header has 3"
    ]


def test_short_row_is_a_table_finding(tmp_path: Path) -> None:
    text = CHAPTER_02.replace(
        "| `perf.message_create_p99` | Load test | `150ms` | Target |",
        "| `perf.message_create_p99` |",
    )
    assert table_findings(tmp_path, text, "02-architecture") == [
        "config-doc: docs/spec/02-architecture.md:11: table: 1 cells, the header has 4"
    ]


def test_empty_default_cell_is_a_table_finding(tmp_path: Path) -> None:
    text = CHAPTER_16.replace("| `database.sslrootcert` | empty |", "| `database.sslrootcert` |  |")
    assert table_findings(tmp_path, text) == [
        "config-doc: docs/spec/16-operations.md:10: table: empty Default cell; write `empty` for an"
        " empty default"
    ]


def test_unknown_header_is_a_table_finding(tmp_path: Path) -> None:
    text = CHAPTER_16.replace(
        "| Name | Default | Meaning |\n|---|---|---|",
        "| Name | Default | Unit | Meaning |\n|---|---|---|---|",
    )
    assert table_findings(tmp_path, text) == [
        "config-doc: docs/spec/16-operations.md:5: table: header must be | Name | Default |"
        " Meaning | or | Name | Place | Default | Meaning |"
    ]


def test_missing_separator_is_a_table_finding(tmp_path: Path) -> None:
    text = CHAPTER_16.replace("|---|---|---|\n", "")
    assert table_findings(tmp_path, text) == [
        "config-doc: docs/spec/16-operations.md:6: table: the header needs a separator row (|---|)"
    ]


def test_unknown_place_is_a_table_finding(tmp_path: Path) -> None:
    text = CHAPTER_09.replace("| Server | `262144` |", "| server | `262144` |")
    found = table_findings(tmp_path, text, "09-agents")
    assert len(found) == 1
    assert found[0].startswith(
        "config-doc: docs/spec/09-agents.md:7: table: unknown place 'server'; known: "
    )


def test_indented_row_is_read(tmp_path: Path) -> None:
    text = CHAPTER_16.replace("| `secrets.dir` |", "   | `secrets.dir` |")
    chapters = chapters_of(tmp_path, **{"16-operations": text})
    assert "secrets.dir" in [v.name for v in chapters[0].values]


def test_fenced_block_in_section_is_not_read(tmp_path: Path) -> None:
    text = CHAPTER_16 + "\n```text\n| `server.role` | `api` | Example |\n```\n"
    root = written(
        write_repo(
            tmp_path,
            chapters={
                "02-architecture": CHAPTER_02,
                "09-agents": CHAPTER_09,
                "16-operations": text,
            },
        )
    )
    assert findings(root) == []


def test_heading_variant_is_a_table_finding(tmp_path: Path) -> None:
    text = CHAPTER_16.replace(
        "## 14. Configuration values of this chapter",
        "## 14. Configuration values of this chapter (Server)",
    )
    found = table_findings(tmp_path, text)
    assert found == [
        "config-doc: docs/spec/16-operations.md:3: table: heading must read"
        " '## <number>. Configuration values of this chapter'"
    ]


def test_second_section_is_a_table_finding(tmp_path: Path) -> None:
    text = (
        CHAPTER_16
        + "\n## 15. Configuration values of this chapter\n\n"
        + "| Name | Default | Meaning |\n|---|---|---|\n"
    )
    assert table_findings(tmp_path, text) == [
        "config-doc: docs/spec/16-operations.md:12: table: second section Configuration values of"
        " this chapter; one table per chapter"
    ]


def test_unreadable_chapter_exits_2(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    root = write_repo(tmp_path, extra={"docs/spec/07-permissions.md": "caf\xe9".encode("latin-1")})
    assert configdoc.main(["--root", str(root), "--check"]) == 2
    assert "config-doc: docs/spec/07-permissions.md: not UTF-8" in capsys.readouterr().err


def test_reference_that_is_a_directory_exits_2(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    root = write_repo(tmp_path)
    ref = root / "docs/operations/configuration.md"
    ref.unlink()
    ref.mkdir()
    assert configdoc.main(["--root", str(root), "--check"]) == 2
    assert "config-doc: docs/operations/configuration.md: cannot read" in capsys.readouterr().err


# ---- markers -----------------------------------------------------------------------------


def test_marker_mentioned_in_prose_is_not_a_marker(tmp_path: Path) -> None:
    reference = REFERENCE.replace(
        "Intro stays.",
        "The block starts with `<!-- config-reference:start -->`.\n\nSection stays.",
    )
    root = written(write_repo(tmp_path, reference=reference))
    out = (root / "docs/operations/configuration.md").read_text(encoding="utf-8")
    assert "Section stays." in out
    assert findings(root) == []


@pytest.mark.parametrize(
    "reference",
    [
        REFERENCE + "<!-- config-reference:start -->\nalt\n<!-- config-reference:end -->\n",
        REFERENCE.replace(
            "<!-- config-reference:start -->\nold\n<!-- config-reference:end -->",
            "<!-- config-reference:end -->\nold\n<!-- config-reference:start -->",
        ),
        REFERENCE.replace("<!-- config-reference:end -->\n", ""),
    ],
)
def test_markers_must_be_one_pair_in_order(
    tmp_path: Path, reference: str, capsys: pytest.CaptureFixture[str]
) -> None:
    root = write_repo(tmp_path, reference=reference)
    assert configdoc.main(["--root", str(root), "--check"]) == 2
    assert "markers" in capsys.readouterr().err


# ---- D1 to D5: names and the schema ----------------------------------------------------


def test_name_in_two_chapters_fails(tmp_path: Path) -> None:
    twice = CHAPTER_09.replace("`agent.context_max_bytes`", "`http.max_json_body_bytes`")
    root = write_repo(
        tmp_path,
        chapters={"02-architecture": CHAPTER_02, "09-agents": twice, "16-operations": CHAPTER_16},
    )
    assert (
        "config-doc: docs/spec/09-agents.md:7: D1: http.max_json_body_bytes is named twice"
        " (also docs/spec/02-architecture.md:9); one name per value (ruling L4)"
    ) in findings(root)


def test_name_twice_in_one_table_fails(tmp_path: Path) -> None:
    twice = CHAPTER_16.replace("`secrets.dir`", "`server.role`")
    root = write_repo(tmp_path, chapters={"02-architecture": CHAPTER_02, "16-operations": twice})
    assert "D1" in rule_ids(root)


def test_schema_key_without_chapter_row_fails(tmp_path: Path) -> None:
    schema = [*SCHEMA, key("gateway.write_timeout", "02-architecture.md", "10s", kind="duration")]
    root = write_repo(tmp_path, schema=schema)
    [line] = [f for f in findings(root) if ": D2: " in f]
    assert line.startswith("config-doc: services/server/architecture/config-schema.json:")
    assert line.endswith(
        ": D2: gateway.write_timeout has no row in a chapter table; add it to the table of"
        " 02-architecture.md"
    )
    number = int(line.split(":")[2])
    text = (root / "services/server/architecture/config-schema.json").read_text().splitlines()
    assert '"name": "gateway.write_timeout"' in text[number - 1]


def test_schema_key_in_other_chapter_fails(tmp_path: Path) -> None:
    schema = [key("http.max_json_body_bytes", "16-operations.md", "1048576"), SCHEMA[1]]
    root = write_repo(tmp_path, schema=schema)
    assert (
        "config-doc: docs/spec/02-architecture.md:9: D3: http.max_json_body_bytes is in the table"
        " of 02-architecture.md, the schema names 16-operations.md"
    ) in findings(root)


def test_schema_key_with_non_server_place_fails(tmp_path: Path) -> None:
    schema = [
        *SCHEMA,
        key("adapter.invocation_token_leeway", "09-agents.md", "10s", kind="duration"),
    ]
    root = write_repo(tmp_path, schema=schema)
    assert (
        "config-doc: docs/spec/09-agents.md:8: D4: adapter.invocation_token_leeway is in the"
        " schema, but its row names the place Adapter, not Server"
    ) in findings(root)


def test_schema_default_differs_from_chapter_fails(tmp_path: Path) -> None:
    schema = [key("http.max_json_body_bytes", "02-architecture.md", "2097152"), SCHEMA[1]]
    root = write_repo(tmp_path, schema=schema)
    assert (
        "config-doc: docs/spec/02-architecture.md:9: D5: http.max_json_body_bytes: schema default"
        " '2097152', chapter cell '`1048576`'"
    ) in findings(root)


def test_required_and_empty_defaults_match(tmp_path: Path) -> None:
    schema = [
        *SCHEMA,
        key("server.public_url", "16-operations.md", "", kind="url", required=True),
        key("database.sslrootcert", "16-operations.md", "", kind="string"),
    ]
    planned = PLANNED.replace('  - match: "server."\n    milestone: M1\n', "").replace(
        '  - match: "database."\n    milestone: M7\n', ""
    )
    root = written(write_repo(tmp_path, schema=schema, planned=planned))
    assert findings(root) == []


@pytest.mark.parametrize(
    ("name", "kind", "default", "required", "cell"),
    [
        ("server.public_url", "url", "https://x.example", False, "no default, required"),
        ("server.public_url", "url", "", False, "no default, required"),
        ("database.sslrootcert", "string", "", True, "empty"),
        ("database.sslrootcert", "string", "x", False, "empty"),
    ],
)
def test_required_and_empty_mismatches_fail(
    tmp_path: Path, name: str, kind: str, default: str, required: bool, cell: str
) -> None:
    schema = [*SCHEMA, key(name, "16-operations.md", default, kind=kind, required=required)]
    root = write_repo(tmp_path, schema=schema)
    [line] = [f for f in findings(root) if ": D5: " in f]
    assert f"chapter cell {cell!r}" in line
    assert ("(required)" in line) == required


# ---- D6: planned keys ------------------------------------------------------------------


def test_planned_key_rendered_with_milestone(tmp_path: Path) -> None:
    root = written(write_repo(tmp_path))
    out = (root / "docs/operations/configuration.md").read_text(encoding="utf-8")
    assert (
        "| `agent.context_max_bytes` | `BOTHY_AGENT__CONTEXT_MAX_BYTES` | Server |  | `262144`"
        " | Sum | planned (M2) |" in out
    )
    assert (
        "| `server.role` | `BOTHY_SERVER__ROLE` | Server | enum(api, worker, all) | `all` |" in out
    )
    assert (
        "| `server.public_url` | `BOTHY_SERVER__PUBLIC_URL` | Server |  | no default, required |"
        " Origin | planned (M1) |" in out
    )


def test_planned_key_without_rule_fails(tmp_path: Path) -> None:
    planned = PLANNED.replace('  - match: "agent."\n    milestone: M2\n', "")
    root = write_repo(tmp_path, planned=planned)
    assert (
        "config-doc: docs/spec/09-agents.md:7: D6: agent.context_max_bytes is planned but no"
        " rule of tools/configdoc/planned.yaml gives its milestone"
    ) in findings(root)


def test_stale_planned_rule_fails(tmp_path: Path) -> None:
    planned = (
        PLANNED
        + '  - match: "egress."\n    milestone: M1\n  - match: server.role\n    milestone: M1\n'
    )
    root = write_repo(tmp_path, planned=planned)
    stale = [f for f in findings(root) if ": D6: " in f]
    assert stale == [
        "config-doc: tools/configdoc/planned.yaml:13: D6: rule 'egress.' gives the milestone of no"
        " planned key (stale); remove it",
        "config-doc: tools/configdoc/planned.yaml:15: D6: rule 'server.role' gives the milestone of"
        " no planned key (stale); remove it",
    ]


def test_exact_rule_beats_prefix(tmp_path: Path) -> None:
    rules = [
        configdoc.Rule("a.", "M3", 1),
        configdoc.Rule("a.b.", "M4", 2),
        configdoc.Rule("a.b.c", "M5", 3),
    ]
    assert configdoc.milestone_of("a.b.c", rules) == rules[2]
    assert configdoc.milestone_of("a.b.d", rules) == rules[1]
    assert configdoc.milestone_of("a.x", rules) == rules[0]
    assert configdoc.milestone_of("ab.x", rules) is None
    assert configdoc.milestone_of("a.b", rules) == rules[0]
    planned = PLANNED + "  - match: agent.context_max_bytes\n    milestone: M1\n"
    root = write_repo(tmp_path, planned=planned)
    out = configdoc.evaluate(configdoc.Paths(root)).rendered
    assert "| Sum | planned (M1) |" in out
    assert "rule 'agent.'" in " ".join(findings(root))  # the prefix rule decides nothing now


# ---- D7: retired names -----------------------------------------------------------------


def test_retired_name_in_backticks_fails(tmp_path: Path) -> None:
    root = write_repo(
        tmp_path, extra={"docs/x.md": "a\nListener `http.listen_addr` has another name now\n"}
    )
    assert (
        "config-doc: docs/x.md:2: D7: http.listen_addr is retired (W1); use server.listen_addr"
    ) in findings(root)


def test_retired_name_inside_longer_code_span_fails(tmp_path: Path) -> None:
    root = write_repo(tmp_path, extra={"docs/x.md": "set `db.lock_timeout = 2s` here\n"})
    assert ": D7: db.lock_timeout is retired (W1)" in " ".join(findings(root))


@pytest.mark.parametrize(
    ("text", "line"),
    [
        ("Default `db.lock_timeout =\n2s`: applies.\n", 1),
        ("a\n\n```text\nif t > db.lock_timeout: abort\n```\n", 4),
        ("~~~yaml\n# db.lock_timeout: 2s\n~~~\n", 2),
        ("also `` `db.lock_timeout = 2s` `` here\n", 1),
        ("Value `a =\n2s` and\n`db.lock_timeout` apply.\n", 3),
    ],
)
def test_retired_name_in_any_code_span_or_block_fails(tmp_path: Path, text: str, line: int) -> None:
    root = write_repo(tmp_path, extra={"docs/x.md": text})
    assert [f for f in findings(root) if ": D7: " in f] == [
        f"config-doc: docs/x.md:{line}: D7: db.lock_timeout is retired (W1); use"
        " database.lock_timeout"
    ]


def test_prose_after_wrapped_span_passes(tmp_path: Path) -> None:
    text = "The command `make\ncheck` knows not db.lock_timeout but `database.lock_timeout`.\n"
    root = written(write_repo(tmp_path, extra={"docs/x.md": text}))
    assert findings(root) == []


@pytest.mark.parametrize(
    "text",
    [
        "| a | b |\n|---|---|\n| c | `x |\nValue `db.lock_timeout` here.\n",
        "## Title with ` backtick\nValue `db.lock_timeout` here.\n",
        "- Item with ` backtick\n- Value `db.lock_timeout` here.\n",
        "> Quote with ` backtick\n- Value `db.lock_timeout` here.\n",
    ],
)
def test_stray_backtick_in_another_block_does_not_hide_a_span(tmp_path: Path, text: str) -> None:
    root = write_repo(tmp_path, extra={"docs/x.md": text})
    line = text.count("\n")
    assert [f for f in findings(root) if ": D7: " in f] == [
        f"config-doc: docs/x.md:{line}: D7: db.lock_timeout is retired (W1); use"
        " database.lock_timeout"
    ]
    assert tool_names(text) == oracle_names(text)


def test_wrapped_span_in_list_item_counts(tmp_path: Path) -> None:
    text = "- Item with `db.lock_timeout =\n  2s` as value.\n"
    root = write_repo(tmp_path, extra={"docs/x.md": text})
    assert [f for f in findings(root) if ": D7: " in f] == [
        "config-doc: docs/x.md:1: D7: db.lock_timeout is retired (W1); use database.lock_timeout"
    ]


# CommonMark oracle for code_parts: markdown-it-py (CommonMark plus GFM tables) is the reference
# parser; the tool itself stays with the standard library (tools/README.md).
_MARKDOWN = MarkdownIt("commonmark").enable("table")
_DOTTED = re.compile(
    r"(?<![A-Za-z0-9_.])[a-z][a-z0-9_]*(?:\.[a-z0-9_]+)+(?![A-Za-z0-9_]|\.[A-Za-z0-9_])"
)


def oracle_names(text: str) -> Counter[str]:
    """Dotted names inside fenced code blocks and code spans, as a CommonMark parser sees them."""
    code: list[str] = []

    def walk(tokens: list) -> None:  # type: ignore[type-arg]
        for token in tokens:
            if token.type in ("fence", "code_inline"):
                code.append(token.content)
            if token.children:
                walk(token.children)

    walk(_MARKDOWN.parse(text))
    return Counter(n for c in code for n in _DOTTED.findall(c))


def tool_names(text: str) -> Counter[str]:
    return Counter(n for _, c in configdoc.code_parts(text) for n in _DOTTED.findall(c))


@pytest.mark.parametrize(
    "text",
    [
        "> The old value `db.lock_timeout =\n> 2s` no longer applies.\n",
        "> Value `a\n> b` and db.lock_timeout in prose and `c.d` here.\n",
        "- Item\n  > Quote `db.lock_timeout\n  > more` x\n",
        "On `db.lock_timeout\n11. October` set.\n",
        "Text\n1. Item with `a.b` here\n",
        "- a\n  - b with ` stray\n    - c with `db.lock_timeout` here\n",
        "1. First `a.b\n   c` item\n2. Second `d.e` item\n",
        "A \\`x.y\\` is no code, but `p.q` is.\n",
        "`| \\`go.arch\\` | x |` after it `r.s`\n",
        "| a | `x |\nValue `db.lock_timeout` here.\n",
        "## Title with ` backtick\nValue `db.lock_timeout` here.\n",
        "- Item with ` backtick\n- Value `db.lock_timeout` here.\n",
        "- a\n  2. b\n     ```yaml\n     k: x.y\n     ```\n",
        "| `a.b` \\| `c.d` | e |\n|---|---|\n| `f.g | h.i` |\n",
        "Paragraph `a.b\n\nmore` without code.\n",
        "Value `db.lock_timeout\n2026. retired` here.\n",
        "Value `db.lock_timeout\n| more` here.\n",
        "> Value `db.lock_timeout\n> = 2s` x\n",
        "# Title\nText `a.b\nc` d\n",
        "```yaml\nx: `y.z`\n```\nand `u.v` after it\n",
        "Run `` `m.n` `` and ``` o.p ```\n",
    ],
)
def test_code_parts_match_commonmark(text: str) -> None:
    assert tool_names(text) == oracle_names(text)


def tracked_markdown(root: Path) -> list[str]:
    """Every Markdown file git tracks in `root`, NUL-separated: no name is split or quoted."""
    listing = subprocess.run(
        ["git", "-C", str(root), "ls-files", "-z", "*.md"], check=True, capture_output=True
    ).stdout
    return [rel for rel in os.fsdecode(listing).split("\0") if rel]


def oracle_mismatches(root: Path, files: list[str]) -> dict[str, Counter[str]]:
    """Per file: the names in code that the oracle finds and code_parts misses (M1-08/15).

    Only this direction fails: a span the tool loses hides a retired name from D7, the failure
    class of the second review round. code_parts finds more than the oracle on purpose: it reads
    every cell of a table row, also cells beyond the header that GFM drops (Python-Markdown,
    which renders the docs site, does not split a cell at `|` in code), and spans in HTML
    blocks, HTML comments and indented code blocks, which the oracle does not count.
    """
    mismatches: dict[str, Counter[str]] = {}
    for rel in files:
        text = (root / rel).read_text(encoding="utf-8")
        if missed := oracle_names(text) - tool_names(text):
            mismatches[rel] = missed
    return mismatches


def test_code_parts_match_commonmark_on_the_real_markdown() -> None:
    """The real artifact: code_parts finds every name in code that the oracle finds, in every
    tracked Markdown file of the repository."""
    listing = tracked_markdown(REPO)
    assert len(listing) > 100
    assert oracle_mismatches(REPO, listing) == {}


def test_code_in_a_cell_beyond_the_header_is_allowed_extra_detection(tmp_path: Path) -> None:
    # The row D-W-71 in docs/DECISIONS.md: GFM splits the row at the `|`
    # in the code span, so it has one cell more than the header, and the oracle drops that cell.
    # Python-Markdown, which renders the docs site, keeps it as one cell; code_parts reads it.
    text = (
        "| ID | Rule | Files |\n"
        "|---|---|---|\n"
        "| X-1 | It counts cells so: `\\|` and `|` in code do not split. | `tools/a.py` |\n"
    )
    root = write_repo(tmp_path, extra={"docs/x.md": text})
    assert tool_names(text) - oracle_names(text) == Counter({"a.py": 1})
    assert oracle_mismatches(root, ["docs/x.md"]) == {}


def test_oracle_mismatches_report_a_span_code_parts_loses(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    text = "> The old value `db.lock_timeout =\n> 2s` no longer applies.\n"
    root = write_repo(tmp_path, extra={"docs/x.md": text})
    monkeypatch.setattr(configdoc, "code_parts", lambda text, markdown=True: iter(()))
    assert oracle_mismatches(root, ["docs/x.md"]) == {"docs/x.md": Counter({"db.lock_timeout": 1})}


# A file name with a space and an umlaut (u with diaeresis), built from its code point.
UMLAUT_NAME = "docs/Pr" + chr(0xFC) + "f List.md"


def test_tracked_markdown_keeps_names_with_spaces_and_umlauts(tmp_path: Path) -> None:
    root = write_repo(tmp_path, extra={UMLAUT_NAME: "Value `a.b` here.\n"})
    listing = tracked_markdown(root)
    assert UMLAUT_NAME in listing
    assert oracle_mismatches(root, listing) == {}


def test_wrapped_span_in_comment_lines_of_other_files_counts(tmp_path: Path) -> None:
    text = "# Value `db.lock_timeout\n# = 2s` is old\nkey: 1\n"
    root = write_repo(tmp_path, extra={"infra/x.yaml": text})
    assert [f for f in findings(root) if ": D7: " in f] == [
        "config-doc: infra/x.yaml:1: D7: db.lock_timeout is retired (W1); use database.lock_timeout"
    ]


def test_unclosed_backtick_does_not_reach_the_next_paragraph(tmp_path: Path) -> None:
    text = "A ` alone.\n\nThen db.lock_timeout in prose and ` after it.\n"
    root = written(write_repo(tmp_path, extra={"docs/x.md": text}))
    assert findings(root) == []


def test_retiring_a_name_only_section_4_holds_regenerates(tmp_path: Path) -> None:
    root = written(write_repo(tmp_path))
    retired = RETIRED + "  - {name: secrets.dir, replaced_by: secrets.root}\n"
    chapters = {
        "02-architecture": CHAPTER_02,
        "09-agents": CHAPTER_09,
        "16-operations": CHAPTER_16.replace("secrets.dir", "secrets.root"),
    }
    reference = (root / "docs/operations/configuration.md").read_text(encoding="utf-8")
    assert "`secrets.dir`" in reference
    root = write_repo(
        root,
        retired=retired,
        chapters=chapters,
        planned=PLANNED.replace("secrets.dir", "secrets.root"),
        reference=reference,
    )
    assert configdoc.main(["--root", str(root)]) == 0
    assert findings(root) == []


def test_retired_env_form_fails(tmp_path: Path) -> None:
    root = write_repo(tmp_path, extra={"infra/x.env": "BOTHY_HTTP__LISTEN_ADDR=:8080\n"})
    assert (
        "config-doc: infra/x.env:1: D7: BOTHY_HTTP__LISTEN_ADDR is the environment form of the"
        " retired http.listen_addr (W1); use server.listen_addr"
    ) in findings(root)


def test_retired_name_in_allowlisted_file_passes(tmp_path: Path) -> None:
    text = "| `http.listen_addr` (02) | BOTHY_DB__LOCK_TIMEOUT |\n"
    extra = {
        "docs/PLAN.md": text,
        "docs/DECISIONS.md": text,
        "docs/briefs/M1-08-config-reference.md": text,
        "tools/configdoc/tests/test_x.py": text,
    }
    root = written(write_repo(tmp_path, extra=extra))
    assert findings(root) == []


def test_otel_attribute_db_system_name_passes(tmp_path: Path) -> None:
    root = written(
        write_repo(tmp_path, extra={"docs/x.md": "Attribute `db.system.name` and `db.statement`\n"})
    )
    assert findings(root) == []


def test_longer_name_is_not_retired(tmp_path: Path) -> None:
    text = (
        "`http.listen_addr_v2`, `xdb.lock_timeout`, `db.lock_timeout.extra`,"
        " BOTHY_DB__LOCK_TIMEOUT_X, XBOTHY_HTTP__LISTEN_ADDR, http.listen_addr ohne Backticks\n"
    )
    root = written(write_repo(tmp_path, extra={"docs/x.md": text}))
    assert findings(root) == []


def test_binary_file_is_skipped(tmp_path: Path) -> None:
    extra: dict[str, str | bytes] = {
        "img.png": b"\x89PNG\x00`http.listen_addr`",
        "latin.txt": "`http.listen_addr` caf\xe9".encode("latin-1"),
    }
    root = written(write_repo(tmp_path, extra=extra))
    assert findings(root) == []


def test_untracked_file_is_not_scanned(tmp_path: Path) -> None:
    root = written(write_repo(tmp_path))
    (root / "notes.md").write_text("`http.listen_addr`\n", encoding="utf-8")
    assert findings(root) == []


def test_not_a_git_repository_exits_2(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    root = write_repo(tmp_path)
    shutil.rmtree(root / ".git")
    assert configdoc.main(["--root", str(root), "--check"]) == 2
    assert "git ls-files" in capsys.readouterr().err


# ---- D8: Compose stop_grace_period -----------------------------------------------------


def test_missing_stop_grace_period_fails(tmp_path: Path) -> None:
    compose = COMPOSE.replace(
        '    command: ["serve", "--role=worker"]\n    stop_grace_period: 75s\n',
        '    command: ["serve", "--role=worker"]\n',
    )
    root = write_repo(tmp_path, compose=compose)
    assert (
        "config-doc: infra/compose/compose.yaml:12: D8: service worker runs `serve` without"
        " stop_grace_period; set it longer than shutdown.total_timeout (60s)"
    ) in findings(root)


def test_short_stop_grace_period_fails(tmp_path: Path) -> None:
    compose = COMPOSE.replace(
        '  command: ["serve", "--role=api"]\n  stop_grace_period: 75s',
        '  command: ["serve", "--role=api"]\n  stop_grace_period: 60s',
    )
    root = write_repo(tmp_path, compose=compose)
    assert (
        "config-doc: infra/compose/compose.yaml:10: D8: service api: stop_grace_period 60s is not"
        " longer than shutdown.total_timeout (60s)"
    ) in findings(root)


def test_merge_key_anchor_is_followed(tmp_path: Path) -> None:
    compose = COMPOSE.replace("  stop_grace_period: 75s\n  <<: *hardening", "  <<: *hardening")
    root = write_repo(tmp_path, compose=compose)
    d8 = [f for f in findings(root) if ": D8: " in f]
    assert d8 == [
        "config-doc: infra/compose/compose.yaml:9: D8: service api runs `serve` without"
        " stop_grace_period; set it longer than shutdown.total_timeout (60s)"
    ]


def test_merge_key_override_in_service_wins(tmp_path: Path) -> None:
    compose = COMPOSE.replace("    <<: *api\n", "    <<: *api\n    stop_grace_period: 30s\n")
    root = write_repo(tmp_path, compose=compose)
    assert "service api: stop_grace_period 30s is not longer" in " ".join(findings(root))


def test_command_as_string_is_followed(tmp_path: Path) -> None:
    compose = COMPOSE.replace(
        '    command: ["serve", "--role=worker"]\n    stop_grace_period: 75s\n',
        "    command: serve --role=worker\n",
    )
    root = write_repo(tmp_path, compose=compose)
    assert "service worker runs `serve` without stop_grace_period" in " ".join(findings(root))


def test_compound_compose_duration_compares(tmp_path: Path) -> None:
    for value, ok in [
        ("1m30s", True),
        ("61s", True),
        ("1m", False),
        ("59500ms", False),
        ("1h", True),
    ]:
        compose = COMPOSE.replace("stop_grace_period: 75s", f"stop_grace_period: {value}")
        root = write_repo(tmp_path / value, compose=compose)
        assert ("D8" not in rule_ids(root)) == ok, value


def test_unreadable_stop_grace_period_fails(tmp_path: Path) -> None:
    compose = COMPOSE.replace("stop_grace_period: 75s", "stop_grace_period: ${GRACE:-75s}", 1)
    root = write_repo(tmp_path, compose=compose)
    assert (
        "config-doc: infra/compose/compose.yaml:10: D8: service api: cannot read"
        " stop_grace_period '${GRACE:-75s}' (want a Compose duration such as 75s or 1m30s)"
    ) in findings(root)


def test_total_timeout_from_schema_wins(tmp_path: Path) -> None:
    schema = [*SCHEMA, key("shutdown.total_timeout", "02-architecture.md", "90s", kind="duration")]
    chapters = {
        "02-architecture": CHAPTER_02.replace("`60s`", "`90s`"),
        "09-agents": CHAPTER_09,
        "16-operations": CHAPTER_16,
    }
    root = write_repo(tmp_path, schema=schema, chapters=chapters)
    d8 = [f for f in findings(root) if ": D8: " in f]
    assert len(d8) == 2
    assert all("shutdown.total_timeout (90s)" in f for f in d8)


def test_total_timeout_missing_fails(tmp_path: Path) -> None:
    chapters = {
        "02-architecture": CHAPTER_02.replace(
            "| `shutdown.total_timeout` | Server | `60s` | total deadline |\n", ""
        ),
        "16-operations": CHAPTER_16,
    }
    root = write_repo(
        tmp_path,
        chapters=chapters,
        planned=PLANNED.replace('  - match: "shutdown."\n    milestone: M1\n', "").replace(
            '  - match: "agent."\n    milestone: M2\n', ""
        ),
    )
    assert (
        "config-doc: infra/compose/compose.yaml: D8: shutdown.total_timeout is neither in the"
        " schema nor in the table of 02-architecture.md"
    ) in findings(root)


def test_unreadable_total_timeout_fails(tmp_path: Path) -> None:
    chapters = {
        "02-architecture": CHAPTER_02.replace("`60s`", "`1m30s`"),
        "09-agents": CHAPTER_09,
        "16-operations": CHAPTER_16,
    }
    root = write_repo(tmp_path, chapters=chapters)
    assert any(
        f.endswith("D8: cannot read shutdown.total_timeout '1m30s' (want a duration such as 60s)")
        for f in findings(root)
    )


def test_no_service_runs_serve_fails(tmp_path: Path) -> None:
    compose = 'services:\n  migrate:\n    command: ["migrate", "up"]\n'
    root = write_repo(tmp_path, compose=compose)
    assert (
        "config-doc: infra/compose/compose.yaml: D8: no service runs `serve`; D8 would hold"
        " nothing (a service must run serve or bothy serve)"
    ) in findings(root)


def test_huge_durations_are_unreadable(tmp_path: Path) -> None:
    compose = COMPOSE.replace("stop_grace_period: 75s", "stop_grace_period: 99999999999h", 1)
    root = write_repo(tmp_path, compose=compose)
    assert "cannot read stop_grace_period '99999999999h'" in " ".join(findings(root))
    chapters = {
        "02-architecture": CHAPTER_02.replace("`60s`", "`99999999999d`"),
        "09-agents": CHAPTER_09,
        "16-operations": CHAPTER_16,
    }
    root = write_repo(tmp_path / "b", chapters=chapters)
    assert "cannot read shutdown.total_timeout '99999999999d'" in " ".join(findings(root))


@pytest.mark.parametrize(
    "form",
    [
        '  entrypoint: ["/usr/local/bin/bothy"]\n  command: ["serve", "--role=api"]\n',
        '  command: ["/usr/local/bin/bothy", "serve", "--role=api"]\n',
        '  entrypoint: ["bothy", "serve"]\n  command: ["--role=api"]\n',
    ],
)
def test_bothy_serve_behind_entrypoint_or_binary_is_followed(tmp_path: Path, form: str) -> None:
    compose = COMPOSE.replace(
        '  command: ["serve", "--role=api"]\n  stop_grace_period: 75s\n', form
    )
    root = write_repo(tmp_path, compose=compose)
    assert "service api runs `serve` without stop_grace_period" in " ".join(findings(root))


def test_other_programs_serve_is_not_the_server(tmp_path: Path) -> None:
    compose = COMPOSE + '  docs:\n    command: ["mkdocs", "serve"]\n'
    root = written(write_repo(tmp_path, compose=compose))
    assert findings(root) == []


@pytest.mark.parametrize(
    ("environment", "message"),
    [
        (
            "    environment:\n      BOTHY_SHUTDOWN__TOTAL_TIMEOUT: 90s\n",
            "service worker: stop_grace_period 75s is not longer than shutdown.total_timeout (90s,"
            " BOTHY_SHUTDOWN__TOTAL_TIMEOUT of the service)",
        ),
        (
            '    environment: ["BOTHY_SHUTDOWN__TOTAL_TIMEOUT=2m"]\n',
            "service worker: stop_grace_period 75s is not longer than shutdown.total_timeout (2m,"
            " BOTHY_SHUTDOWN__TOTAL_TIMEOUT of the service)",
        ),
        (
            "    environment:\n      BOTHY_SHUTDOWN__TOTAL_TIMEOUT: ${T:-60s}\n",
            "service worker: cannot read BOTHY_SHUTDOWN__TOTAL_TIMEOUT '${T:-60s}' (want a duration"
            " such as 60s)",
        ),
    ],
)
def test_total_timeout_set_by_compose_is_used(
    tmp_path: Path, environment: str, message: str
) -> None:
    compose = COMPOSE.replace(
        '    command: ["serve", "--role=worker"]\n',
        '    command: ["serve", "--role=worker"]\n' + environment,
    )
    root = write_repo(tmp_path, compose=compose)
    assert [f.split(": D8: ")[1] for f in findings(root) if ": D8: " in f] == [message]


def test_non_string_service_key_does_not_crash(tmp_path: Path) -> None:
    root = written(
        write_repo(
            tmp_path, compose=COMPOSE + "  1:\n    command: [serve]\n    stop_grace_period: 75s\n"
        )
    )
    assert findings(root) == []


def test_empty_total_timeout_in_environment_mapping_is_unreadable(tmp_path: Path) -> None:
    compose = COMPOSE.replace(
        '    command: ["serve", "--role=worker"]\n',
        '    command: ["serve", "--role=worker"]\n    environment:\n'
        "      BOTHY_SHUTDOWN__TOTAL_TIMEOUT:\n",
    )
    root = write_repo(tmp_path, compose=compose)
    assert [f.split(": D8: ")[1] for f in findings(root) if ": D8: " in f] == [
        "service worker: cannot read BOTHY_SHUTDOWN__TOTAL_TIMEOUT '' (want a duration such as 60s)"
    ]


def test_compose_without_services_fails(tmp_path: Path) -> None:
    root = write_repo(tmp_path, compose="name: bothy\n")
    assert "config-doc: infra/compose/compose.yaml: D8: no services" in " ".join(findings(root))


# ---- D9, D10 and rendering ---------------------------------------------------------------


def test_env_name_collision_fails(tmp_path: Path) -> None:
    clash = (
        CHAPTER_16
        + "| `server.role__x` | `a` | Collision |\n| `server.role.x` | `b` | Collision |\n"
    )
    chapters = {"02-architecture": CHAPTER_02, "09-agents": CHAPTER_09, "16-operations": clash}
    root = write_repo(tmp_path, chapters=chapters)
    assert (
        "config-doc: docs/spec/16-operations.md:12: D9: BOTHY_SERVER__ROLE__X is the environment"
        " variable of server.role__x and server.role.x"
    ) in findings(root)


def test_schema_env_differs_from_name_rule_fails(tmp_path: Path) -> None:
    schema = [{**SCHEMA[0], "env": "BOTHY_HTTP_MAX_JSON_BODY_BYTES"}, SCHEMA[1]]
    root = write_repo(tmp_path, schema=schema)
    [line] = [f for f in findings(root) if ": D9: " in f]
    assert line.endswith(
        "D9: http.max_json_body_bytes: the schema names BOTHY_HTTP_MAX_JSON_BODY_BYTES, the name"
        " rule gives BOTHY_HTTP__MAX_JSON_BODY_BYTES"
    )


def test_check_detects_stale_reference(tmp_path: Path) -> None:
    root = written(write_repo(tmp_path))
    ref = root / "docs/operations/configuration.md"
    ref.write_text(
        ref.read_text(encoding="utf-8").replace("| Sum |", "| Sum (old) |"), encoding="utf-8"
    )
    assert findings(root) == [
        "config-doc: docs/operations/configuration.md:7: D10: section 4 is stale; run"
        " `make config-reference`"
    ]


def test_render_is_deterministic(tmp_path: Path) -> None:
    root = write_repo(tmp_path)
    first = configdoc.evaluate(configdoc.Paths(root)).rendered
    reordered = [
        key("server.role", "16-operations.md", "all", kind="enum", enum=["api", "worker", "all"]),
        SCHEMA[0],
    ]
    (root / "services/server/architecture/config-schema.json").write_text(
        json.dumps({"version": 1, "keys": reordered}, indent=4) + "\n", encoding="utf-8"
    )
    assert configdoc.evaluate(configdoc.Paths(root)).rendered == first
    assert configdoc.evaluate(configdoc.Paths(root)).rendered == first


def test_perf_keys_have_no_env_name(tmp_path: Path) -> None:
    out = configdoc.evaluate(configdoc.Paths(write_repo(tmp_path))).rendered
    assert "| `perf.message_create_p99` |  | Load test |  | `150ms` | Target |  |" in out
    assert "BOTHY_PERF__" not in out


# ---- input errors (exit 2) ---------------------------------------------------------------


@pytest.mark.parametrize(
    "rel",
    [
        "services/server/architecture/config-schema.json",
        "tools/configdoc/planned.yaml",
        "tools/configdoc/retired.yaml",
        "infra/compose/compose.yaml",
        "docs/operations/configuration.md",
    ],
)
def test_missing_file_exits_2(tmp_path: Path, rel: str, capsys: pytest.CaptureFixture[str]) -> None:
    root = write_repo(tmp_path)
    (root / rel).unlink()
    assert configdoc.main(["--root", str(root), "--check"]) == 2
    assert rel in capsys.readouterr().err


def test_missing_spec_directory_exits_2(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    root = write_repo(tmp_path, chapters={})
    (root / "docs/spec").rmdir()
    assert configdoc.main(["--root", str(root), "--check"]) == 2
    assert "docs/spec" in capsys.readouterr().err


@pytest.mark.parametrize(
    ("document", "message"),
    [
        ("[]", "an object with version and keys"),
        ('{"version": 2, "keys": []}', "version 2"),
        ('{"version": 1, "keys": {}}', "keys must be a list"),
        ('{"version": 1, "keys": [{"name": "a.b"}]}', "fields"),
        (
            '{"version": 1, "keys": [' + json.dumps({**key("a.b", "x"), "name": 7}) + "]}",
            "name must be",
        ),
        (
            '{"version": 1, "keys": ['
            + json.dumps(key("a.b", "02-architecture.md", required="no"))
            + "]}",
            "required",
        ),
        (
            '{"version": 1, "keys": ['
            + json.dumps(key("a.b", "02-architecture.md", enum=[1]))
            + "]}",
            "enum",
        ),
        (
            '{"version": 1, "keys": [' + json.dumps(key("a.b", "02-architecture.md", doc=3)) + "]}",
            "doc",
        ),
        (
            '{"version": 1, "keys": ['
            + json.dumps(key("a.b", "x"))
            + ","
            + json.dumps(key("a.b", "x"))
            + "]}",
            "twice",
        ),
        ("{not json", "not JSON"),
    ],
)
def test_malformed_schema_exits_2(
    tmp_path: Path, document: str, message: str, capsys: pytest.CaptureFixture[str]
) -> None:
    root = write_repo(tmp_path)
    (root / "services/server/architecture/config-schema.json").write_text(
        document, encoding="utf-8"
    )
    assert configdoc.main(["--root", str(root), "--check"]) == 2
    err = capsys.readouterr().err
    assert "config-doc: services/server/architecture/config-schema.json" in err
    assert message in err


@pytest.mark.parametrize(
    ("text", "message"),
    [
        ("- a\n", "a mapping with the key rules"),
        ("rules: x\n", "rules must be a list"),
        ("rules:\n  - match: a.\n", "match and milestone"),
        ("rules:\n  - {match: a., milestone: M9}\n", "milestone 'M9'"),
        ("rules:\n  - {match: A.b, milestone: M1}\n", "match 'A.b'"),
        ("rules:\n  - {match: [a], milestone: M1}\n", "a string"),
        ("rules:\n  - {match: a., milestone: M1}\n  - {match: a., milestone: M2}\n", "twice"),
        ("rules: [\n", "not YAML"),
        ("rules:\n  - ? [x]\n    : 1\n    match: a.b\n", "match and milestone"),
        ("", "a mapping with the key rules"),
    ],
)
def test_malformed_planned_exits_2(
    tmp_path: Path, text: str, message: str, capsys: pytest.CaptureFixture[str]
) -> None:
    root = write_repo(tmp_path, planned=text)
    assert configdoc.main(["--root", str(root), "--check"]) == 2
    err = capsys.readouterr().err
    assert "config-doc: tools/configdoc/planned.yaml" in err
    assert message in err


@pytest.mark.parametrize(
    ("text", "message"),
    [
        ("retired:\n  - {name: http., replaced_by: x}\n", "name 'http.'"),
        ("retired:\n  - {name: http.x}\n", "name and replaced_by"),
        ("retired:\n  - {name: a.b, replaced_by: x}\n  - {name: a.b, replaced_by: y}\n", "twice"),
        ("other: []\n", "a mapping with the key retired"),
    ],
)
def test_malformed_retired_exits_2(
    tmp_path: Path, text: str, message: str, capsys: pytest.CaptureFixture[str]
) -> None:
    root = write_repo(tmp_path, retired=text)
    assert configdoc.main(["--root", str(root), "--check"]) == 2
    assert message in capsys.readouterr().err


def test_empty_retired_list_finds_nothing(tmp_path: Path) -> None:
    root = written(
        write_repo(tmp_path, retired="retired: []\n", extra={"docs/x.md": "`http.listen_addr`\n"})
    )
    assert findings(root) == []


def test_compose_merge_error_exits_2(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    root = write_repo(tmp_path, compose="services:\n  api:\n    <<: 5\n")
    assert configdoc.main(["--root", str(root), "--check"]) == 2
    assert "config-doc: infra/compose/compose.yaml: not YAML" in capsys.readouterr().err


def test_service_without_mapping_is_skipped(tmp_path: Path) -> None:
    root = written(write_repo(tmp_path, compose=COMPOSE + "  sidecar:\n"))
    assert findings(root) == []


def test_services_from_root_merge_key_have_no_line(tmp_path: Path) -> None:
    compose = (
        "x-all: &all\n  services:\n    worker:\n      command: [serve]\n<<: *all\nname: bothy\n"
    )
    root = write_repo(tmp_path, compose=compose)
    assert (
        "config-doc: infra/compose/compose.yaml: D8: service worker runs `serve` without"
        " stop_grace_period; set it longer than shutdown.total_timeout (60s)"
    ) in findings(root)


def test_malformed_compose_exits_2(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    root = write_repo(tmp_path, compose="services: [\n")
    assert configdoc.main(["--root", str(root), "--check"]) == 2
    assert "config-doc: infra/compose/compose.yaml: not YAML" in capsys.readouterr().err


def test_finding_without_line_has_no_line_part() -> None:
    assert str(configdoc.Finding("a.md", 0, "D2", "m")) == "config-doc: a.md: D2: m"
    assert str(configdoc.Finding("a.md", 3, "D2", "m")) == "config-doc: a.md:3: D2: m"


# ---- the real repository -----------------------------------------------------------------


def test_real_repository_has_no_duplicates_and_no_retired_names() -> None:
    """The counters PLAN M1-08 asks for: zero duplicate names and zero retired synonyms of W1."""
    chapters, problems = configdoc.collect(REPO / "docs" / "spec")
    assert problems == []
    duplicates = configdoc.check_names(chapters)
    retired = configdoc.load_retired(
        REPO / "tools" / "configdoc" / "retired.yaml", "tools/configdoc/retired.yaml"
    )
    occurrences = configdoc.check_retired(REPO, retired)
    assert (len(duplicates), len(occurrences)) == (0, 0), [str(f) for f in duplicates + occurrences]
    assert len(retired) == 15


def test_real_repository_d8_holds_every_server_service() -> None:
    """D8 sees the three services of infra/compose/compose.yaml that run the server."""
    import yaml

    doc = yaml.safe_load((REPO / "infra/compose/compose.yaml").read_text(encoding="utf-8"))
    assert configdoc.serve_services(doc["services"]) == ["api", "api-tailscale", "worker"]


def test_real_repository_check_passes(capsys: pytest.CaptureFixture[str]) -> None:
    """Same as `make config-doc-check`."""
    assert configdoc.main(["--check"]) == 0, capsys.readouterr().err


def test_real_reference_says_fences_end_a_paragraph_in_other_text_files() -> None:
    """The D7 row of section 10 states what code_parts does outside Markdown."""
    text = "# Value `x.y\n```\nk: v\n```\n# more` here\n"
    assert "x.y" not in "".join(code for _, code in configdoc.code_parts(text, markdown=False))
    reference = (REPO / "docs/operations/configuration.md").read_text(encoding="utf-8")
    [row] = [line for line in reference.splitlines() if line.startswith("| D7 |")]
    assert "in other text files only blank lines and fences end a paragraph" in row


def test_real_reference_names_the_exit_code_of_make() -> None:
    """Section 10 names the exit codes of the tool and the one an operator sees from make."""
    reference = (REPO / "docs/operations/configuration.md").read_text(encoding="utf-8")
    assert "`make` itself ends with exit code 2 on every failure of a target" in reference
