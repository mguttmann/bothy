# SPDX-License-Identifier: Apache-2.0
"""Tests for the workflow policy check (GitHub-hosted runners, triggers, pinning, limits)."""

from __future__ import annotations

import textwrap
from pathlib import Path

import check_runners as cr
import pytest

REPO = Path(__file__).resolve().parents[3]
SHA = "3d3c42e5aac5ba805825da76410c181273ba90b1"
HOSTED = "ubuntu-24.04"
POST_MERGE = "on:\n  push:\n    branches: [main]\n"
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
            runs-on: {HOSTED}
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


@pytest.mark.parametrize(
    "label", ["ubuntu-24.04", "ubuntu-24.04-arm", "macos-15", "windows-2025", "windows-11-arm"]
)
@pytest.mark.parametrize("name", ["ci.yml", "denylist-main.yml"])
def test_fixed_github_hosted_label_passes_in_every_workflow(label: str, name: str) -> None:
    """ADR 0039: every job of every workflow runs on a GitHub-hosted runner."""
    text = ok_job().replace(f"runs-on: {HOSTED}", f"runs-on: {label}")
    if name == "denylist-main.yml":
        text = text.replace("on:\n  pull_request:\n    branches: [main]\n", POST_MERGE)
    assert check(text, name) == []


@pytest.mark.parametrize(
    "runs_on",
    [
        "ubuntu-latest",  # moves to a new image without a change in the repository
        "macos-latest",
        "ubuntu-24.04-4core",  # larger runner: not a standard runner
        "macos-15-xlarge",
        "linux",  # label of a runner outside GitHub
        "[ubuntu-24.04, gpu]",
        "[ubuntu-24.04]",
        "{group: build}",
        "${{ inputs.runner }}",
        '${{ github.event.repository.private && fromJSON(\'["linux","x64"]\') '
        "|| 'ubuntu-24.04' }}",
        "${{ matrix.os }}",  # no matrix, so the label cannot be known
    ],
)
def test_runner_outside_fixed_github_hosted_labels_is_rejected(runs_on: str) -> None:
    text = ok_job().replace(f"runs-on: {HOSTED}", f"runs-on: {runs_on}")
    assert text != ok_job()
    assert check(text) == ["runner-not-github-hosted"]


def test_release_workflow_uses_the_same_runner_rule() -> None:
    text = (
        ok_job()
        .replace(f"runs-on: {HOSTED}", "runs-on: windows-latest")
        .replace(
            "on:\n  pull_request:\n    branches: [main]\n",
            "on:\n  push:\n    tags: ['client/v*']\n",
        )
    )
    assert check(text, "release.yml") == ["runner-not-github-hosted"]
    assert check(text.replace("windows-latest", "windows-2025"), "release.yml") == []


SCHEDULE_ON = "on:\n  schedule:\n    - cron: '0 3 * * 1'\n"


def scheduled(condition: str) -> str:
    return (
        ok_job()
        .replace("on:\n  pull_request:\n    branches: [main]\n", SCHEDULE_ON)
        .replace(f"    runs-on: {HOSTED}\n", f"    if: {condition}\n    runs-on: {HOSTED}\n")
    )


def test_public_only_job_may_use_hosted_runner() -> None:
    text = ok_job().replace(f"    runs-on: {HOSTED}\n", f"    if: {GUARD}\n    runs-on: {HOSTED}\n")
    assert check(text) == []
    assert check(scheduled(GUARD)) == []


def test_public_guard_must_not_be_weakened() -> None:
    assert check(scheduled(f"{GUARD} || true")) == ["schedule-without-public-guard"]


def test_public_guard_with_extra_condition() -> None:
    cond = f"({GUARD}) && github.event_name != 'workflow_dispatch'"
    assert check(scheduled(cond)) == []


def test_schedule_requires_public_guard_on_every_job() -> None:
    text = ok_job().replace("on:\n  pull_request:\n    branches: [main]\n", SCHEDULE_ON)
    assert check(text) == ["schedule-without-public-guard"]


def test_release_workflow_triggers_are_limited() -> None:
    text = ok_job()  # pull_request trigger
    assert "release-trigger" in check(text, "release.yml")


PORTABILITY_ON = "on:\n  pull_request:\n    branches: [main]\n"


def test_portability_starts_only_from_the_release_or_by_hand() -> None:
    # K3-02: release.yml calls it after publish; a tag push would start it before install.sh
    # and the bundle exist under the tag.
    called = ok_job().replace(
        PORTABILITY_ON,
        "on:\n  workflow_call:\n    inputs:\n      tag:\n        type: string\n"
        "        required: true\n  workflow_dispatch:\n",
    )
    assert check(called, "portability.yml") == []
    pushed = ok_job().replace(PORTABILITY_ON, "on:\n  push:\n    tags: ['v*']\n")
    assert check(pushed, "portability.yml") == ["release-trigger"]


@pytest.mark.parametrize("name", ["release.yml", "portability.yml"])
def test_release_workflows_reject_repository_dispatch(name: str) -> None:
    """contents: write suffices for a repository_dispatch (no actions: write needed), so a
    release workflow runs once per tag only because none names this trigger (D-W-111)."""
    text = ok_job().replace(PORTABILITY_ON, "on:\n  repository_dispatch:\n")
    assert check(text, name) == ["release-trigger"]


CALLER = wf("""
    on:
      pull_request:
        branches: [main]
    permissions:
      contents: read
    jobs:
      porta:
        uses: USES
        with:
          tag: v1.0.0
""")


@pytest.mark.parametrize(
    "uses",
    [
        "./.github/workflows/portability.yml",
        "$/.github/workflows/portability.yml",
        f"example/bothy/.github/workflows/portability.yml@{SHA}",
        "./.github/workflows/release.yml",
    ],
)
def test_only_release_yml_calls_release_workflows(uses: str) -> None:
    """C4-04: portability.yml tests one published release, started by release.yml only."""
    assert check(CALLER.replace("USES", uses), "ci.yml") == ["reusable-release-caller"]


def test_other_reusable_workflows_may_be_called_anywhere() -> None:
    assert check(CALLER.replace("USES", "./.github/workflows/lint.yml"), "ci.yml") == []


RELEASE_CALL = wf("""
    on:
      push:
        tags: ['v*']
    permissions:
      contents: read
    jobs:
      portability:
        uses: ./.github/workflows/portability.yml
        with:
          tag: ${{ github.ref_name }}
""")


def test_release_yml_calls_each_release_workflow_once_without_matrix() -> None:
    """portability.yml runs once per tag; a second call or a matrix on the calling job would
    repeat it (C4-04)."""
    assert check(RELEASE_CALL, "release.yml") == []
    twice = RELEASE_CALL + textwrap.indent(
        "again:\n  uses: ./.github/workflows/portability.yml\n  with:\n    tag: v1.0.0\n", "  "
    )
    assert check(twice, "release.yml") == ["reusable-release-caller"]
    matrix = RELEASE_CALL.replace(
        "    uses: ./", "    strategy:\n      matrix:\n        n: [1, 2]\n    uses: ./"
    )
    assert matrix != RELEASE_CALL
    assert check(matrix, "release.yml") == ["reusable-release-caller"]


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


# --- release environments (L-06), post-merge deny-list workflow (L-18) ---------------------


def release_job(runs_on: str, environment: str = "release", name: str = "release.yml") -> list[str]:
    text = (
        ok_job()
        .replace(
            f"    runs-on: {HOSTED}\n", f"    environment: {environment}\n    runs-on: {runs_on}\n"
        )
        .replace(
            "on:\n  pull_request:\n    branches: [main]\n",
            "on:\n  push:\n    tags: ['v*']\n",
        )
    )
    return check(text, name)


@pytest.mark.parametrize("environment", ["release", "release-apple", "{name: release-windows}"])
@pytest.mark.parametrize("label", ["ubuntu-24.04", "macos-15", "windows-2025"])
def test_release_environment_runs_on_github_hosted_runner(environment: str, label: str) -> None:
    assert release_job(label, environment) == []


def test_release_environment_outside_github_hosted_labels_is_rejected() -> None:
    assert release_job("[linux, release]") == ["runner-not-github-hosted"]


def test_post_merge_workflow_may_trigger_on_main() -> None:
    text = ok_job().replace("on:\n  pull_request:\n    branches: [main]\n", POST_MERGE)
    assert check(text, "denylist-main.yml") == []


@pytest.mark.parametrize(
    "trigger",
    [
        "on:\n  push:\n    branches: [main, develop]\n",
        "on:\n  push:\n    branches: ['**']\n",
        "on:\n  push:\n    branches: [main]\n    paths: ['docs/**']\n",
        "on:\n  push:\n    branches: [main]\n  pull_request:\n",
    ],
)
def test_post_merge_workflow_is_limited_to_main(trigger: str) -> None:
    text = ok_job().replace("on:\n  pull_request:\n    branches: [main]\n", trigger)
    assert check(text, "denylist-main.yml") != []


def test_push_on_main_outside_post_merge_workflow_is_rejected() -> None:
    text = ok_job().replace("on:\n  pull_request:\n    branches: [main]\n", POST_MERGE)
    assert check(text, "ci.yml") == ["push-on-branch"]


# --- matrix (K2-11) ------------------------------------------------------------------------


def matrix_job(matrix: str, runs_on: str = "ubuntu-24.04", minutes: int = 30) -> str:
    body = textwrap.indent(textwrap.dedent(matrix).strip(), " " * 8)
    return (
        "on:\n  workflow_call:\n  workflow_dispatch:\npermissions:\n  contents: read\n"
        "jobs:\n  stack:\n"
        f"    runs-on: {runs_on}\n    timeout-minutes: {minutes}\n"
        f"    strategy:\n      matrix:\n{body}\n"
        "    steps:\n      - run: make portability\n"
    )


@pytest.mark.parametrize(
    ("matrix", "count"),
    [
        ("arch: [amd64, arm64]\nprofile: [a, b, c]\n", 6),
        ("arch: [amd64, arm64]\nprofile: [a, b, c]\nexclude:\n  - {arch: arm64, profile: c}\n", 5),
        ("arch: [amd64, arm64]\ninclude:\n  - {arch: arm64, extra: x}\n", 2),
        ("arch: [amd64, arm64]\ninclude:\n  - {arch: riscv}\n", 3),
        ("include:\n  - {arch: amd64}\n  - {arch: arm64}\n", 2),
    ],
)
def test_matrix_combinations(matrix: str, count: int) -> None:
    doc = cr.load_yaml(matrix_job(matrix))
    combos = cr.matrix_combinations(doc["jobs"]["stack"])
    assert combos is not None
    assert len(combos) == count


def test_include_does_not_overwrite_original_values() -> None:
    doc = cr.load_yaml(matrix_job("arch: [amd64, arm64]\ninclude:\n  - {arch: arm64, extra: x}\n"))
    assert cr.matrix_combinations(doc["jobs"]["stack"]) == [
        {"arch": "amd64"},
        {"arch": "arm64", "extra": "x"},
    ]


def test_matrix_runner_labels_are_checked() -> None:
    text = matrix_job(
        "runner: [ubuntu-24.04, ubuntu-24.04-arm, windows-2025]\n",
        runs_on="${{ matrix.runner }}",
    )
    assert check(text, "portability.yml") == []
    assert check(text, "ci.yml") == []


def test_matrix_runner_with_other_value_is_rejected() -> None:
    text = matrix_job("runner: [ubuntu-24.04, linux]\n", runs_on="${{ matrix.runner }}")
    assert check(text, "portability.yml") == ["runner-not-github-hosted"]


DYNAMIC = "      matrix: ${{ fromJSON(needs.plan.outputs.matrix) }}"


def test_dynamic_matrix_needs_a_fixed_runner_label() -> None:
    """A matrix computed at run time may vary anything but the runner (K2-11)."""
    fixed = matrix_job("x: [1]").replace("      matrix:\n        x: [1]", DYNAMIC)
    assert fixed != matrix_job("x: [1]")
    assert check(fixed, "portability.yml") == []
    from_matrix = matrix_job("x: [1]", runs_on="${{ matrix.runner }}").replace(
        "      matrix:\n        x: [1]", DYNAMIC
    )
    assert check(from_matrix, "portability.yml") == ["runner-not-github-hosted"]


# --- release jobs never consume output of jobs without a release environment (K2-04) -------

RELEASE_WF = wf(f"""
    on:
      push:
        tags: ['v*']
    permissions:
      contents: read
    jobs:
      resolve:
        runs-on: {HOSTED}
        timeout-minutes: 5
        outputs:
          path: ${{{{ steps.info.outputs.path }}}}
        steps:
          - run: echo path=. >> "$GITHUB_OUTPUT"
      build:
        needs: resolve
        if: needs.resolve.outputs.path != ''
        environment: release
        runs-on: {HOSTED}
        timeout-minutes: 30
        outputs:
          dist_id: ${{{{ steps.up.outputs.artifact-id }}}}
        steps:
          - id: info
            run: echo path=. >> "$GITHUB_OUTPUT"
          - if: steps.info.outputs.path != needs.resolve.outputs.path
            run: exit 1
          - env:
              COMPONENT_PATH: ${{{{ steps.info.outputs.path }}}}
            run: make -C "$COMPONENT_PATH" release-build
      publish:
        needs: [resolve, build]
        environment: release
        runs-on: {HOSTED}
        timeout-minutes: 10
        steps:
          - uses: actions/download-artifact@{SHA}
            with:
              artifact-ids: ${{{{ needs.build.outputs.dist_id }}}}
              path: dist
""")


def test_release_job_may_use_other_job_output_only_in_if() -> None:
    assert check(RELEASE_WF, "release.yml") == []


@pytest.mark.parametrize(
    "leak",
    [
        "${{ needs.resolve.outputs.path }}",
        "${{ needs['resolve'].outputs.path }}",
        "${{ toJSON(needs) }}",
        "${{ join(needs.*.outputs.path) }}",
        "${{ toJSON(needs.resolve) }}",
        "${{ needs.resolve['outputs'].path }}",
        "${{ NEEDS.resolve.outputs.path }}",
        "${{ needs.build['outputs'].dist_id }}",
        "${{ format('{0}', needs.resolve.result && needs.resolve.outputs.path) }}",
        "${{ format('}}', needs.resolve.outputs.path) }}",
        "${{ format('it''s }}', needs.resolve.outputs.path) }}",
        "${{ steps.info.outputs.path }} ${{ format('}}', needs.resolve.outputs.path) }}",
        "${{ format('{0}', needs.resolve.outputs.path",
    ],
)
def test_release_job_rejects_other_job_output_in_shell(leak: str) -> None:
    text = RELEASE_WF.replace(
        "COMPONENT_PATH: ${{ steps.info.outputs.path }}", f"COMPONENT_PATH: {leak}"
    )
    assert text != RELEASE_WF
    assert check(text, "release.yml") == ["release-untrusted-output"]


@pytest.mark.parametrize(
    "allowed",
    [
        "${{ needs.resolve.result }}",
        "${{ needs.build-x.result == 'success' }}",
        "${{ steps.info.outputs.path }}",
        "needs resolving",
    ],
)
def test_release_job_allows_results_and_plain_text(allowed: str) -> None:
    """Outside `if:` only `needs.<job>.result` and outputs of release jobs pass (K3-04)."""
    text = RELEASE_WF.replace(
        "COMPONENT_PATH: ${{ steps.info.outputs.path }}", f"COMPONENT_PATH: {allowed}"
    )
    assert check(text, "release.yml") == []


def test_release_job_rejects_other_job_output_in_action_input() -> None:
    text = RELEASE_WF.replace("needs.build.outputs.dist_id", "needs.resolve.outputs.path")
    assert check(text, "release.yml") == ["release-untrusted-output"]


LEAK = "${{ needs.resolve.outputs.path }}"


@pytest.mark.parametrize(
    ("old", "new"),
    [
        # a variable named `if` in the env of a step reaches the shell as $if
        ("COMPONENT_PATH: ${{ steps.info.outputs.path }}", f"if: {LEAK}"),
        # an action input named `if`
        ("path: dist", f"if: {LEAK}"),
        # the env of the job
        ("    timeout-minutes: 10\n", f"    timeout-minutes: 10\n    env:\n      if: {LEAK}\n"),
        # an expression as a key counts as well (conservative)
        ("COMPONENT_PATH: ${{ steps.info.outputs.path }}", f"{LEAK}: x"),
    ],
)
def test_release_job_checks_if_keys_below_job_and_step(old: str, new: str) -> None:
    """Only the `if:` of the job itself and of each step is a condition (C4-03)."""
    text = RELEASE_WF.replace(old, new)
    assert text != RELEASE_WF
    assert check(text, "release.yml") == ["release-untrusted-output"]


@pytest.mark.parametrize(
    "inputs",
    ["pattern: dist-*", "name: dist-linux"],
)
def test_release_job_downloads_artifacts_by_id_only(inputs: str) -> None:
    text = RELEASE_WF.replace("artifact-ids: ${{ needs.build.outputs.dist_id }}", inputs)
    assert check(text, "release.yml") == ["release-artifact-by-id"]


def test_repository_release_workflow_resolves_again_in_every_release_job() -> None:
    """Every release* job of release.yml recomputes the tag and compares it (K2-04)."""
    doc = cr.load_yaml((REPO / ".github" / "workflows" / "release.yml").read_text())
    for name, job in doc["jobs"].items():
        if not cr.is_release_environment(job):
            continue
        runs = " ".join(str(s.get("run", "")) for s in job["steps"])
        conds = " ".join(str(s.get("if", "")) for s in job["steps"])
        assert "components.py release" in runs, name
        for key in ("component", "version", "path", "gate_components", "prerelease"):
            assert f"steps.info.outputs.{key} != needs.resolve.outputs.{key}" in conds, (name, key)


# --- release jobs are found by any spelling of their environment (D-W-110) -----------------


@pytest.mark.parametrize("environment", ["Release", "RELEASE-apple", "{name: Release-Windows}"])
def test_release_environment_is_found_in_any_case(environment: str) -> None:
    """GitHub treats environment names without case: `Release` holds the secrets of `release`."""
    job = {"environment": cr.load_yaml(f"e: {environment}")["e"]}
    assert cr.is_release_environment(job)
    assert release_job(HOSTED, environment) == []


def test_release_untrusted_output_applies_to_any_case() -> None:
    upper = RELEASE_WF.replace("environment: release\n", "environment: RELEASE\n")
    assert upper.count("environment: RELEASE\n") == 2
    assert check(upper, "release.yml") == []
    leak = upper.replace(
        "COMPONENT_PATH: ${{ steps.info.outputs.path }}", f"COMPONENT_PATH: {LEAK}"
    )
    assert check(leak, "release.yml") == ["release-untrusted-output"]


@pytest.mark.parametrize(
    "environment",
    [
        "${{ needs.resolve.outputs.environment }}",
        "\"${{ github.ref_name == 'v1' && 'release' || 'ci' }}\"",
        "{name: '${{ github.ref_name }}'}",
        "release-${{ matrix.os }}",
    ],
)
def test_environment_name_must_be_literal(environment: str) -> None:
    """A computed name could pick a release environment that no rule sees."""
    assert release_job(HOSTED, environment) == ["environment-not-literal"]
    assert release_job(HOSTED, environment, "ci.yml") == ["environment-not-literal"]


# --- no workflow_dispatch from a workflow (actions: write, D-W-111) -------------------------

ACTIONS_WRITE = "permissions:\n  contents: read\n  actions: write\n"


def test_actions_write_on_top_level_is_rejected() -> None:
    """With actions: write the GITHUB_TOKEN creates workflow_dispatch runs, for example of
    portability.yml from a pull request, outside the one run per tag of D-W-73."""
    text = ok_job().replace("permissions:\n  contents: read\n", ACTIONS_WRITE)
    assert text != ok_job()
    assert check(text) == ["actions-write"]
    assert check(text.replace("actions: write", "actions: read")) == []


@pytest.mark.parametrize("name", ["ci.yml", "release.yml", "denylist-main.yml"])
def test_actions_write_on_a_job_is_rejected_in_every_workflow(name: str) -> None:
    text = ok_job().replace(
        "    timeout-minutes: 30\n",
        "    timeout-minutes: 30\n    permissions:\n      contents: read\n      actions: write\n",
    )
    assert text != ok_job()
    assert "actions-write" in check(text, name)


def test_write_all_on_a_job_is_rejected() -> None:
    text = ok_job().replace(
        "    timeout-minutes: 30\n", "    timeout-minutes: 30\n    permissions: write-all\n"
    )
    assert text != ok_job()
    assert check(text) == ["broad-permissions"]


def test_actions_write_on_a_reusable_call_is_rejected() -> None:
    text = CALLER.replace("USES", "./.github/workflows/lint.yml").replace(
        "    with:\n", "    permissions:\n      actions: write\n    with:\n"
    )
    assert "actions: write" in text
    assert check(text, "ci.yml") == ["actions-write"]


def signing_job(step: str, environment: str = "") -> str:
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
    assert check(signing_job(step), "release.yml") == ["signing-outside-release"]
    assert check(signing_job(step, "release-apple"), "release.yml") == ["signing-outside-release"]


@pytest.mark.parametrize("environment", ["release", "Release ", "{name: release}"])
@pytest.mark.parametrize("step", SIGNING_STEPS)
def test_signing_step_in_the_release_environment(step: str, environment: str) -> None:
    assert check(signing_job(step, environment), "release.yml") == []


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
    assert check(signing_job(step), "release.yml") == []
