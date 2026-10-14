# SPDX-License-Identifier: Apache-2.0
"""CI installs Go and Flutter for every tracked component (brief M2-38, Design note 3).

The jobs `gates` and `sonar` of .github/workflows/ci.yml install Go when any `services/*/go.mod`
is tracked and Flutter when any Dart package is tracked, each pinned by a version file. Every Go
module shares one `go` and `toolchain` line (ADR 0005: CI installs one Go), and every pubspec
that pins Flutter pins the same exact version.
"""

from __future__ import annotations

import re
import subprocess
from pathlib import Path
from typing import Any

import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]
WORKFLOW = REPO / ".github" / "workflows" / "ci.yml"
GO_IF = "hashFiles('services/*/go.mod') != ''"
GO_VERSION_FILE = (
    "${{ hashFiles('services/server/go.mod') != '' && 'services/server/go.mod' "
    "|| 'services/host/go.mod' }}"
)
FLUTTER_IF = "hashFiles('apps/*/pubspec.yaml', 'packages/*/pubspec.yaml') != ''"
FLUTTER_VERSION_FILE = (
    "${{ hashFiles('apps/client/pubspec.yaml') != '' && 'apps/client/pubspec.yaml' "
    "|| 'packages/bothy_noise/pubspec.yaml' }}"
)


def _tracked(*patterns: str) -> list[str]:
    out = subprocess.run(
        ["git", "-C", str(REPO), "ls-files", "--", *patterns],
        check=True,
        capture_output=True,
        text=True,
    ).stdout
    return [line for line in out.splitlines() if line]


def _steps(job: str) -> list[dict[str, Any]]:
    data = yaml.safe_load(WORKFLOW.read_text(encoding="utf-8"))
    steps = data["jobs"][job]["steps"]
    assert isinstance(steps, list)
    return steps


def _uses(step: dict[str, Any], action: str) -> bool:
    return str(step.get("uses", "")).startswith(action + "@")


@pytest.mark.parametrize("job", ["gates", "sonar"])
def test_setup_go_covers_every_go_module(job: str) -> None:
    [step] = [s for s in _steps(job) if _uses(s, "actions/setup-go")]
    assert step["if"] == GO_IF
    assert step["with"]["go-version-file"] == GO_VERSION_FILE
    assert step["with"]["cache-dependency-path"] == "services/*/go.sum"
    # Every tracked module is matched by the condition and its sums by the cache key.
    for go_mod in _tracked("services/*go.mod"):
        assert re.fullmatch(r"services/[^/]+/go\.mod", go_mod), go_mod


@pytest.mark.parametrize("job", ["gates", "sonar"])
def test_flutter_covers_every_dart_package(job: str) -> None:
    [step] = [s for s in _steps(job) if _uses(s, "subosito/flutter-action")]
    assert step["if"] == FLUTTER_IF
    assert step["with"]["flutter-version-file"] == FLUTTER_VERSION_FILE
    assert step["with"]["channel"] == "stable"
    for pubspec in _tracked("apps/*pubspec.yaml", "packages/*pubspec.yaml"):
        assert re.fullmatch(r"(apps|packages)/[^/]+/pubspec\.yaml", pubspec), pubspec
    # The fallback version file exists while apps/client has none.
    if not (REPO / "apps" / "client" / "pubspec.yaml").exists():
        assert (REPO / "packages" / "bothy_noise" / "pubspec.yaml").exists()


def _directive(go_mod: str, name: str) -> str:
    lines = (REPO / go_mod).read_text(encoding="utf-8").splitlines()
    values = [line.split()[1] for line in lines if line.startswith(name + " ")]
    assert len(values) == 1, f"{go_mod}: {len(values)} {name} lines"
    return values[0]


def test_go_directives_agree() -> None:
    modules = _tracked("services/*go.mod")
    assert "services/host/go.mod" in modules
    assert len(modules) >= 2, "the test needs two modules to compare"
    for name in ("go", "toolchain"):
        values = {m: _directive(m, name) for m in modules}
        assert len(set(values.values())) == 1, f"{name} lines differ: {values}"


def test_flutter_pins_agree() -> None:
    pins: dict[str, str] = {}
    for pubspec in _tracked("apps/*pubspec.yaml", "packages/*pubspec.yaml"):
        data = yaml.safe_load((REPO / pubspec).read_text(encoding="utf-8"))
        flutter = (data.get("environment") or {}).get("flutter")
        if flutter is None:
            continue
        assert isinstance(flutter, str), pubspec
        assert re.fullmatch(r"\d+\.\d+\.\d+", flutter), f"{pubspec}: {flutter!r} is not exact"
        pins[pubspec] = flutter
    assert "packages/bothy_noise/pubspec.yaml" in pins
    assert len(set(pins.values())) == 1, pins
