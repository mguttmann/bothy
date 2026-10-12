# SPDX-License-Identifier: Apache-2.0
"""Gate G4 for Python (docs/spec/06-api.md 15): the checked-in core imports completely and has a
function for every operation of docs/api/openapi.yaml. `make generate-check` proves that it
equals a fresh generation and type checks."""

from __future__ import annotations

import importlib
import pkgutil
import tomllib
from types import ModuleType

from conftest import REPO, spec
from openapi_python_client.strings import snake_case

import bothy_sdk
from bothy_sdk import generated
from bothy_sdk.generated import api

_METHODS = ("get", "put", "post", "delete", "options", "head", "patch", "trace")


def _walk(package: ModuleType) -> list[str]:
    return [info.name for info in pkgutil.walk_packages(package.__path__, package.__name__ + ".")]


def test_g4_every_generated_module_imports() -> None:
    names = _walk(generated)
    assert len(names) > 500
    for name in names:
        importlib.import_module(name)


def test_g4_every_operation_is_generated() -> None:
    operations = {
        op["operationId"]
        for item in spec()["paths"].values()
        for method, op in item.items()
        if method in _METHODS
    }
    assert len(operations) > 200
    modules: dict[str, ModuleType] = {}
    for name in _walk(api):
        modules[name.rsplit(".", 1)[1]] = importlib.import_module(name)
    missing = []
    for operation in sorted(operations):
        module = modules.get(snake_case(operation))
        if module is None or not all(
            callable(getattr(module, f, None)) for f in ("sync_detailed", "asyncio_detailed")
        ):
            missing.append(operation)
    assert missing == []


def test_version_matches_version_file() -> None:
    root = REPO / "sdk" / "python"
    project = tomllib.loads((root / "pyproject.toml").read_text(encoding="utf-8"))["project"]
    version = (root / "VERSION").read_text(encoding="utf-8").strip()
    assert project["version"] == version
    assert bothy_sdk.__version__ == version
