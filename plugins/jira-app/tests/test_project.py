# SPDX-License-Identifier: Apache-2.0
"""Project file of the app (brief M2-50, Tests section ``test_project.py``; Design note 2 on the
dependencies): ``bothy-sdk`` comes from ``../../sdk/python`` as an editable path source; every
dev tool shared with ``sdk/python/pyproject.toml`` has the same exact ``==`` version;
``fail_under`` equals ``quality.coverage.python_min_percent`` of ``docs/spec/23-quality.md``;
every Python file of the app and its tests carries the SPDX header in the window of the license
gate; ``uv.lock`` pins every registry package with a hash.
"""

from __future__ import annotations

import re
import tomllib
from pathlib import Path
from typing import Any

APP = Path(__file__).resolve().parents[1]
SPDX_LINE = "# SPDX-License-Identifier: Apache-2.0"
_REQUIREMENT = re.compile(r"^(?P<name>[A-Za-z0-9][A-Za-z0-9._-]*)(?P<rest>.*)$")
_PIN = re.compile(r"^==(?P<version>[A-Za-z0-9.+!-]+)$")
_ROW = re.compile(r"^\| `(?P<name>[a-z0-9_.]+)` \| `(?P<value>[^`]+)` \|", re.MULTILINE)
_HEADER_LINES = re.compile(r"^HEADER_SEARCH_LINES = (?P<n>\d+)$", re.MULTILINE)
_HASH = re.compile(r"^sha256:[0-9a-f]{64}$")


def _repo_root() -> Path:
    """The first ancestor of this file that holds ``docs/api``."""
    for parent in Path(__file__).resolve().parents:
        if (parent / "docs" / "api").is_dir():
            return parent
    raise AssertionError("repository root with docs/api not found")


REPO = _repo_root()


def _toml(path: Path) -> dict[str, Any]:
    return tomllib.loads(path.read_text(encoding="utf-8"))


def _canonical(name: str) -> str:
    return re.sub(r"[-_.]+", "-", name).lower()


def _dev_pins(pyproject: dict[str, Any]) -> dict[str, str]:
    """Map of canonical name to the specifier text after the name (extras kept)."""
    pins: dict[str, str] = {}
    for entry in pyproject["dependency-groups"]["dev"]:
        assert isinstance(entry, str), entry
        m = _REQUIREMENT.match(entry.strip())
        assert m, entry
        pins[_canonical(m.group("name"))] = m.group("rest").strip()
    return pins


def test_sdk_is_an_editable_path_source() -> None:
    pyproject = _toml(APP / "pyproject.toml")
    source = pyproject["tool"]["uv"]["sources"]["bothy-sdk"]
    assert source == {"path": "../../sdk/python", "editable": True}
    assert (APP / source["path"]).resolve() == (REPO / "sdk" / "python").resolve()
    assert (REPO / "sdk" / "python" / "pyproject.toml").is_file()


def test_shared_dev_tools_have_the_sdk_versions() -> None:
    app = _dev_pins(_toml(APP / "pyproject.toml"))
    sdk = _dev_pins(_toml(REPO / "sdk" / "python" / "pyproject.toml"))
    shared = sorted(set(app) & set(sdk))
    # The test tools the brief names are among the shared ones, so the check is not vacuous.
    assert {"mypy", "pytest", "pytest-cov", "ruff"} <= set(shared), shared
    for name in shared:
        app_pin = _PIN.match(app[name])
        sdk_pin = _PIN.match(sdk[name])
        assert app_pin, f"{name} is not pinned with == in the app: {app[name]!r}"
        assert sdk_pin, f"{name} is not pinned with == in the SDK: {sdk[name]!r}"
        assert app_pin.group("version") == sdk_pin.group("version"), name


def test_fail_under_equals_the_quality_chapter() -> None:
    text = (REPO / "docs" / "spec" / "23-quality.md").read_text(encoding="utf-8")
    table = text.split("## 11. Configuration values of this chapter", 1)[1]
    values = {m.group("name"): m.group("value") for m in _ROW.finditer(table)}
    expected = values["quality.coverage.python_min_percent"]
    fail_under = _toml(APP / "pyproject.toml")["tool"]["coverage"]["report"]["fail_under"]
    assert str(fail_under) == expected


def _python_files() -> list[Path]:
    files = [
        path
        for top in ("jira_app", "tests")
        for path in sorted((APP / top).rglob("*.py"))
        if not {"__pycache__", ".venv"} & set(path.relative_to(APP).parts)
    ]
    assert files
    return files


def test_every_python_file_has_the_spdx_header() -> None:
    gate = (REPO / "tools" / "ci" / "license_check.py").read_text(encoding="utf-8")
    m = _HEADER_LINES.search(gate)
    assert m, "HEADER_SEARCH_LINES not found in the license gate"
    window = int(m.group("n"))
    files = _python_files()
    assert APP / "jira_app" / "__main__.py" in files
    assert Path(__file__).resolve() in files
    missing = []
    for path in files:
        with path.open(encoding="utf-8", errors="replace") as fh:
            head = [fh.readline().rstrip("\n") for _ in range(window)]
        if SPDX_LINE not in head:
            missing.append(str(path.relative_to(APP)))
    assert missing == []


def test_lock_has_hashes_for_every_registry_package() -> None:
    lock = _toml(APP / "uv.lock")
    registry = [p for p in lock["package"] if "registry" in p.get("source", {})]
    assert registry
    names = {p["name"] for p in registry}
    assert {"aiohttp", "httpx", "cryptography", "pytest", "ruff", "mypy"} <= names
    unhashed = []
    for package in registry:
        artifacts = list(package.get("wheels", []))
        if "sdist" in package:
            artifacts.append(package["sdist"])
        if not artifacts or not all(_HASH.match(a.get("hash", "")) for a in artifacts):
            unhashed.append(f"{package['name']}=={package.get('version')}")
    assert unhashed == []
