# SPDX-License-Identifier: Apache-2.0
"""CI installs Go and Flutter for every tracked component (briefs M2-38 note 3, M4-01 note 1).

The jobs `gates` and `sonar` of .github/workflows/ci.yml install Go when any `services/*/go.mod`
is tracked and Flutter when any Dart package is tracked, each pinned by a version file. Every Go
module shares one `go` and `toolchain` line (ADR 0005: CI installs one Go). Flutter comes from
the root `pubspec.yaml` of the Dart workspace in every job that installs it, and the root and
every member pin the same exact version.
"""

from __future__ import annotations

import os
import re
import subprocess
from pathlib import Path
from typing import Any

import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]
WORKFLOW = REPO / ".github" / "workflows" / "ci.yml"
DOCS_WORKFLOW = REPO / ".github" / "workflows" / "docs.yml"
GO_IF = "hashFiles('services/*/go.mod') != ''"
GO_VERSION_FILE = (
    "${{ hashFiles('services/server/go.mod') != '' && 'services/server/go.mod' "
    "|| 'services/host/go.mod' }}"
)
FLUTTER_IF = "hashFiles('apps/*/pubspec.yaml', 'packages/*/pubspec.yaml') != ''"
# The root pubspec.yaml of the workspace carries the pin (docs/DECISIONS.md D-M4-01-1).
FLUTTER_VERSION_FILE = "pubspec.yaml"
APPS_JOBS = ("apps-macos", "apps-windows")


def _tracked(*patterns: str) -> list[str]:
    out = subprocess.run(
        ["git", "-C", str(REPO), "ls-files", "--", *patterns],
        check=True,
        capture_output=True,
        text=True,
    ).stdout
    return [line for line in out.splitlines() if line]


def _jobs(workflow: Path = WORKFLOW) -> dict[str, Any]:
    data = yaml.safe_load(workflow.read_text(encoding="utf-8"))
    jobs = data["jobs"]
    assert isinstance(jobs, dict)
    return jobs


def _steps(job: str, workflow: Path = WORKFLOW) -> list[dict[str, Any]]:
    steps = _jobs(workflow)[job]["steps"]
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
    assert step["with"]["channel"] == "stable"
    for pubspec in _tracked("apps/*pubspec.yaml", "packages/*pubspec.yaml"):
        assert re.fullmatch(r"(apps|packages)/[^/]+/pubspec\.yaml", pubspec), pubspec


@pytest.mark.parametrize(
    ("workflow", "job"),
    [
        (WORKFLOW, "gates"),
        (WORKFLOW, "sonar"),
        (WORKFLOW, "apps-macos"),
        (WORKFLOW, "apps-windows"),
        (DOCS_WORKFLOW, "build"),
    ],
)
def test_flutter_version_file_is_root_pubspec(workflow: Path, job: str) -> None:
    [step] = [s for s in _steps(job, workflow) if _uses(s, "subosito/flutter-action")]
    assert step["with"]["flutter-version-file"] == FLUTTER_VERSION_FILE
    assert step["with"]["channel"] == "stable"
    assert (REPO / FLUTTER_VERSION_FILE).is_file()


def test_docs_workflow_installs_flutter_before_make_docs() -> None:
    steps = _steps("build", DOCS_WORKFLOW)
    flutter = next(i for i, s in enumerate(steps) if _uses(s, "subosito/flutter-action"))
    docs = next(i for i, s in enumerate(steps) if s.get("run") == "make docs")
    assert flutter < docs


@pytest.mark.parametrize(
    ("job", "runner"), [("apps-macos", "macos-15"), ("apps-windows", "windows-2025")]
)
def test_apps_jobs_run_the_platform_tests(job: str, runner: str) -> None:
    spec = _jobs()[job]
    assert spec["runs-on"] == runner
    assert spec["needs"] == "plan"
    assert spec["permissions"] == {"contents": "read"}
    assert isinstance(spec["timeout-minutes"], int)
    assert "needs.plan.outputs.components" in spec["if"]
    [run] = [s for s in spec["steps"] if "run" in s]
    assert run["run"] == "uv run --project tools --locked --quiet python tools/ci/apps_platform.py"
    assert run["shell"] == "bash"
    assert any(_uses(s, "astral-sh/setup-uv") for s in spec["steps"])


def _ci_ok(**results: str) -> int:
    """Run the script of the ci-ok job with the given job results (others: success)."""
    job = _jobs()["ci-ok"]
    [step] = job["steps"]
    env = dict.fromkeys(step["env"], "success")
    env.update(results)
    return subprocess.run(
        ["bash", "-e", "-c", step["run"]],
        env={**env, "PATH": os.environ["PATH"]},
        capture_output=True,
        text=True,
        check=False,
    ).returncode


def test_ci_ok_waits_for_the_apps_jobs() -> None:
    needs = _jobs()["ci-ok"]["needs"]
    assert set(APPS_JOBS) <= set(needs)
    env = _jobs()["ci-ok"]["steps"][0]["env"]
    assert env["APPS_MACOS"] == "${{ needs.apps-macos.result }}"
    assert env["APPS_WINDOWS"] == "${{ needs.apps-windows.result }}"


@pytest.mark.parametrize("variable", ["APPS_MACOS", "APPS_WINDOWS"])
@pytest.mark.parametrize(
    ("result", "code"), [("success", 0), ("skipped", 0), ("failure", 1), ("cancelled", 1)]
)
def test_ci_ok_accepts_success_or_skipped_apps_jobs(variable: str, result: str, code: int) -> None:
    assert _ci_ok(**{variable: result}) == code


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


def _pubspec(rel: str) -> dict[str, Any]:
    data = yaml.safe_load((REPO / rel).read_text(encoding="utf-8"))
    assert isinstance(data, dict), rel
    return data


def test_flutter_pins_agree() -> None:
    """The root and every member pin the same exact Flutter version (D-M4-01-1)."""
    pins: dict[str, object] = {"pubspec.yaml": _pubspec("pubspec.yaml")["environment"]["flutter"]}
    sdks: dict[str, object] = {"pubspec.yaml": _pubspec("pubspec.yaml")["environment"]["sdk"]}
    for pubspec in _tracked("apps/*pubspec.yaml", "packages/*pubspec.yaml"):
        environment = _pubspec(pubspec).get("environment") or {}
        pins[pubspec] = environment.get("flutter")
        sdks[pubspec] = environment.get("sdk")
    assert len(pins) == 15, sorted(pins)
    assert all(isinstance(v, str) and re.fullmatch(r"\d+\.\d+\.\d+", v) for v in pins.values()), (
        pins
    )
    assert len(set(pins.values())) == 1, pins
    assert len(set(sdks.values())) == 1, sdks


def test_members_resolve_in_the_workspace_and_are_never_published() -> None:
    root = _pubspec("pubspec.yaml")
    assert root["name"] == "bothy_workspace"
    assert root["publish_to"] == "none"
    members = _tracked("apps/*pubspec.yaml", "packages/*pubspec.yaml")
    assert sorted(f"{m}/pubspec.yaml" for m in root["workspace"]) == sorted(members)
    for pubspec in members:
        data = _pubspec(pubspec)
        assert data["resolution"] == "workspace", pubspec
        assert data["publish_to"] == "none", pubspec
        assert data["version"] == "0.0.0", pubspec
    # One resolution: only the root tracks a lock file.
    assert _tracked("*pubspec.lock") == ["pubspec.lock"]


def test_sonar_reads_the_lcov_of_every_member() -> None:
    text = (REPO / "sonar-project.properties").read_text(encoding="utf-8")
    [line] = [x for x in text.splitlines() if x.startswith("sonar.dart.lcov.reportPaths=")]
    paths = line.split("=", 1)[1].split(",")
    members = _pubspec("pubspec.yaml")["workspace"]
    assert paths == [f"{m}/coverage/lcov.info" for m in members]
    [exclusions] = [x for x in text.splitlines() if x.startswith("sonar.exclusions=")]
    for pattern in ("**/*.g.dart", "**/*.freezed.dart", "**/*.drift.dart", "**/generated/**"):
        assert pattern in exclusions.split("=", 1)[1].split(","), pattern
