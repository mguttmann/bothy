# SPDX-License-Identifier: Apache-2.0
"""Tests of the enable list reader."""

from __future__ import annotations

from pathlib import Path

import operations
import pytest

SPEC = {
    "paths": {
        "/me": {"get": {"operationId": "getMe"}, "parameters": []},
        "/spaces": {"get": {"operationId": "listSpaces"}, "post": {"operationId": "createSpace"}},
    }
}


def _write(tmp_path: Path, text: str) -> Path:
    path = tmp_path / "operations.yaml"
    path.write_text(text, encoding="utf-8")
    return path


def test_enabled_ids_are_returned_sorted(tmp_path: Path) -> None:
    path = _write(
        tmp_path,
        "version: 1\nenabled:\n  - listSpaces\n  - getMe\nlater:\n  - operation_id: createSpace\n",
    )
    assert operations.enabled_operations(path, SPEC) == ["getMe", "listSpaces"]


def test_unknown_id_fails(tmp_path: Path) -> None:
    path = _write(tmp_path, "enabled:\n  - getMe\n  - deleteEverything\n")
    with pytest.raises(operations.UnknownOperationError, match="deleteEverything"):
        operations.enabled_operations(path, SPEC)


def test_missing_list_fails(tmp_path: Path) -> None:
    with pytest.raises(ValueError, match="enabled"):
        operations.enabled_operations(_write(tmp_path, "later: []\n"), SPEC)


def test_spec_operation_ids_ignores_path_parameters() -> None:
    assert operations.spec_operation_ids(SPEC) == {"getMe", "listSpaces", "createSpace"}


def test_load_spec(tmp_path: Path) -> None:
    path = tmp_path / "openapi.yaml"
    path.write_text("openapi: 3.1.0\npaths: {}\n", encoding="utf-8")
    assert operations.load_spec(path)["openapi"] == "3.1.0"
    path.write_text("- a\n", encoding="utf-8")
    with pytest.raises(ValueError, match="not an OpenAPI"):
        operations.load_spec(path)
