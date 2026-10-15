# SPDX-License-Identifier: Apache-2.0
"""Tests for tools/catalog/event_catalog_dart.py, the generator of the client event catalogue."""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path

import pytest
import yaml

_SPEC = importlib.util.spec_from_file_location(
    "event_catalog_dart_tool", Path(__file__).resolve().parents[1] / "event_catalog_dart.py"
)
assert _SPEC is not None
assert _SPEC.loader is not None
gen = importlib.util.module_from_spec(_SPEC)
sys.modules[_SPEC.name] = gen
_SPEC.loader.exec_module(gen)

REPO = Path(__file__).resolve().parents[3]

CATALOGUE = """
version: 1
events:
- type: message.created
  kind: durable
  resource: message
  resource_version: true
- type: read_state.updated
  kind: durable
  resource: thread
  audience: self
  resource_version: false
- type: notification.created
  kind: ephemeral
  resource: thread; for a hint without a thread the recipient (`user`)
  audience: self
  resource_version: false
"""


def _repo(tmp_path: Path, text: str) -> Path:
    (tmp_path / "docs/spec").mkdir(parents=True)
    (tmp_path / gen.CATALOGUE).write_text(text, encoding="utf-8")
    return tmp_path


def test_events_reads_flags_sorted() -> None:
    table = gen.events(CATALOGUE)
    assert [e.type for e in table] == [
        "message.created",
        "notification.created",
        "read_state.updated",
    ]
    assert table[0] == gen.EventEntry("message.created", True, True, "message", False)
    assert table[1].resource_kind == "thread"
    assert table[1].durable is False
    assert table[2].personal is True


@pytest.mark.parametrize(
    ("text", "message"),
    [
        ("version: 1\n", "no `events:` list"),
        ("events: [3]\n", "not a mapping"),
        ("events: [{type: Bad}]\n", "invalid or missing type"),
        ("events: [{type: a.b, kind: x, resource: m, resource_version: true}]\n", "kind must"),
        ("events: [{type: a.b, kind: durable, resource: m, resource_version: 1}]\n", "true or"),
        ("events: [{type: a.b, kind: durable, resource_version: true}]\n", "resource is missing"),
        (
            "events: [{type: a.b, kind: durable, resource: m, resource_version: true},"
            " {type: a.b, kind: durable, resource: m, resource_version: true}]\n",
            "twice",
        ),
    ],
)
def test_events_rejects_invalid_entries(text: str, message: str) -> None:
    with pytest.raises(gen.CatalogueError, match=message):
        gen.events(text)


def test_render_is_deterministic() -> None:
    first = gen.render(gen.events(CATALOGUE))
    assert first == gen.render(gen.events(CATALOGUE))
    assert "part of 'event_catalog.dart';" in first
    assert (
        "'read_state.updated': EventInfo(type: 'read_state.updated', durable: true, "
        "carriesResourceVersion: false, resourceKind: 'thread', personal: true)," in first
    )


def test_main_check_fails_when_stale(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    root = _repo(tmp_path, CATALOGUE)
    assert gen.main(["--root", str(root), "--check"]) == 1
    assert "is stale" in capsys.readouterr().err
    assert gen.main(["--root", str(root)]) == 0
    assert gen.main(["--root", str(root), "--check"]) == 0
    target = root / gen.OUTPUT
    target.write_text(target.read_text(encoding="utf-8").replace("true", "false"), encoding="utf-8")
    assert gen.main(["--root", str(root), "--check"]) == 1


def test_main_reports_errors(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    root = _repo(tmp_path, "events: [\n")
    assert gen.main(["--root", str(root)]) == 1
    assert "event-catalog:" in capsys.readouterr().err
    assert gen.main(["--bogus"]) == 2


def test_repository_file_is_current() -> None:
    assert gen.main(["--root", str(REPO), "--check"]) == 0


def test_repository_catalogue_has_every_event() -> None:
    data = yaml.safe_load((REPO / gen.CATALOGUE).read_text(encoding="utf-8"))
    table = {e.type: e for e in gen.events((REPO / gen.CATALOGUE).read_text(encoding="utf-8"))}
    assert set(table) == {item["type"] for item in data["events"]}
    assert table["reaction.added"].carries_resource_version is False
    assert table["message.updated"].carries_resource_version is True
