# SPDX-License-Identifier: Apache-2.0
"""Tests for the workflow policy check (runner switch, triggers, pinning, limits)."""

from __future__ import annotations

import textwrap
from pathlib import Path

import check_runners as cr
import pytest

REPO = Path(__file__).resolve().parents[3]
SHA = "3d3c42e5aac5ba805825da76410c181273ba90b1"
SWITCH = (
    '${{ github.event.repository.private && fromJSON(\'["self-hosted","bothy-ci"]\') '
    "|| 'ubuntu-24.04' }}"
)
GUARD = (
    "github.event.repository.visibility == 'public' || "
    "(github.event_name == 'schedule' && vars.BOTHY_VISIBILITY == 'public')"
)


def wf(body: str) -> str:
    return textwrap.dedent(body).lstrip()


def ok_job(extra: str = "") -> str:
    return wf(f"""
        on:
          pull_request:
            branches: [main]
        permissions:
          contents: read
        jobs:
          gates:
            runs-on: {SWITCH}
            timeout-minutes: 30
            steps:
              - uses: actions/checkout@{SHA} # v7.0.1
                with:
                  persist-credentials: false
              - run: make gates
        {extra}
    """)


def check(text: str, name: str = "ci.yml") -> list[str]:
    return [f.rule for f in cr.check_workflow(name, text)]


def test_compliant_workflow_passes() -> None:
    assert check(ok_job()) == []


def test_on_key_is_not_parsed_as_boolean() -> None:
    doc = cr.load_yaml(ok_job())
    assert "on" in doc
    assert True not in doc


def test_missing_permissions() -> None:
    text = ok_job().replace("permissions:\n  contents: read\n", "")
    assert check(text) == ["missing-permissions"]


def test_write_all_permissions() -> None:
    text = ok_job().replace("permissions:\n  contents: read\n", "permissions: write-all\n")
    assert check(text) == ["broad-permissions"]


def test_pull_request_target_forbidden() -> None:
    text = ok_job().replace("  pull_request:\n", "  pull_request_target:\n")
    assert "pull-request-target" in check(text)


@pytest.mark.parametrize(
    "trigger",
    [
        "on: push\n",
        "on: [push, pull_request]\n",
        "on:\n  push:\n    branches: [main]\n",
        "on:\n  push:\n    tags: ['v*']\n    branches: [main]\n",
        "on:\n  push:\n    paths: ['docs/**']\n",
    ],
)
def test_push_on_branches_forbidden(trigger: str) -> None:
    text = ok_job().replace("on:\n  pull_request:\n    branches: [main]\n", trigger)
    assert "push-on-branch" in check(text)


def test_push_on_tags_allowed() -> None:
    text = ok_job().replace(
        "on:\n  pull_request:\n    branches: [main]\n", "on:\n  push:\n    tags: ['server/v*']\n"
    )
    assert check(text) == []


@pytest.mark.parametrize(
    "uses",
    [
        "actions/checkout@v7.0.1",
        "actions/checkout@main",
        f"actions/checkout@{SHA[:12]}",
        "docker://alpine:3.22",
        "github/codeql-action/init@v4",
    ],
)
def test_unpinned_actions(uses: str) -> None:
    text = ok_job().replace(f"actions/checkout@{SHA} # v7.0.1", uses)
    assert check(text) == ["unpinned-action"]


@pytest.mark.parametrize(
    "uses",
    [
        f"github/codeql-action/init@{SHA}",
        "./.github/actions/setup",
        "$/.github/actions/setup",
        "docker://alpine@sha256:" + "a" * 64,
    ],
)
def test_pinned_and_local_actions(uses: str) -> None:
    text = ok_job().replace(f"actions/checkout@{SHA} # v7.0.1", uses)
    assert check(text) == []


def test_reusable_workflow_must_be_pinned_unless_local() -> None:
    text = wf("""
        on:
          push:
            tags: ['v*']
        permissions:
          contents: read
        jobs:
          gates:
            uses: ./.github/workflows/ci.yml
          remote:
            uses: example/workflows/.github/workflows/x.yml@v1
    """)
    assert check(text, "release.yml") == ["unpinned-action"]
    assert check(text.replace("@v1", f"@{SHA}"), "release.yml") == []


def test_missing_timeout() -> None:
    text = ok_job().replace("    timeout-minutes: 30\n", "")
    assert check(text) == ["missing-timeout"]


def test_reusable_call_job_needs_no_timeout() -> None:
    text = wf("""
        on:
          push:
            tags: ['v*']
        permissions:
          contents: read
        jobs:
          gates:
            uses: ./.github/workflows/ci.yml
    """)
    assert check(text, "release.yml") == []


@pytest.mark.parametrize("label", ["ubuntu-24.04", "ubuntu-latest", "macos-15", "windows-2025"])
def test_hosted_runner_without_switch_outside_release(label: str) -> None:
    text = ok_job().replace(SWITCH, label)
    assert check(text) == ["runner-without-switch"]


def test_hosted_runner_allowed_in_release_workflow() -> None:
    text = (
        ok_job()
        .replace(SWITCH, "windows-2025")
        .replace(
            "on:\n  pull_request:\n    branches: [main]\n",
            "on:\n  push:\n    tags: ['client/v*']\n",
        )
    )
    assert check(text, "release.yml") == []


@pytest.mark.parametrize("runs_on", ["self-hosted", "[self-hosted, bothy-ci]", "${{ matrix.os }}"])
def test_self_hosted_or_unknown_runner_needs_switch(runs_on: str) -> None:
    text = ok_job().replace(SWITCH, runs_on)
    assert check(text) == ["runner-without-switch"]


def test_inverted_switch_is_rejected() -> None:
    inverted = (
        "${{ github.event.repository.private && 'ubuntu-24.04' || "
        'fromJSON(\'["self-hosted","bothy-ci"]\') }}'
    )
    assert check(ok_job().replace(SWITCH, inverted)) == ["runner-without-switch"]


def test_macos_switch_variant() -> None:
    mac = (
        "${{ github.event.repository.private && "
        'fromJSON(\'["self-hosted","bothy-ci","macOS"]\') || \'macos-15\' }}'
    )
    assert check(ok_job().replace(SWITCH, mac)) == []


def test_public_only_job_may_use_hosted_runner() -> None:
    text = ok_job().replace(
        f"    runs-on: {SWITCH}\n", f"    if: {GUARD}\n    runs-on: ubuntu-24.04\n"
    )
    assert check(text) == []


def matrix_job(runners: str, condition: str = GUARD) -> str:
    """A job that picks its runner from a matrix (the native build per architecture)."""
    guard = f"    if: {condition}\n" if condition else ""
    return ok_job().replace(
        f"    runs-on: {SWITCH}\n",
        f"{guard}    strategy:\n      matrix:\n        runner: {runners}\n"
        "    runs-on: ${{ matrix.runner }}\n",
    )


def test_public_only_matrix_of_hosted_runners() -> None:
    assert check(matrix_job("[ubuntu-24.04, ubuntu-24.04-arm]")) == []


@pytest.mark.parametrize(
    "runners", ["[ubuntu-24.04, self-hosted]", "[]", "ubuntu-24.04", "[ubuntu-24.04, 3]"]
)
def test_matrix_runner_values_must_be_hosted_labels(runners: str) -> None:
    assert check(matrix_job(runners)) == ["runner-without-switch"]


def test_matrix_of_hosted_runners_needs_the_public_guard() -> None:
    assert check(matrix_job("[ubuntu-24.04, ubuntu-24.04-arm]", condition="")) == [
        "runner-without-switch"
    ]


def test_matrix_runner_key_must_exist() -> None:
    text = matrix_job("[ubuntu-24.04]").replace("matrix.runner }}", "matrix.other }}")
    assert check(text) == ["runner-without-switch"]


def test_public_guard_must_not_be_weakened() -> None:
    weak = f"{GUARD} || true"
    text = ok_job().replace(
        f"    runs-on: {SWITCH}\n", f"    if: {weak}\n    runs-on: ubuntu-24.04\n"
    )
    assert check(text) == ["runner-without-switch"]


def test_public_guard_with_extra_condition() -> None:
    cond = f"({GUARD}) && github.event_name != 'workflow_dispatch'"
    text = ok_job().replace(
        f"    runs-on: {SWITCH}\n", f"    if: {cond}\n    runs-on: ubuntu-24.04\n"
    )
    assert check(text) == []


def test_schedule_requires_public_guard_on_every_job() -> None:
    text = ok_job().replace(
        "on:\n  pull_request:\n    branches: [main]\n",
        "on:\n  schedule:\n    - cron: '0 3 * * 1'\n",
    )
    assert check(text) == ["schedule-without-public-guard"]


def test_release_workflow_triggers_are_limited() -> None:
    text = ok_job()  # pull_request trigger
    assert "release-trigger" in check(text, "release.yml")


def release_job(step: str, environment: str = "") -> str:
    """A tag-triggered release workflow with one job that runs `step`."""
    env_line = f"\n    environment: {environment}" if environment else ""
    return (
        "on:\n  push:\n    tags: ['server/v*']\npermissions:\n  contents: read\njobs:\n"
        f"  sign:{env_line}\n    runs-on: ubuntu-24.04\n    timeout-minutes: 10\n"
        f"    steps:\n      - {step}\n"
    )


SIGNING_STEPS = [
    "run: cosign sign --yes --key env://COSIGN_PRIVATE_KEY img@sha256:00",
    "run: |\n          echo start\n          cosign sign-blob --bundle b.json SHA256SUMS",
    "run: uv run --project tools python tools/ci/release_chain.py sign --dist dist",
    f"uses: actions/attest@{SHA} # v4.2.2",
]


@pytest.mark.parametrize("step", SIGNING_STEPS)
def test_signing_step_outside_the_release_environment(step: str) -> None:
    assert check(release_job(step), "release.yml") == ["signing-outside-release"]
    assert check(release_job(step, "release-apple"), "release.yml") == ["signing-outside-release"]


@pytest.mark.parametrize("environment", ["release", "Release ", "{name: release}"])
@pytest.mark.parametrize("step", SIGNING_STEPS)
def test_signing_step_in_the_release_environment(step: str, environment: str) -> None:
    assert check(release_job(step, environment), "release.yml") == []


def test_signing_rule_applies_to_every_workflow() -> None:
    text = ok_job().replace("- run: make gates", "- run: cosign sign-blob --bundle x.json x")
    assert check(text) == ["signing-outside-release"]


@pytest.mark.parametrize(
    "step",
    [
        "run: cosign verify --key k.pub img@sha256:00",
        "run: cosign verify-blob --key k.pub --bundle b.json f",
        "run: echo cosign-signing-config",
    ],
)
def test_verification_is_not_signing(step: str) -> None:
    assert check(release_job(step), "release.yml") == []


def test_invalid_yaml_is_reported() -> None:
    assert check("on: [unclosed\n") == ["invalid-yaml"]


def test_main_reports_and_fails(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    (tmp_path / "bad.yml").write_text(ok_job().replace("    timeout-minutes: 30\n", ""))
    (tmp_path / "good.yaml").write_text(ok_job())
    rc = cr.main([str(tmp_path)])
    out = capsys.readouterr().out
    assert rc == 1
    assert "bad.yml: job 'gates': missing-timeout" in out
    assert "good.yaml" not in out


def test_main_fails_on_empty_directory(tmp_path: Path) -> None:
    assert cr.main([str(tmp_path)]) == 2


def test_repository_workflows_pass() -> None:
    """The gate itself: the real workflow files satisfy the policy."""
    assert cr.main([str(REPO / ".github" / "workflows")]) == 0
