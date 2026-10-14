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
PROJECT = "example-project"
ORG = "example-org"
CLOUD_LANGUAGES = ("docker", "go", "js", "kotlin", "py", "ts")


@pytest.fixture
def server() -> Iterator[tuple[State, str]]:
    state = State()
    with running(state) as url:
        yield state, url


def _client(url: str, organization: str | None = None) -> qs.Client:
    return qs.Client(url, TOKEN, organization, PROJECT if organization else None)


def _posts(state: State, path: str) -> list[dict[str, str]]:
    return [params for method, p, params in state.calls if method == "POST" and p == path]


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
    assert any(params.get("project") == PROJECT for _, _, params in state.calls)
    assert all(params.get("organization") == "example-org" for _, _, params in state.calls)


def test_apply_then_check_on_cloud_api() -> None:
    # SonarQube Cloud has no profile "Sonar way" (api/qualityprofiles/search, 2026-10-13).
    state = State(cloud=True, builtin_profile="Sonar way comprehensive")
    config = qs.load_config(REPO / "quality" / "sonar")
    with running(state) as url:
        qs.apply(_client(url, organization="example-org"), config)
        assert state.default_gate == "Bothy way"
        go = state.profile("go", "Bothy way")
        assert go is not None
        assert go.parent == "Sonar way comprehensive"
        assert [(c["metric"], c["op"], c["error"]) for c in state.gates["Bothy way"]] == [
            (c.metric, c.op, c.error) for c in config.conditions
        ]
        # The profiles are assigned to the project, never made the organization default.
        assert _posts(state, "/api/qualityprofiles/set_default") == []
        assert go.default is False
        assert state.project_profiles == dict.fromkeys(CLOUD_LANGUAGES, "Bothy way")
        assert {p["project"] for p in _posts(state, "/api/qualityprofiles/add_project")} == {
            PROJECT
        }
        assert state.project_gate == "Bothy way"
        assert _posts(state, "/api/qualitygates/select") == [
            {"gateId": state.gate_ids["Bothy way"], "projectKey": PROJECT, "organization": ORG}
        ]
        checked = qs.check(_client(url, organization="example-org"), config)
        assert checked.problems == []
        assert checked.gaps == []


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
    # CI sets SONAR_ORGANIZATION for the cloud; this test talks to the local server API.
    monkeypatch.delenv("SONAR_ORGANIZATION", raising=False)
    assert qs.main(["apply", "--url", url]) == 0
    assert qs.main(["check", "--url", url]) == 0


def test_cloud_plan_refusing_default_gate_still_applies_profiles() -> None:
    # SonarQube Cloud free plan: api/qualitygates/set_as_default answers HTTP 403.
    state = State(
        cloud=True, builtin_profile="Sonar way comprehensive", gate_default_forbidden=True
    )
    config = qs.load_config(REPO / "quality" / "sonar")
    with running(state) as url:
        applied = qs.apply(_client(url, organization="example-org"), config)
        assert applied.problems == []
        assert applied.gaps == [qs.GATE_NOT_DEFAULT]
        assert state.default_gate is None
        for language in ("docker", "go", "js", "kotlin", "py", "ts"):
            prof = state.profile(language, "Bothy way")
            assert prof is not None, language
            assert prof.parent == "Sonar way comprehensive"
            assert state.project_profiles[language] == "Bothy way"
        assert state.project_gate == "Bothy way"
        checked = qs.check(_client(url, organization="example-org"), config)
        assert checked.problems == []
        assert checked.gaps == [qs.GATE_NOT_DEFAULT]
        # Any other difference still fails the check.
        state.gates["Bothy way"][2]["error"] = "60"
        prof = state.profile("py", "Bothy way")
        assert prof is not None
        prof.own_rules.add("py:S9999")
        assert qs.check(_client(url, organization="example-org"), config).problems == [
            "gate 'Bothy way': condition new_coverage LT 60 differs (want LT 80)",
            "profile py: activated beyond parent ['py:S9999'], want []",
        ]


def test_main_reports_gap_and_passes_on_cloud(
    monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    state = State(
        cloud=True, builtin_profile="Sonar way comprehensive", gate_default_forbidden=True
    )
    monkeypatch.setenv("SONAR_TOKEN", TOKEN)
    monkeypatch.setenv("SONAR_PROJECT_KEY", PROJECT)
    with running(state) as url:
        args = ["--url", url, "--organization", "example-org"]
        assert qs.main(["apply", *args]) == 0
        assert qs.main(["check", *args]) == 0
    out = capsys.readouterr().out
    assert out.count("quality_sync: gap: gate not default: plan does not allow it") == 2


def test_cloud_plan_refusing_project_assignment_is_a_gap_per_language() -> None:
    state = State(
        cloud=True,
        builtin_profile="Sonar way comprehensive",
        gate_default_forbidden=True,
        profile_default_forbidden=True,
        profile_project_forbidden=True,
        gate_select_forbidden=True,
    )
    config = qs.load_config(REPO / "quality" / "sonar")
    with running(state) as url:
        applied = qs.apply(_client(url, organization=ORG), config)
        assert applied.problems == []
        # Every language was still tried, and each refusal names the server's reason.
        assert len(_posts(state, "/api/qualityprofiles/add_project")) == len(CLOUD_LANGUAGES)
        assert applied.gaps == [
            qs.GATE_NOT_DEFAULT,
            *(
                f"profile {lang}: {qs.PROFILE_NOT_ON_PROJECT} (POST {url}/api/qualityprofiles/"
                f"add_project: HTTP 400: Organization '{ORG}' is not allowed to perform this "
                "action)"
                for lang in CLOUD_LANGUAGES
            ),
            f"{qs.GATE_NOT_ON_PROJECT} (POST {url}/api/qualitygates/select: HTTP 403: "
            f"Organization '{ORG}' is not allowed to modify Quality gates)",
        ]
        for language in CLOUD_LANGUAGES:
            prof = state.profile(language, "Bothy way")
            assert prof is not None, language
            assert prof.parent == "Sonar way comprehensive"
        checked = qs.check(_client(url, organization=ORG), config)
        assert checked.problems == []
        assert checked.gaps == [
            qs.GATE_NOT_DEFAULT,
            f"{qs.GATE_NOT_ON_PROJECT}: project uses the built-in gate 'Sonar way'",
        ] + [
            f"profile {lang}: {qs.PROFILE_NOT_ON_PROJECT}: project uses the built-in profile "
            "'Sonar way comprehensive'"
            for lang in CLOUD_LANGUAGES
        ]


def test_cloud_check_fails_when_project_uses_another_custom_setup() -> None:
    state = State(cloud=True, builtin_profile="Sonar way comprehensive")
    config = qs.load_config(REPO / "quality" / "sonar")
    with running(state) as url:
        qs.apply(_client(url, organization=ORG), config)
        state.gates["Other gate"] = []
        state.gate_ids["Other gate"] = "900"
        state.project_gate = "Other gate"
        state.project_profiles["go"] = "Sonar way comprehensive"
        state.profile("go", "Sonar way comprehensive").builtin = False  # type: ignore[union-attr]
        state.project_profiles["py"] = "Missing"
        assert qs.check(_client(url, organization=ORG), config).problems == [
            "gate 'Bothy way': project uses gate 'Other gate'",
            "profile go: project uses profile 'Sonar way comprehensive', want 'Bothy way'",
            "profile py: project uses no profile, want 'Bothy way'",
        ]


def test_cloud_permission_refusal_is_not_a_plan_gap() -> None:
    # A token without the permission gets HTTP 403 "Insufficient privileges": that fails.
    state = State(
        cloud=True, builtin_profile="Sonar way comprehensive", project_permission_denied=True
    )
    config = qs.load_config(REPO / "quality" / "sonar")
    with running(state) as url, pytest.raises(qs.SonarError, match="HTTP 403: Insufficient"):
        qs.apply(_client(url, organization=ORG), config)


def test_cloud_needs_a_project_key(server: tuple[State, str]) -> None:
    _, url = server
    config = qs.load_config(REPO / "quality" / "sonar")
    client = qs.Client(url, TOKEN, ORG)
    for run in (qs.apply, qs.check):
        with pytest.raises(qs.SonarError, match="project key"):
            run(client, config)


def test_server_mode_does_not_assign_to_a_project(server: tuple[State, str]) -> None:
    state, url = server
    config = qs.load_config(REPO / "quality" / "sonar")
    qs.apply(qs.Client(url, TOKEN, None, PROJECT), config)
    assert _posts(state, "/api/qualityprofiles/add_project") == []
    assert _posts(state, "/api/qualitygates/select") == []
    assert len(_posts(state, "/api/qualityprofiles/set_default")) == 6


def test_server_refusing_default_gate_still_fails() -> None:
    state = State(gate_default_forbidden=True)
    config = qs.load_config(REPO / "quality" / "sonar")
    with running(state) as url, pytest.raises(qs.SonarError, match="HTTP 403"):
        qs.apply(_client(url), config)


def test_check_reports_gate_not_default_on_server(server: tuple[State, str]) -> None:
    state, url = server
    config = qs.load_config(REPO / "quality" / "sonar")
    qs.apply(_client(url), config)
    state.default_gate = None
    report = qs.check(_client(url), config)
    assert report.problems == ["gate 'Bothy way' is not the default gate"]
    assert report.gaps == []


def test_error_names_the_server_message(server: tuple[State, str]) -> None:
    # A refusal is only diagnosable from the run log if the server's reason is printed.
    _, url = server
    with pytest.raises(qs.SonarError, match=r"HTTP 400: missing parameter$") as err:
        _client(url).post("api/qualitygates/create_condition", gateName="absent", metric="m")
    assert err.value.status == 400
    assert TOKEN not in str(err.value)


@pytest.mark.parametrize(
    "body", [b"", b"<html>bad gateway</html>", b"{}", b'{"errors": "x"}', b'{"errors": [{}]}']
)
def test_error_without_server_message_keeps_the_status_only(body: bytes) -> None:
    assert qs._error_messages(body) == ""


def test_error_message_is_shortened() -> None:
    body = json.dumps({"errors": [{"msg": "x" * 1000}]}).encode()
    assert qs._error_messages(body) == ": " + "x" * qs._MAX_ERROR_CHARS


def test_error_body_is_read_bounded() -> None:
    class Body:
        def __init__(self) -> None:
            self.asked: int | None = None

        def read(self, size: int = -1) -> bytes:
            self.asked = size
            return b'{"errors": [{"msg": "refused"}]}'

    body = Body()
    assert qs._read_error_body(body) == b'{"errors": [{"msg": "refused"}]}'
    assert body.asked == qs._MAX_ERROR_BODY_BYTES
