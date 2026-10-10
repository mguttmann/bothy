# SPDX-License-Identifier: Apache-2.0
"""Tests for applying and checking the SonarQube quality gate and profiles."""

from __future__ import annotations

import json
from collections.abc import Iterator
from pathlib import Path

import pytest
import quality_sync as qs
from fake_sonar import State, running

REPO = Path(__file__).resolve().parents[3]
TOKEN = "fake-token-for-tests"


@pytest.fixture
def server() -> Iterator[tuple[State, str]]:
    state = State()
    with running(state) as url:
        yield state, url


def _client(url: str, organization: str | None = None) -> qs.Client:
    return qs.Client(url, TOKEN, organization)


def test_repository_config_is_consistent() -> None:
    config = qs.load_config(REPO / "quality" / "sonar")
    assert config.gate_name == "Bothy way"
    assert {c.metric for c in config.conditions} == {
        "new_violations",
        "new_security_hotspots_reviewed",
        "new_coverage",
        "new_duplicated_lines_density",
    }
    assert set(config.profiles) == {"dart", "docker", "go", "js", "kotlin", "py", "swift", "ts"}


def test_apply_creates_gate_and_profiles(server: tuple[State, str]) -> None:
    state, url = server
    config = qs.load_config(REPO / "quality" / "sonar")
    report = qs.apply(_client(url), config)
    assert [(c["metric"], c["op"], c["error"]) for c in state.gates["Bothy way"]] == [
        (c.metric, c.op, c.error) for c in config.conditions
    ]
    assert state.default_gate == "Bothy way"
    go = state.profile("go", "Bothy way")
    assert go is not None
    assert go.parent == "Sonar way"
    assert go.default is True
    assert report.skipped_languages == ["dart", "swift"]


def test_apply_is_idempotent(server: tuple[State, str]) -> None:
    state, url = server
    config = qs.load_config(REPO / "quality" / "sonar")
    qs.apply(_client(url), config)
    state.calls.clear()
    qs.apply(_client(url), config)
    writes = {path for method, path, _ in state.calls if method == "POST"}
    assert "/api/qualitygates/create" not in writes
    assert "/api/qualitygates/create_condition" not in writes
    assert "/api/qualityprofiles/create" not in writes


def test_apply_repairs_condition_drift(server: tuple[State, str]) -> None:
    state, url = server
    config = qs.load_config(REPO / "quality" / "sonar")
    qs.apply(_client(url), config)
    gate = state.gates["Bothy way"]
    gate[0]["error"] = "5"
    gate.append({"id": "99", "metric": "new_bugs", "op": "GT", "error": "0"})
    qs.apply(_client(url), config)
    assert sorted((c["metric"], c["error"]) for c in state.gates["Bothy way"]) == sorted(
        (c.metric, c.error) for c in config.conditions
    )


def test_check_passes_after_apply(server: tuple[State, str]) -> None:
    _, url = server
    config = qs.load_config(REPO / "quality" / "sonar")
    qs.apply(_client(url), config)
    assert qs.check(_client(url), config).problems == []


def test_check_reports_gate_difference(server: tuple[State, str]) -> None:
    state, url = server
    config = qs.load_config(REPO / "quality" / "sonar")
    qs.apply(_client(url), config)
    state.gates["Bothy way"][2]["error"] = "60"
    problems = qs.check(_client(url), config).problems
    assert problems == ["gate 'Bothy way': condition new_coverage LT 60 differs (want LT 80)"]


def test_check_reports_extra_rule_in_profile(server: tuple[State, str]) -> None:
    state, url = server
    config = qs.load_config(REPO / "quality" / "sonar")
    qs.apply(_client(url), config)
    prof = state.profile("py", "Bothy way")
    assert prof is not None
    prof.own_rules.add("py:S9999")
    problems = qs.check(_client(url), config).problems
    assert problems == ["profile py: activated beyond parent ['py:S9999'], want []"]


def test_check_reports_missing_gate(server: tuple[State, str]) -> None:
    _, url = server
    config = qs.load_config(REPO / "quality" / "sonar")
    assert "gate 'Bothy way' is missing" in qs.check(_client(url), config).problems


def test_restore_activates_rules_from_xml(server: tuple[State, str], tmp_path: Path) -> None:
    state, url = server
    src = REPO / "quality" / "sonar"
    for f in src.iterdir():
        (tmp_path / f.name).write_bytes(f.read_bytes())
    xml = tmp_path / "quality-profile-go.xml"
    xml.write_text(
        xml.read_text().replace(
            "<rules/>",
            "<rules><rule><repositoryKey>go</repositoryKey><key>S1135</key>"
            "<priority>INFO</priority><parameters/></rule></rules>",
        )
    )
    config = qs.load_config(tmp_path)
    qs.apply(_client(url), config)
    prof = state.profile("go", "Bothy way")
    assert prof is not None
    assert "go:S1135" in prof.own_rules
    assert qs.check(_client(url), config).problems == []


def test_missing_parent_is_reported(server: tuple[State, str], tmp_path: Path) -> None:
    _, url = server
    for f in (REPO / "quality" / "sonar").iterdir():
        (tmp_path / f.name).write_bytes(f.read_bytes())
    manifest = json.loads((tmp_path / "profiles.json").read_text())
    manifest["parent"] = "Sonar way core"
    (tmp_path / "profiles.json").write_text(json.dumps(manifest))
    with pytest.raises(qs.SonarError, match="parent profile 'Sonar way core'"):
        qs.apply(_client(url), qs.load_config(tmp_path))


def test_organization_is_sent_for_cloud(server: tuple[State, str]) -> None:
    state, url = server
    qs.check(_client(url, organization="example-org"), qs.load_config(REPO / "quality" / "sonar"))
    assert state.calls
    assert all(params.get("organization") == "example-org" for _, _, params in state.calls)


def test_token_is_sent_as_bearer_and_never_printed(
    server: tuple[State, str], capsys: pytest.CaptureFixture[str], monkeypatch: pytest.MonkeyPatch
) -> None:
    state, url = server
    monkeypatch.setenv("SONAR_TOKEN", TOKEN)
    rc = qs.main(["check", "--url", url, "--config", str(REPO / "quality" / "sonar")])
    captured = capsys.readouterr()
    assert rc == 1  # nothing applied yet
    assert set(state.tokens) == {f"Bearer {TOKEN}"}
    assert TOKEN not in captured.out + captured.err


def test_main_requires_token(
    monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    monkeypatch.delenv("SONAR_TOKEN", raising=False)
    assert qs.main(["check", "--url", "http://127.0.0.1:9"]) == 2
    assert "SONAR_TOKEN" in capsys.readouterr().err


def test_main_apply_then_check(server: tuple[State, str], monkeypatch: pytest.MonkeyPatch) -> None:
    _, url = server
    monkeypatch.setenv("SONAR_TOKEN", TOKEN)
    assert qs.main(["apply", "--url", url]) == 0
    assert qs.main(["check", "--url", url]) == 0
