# SPDX-License-Identifier: Apache-2.0
"""Tests for the generated configuration reference (docs/operations/configuration.md)."""

from __future__ import annotations

from pathlib import Path

import config_reference
import pytest

REPO = Path(__file__).resolve().parents[3]

CHAPTER_A = """# 04 Sync

## 18. Configuration values of this chapter

| Name | Default | Meaning |
|---|---|---|
| `sync.event_retention` | `30d` | events stay available for catch-up this long |
| `ratelimit.rl.read.per_minute` / `.burst` | `600` / `100` | per principal |
| `file.s3.access_key_ref`, `file.s3.secret_key_ref` | empty | references |

## 19. After

| `not.a_value` | `1` | not part of the table |
"""

CHAPTER_B = """# 09 Agents

## 21. Configuration values of this chapter

| Name | Place | Default | Meaning |
|---|---|---|---|
| `agent.context_max_bytes` | Server | `262144` | sum |
| `adapter.invocation_token_leeway` | Adapter | `10s` | clock tolerance |
"""

REFERENCE = """# Configuration reference

Intro stays.

<!-- config-reference:start -->
old
<!-- config-reference:end -->

Footer stays.
"""


def _spec(tmp_path: Path, **chapters: str) -> Path:
    d = tmp_path / "spec"
    d.mkdir()
    for name, text in chapters.items():
        (d / f"{name}.md").write_text(text, encoding="utf-8")
    return d


def test_parse_expands_shorthand_and_lists(tmp_path: Path) -> None:
    spec = _spec(tmp_path, **{"04-sync-protocol": CHAPTER_A})
    [chapter] = config_reference.collect(spec)
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


def test_place_column_and_environment_name(tmp_path: Path) -> None:
    spec = _spec(tmp_path, **{"09-agents": CHAPTER_B})
    [chapter] = config_reference.collect(spec)
    server, adapter = chapter.values
    assert (server.place, server.env) == ("Server", "BOTHY_AGENT__CONTEXT_MAX_BYTES")
    assert (adapter.place, adapter.env) == ("Adapter", "")


def test_default_place_per_chapter(tmp_path: Path) -> None:
    text = CHAPTER_A.replace("# 04 Sync", "# 22 Import")
    spec = _spec(tmp_path, **{"22-import-migration": text})
    [chapter] = config_reference.collect(spec)
    assert {v.place for v in chapter.values} == {"Tool `bothy import`"}
    assert {v.env for v in chapter.values} == {""}


def test_excluded_chapter_is_skipped(tmp_path: Path) -> None:
    spec = _spec(tmp_path, **{"23-quality": CHAPTER_A})
    assert config_reference.collect(spec) == []


def test_duplicate_name_fails(tmp_path: Path) -> None:
    spec = _spec(tmp_path, **{"04-sync-protocol": CHAPTER_A, "05-actions": CHAPTER_A})
    with pytest.raises(config_reference.ConfigError, match=r"sync\.event_retention"):
        config_reference.collect(spec)


def test_bad_name_fails(tmp_path: Path) -> None:
    bad = CHAPTER_A.replace("`sync.event_retention`", "`Sync.Event`")
    spec = _spec(tmp_path, **{"04-sync-protocol": bad})
    with pytest.raises(config_reference.ConfigError, match="no value name"):
        config_reference.collect(spec)


def test_split_default_count_must_match(tmp_path: Path) -> None:
    bad = CHAPTER_A.replace("`600` / `100`", "`600` / `100` / `7`")
    spec = _spec(tmp_path, **{"04-sync-protocol": bad})
    with pytest.raises(config_reference.ConfigError, match="defaults"):
        config_reference.collect(spec)


def test_render_replaces_only_the_marked_block(tmp_path: Path) -> None:
    spec = _spec(tmp_path, **{"04-sync-protocol": CHAPTER_A, "09-agents": CHAPTER_B})
    out = config_reference.render(REFERENCE, config_reference.collect(spec))
    assert out.startswith("# Configuration reference\n\nIntro stays.\n")
    assert out.endswith("Footer stays.\n")
    assert "old" not in out
    assert "### 04 Sync\n" in out
    assert "Source: [04-sync-protocol.md](../spec/04-sync-protocol.md), section 18." in out
    assert "| Key | Environment | Place | Default | Meaning |\n|---|---|---|---|---|\n" in out
    assert (
        "| `ratelimit.rl.read.burst` | `BOTHY_RATELIMIT__RL__READ__BURST` | Server | `100` |" in out
    )
    assert "| `adapter.invocation_token_leeway` |  | Adapter | `10s` | clock tolerance |" in out


def test_render_rewrites_chapter_relative_links(tmp_path: Path) -> None:
    linked = CHAPTER_A.replace(
        "| events stay available for catch-up this long |",
        "| see [19](19-security.md#x), [ADR](../adr/README.md), [W](https://example.com) |",
    )
    spec = _spec(tmp_path, **{"04-sync-protocol": linked})
    out = config_reference.render(REFERENCE, config_reference.collect(spec))
    assert "[19](../spec/19-security.md#x)" in out
    assert "[ADR](../adr/README.md)" in out
    assert "[W](https://example.com)" in out


def test_render_needs_markers() -> None:
    with pytest.raises(config_reference.ConfigError, match="markers"):
        config_reference.render("no markers", [])


def test_main_writes_then_check_passes(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    spec = _spec(tmp_path, **{"04-sync-protocol": CHAPTER_A})
    ref = tmp_path / "configuration.md"
    ref.write_text(REFERENCE, encoding="utf-8")
    args = ["--spec", str(spec), "--reference", str(ref)]
    assert config_reference.main([*args, "--check"]) == 1
    assert "stale" in capsys.readouterr().err
    assert config_reference.main(args) == 0
    assert config_reference.main([*args, "--check"]) == 0


def test_main_reports_errors(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    spec = _spec(tmp_path, **{"04-sync-protocol": CHAPTER_A, "05-actions": CHAPTER_A})
    ref = tmp_path / "configuration.md"
    ref.write_text(REFERENCE, encoding="utf-8")
    assert config_reference.main(["--spec", str(spec), "--reference", str(ref)]) == 1
    assert "twice" in capsys.readouterr().err


def test_repository_reference_is_current() -> None:
    """The committed reference equals the generated one (same as `make docs`)."""
    assert config_reference.main(["--check"]) == 0


def _value_rows_outside_chapters(name: str, text: str) -> list[str]:
    """Rows `| `a.b` | ...` under a heading "Configuration values" (until the next heading)."""
    import re

    section = re.compile(r"^#+ [^\n]*Configuration values.*?(?=^#|\Z)", re.MULTILINE | re.DOTALL)
    row = re.compile(r"^\| `[a-z][a-z0-9_]*(?:\.[a-z0-9_]+)+`", re.MULTILINE)
    return [name for block in section.findall(text) if row.search(block)]


def test_value_table_outside_chapters_is_found() -> None:
    table = "## 9. Configuration values\n\n| Name | Default |\n|---|---|\n| `webhook.x` | `1` |\n"
    pointer = "## 9. Configuration values\n\nSee 11.\n\n## 10. Rest\n\n| `a.b` | 1 |\n"
    assert _value_rows_outside_chapters("x.md", table) == ["x.md"]
    assert _value_rows_outside_chapters("x.md", pointer) == []


def test_value_tables_live_only_in_chapters() -> None:
    """Ruling L4: a value table outside docs/spec/ never reaches the reference (L-11)."""
    import subprocess

    files = subprocess.run(
        ["git", "ls-files", "--", "*.md"], cwd=REPO, check=True, capture_output=True, text=True
    ).stdout.split()
    outside: list[str] = []
    for name in files:
        if name.startswith("docs/spec/") or name == "docs/operations/configuration.md":
            continue
        outside += _value_rows_outside_chapters(name, (REPO / name).read_text(encoding="utf-8"))
    assert outside == []
