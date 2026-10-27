# SPDX-License-Identifier: Apache-2.0
"""Component map of the monorepo: CI path filter and release tag resolution.

Gate components group code by tool chain. `make gates COMPONENTS=...` runs the
component-specific gates only for the listed ones; repository-wide gates always run.
A change outside every known area selects `all` (fail safe).

Release components map a tag `<component>/v<semver>` (or `v<semver>` for the whole
product) to a directory with its own VERSION file and a CHANGELOG.md with a section for that
version (docs/spec/18-release.md); `release` rejects the tag otherwise.

The product bundle pins the server version in the line `BOTHY_VERSION=` of
infra/compose/.env.example (docs/spec/18-release.md, 2.1). `pins` checks the committed state
before any tag exists: the value is SemVer without "v" (or the template placeholder), and a
stable product (VERSION without pre-release) never pins a server pre-release; only a
product beta (`X.Y.Z-beta.N`) may.

Usage:
  components.py changed --base <sha> --head <sha>   print components for `make COMPONENTS=`
  components.py changed --files <path>...
  components.py ci-plan (--base <sha> --head <sha> | --files <path>... | --components <names>)
                        --public true|false        print `components=` and `jobs=` for the
                                                    plan of ci.yml and build-verify.yml
  components.py release --tag <tag> [--github-output]
                                                    print release info as JSON, or as
                                                    key=value lines for $GITHUB_OUTPUT
  components.py pins [--repo <dir>]                 check the server version the product
                                                    bundle pins (make release-pins)
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Any

# Gate component -> path prefixes owned by it.
GATE_COMPONENTS: dict[str, tuple[str, ...]] = {
    # The terminal suite in services/server/test/terminal starts the installation Compose file
    # with its test overlays and the collector configuration (docs/decisions.d/M2-48.md, brief M2-48
    # Design note 14).
    # The agent conformance suite in services/server/test/agents runs the Fake-Agent of
    # sdk/python against the server (M2-31, docs/decisions.d/M2-31.md), so an SDK change runs the
    # server gates too.
    "server": ("services/", "infra/compose/", "infra/otel/", "infra/host/", "sdk/python/"),
    # The Dart tests of packages/bothy_noise read the Noise test vectors, recordings and
    # fingerprint vectors of the host service (docs/DECISIONS.md D-M2-38-5). The architecture
    # test of the Dart workspace belongs to its gates (docs/DECISIONS.md D-M4-01-8).
    "apps": ("apps/", "packages/", "design/", "services/host/testdata/noise/", "tools/arch-check/"),
    # sdk/testdata/ holds the shared fixtures that both SDK test suites read. The real-server
    # tests of both SDKs build and run the server of this tree through
    # services/server/test/sdkserver (M2-29, M2-30), so a server change runs them too.
    "sdk-python": ("sdk/python/", "sdk/testdata/", "services/server/"),
    "sdk-typescript": ("sdk/typescript/", "sdk/testdata/", "services/server/"),
    # The GitHub app links the TypeScript SDK as a local package and its live test runs the
    # server of this tree through sdkserver (M2-49, docs/decisions.d/M2-49.md), as the SDK's do;
    # the Jira app does the same with the Python SDK as an editable path source (M2-50).
    "plugins": ("plugins/", "sdk/typescript/", "sdk/python/", "services/server/"),
}
# The API contract feeds generated server stubs and every client.
API_PREFIX = "docs/api/"
# Files outside a component's directory that its generated code is built from; a change selects
# the component, so that its codegen-check sees the drift.
GATE_COMPONENT_INPUTS: dict[str, tuple[str, ...]] = {
    # services/server/api/generate.go: route table and action catalogue of the server.
    "server": (
        "docs/spec/actions.yaml",
        # services/server/internal/kernel/designkeys: go generate and its tests read the tokens.
        "design/build/tokens.resolved.json",
        # services/server/test/guide runs the guide against a fresh server and reads its step
        # titles from chapter 11 section 13 (M2-37, docs/decisions.d/M2-37.md); its example in
        # sdk/python/examples/ selects the server through "sdk/python/" above.
        "docs/guides/connect-agent.md",
        "docs/spec/11-extensions.md",
    ),
    # The guide shows the example adapter of the Python SDK, whose tests and lint gates run
    # with the SDK's (M2-37).
    "sdk-python": ("docs/guides/connect-agent.md", "docs/spec/11-extensions.md"),
    # Chapter 09 is the contract of the adapters under plugins/; its section 17.10 is bound to the
    # recording evidence of plugins/hermes-adapter by the offline fixture tests.
    # The unit tests of plugins/grafana-bothy-live hold its dashboard to queries.yaml and its
    # Grafana pin to the Compose stack (M7-25).
    "plugins": (
        "docs/spec/09-agents.md",
        "infra/observability/grafana/dashboards/bothy-live.json",
        "infra/compose/compose.yaml",
        "infra/compose/.env.example",
    ),
    # The root files of the Dart workspace: manifest with the Flutter pin, the one lock file and
    # the shared analysis rules (docs/DECISIONS.md D-M4-01-8).
    "apps": ("pubspec.yaml", "pubspec.lock", "analysis_options.yaml"),
}
# Directories outside a component whose files feed its generated code: the SLI bounds generator
# and the Sloth files it reads write services/server/internal/stats/domain/slibounds.gen.go and
# packages/bothy_viz/lib/src/slo/slo_catalog.g.dart (brief M6-11); their checks `sli-bounds`
# and `sli-labels` run in `make ops-check`, the generated files in the Go and Dart gates.
GATE_COMPONENT_INPUT_PREFIXES: dict[str, tuple[str, ...]] = {
    "server": ("tools/observability/", "infra/observability/slo/"),
    "apps": ("tools/observability/", "infra/observability/slo/"),
    # The panel plugin imports the TypeScript tokens and bundles their fonts (M7-24, M7-25). The
    # Hermes adapter imports the Python SDK from its source tree (M2-33), so an SDK change runs
    # the adapter's tests too.
    "plugins": ("design/build/ts/", "sdk/python/src/"),
}
# Self-contained tool folders under tools/ (and single files in them) with the gate components
# whose gates or tests read them; the longest matching key decides. A tool without a component is
# read only by repository-wide gates and the tools tests, which run on every change
# (docs/quality/gates.md, "Jobs a change needs"). tools/arch-check and tools/observability are
# component inputs above. Every other path under tools/ is a shared input: SHARED_TOOL_FOLDERS,
# the root files of the tools project (pyproject.toml, uv.lock, conftest.py) and any new folder.
TOOL_COMPONENTS: dict[str, tuple[str, ...]] = {
    "tools/api-checks/": (),
    # `make api-breaking` runs only for the component server.
    "tools/api-checks/breaking.py": ("server",),
    "tools/catalog/": (),
    "tools/configdoc/": (),
    # `make contract-test` (G7) runs tools/contract/live_contract.py in the job
    # test (server contract).
    "tools/contract/": ("server",),
    "tools/denylist/": (),
    # The desktop release steps (M7-12); their trial jobs run when they change (DISTRIBUTION_JOBS).
    "tools/distribution/": (),
    # The release steps of the host service (M7-23): a test of services/host reads them.
    "tools/distribution/hostdist.py": ("server",),
    # The token generator writes the committed Dart tokens and icons of packages/design_system,
    # like design/, and design/build/tokens.resolved.json, which the server's designkeys
    # generator and tests read.
    "tools/design-tokens/": ("apps", "server"),
    "tools/doccheck/": (),
    # `make sql-visibility` (AZ-P12) checks the queries of the server module and runs only when the
    # component server is selected.
    "tools/lint/": ("server",),
    # The runner of the capacity load test (M7-18) runs by hand only (`make load-capacity`).
    "tools/load/": (),
    "tools/docs/": (),
    # ID-T16 (M5-06) is a manual end-to-end run on a Mac with Docker; no CI job runs it.
    "tools/e2e/": (),
    "tools/opscheck/": (),
    # The runner of the portability test (M7-14) runs in the release workflow only.
    "tools/portability/": (),
    "tools/scancode/": (),
    # `make signal-check` (M1-68) reads the catalogue and the dashboards, rules and SLOs, no
    # component source; it runs in the repository-wide docs and contracts job.
    "tools/signalcheck/": (),
    "tools/sonarqube/": (),
    # The test of services/server/internal/platform/pg/pgtest reads the image pin of this file.
    "tools/sonarqube/compose.yaml": ("server",),
    "tools/testing/": (),
    "tools/README.md": (),
}
# Tool folders every gate and job depends on: the pinned binaries, the CI helpers with this map,
# the hooks (commitlint and the pre-push plan) and the Node tools with their lock file.
SHARED_TOOL_FOLDERS = ("bin", "ci", "console-e2e", "hooks", "node")
# Areas without component gates; their repository-wide gates always run.
NO_COMPONENT_PREFIXES = ("docs/", "infra/")
NO_COMPONENT_ROOT_FILES = re.compile(r"^[^/]+\.md$|^(?:LICENSE|NOTICE)$")

# Source files of a component (component directory and source suffix). A push of only these
# leaves the tools gates out of the pre-push plan (tools/hooks/push_plan.py), except the tools
# tests in TOOLS_SOURCE_TESTS of the Makefile; manifests, lock files, configuration, test data and
# docs of a component are no source files, because tools tests read some of them (go.mod,
# Dockerfile, .golangci.yml). CI runs the tools tests on every change, because some also build
# component sources for other platforms (the release builds).
COMPONENT_DIRS = ("services/", "apps/", "packages/", "sdk/", "plugins/")
SOURCE_SUFFIXES = (".go", ".dart", ".ts", ".tsx", ".kt", ".swift", ".py")

# ---- CI jobs a change needs (docs/quality/gates.md, "Jobs a change needs") -----------------
# Job ids, the same in ci.yml and build-verify.yml. Every other job runs on every change, test
# (tools) included.
COMPONENT_TEST_JOBS: dict[str, tuple[str, ...]] = {
    "test-server": ("server",),
    "test-apps": ("apps",),
    "test-sdk-plugins": ("sdk-python", "sdk-typescript", "plugins"),
}
# The platform jobs of the desktop hosts; apps-ios runs the iOS simulator entries on macOS (PL/44).
DESKTOP_JOBS = ("apps-macos", "apps-ios", "apps-windows")
# The emulator jobs run only while the repository is public (docs/DECISIONS.md D-M5-01-5).
ANDROID_JOBS = ("apps-android-26", "apps-android-36")
POSTGRES_IMAGE_JOBS = ("postgres-image-amd64", "postgres-image-arm64")
# SEC-T05 (M6-03): the console of the server image under its CSP in headless browsers. The image
# builds the server and the console: the job runs when the build context of
# services/server/Dockerfile.dockerignore or the Compose stack of the test changes, and for the
# component server or apps without a diff.
CONSOLE_E2E_JOBS = ("console-e2e",)
CONSOLE_E2E_PREFIXES = (
    "services/server/api/",
    "services/server/cmd/",
    "services/server/internal/",
    "services/server/migrations/",
    "services/server/test/smoke/",
    "apps/console/",
    "packages/",
    "infra/compose/test/console-e2e/",
)
CONSOLE_E2E_FILES = (
    "services/server/go.mod",
    "services/server/go.sum",
    "services/server/VERSION",
    "services/server/Dockerfile",
    "services/server/Dockerfile.dockerignore",
    "pubspec.yaml",
    "pubspec.lock",
    "analysis_options.yaml",
    "infra/compose/compose.yaml",
    "infra/compose/test/compose.smoke.yaml",
    "infra/compose/test/compose.console-e2e.yaml",
    "infra/compose/test/compose.console-e2e-grafana.yaml",
    "infra/compose/test/smoke.env",
    "infra/compose/test/console-e2e.env",
)
# The trials of the desktop distribution (brief M7-12, docs/decisions.d/M7-12.md): each builds
# the app twice with a test publisher and updates an installed build through Sparkle or App
# Installer. They run only when a change touches the release steps or the files they put into
# the package, not for every change of the apps and not on main (a macOS runner per merge).
DISTRIBUTION_JOBS = ("desktop-trial-macos", "desktop-trial-windows")
DISTRIBUTION_PREFIXES = ("apps/distribution/", "tools/distribution/")
DISTRIBUTION_FILES = (
    "apps/client/Makefile",
    "apps/client/macos/Runner/AppDelegate.swift",
    "apps/client/macos/Runner/MainFlutterWindow.swift",
    "apps/client/macos/Runner/Info.plist",
    "apps/client/macos/Runner/Release.entitlements",
    "apps/client/macos/NotificationService/Release.entitlements",
    "apps/client/macos/Runner.xcodeproj/project.pbxproj",
    "apps/client/windows/CMakeLists.txt",
    "apps/client/windows/runner/CMakeLists.txt",
    "apps/client/windows/runner/resources/app_icon.ico",
    "apps/client/tool/branding_defines.dart",
    "apps/client/branding.example.yaml",
    ".github/workflows/release.yml",
)
# The trials of the phone distribution (brief M7-13, docs/decisions.d/M7-13.md): the unsigned iOS
# archive of a test publisher with its structure and entitlements checked, and the Android App
# Bundle signed with a throwaway upload key and verified. They follow the release steps
# (DISTRIBUTION_PREFIXES, like the desktop trials) and the files that shape the phone builds, not
# every change of the apps and not main.
MOBILE_TRIAL_JOBS = ("mobile-trial-ios", "mobile-trial-android")
MOBILE_TRIAL_FILES = (
    "apps/client/Makefile",
    "apps/client/android/app/build.gradle.kts",
    "apps/client/ios/Runner.xcodeproj/project.pbxproj",
    "apps/client/tool/branding_defines.dart",
    "apps/client/branding.example.yaml",
    ".github/workflows/release.yml",
)
# The platform trials of the host packages (brief M7-23, docs/decisions.d/M7-23.md): each builds,
# signs and installs the .pkg or the MSI with a test publisher on its runner. They run when the
# host service, its package recipes or the release steps change (also the desktop steps they
# reuse), not on main. The desktop trials ignore the host release steps.
HOST_TRIAL_JOBS = ("host-trial-macos", "host-trial-windows")
HOST_TRIAL_PREFIXES = ("services/host/", "infra/host/", "tools/distribution/")
HOST_TRIAL_FILES = (".github/workflows/release.yml",)
HOST_DISTRIBUTION_FILES = (
    "tools/distribution/hostdist.py",
    "tools/distribution/tests/test_hostdist.py",
)
CONDITIONAL_JOBS: tuple[str, ...] = (
    *COMPONENT_TEST_JOBS,
    *DESKTOP_JOBS,
    *ANDROID_JOBS,
    *POSTGRES_IMAGE_JOBS,
    *CONSOLE_E2E_JOBS,
    *DISTRIBUTION_JOBS,
    *MOBILE_TRIAL_JOBS,
    *HOST_TRIAL_JOBS,
)
# What the platform tests (tools/ci/apps_platform.py) build and run: the members of the Dart
# workspace with their native code, and its root manifest (Flutter pin) and lock file. The other
# inputs of the component apps (design/, the SLO generator) feed generated Dart files that are
# committed under packages/, and the Dart gates of test (apps) catch a stale one.
PLATFORM_PREFIXES = ("apps/", "packages/")
PLATFORM_FILES = ("pubspec.yaml", "pubspec.lock")
# What bothy-postgres is built and tested from: the build context of
# services/server/images/bothy-postgres/Dockerfile.dockerignore (bothy-backupd and the internal
# packages it imports, the scripts of the image), the Dockerfile, the two-container test, the
# migrations and the module files and lint configuration the test is built with.
POSTGRES_IMAGE_PREFIXES = (
    "services/server/images/bothy-postgres/",
    "services/server/test/postgresimage/",
    "services/server/internal/",
    "services/server/cmd/bothy-backupd/",
    "services/server/migrations/",
)
POSTGRES_IMAGE_FILES = (
    "services/server/go.mod",
    "services/server/go.sum",
    "services/server/VERSION",
    "services/server/migrations.sha256",
    "services/server/.golangci.yml",
)

# Release component -> (directory, gate component).
RELEASE_COMPONENTS: dict[str, tuple[str, str]] = {
    "bothy": (".", "all"),
    "server": ("services/server", "server"),
    "host": ("services/host", "server"),
    "client": ("apps/client", "apps"),
    "console": ("apps/console", "apps"),
    "sdk-python": ("sdk/python", "sdk-python"),
    "sdk-typescript": ("sdk/typescript", "sdk-typescript"),
    "hermes-adapter": ("plugins/hermes-adapter", "plugins"),
    "claude-code-runner": ("plugins/claude-code-runner", "plugins"),
    "grafana-bothy-live": ("plugins/grafana-bothy-live", "plugins"),
}
# Paths outside a release component's directory whose commits belong to its releases
# (ADR 0060 item 2): the API contract ships with the server.
RELEASE_EXTRA_PATHS: dict[str, tuple[str, ...]] = {"server": ("docs/api",)}
_SEMVER = re.compile(
    r"^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)"
    r"(?:-((?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*))*))?"
    r"(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$"
)


# Server version pinned by the product bundle (docs/spec/18-release.md, 2.1).
BUNDLE_ENV_TEMPLATE = "infra/compose/.env.example"
SERVER_VERSION_PLACEHOLDER = "0.0.0-placeholder"
_SERVER_VERSION_LINE = re.compile(r"^\s*BOTHY_VERSION=(.*)$", re.MULTILINE)
# The only product pre-release that `make release-prep CHANNEL=beta` writes.
_PRODUCT_BETA = re.compile(r"^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)-beta\.(0|[1-9]\d*)$")


class ReleaseError(Exception):
    """The tag does not describe a releasable component state."""


def tool_components(path: str) -> tuple[str, ...] | None:
    """Return the components of a tool path of TOOL_COMPONENTS, or None for any other path."""
    keys = [k for k in TOOL_COMPONENTS if path == k or (k.endswith("/") and path.startswith(k))]
    return TOOL_COMPONENTS[max(keys, key=len)] if keys else None


def changed(files: list[str]) -> list[str]:
    """Return the gate components touched by the changed `files` (all for unknown paths)."""
    selected: set[str] = set()
    for path in files:
        owners = [c for c, prefixes in GATE_COMPONENTS.items() if path.startswith(prefixes)]
        owners += [c for c, inputs in GATE_COMPONENT_INPUTS.items() if path in inputs]
        owners += [
            c for c, prefixes in GATE_COMPONENT_INPUT_PREFIXES.items() if path.startswith(prefixes)
        ]
        tool = tool_components(path)
        if owners:
            selected.update(owners)
        elif tool is not None:
            selected.update(tool)
        elif path.startswith(API_PREFIX):
            selected.update(GATE_COMPONENTS)
        elif path.startswith(NO_COMPONENT_PREFIXES) or NO_COMPONENT_ROOT_FILES.match(path):
            continue
        else:
            return ["all"]
    return sorted(selected)


def is_component_source(path: str) -> bool:
    """Return whether `path` is a source file of a component (see COMPONENT_DIRS)."""
    return path.startswith(COMPONENT_DIRS) and path.endswith(SOURCE_SUFFIXES)


@dataclass(frozen=True)
class CIPlan:
    """The gate components of a CI run and the conditional jobs it needs (CONDITIONAL_JOBS)."""

    components: list[str]
    jobs: list[str]


def ci_plan(
    *, files: list[str] | None = None, selected: list[str] | None = None, public: bool
) -> CIPlan:
    """Return the components and conditional jobs a change needs.

    Args:
        files: the changed files of a pull request or build branch. An empty list, a shared
            input (`changed` gives `all`) or the API contract runs every job but the trials of
            the desktop distribution (DISTRIBUTION_JOBS), which run only when a file of
            DISTRIBUTION_PREFIXES (but HOST_DISTRIBUTION_FILES) or DISTRIBUTION_FILES changes,
            the phone trials (MOBILE_TRIAL_JOBS), which run for the same prefixes and
            MOBILE_TRIAL_FILES, and the host trials (HOST_TRIAL_JOBS), which run only for
            HOST_TRIAL_PREFIXES and HOST_TRIAL_FILES.
        selected: gate components without a diff (`all` for a push to main or a manual run,
            the gate components of a release tag); every job of these components runs, the
            distribution, phone and host trials not.
        public: whether the repository is public; the Android emulator jobs need it.

    Returns:
        The plan; `jobs` lists job ids of CONDITIONAL_JOBS, sorted.

    Raises:
        ValueError: not exactly one of `files` and `selected`, or an unknown component.

    Example: `ci_plan(files=["services/host/x.go"], public=True).jobs` is
    `["host-trial-macos", "host-trial-windows", "test-server"]`.
    """
    if (files is None) == (selected is None):
        raise ValueError("ci_plan needs exactly one of files or selected")
    if files is not None:
        names = changed(files) if files else ["all"]
        full = names == ["all"] or any(f.startswith(API_PREFIX) for f in files)
        platform = any(f.startswith(PLATFORM_PREFIXES) or f in PLATFORM_FILES for f in files)
        postgres = any(
            f.startswith(POSTGRES_IMAGE_PREFIXES) or f in POSTGRES_IMAGE_FILES for f in files
        )
        console = any(f.startswith(CONSOLE_E2E_PREFIXES) or f in CONSOLE_E2E_FILES for f in files)
        distribution = any(
            (f.startswith(DISTRIBUTION_PREFIXES) and f not in HOST_DISTRIBUTION_FILES)
            or f in DISTRIBUTION_FILES
            for f in files
        )
        mobile = any(
            (f.startswith(DISTRIBUTION_PREFIXES) and f not in HOST_DISTRIBUTION_FILES)
            or f in MOBILE_TRIAL_FILES
            for f in files
        )
        host_trials = any(f.startswith(HOST_TRIAL_PREFIXES) or f in HOST_TRIAL_FILES for f in files)
    else:
        names = list(selected or [])
        unknown = [n for n in names if n != "all" and n not in GATE_COMPONENTS]
        if unknown:
            raise ValueError(f"unknown component '{unknown[0]}'")
        full = "all" in names
        platform = "apps" in names
        postgres = "server" in names
        console = bool({"apps", "server"} & set(names))
        distribution = mobile = host_trials = False
    trials = {*DISTRIBUTION_JOBS, *MOBILE_TRIAL_JOBS, *HOST_TRIAL_JOBS}
    jobs: set[str] = set(CONDITIONAL_JOBS) - trials if full else set()
    if not full:
        jobs.update(job for job, owners in COMPONENT_TEST_JOBS.items() if set(owners) & set(names))
        if platform:
            jobs.update((*DESKTOP_JOBS, *ANDROID_JOBS))
        if postgres:
            jobs.update(POSTGRES_IMAGE_JOBS)
        if console:
            jobs.update(CONSOLE_E2E_JOBS)
    if distribution:
        jobs.update(DISTRIBUTION_JOBS)
    if mobile:
        jobs.update(MOBILE_TRIAL_JOBS)
    if host_trials:
        jobs.update(HOST_TRIAL_JOBS)
    if not public:
        jobs.difference_update(ANDROID_JOBS)
    return CIPlan(components=names, jobs=sorted(jobs & set(CONDITIONAL_JOBS)))


def format_for_make(names: list[str]) -> str:
    """Return component names as the space separated make argument, or `none`."""
    if not names:
        return "none"
    return " ".join(sorted(names))


def tag_prefix(component: str) -> str:
    """Return the tag prefix of a release component, the part before `v<semver>`.

    Args:
        component: a key of RELEASE_COMPONENTS.

    Returns:
        `<component>/`, or an empty string for the product `bothy` (tags `v<semver>`).

    Raises:
        ReleaseError: the component is unknown.

    Example: `tag_prefix("server")` is `"server/"`.
    """
    if component not in RELEASE_COMPONENTS:
        raise ReleaseError(f"unknown release component '{component}'")
    return "" if component == "bothy" else f"{component}/"


def release_paths(component: str) -> tuple[str, ...]:
    """Return the repository paths whose commits count for a release of `component`.

    Args:
        component: a key of RELEASE_COMPONENTS.

    Returns:
        The component directory plus RELEASE_EXTRA_PATHS; an empty tuple for the product
        `bothy`, whose directory is the repository root (every commit counts).

    Raises:
        ReleaseError: the component is unknown.
    """
    tag_prefix(component)
    directory = RELEASE_COMPONENTS[component][0]
    if directory == ".":
        return ()
    return (directory, *RELEASE_EXTRA_PATHS.get(component, ()))


def release_info(tag: str, repo: Path) -> dict[str, Any]:
    """Resolve a release tag to component, version and paths; raises ReleaseError."""
    component, sep, rest = tag.rpartition("/")
    if not sep:
        component, rest = "bothy", tag
    if component not in RELEASE_COMPONENTS:
        raise ReleaseError(f"unknown release component '{component}' in tag '{tag}'")
    if not rest.startswith("v") or not _SEMVER.match(rest[1:]):
        raise ReleaseError(f"'{tag}' is not a release tag '<component>/v<semver>'")
    version = rest[1:]
    directory, gate = RELEASE_COMPONENTS[component]
    version_file = repo / directory / "VERSION"
    if not version_file.is_file():
        raise ReleaseError(f"{directory}/VERSION is missing; the component has nothing to release")
    recorded = version_file.read_text(encoding="utf-8").strip()
    if recorded != version:
        raise ReleaseError(f"{directory}/VERSION says {recorded}, tag says {version}")
    # The release notes come from this section; without it nothing may be built or published
    # (ADR 0036 "Confirmation", ADR 0060).
    changelog = repo / directory / "CHANGELOG.md"
    if not changelog.is_file():
        raise ReleaseError(f"{directory}/CHANGELOG.md is missing")
    heading = re.compile(rf"^## \[{re.escape(version)}\]", re.MULTILINE)
    if not heading.search(changelog.read_text(encoding="utf-8")):
        raise ReleaseError(f"{directory}/CHANGELOG.md has no section [{version}]")
    return {
        "component": component,
        "version": version,
        "path": directory,
        "gate_components": gate,
        "prerelease": _is_prerelease(version),
    }


def _is_prerelease(version: str) -> bool:
    match = _SEMVER.match(version)
    return bool(match and match.group(4))


def pinned_server_problems(repo: Path) -> list[str]:
    """Return what is wrong with the server version the product bundle pins.

    Checks the committed product VERSION and the line `BOTHY_VERSION=` of the bundle
    template: exactly one such line, its value SemVer without "v" or the placeholder, and
    no server pre-release in a stable product. The placeholder passes here because the
    template carries it until the first product release; make release-prep and make
    release-build refuse it (docs/spec/18-release.md, 2.1).
    """
    problems: list[str] = []
    version_file = repo / "VERSION"
    product = ""
    if not version_file.is_file():
        problems.append("VERSION is missing")
    else:
        product = version_file.read_text(encoding="utf-8").strip()
        if not _SEMVER.match(product):
            problems.append(f"VERSION says '{product}', not a SemVer version without 'v'")
            product = ""
    template = repo / BUNDLE_ENV_TEMPLATE
    if not template.is_file():
        problems.append(f"{BUNDLE_ENV_TEMPLATE} is missing")
        return problems
    values = [v.strip() for v in _SERVER_VERSION_LINE.findall(template.read_text("utf-8"))]
    if len(values) != 1:
        problems.append(
            f"{BUNDLE_ENV_TEMPLATE}: expected exactly one line BOTHY_VERSION=, found {len(values)}"
        )
        return problems
    server = values[0]
    if server == SERVER_VERSION_PLACEHOLDER:
        return problems
    if not _SEMVER.match(server):
        problems.append(
            f"{BUNDLE_ENV_TEMPLATE}: BOTHY_VERSION '{server}' is not a SemVer version without 'v'"
        )
    elif product and _is_prerelease(server) and not _is_prerelease(product):
        problems.append(
            f"stable product {product} pins the server pre-release {server}; pin a stable"
            " server version or prepare a product beta (CHANNEL=beta)"
        )
    elif product and _is_prerelease(server) and not _PRODUCT_BETA.match(product):
        problems.append(
            f"product {product} is not a beta (X.Y.Z-beta.N) and pins the server pre-release"
            f" {server}; only a product beta (CHANNEL=beta) may pin one"
        )
    return problems


def _diff_files(base: str, head: str) -> list[str]:
    out = subprocess.run(
        # --no-renames lists the old path of a moved file too, so it counts for its old place.
        ["git", "diff", "--name-only", "--no-renames", f"{base}...{head}"],
        check=True,
        capture_output=True,
        text=True,
    ).stdout
    return [line for line in out.splitlines() if line]


def _print_ci_plan(args: argparse.Namespace, parser: argparse.ArgumentParser) -> int:
    public = args.public == "true"
    if args.files is not None:
        plan = ci_plan(files=args.files, public=public)
    elif args.base and args.head:
        plan = ci_plan(files=_diff_files(args.base, args.head), public=public)
    elif args.components:
        plan = ci_plan(selected=args.components.split(), public=public)
    else:
        parser.error("ci-plan needs --files, --base and --head, or --components")
    print(f"components={format_for_make(plan.components)}")
    print(f"jobs={' '.join(plan.jobs) or 'none'}")
    return 0


def main(argv: list[str] | None = None) -> int:
    """Command line entry point (`changed`, `ci-plan`, `pins`, `release`); returns the exit code."""
    parser = argparse.ArgumentParser(description="Component map for CI and releases.")
    sub = parser.add_subparsers(dest="cmd", required=True)
    p_changed = sub.add_parser("changed")
    p_changed.add_argument("--base")
    p_changed.add_argument("--head")
    p_changed.add_argument("--files", nargs="*")
    p_plan = sub.add_parser("ci-plan")
    p_plan.add_argument("--base")
    p_plan.add_argument("--head")
    p_plan.add_argument("--files", nargs="*")
    p_plan.add_argument("--components")
    p_plan.add_argument("--public", choices=["true", "false"], required=True)
    p_release = sub.add_parser("release")
    p_release.add_argument("--tag", required=True)
    p_release.add_argument("--repo", type=Path, default=Path.cwd())
    p_release.add_argument("--github-output", action="store_true")
    p_pins = sub.add_parser("pins")
    p_pins.add_argument("--repo", type=Path, default=Path.cwd())
    args = parser.parse_args(argv)

    if args.cmd == "changed":
        if args.files is not None:
            files = args.files
        elif args.base and args.head:
            files = _diff_files(args.base, args.head)
        else:
            parser.error("changed needs --files or --base and --head")
        print(format_for_make(changed(files)))
        return 0
    if args.cmd == "ci-plan":
        return _print_ci_plan(args, parser)
    if args.cmd == "pins":
        problems = pinned_server_problems(args.repo)
        for problem in problems:
            print(f"components: {problem}", file=sys.stderr)
        if problems:
            return 1
        print("components: the server version pinned by the product bundle is valid")
        return 0
    try:
        info = release_info(args.tag, args.repo)
        if args.github_output:
            for key, value in info.items():
                print(f"{key}={str(value).lower() if isinstance(value, bool) else value}")
        else:
            print(json.dumps(info))
    except ReleaseError as exc:
        print(f"components: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
