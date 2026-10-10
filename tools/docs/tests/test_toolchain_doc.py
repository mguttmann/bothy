# SPDX-License-Identifier: Apache-2.0
"""Version guard: docs/quality/toolchain.md names exactly the pinned versions."""

from __future__ import annotations

import json
import re
import tomllib
from pathlib import Path

import pytest

REPO = Path(__file__).resolve().parents[3]
DOC = REPO / "docs" / "quality" / "toolchain.md"
_ROW = re.compile(r"^\| `(?P<name>[^`]+)` \| `(?P<version>[^`]+)` \|", re.MULTILINE)
_USES = re.compile(r"uses: (?P<name>[\w.-]+/[\w.-]+)(?:/[\w./-]+)?@[0-9a-f]{40} # (?P<version>\S+)")


def doc_versions(section: str) -> dict[str, str]:
    text = DOC.read_text(encoding="utf-8")
    part = text.split(f"## {section}", 1)[1].split("\n## ", 1)[0]
    return {m.group("name"): m.group("version") for m in _ROW.finditer(part)}


def pinned_tools() -> dict[str, str]:
    pins: dict[str, str] = {}
    lock = tomllib.loads((REPO / "tools" / "uv.lock").read_text())
    for pkg in lock["package"]:
        pins[pkg["name"].lower()] = pkg["version"]
    npm = json.loads((REPO / "tools" / "node" / "package-lock.json").read_text())
    for path, meta in npm["packages"].items():
        if path.startswith("node_modules/") and path.count("node_modules/") == 1:
            pins[path.removeprefix("node_modules/")] = meta["version"]
    binaries = json.loads((REPO / "tools" / "bin" / "tools.lock.json").read_text())
    for name, tool in binaries["tools"].items():
        pins[name] = tool["version"]
    makefile = (REPO / "Makefile").read_text()
    for var, name in [
        ("SEMGREP_VERSION", "semgrep"),
        ("GOVULNCHECK_VERSION", "govulncheck"),
        ("GO_LICENSES_VERSION", "go-licenses"),
    ]:
        m = re.search(rf"^{var} := (\S+)$", makefile, re.MULTILINE)
        assert m, var
        pins[name] = m.group(1)
    compose = (REPO / "tools" / "sonarqube" / "compose.yaml").read_text()
    for image in ("sonarqube", "postgres"):
        m = re.search(rf"image: {image}:([^@\s]+)@sha256:[0-9a-f]{{64}}", compose)
        assert m, image
        pins[image] = m.group(1)
    hooks = (REPO / ".pre-commit-config.yaml").read_text()
    m = re.search(r"pre-commit-hooks\n\s+rev: [0-9a-f]{40}  # frozen: (\S+)", hooks)
    assert m, "pre-commit-hooks"
    pins["pre-commit-hooks"] = m.group(1)
    pins["uv"] = "Systemwerkzeug"
    return pins


def pinned_actions() -> dict[str, str]:
    found: dict[str, set[str]] = {}
    for wf in (REPO / ".github" / "workflows").glob("*.yml"):
        for m in _USES.finditer(wf.read_text()):
            found.setdefault(m.group("name"), set()).add(m.group("version"))
    inconsistent = {k: v for k, v in found.items() if len(v) > 1}
    assert not inconsistent, f"one action, several versions: {inconsistent}"
    return {k: next(iter(v)) for k, v in found.items()}


@pytest.mark.parametrize("name", sorted(doc_versions("Gate-Werkzeuge")))
def test_tool_version_matches_pin(name: str) -> None:
    pins = pinned_tools()
    assert name in pins, f"{name} is documented but not pinned anywhere"
    assert doc_versions("Gate-Werkzeuge")[name] == pins[name]


def test_every_pinned_gate_tool_is_documented() -> None:
    documented = set(doc_versions("Gate-Werkzeuge"))
    binaries = json.loads((REPO / "tools" / "bin" / "tools.lock.json").read_text())["tools"]
    npm = json.loads((REPO / "tools" / "node" / "package.json").read_text())["devDependencies"]
    project = tomllib.loads((REPO / "tools" / "pyproject.toml").read_text())
    direct = [re.split(r"[=<>]", d)[0].lower() for d in project["project"]["dependencies"]]
    direct += [re.split(r"[=<>]", d)[0].lower() for d in project["dependency-groups"]["dev"]]
    expected = set(binaries) | set(npm) | {d for d in direct if not d.startswith("types-")}
    assert expected - documented == set()


def test_actions_table_matches_workflows() -> None:
    assert doc_versions("GitHub Actions") == pinned_actions()
