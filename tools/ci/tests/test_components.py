# SPDX-License-Identifier: Apache-2.0
"""Tests for the component map (CI path filter and release tag resolution)."""

from __future__ import annotations

import json
import re
import subprocess
from pathlib import Path

import components
import pytest


@pytest.mark.parametrize(
    ("files", "expected"),
    [
        # The real-server tests of both SDKs and the live test of the GitHub app build and run
        # the server of this tree (services/server/test/sdkserver, M2-29, M2-30 and M2-49), so a
        # server change runs them too.
        (
            ["services/server/internal/chat/x.go"],
            ["plugins", "sdk-python", "sdk-typescript", "server"],
        ),
        (
            ["services/server/internal/runs/app/patch.go"],
            ["plugins", "sdk-python", "sdk-typescript", "server"],
        ),
        (
            ["services/server/test/sdkserver/sdkserver_test.go"],
            ["plugins", "sdk-python", "sdk-typescript", "server"],
        ),
        (["apps/client/lib/main.dart", "packages/design_system/lib/a.dart"], ["apps"]),
        (["design/tokens/color.json"], ["apps"]),
        # The root files of the Dart workspace and its architecture test run the Dart gates
        # (docs/DECISIONS.md D-M4-01-8).
        (["pubspec.yaml"], ["apps"]),
        (["pubspec.lock"], ["apps"]),
        (["analysis_options.yaml"], ["apps"]),
        (["tools/arch-check/rules.yaml"], ["apps"]),
        (["tools/arch-check/arch_check.py", "packages/domain/lib/domain.dart"], ["apps"]),
        (["tools/arch-checker/x.py"], ["all"]),
        (["packages/pubspec.yaml.bak"], ["apps"]),
        (["docs/pubspec.yaml"], []),
        # The Dart tests of packages/bothy_noise read the shared Noise test data of the host
        # service, so a change there runs the Go and the Dart gates (docs/DECISIONS.md D-M2-38-5).
        (["services/host/testdata/noise/recordings/go.json"], ["apps", "server"]),
        (["services/host/internal/noise/noise.go"], ["server"]),
        # The Hermes adapter imports the SDK from its source tree (M2-33); the agent conformance
        # suite of the server runs the Fake-Agent of the SDK (M2-31).
        (["sdk/python/src/x.py"], ["plugins", "sdk-python", "server"]),
        (["sdk/python/src/bothy_sdk/testing/fake_agent.py"], ["plugins", "sdk-python", "server"]),
        # The Jira app uses the whole SDK project as an editable path source and locks its
        # dependencies (M2-50): a change outside src/ selects the plugins too.
        (["sdk/python/pyproject.toml"], ["plugins", "sdk-python", "server"]),
        (["sdk/python/uv.lock"], ["plugins", "sdk-python", "server"]),
        # The GitHub app links the TypeScript SDK as a local package (M2-49).
        (["sdk/typescript/src/x.ts"], ["plugins", "sdk-typescript"]),
        (["plugins/hermes-adapter/x.py"], ["plugins"]),
        # Shared G5 fixtures of both SDKs (docs/DECISIONS.md M2-23/6).
        (["sdk/testdata/tolerance/unknown_field.json"], ["sdk-python", "sdk-typescript"]),
        (["docs/api/openapi.yaml"], ["apps", "plugins", "sdk-python", "sdk-typescript", "server"]),
        # The server generates its route table and action catalogue from the action catalogue
        # (services/server/api/generate.go); `make catalog` also rewrites chapter 05.
        (["docs/spec/actions.yaml"], ["server"]),
        (["docs/spec/actions.yaml", "docs/spec/05-actions.md"], ["server"]),
        (["docs/spec/05-actions.md"], []),
        # The terminal suite of the server starts the Compose stack with its test overlays and
        # the collector configuration, so infra/compose/ and infra/otel/ run the server gates
        # (brief M2-48 Design note 14).
        (
            ["docs/spec/07-permissions.md", "infra/compose/compose.yaml", "README.md"],
            ["plugins", "server"],
        ),
        (["docs/spec/07-permissions.md", "infra/compose/secrets.yaml", "README.md"], ["server"]),
        (["infra/compose/test/compose.terminal.yaml"], ["server"]),
        (["infra/otel/collector.yaml"], ["server"]),
        # The panel plugin reads the TypeScript tokens, its dashboard and the Grafana pin (M7-25).
        (["design/build/ts/tokens.gen.ts"], ["apps", "plugins"]),
        (["infra/observability/grafana/dashboards/bothy-live.json"], ["plugins"]),
        (["infra/observability/grafana/dashboards/push.json"], []),
        # Chapter 09 is the contract of the adapters; its 17.10 is bound to their evidence.
        (["docs/spec/09-agents.md"], ["plugins"]),
        # The guide's check run is a server test; its example belongs to the Python SDK (M2-37).
        (["docs/guides/connect-agent.md"], ["sdk-python", "server"]),
        (["docs/spec/11-extensions.md"], ["sdk-python", "server"]),
        (["sdk/python/examples/guide/agent.py"], ["plugins", "sdk-python", "server"]),
        # The SLI bounds generator and the Sloth files feed generated files of the server and
        # of bothy_viz (brief M6-11); infra/ outside slo/ selects nothing.
        (["tools/observability/sli_bounds.py"], ["apps", "server"]),
        (["infra/observability/slo/api.yaml"], ["apps", "server"]),
        (["infra/observability/prometheus/rules/slo-api.yaml"], []),
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
    (path.parent / "CHANGELOG.md").write_text(
        f"# Changelog\n\n## [Unreleased]\n\n## [{version}] - 2026-10-15\n\n- Entry.\n"
    )
    return tmp_path


def test_release_rejects_a_missing_changelog_section(tmp_path: Path) -> None:
    repo = _repo_with_version(tmp_path, "services/server/VERSION", "1.4.0")
    changelog = repo / "services/server/CHANGELOG.md"
    changelog.write_text("# Changelog\n\n## [Unreleased]\n\n- Entry.\n\n## [1.4.0-beta.1]\n")
    with pytest.raises(components.ReleaseError, match=r"no section \[1\.4\.0\]"):
        components.release_info("server/v1.4.0", repo)
    changelog.unlink()
    with pytest.raises(components.ReleaseError, match=r"CHANGELOG\.md is missing"):
        components.release_info("server/v1.4.0", repo)


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
    assert capsys.readouterr().out.strip() == "plugins sdk-python server"


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


# --- server version pinned by the product bundle (docs/spec/18-release.md 2.1) --------------

REPO = Path(__file__).resolve().parents[3]


def _bundle_repo(tmp_path: Path, product: str, lines: str) -> Path:
    tmp_path.mkdir(parents=True, exist_ok=True)
    (tmp_path / "VERSION").write_text(product + "\n")
    env = tmp_path / "infra" / "compose" / ".env.example"
    env.parent.mkdir(parents=True)
    env.write_text(f"# template\nBOTHY_REGISTRY=ghcr.io/OWNER\n{lines}\nBOTHY_ENVIRONMENT=prod\n")
    return tmp_path


@pytest.mark.parametrize(
    ("product", "server"),
    [
        ("1.0.0", "2.0.0"),
        ("1.1.0-beta.1", "2.0.0-beta.1"),
        ("1.1.0-beta.1", "2.0.0"),
        # the template carries the placeholder until the first product release; release-prep
        # and release-build refuse it there
        ("0.0.0", "0.0.0-placeholder"),
    ],
)
def test_pins_accept(tmp_path: Path, product: str, server: str) -> None:
    repo = _bundle_repo(tmp_path, product, f"BOTHY_VERSION={server}")
    assert components.pinned_server_problems(repo) == []


@pytest.mark.parametrize(
    ("product", "lines", "message"),
    [
        ("1.0.0", "BOTHY_VERSION=2.0.0-beta.1", r"stable product 1\.0\.0 pins .*2\.0\.0-beta\.1"),
        ("1.0.0", "BOTHY_VERSION=2.0.0-rc.1", r"2\.0\.0-rc\.1"),
        ("1.0.0", "BOTHY_VERSION=v2.0.0", r"'v2\.0\.0' is not a SemVer version"),
        ("1.0.0", "BOTHY_VERSION=latest", r"'latest' is not a SemVer version"),
        ("1.0.0", "# BOTHY_VERSION=2.0.0", r"exactly one line BOTHY_VERSION=, found 0"),
        ("1.0.0", "BOTHY_VERSION=2.0.0\nBOTHY_VERSION=2.0.1", r"found 2"),
        ("1.0", "BOTHY_VERSION=2.0.0", r"VERSION says '1\.0'"),
        # only a product beta may pin a server pre-release (CHANNEL=beta gives -beta.N)
        ("1.1.0-rc.1", "BOTHY_VERSION=2.0.0-beta.1", r"product 1\.1\.0-rc\.1 is not a beta"),
    ],
)
def test_pins_reject(tmp_path: Path, product: str, lines: str, message: str) -> None:
    repo = _bundle_repo(tmp_path, product, lines)
    problems = components.pinned_server_problems(repo)
    assert len(problems) == 1
    assert re.search(message, problems[0])


def test_pins_need_the_files(tmp_path: Path) -> None:
    problems = components.pinned_server_problems(tmp_path)
    assert any("VERSION is missing" in p for p in problems)
    assert any(".env.example is missing" in p for p in problems)


def test_pins_of_this_repository() -> None:
    """The gate itself: the committed product version and template pass."""
    assert components.pinned_server_problems(REPO) == []


def test_cli_pins(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    good = _bundle_repo(tmp_path / "good", "1.0.0", "BOTHY_VERSION=2.0.0")
    assert components.main(["pins", "--repo", str(good)]) == 0
    bad = _bundle_repo(tmp_path / "bad", "1.0.0", "BOTHY_VERSION=2.0.0-beta.3")
    assert components.main(["pins", "--repo", str(bad)]) == 1
    assert "2.0.0-beta.3" in capsys.readouterr().err


# --- the CI jobs a change needs (ci_plan) ------------------------------------------------

TESTS_SERVER = {"test-server"}
POSTGRES = {"postgres-image-amd64", "postgres-image-arm64"}
DESKTOP = {"apps-macos", "apps-ios", "apps-windows"}
ANDROID = {"apps-android-26", "apps-android-36"}
PLATFORM = DESKTOP | ANDROID
CONSOLE = {"console-e2e"}
# The real-server tests of the Python SDK build the server of this tree (M2-29).
SDK = {"test-sdk-plugins"}
TRIALS = {"desktop-trial-macos", "desktop-trial-windows"}
# The platform trials of the host packages follow the host service and its release steps (M7-23).
HOST_TRIALS = {"host-trial-macos", "host-trial-windows"}
# The phone trials follow the release steps and the files of the phone builds (M7-13).
MOBILE = {"mobile-trial-ios", "mobile-trial-android"}
# Every job a full run starts: the distribution trials only follow their own files (M7-12).
EVERY_JOB = set(components.CONDITIONAL_JOBS) - TRIALS - HOST_TRIALS - MOBILE


@pytest.mark.parametrize(
    ("files", "expected"),
    [
        # Server code inside the build context of bothy-postgres runs the image jobs.
        (["services/server/internal/chat/x.go"], TESTS_SERVER | POSTGRES | CONSOLE | SDK),
        (["services/server/cmd/bothy-backupd/main.go"], TESTS_SERVER | POSTGRES | CONSOLE | SDK),
        (["services/server/images/bothy-postgres/Dockerfile"], TESTS_SERVER | POSTGRES | SDK),
        (["services/server/images/bothy-postgres/entrypoint.sh"], TESTS_SERVER | POSTGRES | SDK),
        (
            ["services/server/test/postgresimage/postgresimage_test.go"],
            TESTS_SERVER | POSTGRES | SDK,
        ),
        (["services/server/migrations/00099_x.sql"], TESTS_SERVER | POSTGRES | CONSOLE | SDK),
        (["services/server/go.mod"], TESTS_SERVER | POSTGRES | CONSOLE | SDK),
        (["services/server/VERSION"], TESTS_SERVER | POSTGRES | CONSOLE | SDK),
        # The console test runs for the build context of the server image and the Compose
        # stack of the test (M6-03), not for other server or app files.
        (["apps/console/lib/main.dart"], {"test-apps"} | PLATFORM | CONSOLE),
        (["services/server/test/smoke/seed.go"], TESTS_SERVER | CONSOLE | SDK),
        (["services/server/Dockerfile"], TESTS_SERVER | CONSOLE | SDK),
        # infra/compose/ also runs the server tests: the terminal suite starts that stack (M2-48).
        (["infra/compose/test/compose.console-e2e.yaml"], TESTS_SERVER | CONSOLE),
        (["infra/compose/test/compose.console-e2e-grafana.yaml"], TESTS_SERVER | CONSOLE),
        (["tools/console-e2e/console_e2e.py"], EVERY_JOB),
        # Server code outside the build context of bothy-postgres does not run its image jobs.
        (["services/server/api/rest/handler.go"], TESTS_SERVER | CONSOLE | SDK),
        (["services/server/cmd/bothy/main.go"], TESTS_SERVER | CONSOLE | SDK),
        (["services/server/test/appsworld/scenarios_api.go"], TESTS_SERVER | SDK),
        (["services/host/internal/noise/noise.go"], TESTS_SERVER | HOST_TRIALS),
        (["docs/spec/actions.yaml"], TESTS_SERVER),
        # Members of the Dart workspace (with their native code) and its root files run the
        # platform jobs; inputs of generated Dart files that are committed under packages/ do not.
        (["apps/client/lib/main.dart"], {"test-apps"} | PLATFORM),
        (["packages/device_key/darwin/Sources/core.swift"], {"test-apps"} | PLATFORM | CONSOLE),
        (["packages/device_key/android/core/src/main/x.kt"], {"test-apps"} | PLATFORM | CONSOLE),
        (["packages/device_key/windows/core/core.cpp"], {"test-apps"} | PLATFORM | CONSOLE),
        (["apps/client/platform-tests.json"], {"test-apps"} | PLATFORM),
        (["pubspec.yaml"], {"test-apps"} | PLATFORM | CONSOLE),
        (["pubspec.lock"], {"test-apps"} | PLATFORM | CONSOLE),
        (["analysis_options.yaml"], {"test-apps"} | CONSOLE),
        (["design/tokens/color.json"], {"test-apps"}),
        (["tools/arch-check/rules.yaml"], {"test-apps"}),
        (
            ["services/host/testdata/noise/recordings/go.json"],
            {"test-apps", "test-server"} | HOST_TRIALS,
        ),
        (["tools/observability/sli_bounds.py"], {"test-apps", "test-server"}),
        # SDKs and plugins.
        (["sdk/python/src/x.py"], {"test-sdk-plugins", "test-server"}),
        (["sdk/typescript/src/x.ts"], {"test-sdk-plugins"}),
        (["plugins/hermes-adapter/x.py"], {"test-sdk-plugins"}),
        (["plugins/hermes-adapter/pyproject.toml"], {"test-sdk-plugins"}),
        # Documentation needs no conditional job; test (tools) runs on every change.
        (["docs/spec/07-permissions.md"], set()),
        # The terminal suite of the server starts the Compose stack and its collector (M2-48).
        (["README.md", "infra/compose/secrets.yaml"], TESTS_SERVER),
        (["infra/otel/collector.yaml"], TESTS_SERVER),
        # The Grafana pin of the Compose stack is an input of the panel plugin's tests (M7-25).
        (
            ["README.md", "infra/compose/compose.yaml"],
            {"test-sdk-plugins"} | TESTS_SERVER | CONSOLE,
        ),
        (
            ["services/server/internal/x.go", "docs/DECISIONS.md"],
            TESTS_SERVER | POSTGRES | CONSOLE | SDK,
        ),
        # Shared inputs run every job: the API contract, the Makefile, tools, workflows, root
        # files outside the component map, and an empty or unknown diff.
        (["docs/api/openapi.yaml"], EVERY_JOB),
        (["Makefile"], EVERY_JOB),
        (["tools/ci/components.py"], EVERY_JOB),
        (["tools/uv.lock"], EVERY_JOB),
        ([".github/workflows/ci.yml"], EVERY_JOB),
        (["sonar-project.properties"], EVERY_JOB),
        (["some-new-top-level.toml"], EVERY_JOB),
        ([], EVERY_JOB),
    ],
)
def test_ci_plan_selects_the_jobs_a_change_needs(files: list[str], expected: set[str]) -> None:
    assert set(components.ci_plan(files=files, public=True).jobs) == expected


def test_ci_plan_keeps_the_components_of_the_map() -> None:
    assert components.ci_plan(files=["services/host/x.go"], public=True).components == ["server"]
    assert components.ci_plan(files=["docs/x.md"], public=True).components == []
    assert components.ci_plan(files=["Makefile"], public=True).components == ["all"]
    assert components.ci_plan(files=[], public=True).components == ["all"]


def test_every_conditional_job_is_triggered_by_a_change_of_its_component() -> None:
    triggered: set[str] = set()
    for example in (
        "services/server/internal/x.go",
        "apps/client/lib/main.dart",
        "sdk/python/src/x.py",
        "tools/distribution/apple.py",
        "infra/host/linux/nfpm.yaml",
    ):
        triggered |= set(components.ci_plan(files=[example], public=True).jobs)
    assert triggered == set(components.CONDITIONAL_JOBS)


@pytest.mark.parametrize(
    ("files", "expected"),
    [
        # The host packages reuse the signing steps of the desktop chain (M7-23).
        (["tools/distribution/apple.py"], TRIALS | MOBILE | HOST_TRIALS),
        (["apps/distribution/distribution.yaml"], {"test-apps"} | PLATFORM | TRIALS | MOBILE),
        (["apps/client/Makefile"], {"test-apps"} | PLATFORM | TRIALS | MOBILE),
        (["apps/client/macos/Runner/AppDelegate.swift"], {"test-apps"} | PLATFORM | TRIALS),
        ([".github/workflows/release.yml"], EVERY_JOB | TRIALS | MOBILE | HOST_TRIALS),
        # The phone builds (M7-13): the release steps of iOS and Android, the Gradle build and
        # the Xcode project run the phone trials only.
        (["tools/distribution/android.py"], TRIALS | MOBILE | HOST_TRIALS),
        (["apps/client/android/app/build.gradle.kts"], {"test-apps"} | PLATFORM | MOBILE),
        (["apps/client/ios/Runner.xcodeproj/project.pbxproj"], {"test-apps"} | PLATFORM | MOBILE),
        # The host release steps and package recipes run the host trials and the host tests,
        # which read them, not the desktop trials.
        (["tools/distribution/hostdist.py"], TESTS_SERVER | HOST_TRIALS),
        (["tools/distribution/tests/test_hostdist.py"], HOST_TRIALS),
        (["infra/host/windows/bothy-host.wxs"], TESTS_SERVER | HOST_TRIALS),
        (["services/host/Makefile"], TESTS_SERVER | HOST_TRIALS),
        # A full run for a shared input, and any other app change, leave the trials out.
        (["Makefile"], EVERY_JOB),
        (["apps/client/lib/main.dart"], {"test-apps"} | PLATFORM),
    ],
)
def test_the_distribution_trials_follow_their_files(files: list[str], expected: set[str]) -> None:
    assert set(components.ci_plan(files=files, public=True).jobs) == expected


def test_ci_plan_leaves_android_out_while_the_repository_is_private() -> None:
    private = set(components.ci_plan(files=["apps/client/lib/main.dart"], public=False).jobs)
    assert private == {"test-apps"} | DESKTOP
    assert set(components.ci_plan(files=["Makefile"], public=False).jobs) == EVERY_JOB - ANDROID


@pytest.mark.parametrize(
    ("selected", "expected"),
    [
        # A push to main, a manual run and the release gate of the product: every job.
        ("all", EVERY_JOB),
        # The release gate of one component (release.yml): its jobs.
        ("server", TESTS_SERVER | POSTGRES | CONSOLE),
        ("apps", {"test-apps"} | PLATFORM | CONSOLE),
        ("plugins", {"test-sdk-plugins"}),
        ("sdk-python", {"test-sdk-plugins"}),
    ],
)
def test_ci_plan_without_a_diff_follows_the_components(selected: str, expected: set[str]) -> None:
    plan = components.ci_plan(selected=selected.split(), public=True)
    assert set(plan.jobs) == expected
    assert plan.components == selected.split()


def test_ci_plan_rejects_an_unknown_component() -> None:
    with pytest.raises(ValueError, match="unknown component 'nope'"):
        components.ci_plan(selected=["nope"], public=True)


def test_ci_plan_needs_exactly_one_source() -> None:
    with pytest.raises(ValueError, match="files or selected"):
        components.ci_plan(public=True)
    with pytest.raises(ValueError, match="files or selected"):
        components.ci_plan(files=[], selected=["all"], public=True)


def test_cli_ci_plan(capsys: pytest.CaptureFixture[str]) -> None:
    files = ["services/host/x.go"]
    assert components.main(["ci-plan", "--files", *files, "--public", "true"]) == 0
    assert (
        capsys.readouterr().out
        == "components=server\njobs=host-trial-macos host-trial-windows test-server\n"
    )
    assert components.main(["ci-plan", "--files", "docs/x.md", "--public", "false"]) == 0
    assert capsys.readouterr().out == "components=none\njobs=none\n"
    assert components.main(["ci-plan", "--components", "all", "--public", "false"]) == 0
    out = capsys.readouterr().out
    assert out.startswith("components=all\njobs=")
    assert "apps-android-26" not in out


def test_cli_ci_plan_writes_none_when_no_conditional_job_runs(
    monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    # The value is never empty, so that ci-ok can tell a missing plan from an empty one.
    monkeypatch.setattr(components, "CONDITIONAL_JOBS", ())
    assert components.main(["ci-plan", "--files", "docs/x.md", "--public", "true"]) == 0
    assert capsys.readouterr().out.endswith("jobs=none\n")


def test_cli_ci_plan_from_a_diff(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    import subprocess

    def git(*args: str) -> str:
        return subprocess.run(
            ["git", "-C", str(tmp_path), *args], check=True, capture_output=True, text=True
        ).stdout.strip()

    identity = ("-c", "user.name=Alex", "-c", "user.email=alex@example.com")
    git("init", "-q", "-b", "main")
    git(*identity, "commit", "-q", "--allow-empty", "-m", "base")
    base = git("rev-parse", "HEAD")
    (tmp_path / "sdk" / "python").mkdir(parents=True)
    (tmp_path / "sdk" / "python" / "x.py").write_text("x = 1\n")
    git("add", ".")
    git(*identity, "commit", "-q", "-m", "x")
    head = git("rev-parse", "HEAD")
    monkeypatch.chdir(tmp_path)
    assert components.main(["ci-plan", "--base", base, "--head", head, "--public", "true"]) == 0
    assert (
        capsys.readouterr().out
        == "components=plugins sdk-python server\njobs=test-sdk-plugins test-server\n"
    )


def test_component_source_files() -> None:
    assert components.is_component_source("services/server/internal/x.go")
    assert components.is_component_source("packages/device_key/darwin/x.swift")
    assert not components.is_component_source("services/server/go.mod")
    assert not components.is_component_source("tools/ci/components.py")
    assert not components.is_component_source("docs/x.md")


def test_the_tools_tests_are_no_conditional_job() -> None:
    """Tools tests also cross build component sources (release builds), so they always run."""
    assert "test-tools" not in components.CONDITIONAL_JOBS


def test_a_renamed_file_counts_with_its_old_path(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    import subprocess

    def git(*args: str) -> str:
        return subprocess.run(
            ["git", "-C", str(tmp_path), *args], check=True, capture_output=True, text=True
        ).stdout.strip()

    identity = ("-c", "user.name=Alex", "-c", "user.email=alex@example.com")
    git("init", "-q", "-b", "main")
    old = tmp_path / "services" / "server" / "internal" / "x.go"
    old.parent.mkdir(parents=True)
    old.write_text("package x\n\nfunc X() int { return 1 }\n")
    git("add", ".")
    git(*identity, "commit", "-q", "-m", "base")
    base = git("rev-parse", "HEAD")
    (tmp_path / "services" / "server" / "api").mkdir(parents=True)
    git("mv", "services/server/internal/x.go", "services/server/api/x.go")
    git(*identity, "commit", "-q", "-m", "move")
    head = git("rev-parse", "HEAD")
    monkeypatch.chdir(tmp_path)
    assert components.main(["ci-plan", "--base", base, "--head", head, "--public", "true"]) == 0
    # The image lost a file of its build context, so the image jobs run.
    assert "postgres-image-amd64" in capsys.readouterr().out


# --- tool folders (docs/quality/gates.md, "Jobs a change needs") -----------------------------

ALL_COMPONENTS = ["apps", "plugins", "sdk-python", "sdk-typescript", "server"]


@pytest.mark.parametrize(
    ("path", "expected"),
    [
        # Read only by repository-wide gates and the tools tests, which run on every change.
        ("tools/configdoc/planned.yaml", []),
        ("tools/configdoc/configdoc.py", []),
        ("tools/catalog/action_catalog.py", []),
        ("tools/catalog/event_catalog_dart.py", []),
        ("tools/docs/language_check.py", []),
        ("tools/docs/tests/test_config_values.py", []),
        ("tools/doccheck/register_check.py", []),
        ("tools/opscheck/opscheck.py", []),
        ("tools/denylist/generic.rules", []),
        ("tools/scancode/Dockerfile", []),
        ("tools/sonarqube/quality_sync.py", []),
        ("tools/testing/git_fixtures.py", []),
        ("tools/api-checks/api_checks.py", []),
        ("tools/README.md", []),
        # make api-breaking runs only for the component server.
        ("tools/api-checks/breaking.py", ["server"]),
        # The server test of pgtest reads the image pin of the local SonarQube.
        ("tools/sonarqube/compose.yaml", ["server"]),
        # The token generator writes the committed Dart tokens and icons, like design/.
        ("tools/design-tokens/tokens.py", ["apps", "server"]),
        # The server's designkeys generator and tests read the resolved tokens.
        ("design/build/tokens.resolved.json", ["apps", "server"]),
        ("design/tokens/color.json", ["apps"]),
        ("tools/arch-check/rules.yaml", ["apps"]),
        ("tools/observability/sli_bounds.py", ["apps", "server"]),
    ],
)
def test_a_tool_folder_selects_the_components_that_read_it(path: str, expected: list[str]) -> None:
    assert components.changed([path]) == expected


@pytest.mark.parametrize(
    "path",
    [
        "tools/uv.lock",
        "tools/pyproject.toml",
        "tools/conftest.py",
        "tools/.python-version",
        "tools/bin/tools.lock.json",
        "tools/bin/fetch_tools.py",
        "tools/node/package-lock.json",
        "tools/node/package.json",
        "tools/ci/components.py",
        "tools/ci/ci_ok.py",
        "tools/ci/tests/test_components.py",
        "tools/hooks/push_plan.py",
        "tools/hooks/commit_msg.py",
        # A new tool folder is shared until the map assigns it.
        "tools/newtool/x.py",
        "tools/configdocs/x.py",
        "tools/sonarqube",
    ],
)
def test_shared_tool_inputs_still_select_everything(path: str) -> None:
    assert components.changed([path]) == ["all"]
    plan = components.ci_plan(files=[path], public=True)
    assert plan.jobs == sorted(EVERY_JOB)


def _tracked_tool_entries() -> set[str]:
    repo = Path(__file__).resolve().parents[3]
    out = subprocess.run(
        ["git", "-C", str(repo), "ls-files", "--", "tools/"],
        check=True,
        capture_output=True,
        text=True,
    ).stdout
    return {line.split("/")[1] for line in out.splitlines() if line.count("/") >= 2}


def test_every_tool_folder_is_assigned_or_shared() -> None:
    """A tool folder the map does not name stays shared; this list makes each choice explicit."""
    assigned = {key.split("/")[1] for key in components.TOOL_COMPONENTS}
    assigned |= {"arch-check", "observability"}
    tracked = _tracked_tool_entries()
    assert tracked - assigned == set(components.SHARED_TOOL_FOLDERS)
    for folder in components.SHARED_TOOL_FOLDERS:
        assert components.changed([f"tools/{folder}/x"]) == ["all"], folder


@pytest.mark.parametrize(
    ("files", "expected"),
    [
        # The feature pull request that adds a planned configuration key with server code.
        (
            ["tools/configdoc/planned.yaml", "services/host/internal/x.go"],
            {"test-server"} | HOST_TRIALS,
        ),
        (["tools/configdoc/planned.yaml", "docs/operations/configuration.md"], set()),
        (["tools/sonarqube/compose.yaml"], {"test-server"}),
        (["tools/design-tokens/tokens.py"], {"test-apps", "test-server"}),
        (["tools/api-checks/breaking.py"], {"test-server"}),
        (["tools/contract/auth.py"], {"test-server"}),
        (["tools/lint/sql_visibility.yaml"], {"test-server"}),
        (["tools/catalog/action_catalog.py", "tools/uv.lock"], EVERY_JOB),
    ],
)
def test_ci_plan_of_tool_changes(files: list[str], expected: set[str]) -> None:
    assert set(components.ci_plan(files=files, public=True).jobs) == expected


def test_tool_components_name_known_components() -> None:
    for key, owners in components.TOOL_COMPONENTS.items():
        assert key.startswith("tools/"), key
        assert set(owners) <= set(components.GATE_COMPONENTS), key
