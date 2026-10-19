# SPDX-License-Identifier: Apache-2.0
"""Tests for tools/catalog/contract_documents_dart.py, the embedded API documents of the fake."""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path

import pytest

_SPEC = importlib.util.spec_from_file_location(
    "contract_documents_dart_tool",
    Path(__file__).resolve().parents[1] / "contract_documents_dart.py",
)
assert _SPEC is not None
assert _SPEC.loader is not None
gen = importlib.util.module_from_spec(_SPEC)
sys.modules[_SPEC.name] = gen
_SPEC.loader.exec_module(gen)

REPO = Path(__file__).resolve().parents[3]

OPENAPI = "openapi: 3.1.0\ninfo: {title: t, version: '1'}\npaths: {}\n" + "x: ü\n" * 200
ASYNCAPI = "asyncapi: 3.0.0\ninfo: {title: t, version: '1'}\n"


def _repo(tmp_path: Path, openapi: str = OPENAPI, asyncapi: str = ASYNCAPI) -> Path:
    (tmp_path / "docs/api").mkdir(parents=True)
    (tmp_path / gen.OPENAPI).write_text(openapi, encoding="utf-8")
    (tmp_path / gen.ASYNCAPI).write_text(asyncapi, encoding="utf-8")
    return tmp_path


def test_render_round_trips_both_documents() -> None:
    text = gen.render(OPENAPI, ASYNCAPI)
    assert "part of 'schema_validator.dart';" in text
    assert gen.embedded(text) == (OPENAPI, ASYNCAPI)
    assert all(len(line) <= 100 for line in text.splitlines())


def test_embedded_rejects_a_file_without_both_lists() -> None:
    with pytest.raises(gen.EmbeddedError, match="_openapiGzip"):
        gen.embedded("// nothing\n")


def test_main_check_fails_when_stale(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    root = _repo(tmp_path)
    assert gen.main(["--root", str(root), "--check"]) == 1
    assert "is stale" in capsys.readouterr().err
    assert gen.main(["--root", str(root)]) == 0
    assert gen.main(["--root", str(root), "--check"]) == 0
    (root / gen.ASYNCAPI).write_text(ASYNCAPI + "y: 1\n", encoding="utf-8")
    assert gen.main(["--root", str(root), "--check"]) == 1
    target = root / gen.OUTPUT
    target.write_text("broken", encoding="utf-8")
    assert gen.main(["--root", str(root), "--check"]) == 1


def test_main_reports_errors(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    assert gen.main(["--root", str(tmp_path)]) == 1
    assert "contract-documents:" in capsys.readouterr().err
    assert gen.main(["--bogus"]) == 2


def test_repository_file_is_current() -> None:
    assert gen.main(["--root", str(REPO), "--check"]) == 0
