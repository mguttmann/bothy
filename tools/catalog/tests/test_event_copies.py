# SPDX-License-Identifier: Apache-2.0
"""Tests for tools/catalog/event_copies.py, the check of the hand-kept event type lists.

The failure class (docs/DECISIONS.md D-M3-19-23): a durable event type enters
docs/spec/actions.yaml (added or ported from main) while a hand-kept list of the event types
does not follow. The tests run over the real repository files: every durable type removed from
every copy, and a new type added to the catalogue only, must be reported for that copy.
"""

from __future__ import annotations

import importlib.util
import shutil
import sys
from pathlib import Path

import pytest

_SPEC = importlib.util.spec_from_file_location(
    "event_copies_tool", Path(__file__).resolve().parents[1] / "event_copies.py"
)
assert _SPEC is not None
assert _SPEC.loader is not None
ec = importlib.util.module_from_spec(_SPEC)
sys.modules[_SPEC.name] = ec
_SPEC.loader.exec_module(ec)

REPO = Path(__file__).resolve().parents[3]


def _copy_repo(tmp_path: Path) -> Path:
    """Copy the catalogue and every hand-kept copy of the real repository into tmp_path."""
    for rel in (ec.CATALOGUE, ec.TS_TOLERANCE, ec.DART_CATALOGUE_TEST, ec.DART_README):
        (tmp_path / rel).parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(REPO / rel, tmp_path / rel)
    shutil.copytree(REPO / ec.DART_APPLIERS, tmp_path / ec.DART_APPLIERS)
    return tmp_path


def _durable() -> tuple[str, ...]:
    durable: tuple[str, ...] = ec.durable_types((REPO / ec.CATALOGUE).read_text(encoding="utf-8"))
    return durable


def _remove(root: Path, copy: str, event: str) -> None:
    """Remove event from the hand-kept copy named copy, as an incomplete port leaves it."""
    if copy == "ts":
        path = root / ec.TS_TOLERANCE
        text = path.read_text(encoding="utf-8")
        assert f'  "{event}",\n' in text
        path.write_text(text.replace(f'  "{event}",\n', "", 1), encoding="utf-8")
    elif copy == "readme":
        path = root / ec.DART_README
        text = path.read_text(encoding="utf-8")
        head, sep, tail = text.partition(ec.README_START)
        body, sep2, rest = tail.partition(ec.README_END)
        assert f"`{event}`" in body
        path.write_text(head + sep + body.replace(f"`{event}`", "", 1) + sep2 + rest, "utf-8")
    else:
        path = root / ec.DART_CATALOGUE_TEST
        text = path.read_text(encoding="utf-8")
        if f"  '{event}',\n" in text:
            path.write_text(text.replace(f"  '{event}',\n", "", 1), encoding="utf-8")
            return
        for applier in sorted((root / ec.DART_APPLIERS).glob("*.dart")):
            lines = applier.read_text(encoding="utf-8").splitlines(keepends=True)
            kept = [ln for ln in lines if not ln.lstrip().startswith(f"'{event}':")]
            if kept != lines:
                applier.write_text("".join(kept), encoding="utf-8")
                return
        raise AssertionError(f"{event} is neither skipped nor applied")


def test_repository_copies_are_complete() -> None:
    assert ec.check(REPO) == []


def test_repository_has_durable_types() -> None:
    durable = _durable()
    assert "device.pending" in durable
    assert "message.created" in durable
    assert "typing.started" not in durable


# The table of the whole class: every durable type of the catalogue, missing from every copy.
_TABLE = [
    (copy, event)
    for event in ec.durable_types((REPO / ec.CATALOGUE).read_text(encoding="utf-8"))
    for copy in ("ts", "dart")
]
_SKIPPED = ec.dart_skipped((REPO / ec.DART_CATALOGUE_TEST).read_text(encoding="utf-8"))
_TABLE += [("readme", event) for event in sorted(_SKIPPED)]


@pytest.mark.parametrize(("copy", "event"), _TABLE)
def test_every_missing_type_is_reported(tmp_path: Path, copy: str, event: str) -> None:
    root = _copy_repo(tmp_path)
    _remove(root, copy, event)
    problems = ec.check(root)
    assert problems, f"{copy} without {event} passed"
    assert all(event in p for p in problems), problems
    assert any(ec.COPY_NAMES[copy] in p for p in problems), problems


def test_type_added_to_catalogue_only_is_reported_for_every_copy(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    path = root / ec.CATALOGUE
    text = path.read_text(encoding="utf-8")
    entry = (
        "events:\n- type: widget.created\n  kind: durable\n  resource: widget\n"
        "  resource_version: false\n"
    )
    assert "\nevents:\n" in text
    path.write_text(text.replace("\nevents:\n", "\n" + entry, 1), encoding="utf-8")
    problems = ec.check(root)
    assert sorted(problems) == sorted(
        [
            f"{ec.COPY_NAMES['ts']}: missing widget.created",
            f"{ec.COPY_NAMES['dart']}: missing widget.created",
        ]
    )


def test_unknown_type_in_a_copy_is_reported(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    path = root / ec.TS_TOLERANCE
    text = path.read_text(encoding="utf-8")
    path.write_text(text.replace('  "device.pending",\n', '  "device.gone",\n', 1), "utf-8")
    assert sorted(ec.check(root)) == sorted(
        [
            f"{ec.COPY_NAMES['ts']}: missing device.pending",
            f"{ec.COPY_NAMES['ts']}: unknown device.gone",
        ]
    )


def test_type_both_applied_and_skipped_is_reported(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    path = root / ec.DART_CATALOGUE_TEST
    text = path.read_text(encoding="utf-8")
    extra = "  'device.pending',\n  'message.created',\n"
    path.write_text(text.replace("  'device.pending',\n", extra), "utf-8")
    problems = ec.check(root)
    assert f"{ec.COPY_NAMES['dart']}: applied and skipped message.created" in problems


# Regressions of M3-19: the two instances the red teams found on the device.pending port.


def test_regression_rt1_tolerance_without_device_pending(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _remove(root, "ts", "device.pending")
    assert ec.check(root) == [f"{ec.COPY_NAMES['ts']}: missing device.pending"]


def test_regression_rt2_catalogue_test_without_device_pending(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _remove(root, "dart", "device.pending")
    _remove(root, "readme", "device.pending")
    assert sorted(ec.check(root)) == sorted([f"{ec.COPY_NAMES['dart']}: missing device.pending"])


@pytest.mark.parametrize(
    ("rel", "content", "message"),
    [
        ("TS_TOLERANCE", "export const OTHER = [];\n", "no KNOWN_EVENT_TYPES"),
        ("DART_CATALOGUE_TEST", "void main() {}\n", "no skippedDurable"),
        ("DART_README", "# sync_engine\n", "no skipped list"),
        ("CATALOGUE", "version: 1\n", "no `events:` list"),
    ],
)
def test_unreadable_copies_are_errors(tmp_path: Path, rel: str, content: str, message: str) -> None:
    root = _copy_repo(tmp_path)
    (root / getattr(ec, rel)).write_text(content, encoding="utf-8")
    with pytest.raises(ec.CopyError, match=message):
        ec.check(root)


def test_main_exit_codes(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    root = _copy_repo(tmp_path)
    assert ec.main(["--root", str(root)]) == 0
    _remove(root, "ts", "device.pending")
    assert ec.main(["--root", str(root)]) == 1
    assert "missing device.pending" in capsys.readouterr().err
    (root / ec.CATALOGUE).write_text("version: 1\n", encoding="utf-8")
    assert ec.main(["--root", str(root)]) == 1
    assert "no `events:` list" in capsys.readouterr().err
    assert ec.main(["--bogus"]) == 2
