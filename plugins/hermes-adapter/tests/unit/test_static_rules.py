# SPDX-License-Identifier: Apache-2.0
"""Static rules of the package (09 17.2; Design note 1; ADR 0022)."""

from __future__ import annotations

import ast
from pathlib import Path

from hermes_modules import HERMES_TOP_LEVEL

import bothy_hermes_adapter

PACKAGE = Path(bothy_hermes_adapter.__file__).resolve().parent
COMPONENT = PACKAGE.parents[1]
HERMES_IMPORTERS = {"platform.py", "hermes_config.py"}


def _modules() -> list[Path]:
    return sorted(PACKAGE.glob("*.py"))


def _imported_roots(tree: ast.AST) -> set[str]:
    roots: set[str] = set()
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            roots.update(alias.name.split(".", 1)[0] for alias in node.names)
        elif isinstance(node, ast.ImportFrom) and node.level == 0 and node.module:
            roots.add(node.module.split(".", 1)[0])
    return roots


def test_only_two_modules_import_hermes() -> None:
    importers = {
        path.name
        for path in _modules()
        if _imported_roots(ast.parse(path.read_text(encoding="utf-8"))) & HERMES_TOP_LEVEL
    }
    assert importers == HERMES_IMPORTERS


def test_no_environment_access() -> None:
    offenders = []
    for path in _modules():
        for node in ast.walk(ast.parse(path.read_text(encoding="utf-8"))):
            if isinstance(node, ast.Attribute) and node.attr in ("getenv", "environ", "putenv"):
                offenders.append(f"{path.name}:{node.lineno}")
            if isinstance(node, ast.ImportFrom) and node.module == "os":
                names = {a.name for a in node.names}
                if names & {"getenv", "environ", "putenv"}:
                    offenders.append(f"{path.name}:{node.lineno}")
    assert offenders == []


def test_no_module_starts_a_process() -> None:
    for path in _modules():
        roots = _imported_roots(ast.parse(path.read_text(encoding="utf-8")))
        assert "subprocess" not in roots, path.name


def test_version_matches_version_file() -> None:
    version = (COMPONENT / "VERSION").read_text(encoding="utf-8").strip()
    assert bothy_hermes_adapter.__version__ == version


def test_every_module_carries_the_spdx_header() -> None:
    for path in [*_modules(), *sorted((COMPONENT / "typings").rglob("*.pyi"))]:
        first = path.read_text(encoding="utf-8").splitlines()[0]
        assert first == "# SPDX-License-Identifier: Apache-2.0", path.name
