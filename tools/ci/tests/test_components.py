# SPDX-License-Identifier: Apache-2.0
"""Tests for the component map (CI path filter and release tag resolution)."""

from __future__ import annotations

import json
from pathlib import Path

import components
import pytest


@pytest.mark.parametrize(
    ("files", "expected"),
    [
        (["services/server/internal/chat/x.go"], ["server"]),
        (["apps/client/lib/main.dart", "packages/design_system/lib/a.dart"], ["apps"]),
        (["design/tokens/color.json"], ["apps"]),
        (["sdk/python/src/x.py"], ["sdk-python"]),
        (["sdk/typescript/src/x.ts"], ["sdk-typescript"]),
        (["plugins/hermes-adapter/x.py"], ["plugins"]),
        # Shared G5 fixtures of both SDKs (docs/DECISIONS.md M2-23/6).
        (["sdk/testdata/tolerance/unknown_field.json"], ["sdk-python", "sdk-typescript"]),
        (["docs/api/openapi.yaml"], ["apps", "plugins", "sdk-python", "sdk-typescript", "server"]),
        # The server generates its route table and action catalogue from the action catalogue
        # (services/server/api/generate.go); `make catalog` also rewrites chapter 05.
        (["docs/spec/actions.yaml"], ["server"]),
        (["docs/spec/actions.yaml", "docs/spec/05-aktionen.md"], ["server"]),
        (["docs/spec/05-aktionen.md"], []),
        (["docs/spec/07-rechte.md", "infra/compose/compose.yaml", "README.md"], []),
        (["Makefile"], ["all"]),
        (["tools/ci/check_runners.py"], ["all"]),
        ([".github/workflows/ci.yml"], ["all"]),
        (["some-new-top-level.toml"], ["all"]),
    ],
)
def test_changed_components(files: list[str], expected: list[str]) -> None:
    assert components.changed(files) == expected


def test_format_for_make() -> None:
    assert components.format_for_make([]) == "none"
    assert components.format_for_make(["all"]) == "all"
    assert components.format_for_make(["server", "apps"]) == "apps server"


def _repo_with_version(tmp_path: Path, rel: str, version: str) -> Path:
    path = tmp_path / rel
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(version + "\n")
    return tmp_path


def test_release_resolves_component_tag(tmp_path: Path) -> None:
    repo = _repo_with_version(tmp_path, "services/server/VERSION", "1.4.0")
    info = components.release_info("server/v1.4.0", repo)
    assert info == {
        "component": "server",
        "version": "1.4.0",
        "path": "services/server",
        "gate_components": "server",
        "prerelease": False,
    }


def test_release_resolves_root_tag(tmp_path: Path) -> None:
    repo = _repo_with_version(tmp_path, "VERSION", "0.3.0-rc.1")
    info = components.release_info("v0.3.0-rc.1", repo)
    assert info["component"] == "bothy"
    assert info["gate_components"] == "all"
    assert info["prerelease"] is True


@pytest.mark.parametrize(
    ("tag", "message"),
    [
        ("nope/v1.0.0", "unknown release component"),
        ("server/1.0.0", "not a release tag"),
        ("server/v1.0", "not a release tag"),
        ("server/v1.0.0", "VERSION"),
    ],
)
def test_release_rejects_bad_tags(tmp_path: Path, tag: str, message: str) -> None:
    with pytest.raises(components.ReleaseError, match=message):
        components.release_info(tag, tmp_path)


def test_release_rejects_version_mismatch(tmp_path: Path) -> None:
    repo = _repo_with_version(tmp_path, "services/server/VERSION", "1.3.9")
    with pytest.raises(components.ReleaseError, match=r"1\.3\.9"):
        components.release_info("server/v1.4.0", repo)


def test_cli_changed_from_files(capsys: pytest.CaptureFixture[str]) -> None:
    assert components.main(["changed", "--files", "sdk/python/a.py", "docs/x.md"]) == 0
    assert capsys.readouterr().out.strip() == "sdk-python"


def test_cli_release_prints_json(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    repo = _repo_with_version(tmp_path, "sdk/python/VERSION", "2.0.0")
    assert components.main(["release", "--repo", str(repo), "--tag", "sdk-python/v2.0.0"]) == 0
    assert json.loads(capsys.readouterr().out)["component"] == "sdk-python"


def test_cli_release_github_output(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    repo = _repo_with_version(tmp_path, "apps/client/VERSION", "1.0.0-beta.2")
    rc = components.main(
        ["release", "--repo", str(repo), "--tag", "client/v1.0.0-beta.2", "--github-output"]
    )
    assert rc == 0
    assert capsys.readouterr().out.splitlines() == [
        "component=client",
        "version=1.0.0-beta.2",
        "path=apps/client",
        "gate_components=apps",
        "prerelease=true",
    ]


def test_cli_release_error_exit(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    assert components.main(["release", "--repo", str(tmp_path), "--tag", "x/v1.0.0"]) == 1
    assert "unknown release component" in capsys.readouterr().err
