# SPDX-License-Identifier: Apache-2.0
"""Tests for tools/configdoc/app_config.py, the generator of the `AppConfig` key table."""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path

import pytest

_SPEC = importlib.util.spec_from_file_location(
    "app_config_tool", Path(__file__).resolve().parents[1] / "app_config.py"
)
assert _SPEC is not None
assert _SPEC.loader is not None
app_config = importlib.util.module_from_spec(_SPEC)
sys.modules[_SPEC.name] = app_config
_SPEC.loader.exec_module(app_config)

REPO = Path(__file__).resolve().parents[3]

HEADER = (
    "| Key | Environment | Place | Type | Default | Meaning | Status |\n"
    "|---|---|---|---|---|---|---|\n"
)


def _server_rows(skip: str = "", place: str = "Server") -> str:
    lines = []
    for key in app_config.SERVER_KEYS_READ_BY_APP:
        if key == skip:
            continue
        env = "BOTHY_" + key.upper().replace(".", "__")
        lines.append(f"| `{key}` | `{env}` | {place} |  | `5s` | meaning | planned (M1) |")
    return "\n".join(lines) + "\n"


def _reference(app_rows: str, server: str | None = None) -> str:
    return (
        "# Reference\n\n## 4. Named values\n\n"
        + app_config.START
        + "\n### 01 Product\n\n"
        + HEADER
        + app_rows
        + (server if server is not None else _server_rows())
        + app_config.END
        + "\n"
    )


APP_ROWS = (
    "| `ui.message_group_window` |  | App |  | `5m` | grouping |  |\n"
    "| `client.voice.bitrate` |  | App |  | `64kbps` | bitrate |  |\n"
    "| `viz.timeline_speeds` |  | App |  | `60, 300, 900` | speeds |  |\n"
    "| `search.extract.timeout` | `BOTHY_SEARCH__EXTRACT__TIMEOUT` | Server |  | `60s` | x | |\n"
)


def _repo(tmp_path: Path, reference: str) -> Path:
    (tmp_path / "docs/operations").mkdir(parents=True)
    (tmp_path / app_config.REFERENCE).write_text(reference, encoding="utf-8")
    return tmp_path


@pytest.mark.parametrize(
    ("default", "kind"),
    [
        ("true", "boolean"),
        ("150ms", "duration"),
        ("30d", "duration"),
        ("1GiB", "bytes"),
        ("1MiB/s", "byteRate"),
        ("64kbps", "bitRate"),
        ("1280px", "pixels"),
        ("262144", "integer"),
        ("0.01", "decimal"),
        ("60, 300, 900", "list"),
        ("leer", None),
        ("1h30m", None),
    ],
)
def test_kind_of(default: str, kind: str | None) -> None:
    assert app_config.kind_of(default) == kind


def test_getter_name_drops_the_first_segment() -> None:
    assert app_config.getter_name("client.outbox_stuck_after") == "outboxStuckAfter"
    assert app_config.getter_name("ui.quick_switch_max_results") == "quickSwitchMaxResults"


def test_entries_take_app_rows_and_listed_server_keys() -> None:
    table = app_config.entries(_reference(APP_ROWS))
    keys = [e.key for e in table]
    assert keys == sorted(keys)
    assert "ui.message_group_window" in keys
    assert "search.extract.timeout" not in keys
    assert set(app_config.SERVER_KEYS_READ_BY_APP) <= set(keys)
    voice = next(e for e in table if e.key == "client.voice.bitrate")
    assert (voice.place, voice.kind, voice.default) == ("App", "bitRate", "64kbps")


def test_listed_server_key_missing_fails() -> None:
    reference = _reference(APP_ROWS, _server_rows(skip="sync.typing_ttl"))
    with pytest.raises(app_config.ReferenceError, match=r"sync\.typing_ttl"):
        app_config.entries(reference)


def test_listed_server_key_with_another_place_fails() -> None:
    with pytest.raises(app_config.ReferenceError, match="place Server"):
        app_config.entries(_reference(APP_ROWS, _server_rows(place="App")))


def test_default_without_kind_fails() -> None:
    rows = "| `ui.odd_value` |  | App |  | `1h30m` | odd |  |\n"
    with pytest.raises(app_config.ReferenceError, match=r"ui\.odd_value"):
        app_config.entries(_reference(rows))


def test_getter_collision_fails() -> None:
    rows = (
        "| `ui.same_name` |  | App |  | `1` | a |  |\n"
        "| `viz.same_name` |  | App |  | `2` | b |  |\n"
    )
    with pytest.raises(app_config.ReferenceError, match="getter sameName"):
        app_config.entries(_reference(rows))


def test_duplicate_key_and_missing_markers_fail() -> None:
    rows = "| `ui.a_b` |  | App |  | `1` | a |  |\n| `ui.a_b` |  | App |  | `1` | a |  |\n"
    with pytest.raises(app_config.ReferenceError, match="twice"):
        app_config.entries(_reference(rows))
    with pytest.raises(app_config.ReferenceError, match="markers"):
        app_config.rows("no block here")
    with pytest.raises(app_config.ReferenceError, match="short row"):
        app_config.rows(app_config.START + "\n| `a.b` | x |\n" + app_config.END)


def test_invalid_key_fails() -> None:
    rows = "| `UI.Bad` |  | App |  | `1` | a |  |\n"
    with pytest.raises(app_config.ReferenceError, match="not a valid key"):
        app_config.entries(_reference(rows))


def test_render_is_deterministic_and_escapes() -> None:
    table = app_config.entries(_reference(APP_ROWS))
    first = app_config.render(table)
    assert first == app_config.render(app_config.entries(_reference(APP_ROWS)))
    assert "part of 'app_config.dart';" in first
    assert "Duration get messageGroupWindow => value<Duration>('ui.message_group_window');" in first
    assert "List<String> get timelineSpeeds" in first
    assert app_config._dart_string("a'b$c\\") == "'a\\'b\\$c\\\\'"


def test_main_writes_then_check_passes_and_fails_when_stale(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    root = _repo(tmp_path, _reference(APP_ROWS))
    assert app_config.main(["--root", str(root), "--check"]) == 1
    assert "is stale" in capsys.readouterr().err
    assert app_config.main(["--root", str(root)]) == 0
    assert app_config.main(["--root", str(root), "--check"]) == 0
    target = root / app_config.OUTPUT
    target.write_text(target.read_text(encoding="utf-8") + "// hand edit\n", encoding="utf-8")
    assert app_config.main(["--root", str(root), "--check"]) == 1


def test_main_reports_an_invalid_reference(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    root = _repo(tmp_path, "nothing")
    assert app_config.main(["--root", str(root)]) == 1
    assert "markers" in capsys.readouterr().err
    assert app_config.main(["--bogus"]) == 2


def test_repository_file_is_current() -> None:
    assert app_config.main(["--root", str(REPO), "--check"]) == 0


def test_repository_reference_holds_every_listed_key() -> None:
    reference = (REPO / app_config.REFERENCE).read_text(encoding="utf-8")
    table = {e.key: e for e in app_config.entries(reference)}
    for key in app_config.SERVER_KEYS_READ_BY_APP:
        assert table[key].place == "Server"
    assert table["client.outbox_stuck_after"].default == "2m"
