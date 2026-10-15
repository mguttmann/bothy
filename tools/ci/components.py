# SPDX-License-Identifier: Apache-2.0
"""Component map of the monorepo: CI path filter and release tag resolution.

Gate components group code by tool chain. `make gates COMPONENTS=...` runs the
component-specific gates only for the listed ones; repository-wide gates always run.
A change outside every known area selects `all` (fail safe).

Release components map a tag `<component>/v<semver>` (or `v<semver>` for the whole
product) to a directory with its own VERSION file (docs/spec/18-release.md).

The product bundle pins the server version in the line `BOTHY_VERSION=` of
infra/compose/.env.example (docs/spec/18-release.md, 2.1). `pins` checks the committed state
before any tag exists: the value is SemVer without "v" (or the template placeholder), and a
stable product (VERSION without pre-release) never pins a server pre-release; only a
product beta (`X.Y.Z-beta.N`) may.

Usage:
  components.py changed --base <sha> --head <sha>   print components for `make COMPONENTS=`
  components.py changed --files <path>...
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
from pathlib import Path
from typing import Any

# Gate component -> path prefixes owned by it.
GATE_COMPONENTS: dict[str, tuple[str, ...]] = {
    "server": ("services/",),
    # The Dart tests of packages/bothy_noise read the Noise test vectors, recordings and
    # fingerprint vectors of the host service (docs/DECISIONS.md D-M2-38-5). The architecture
    # test of the Dart workspace belongs to its gates (docs/DECISIONS.md D-M4-01-8).
    "apps": ("apps/", "packages/", "design/", "services/host/testdata/noise/", "tools/arch-check/"),
    # sdk/testdata/ holds the shared G5 fixtures that both SDK test suites read.
    "sdk-python": ("sdk/python/", "sdk/testdata/"),
    "sdk-typescript": ("sdk/typescript/", "sdk/testdata/"),
    "plugins": ("plugins/",),
}
# The API contract feeds generated server stubs and every client.
API_PREFIX = "docs/api/"
# Files outside a component's directory that its generated code is built from; a change selects
# the component, so that its codegen-check sees the drift.
GATE_COMPONENT_INPUTS: dict[str, tuple[str, ...]] = {
    # services/server/api/generate.go: route table and action catalogue of the server.
    "server": ("docs/spec/actions.yaml",),
    # Chapter 09 is the contract of the adapters under plugins/; its section 17.10 is bound to the
    # recording evidence of plugins/hermes-adapter by the offline fixture tests.
    "plugins": ("docs/spec/09-agents.md",),
    # The root files of the Dart workspace: manifest with the Flutter pin, the one lock file and
    # the shared analysis rules (docs/DECISIONS.md D-M4-01-8).
    "apps": ("pubspec.yaml", "pubspec.lock", "analysis_options.yaml"),
}
# Areas without component gates; their repository-wide gates always run.
NO_COMPONENT_PREFIXES = ("docs/", "infra/")
NO_COMPONENT_ROOT_FILES = re.compile(r"^[^/]+\.md$|^(?:LICENSE|NOTICE)$")

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


def changed(files: list[str]) -> list[str]:
    """Return the gate components touched by the changed `files` (all for unknown paths)."""
    selected: set[str] = set()
    for path in files:
        owners = [c for c, prefixes in GATE_COMPONENTS.items() if path.startswith(prefixes)]
        owners += [c for c, inputs in GATE_COMPONENT_INPUTS.items() if path in inputs]
        if owners:
            selected.update(owners)
        elif path.startswith(API_PREFIX):
            selected.update(GATE_COMPONENTS)
        elif path.startswith(NO_COMPONENT_PREFIXES) or NO_COMPONENT_ROOT_FILES.match(path):
            continue
        else:
            return ["all"]
    return sorted(selected)


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
        ["git", "diff", "--name-only", f"{base}...{head}"],
        check=True,
        capture_output=True,
        text=True,
    ).stdout
    return [line for line in out.splitlines() if line]


def main(argv: list[str] | None = None) -> int:
    """Command line entry point (`changed` or `release`); returns the process exit code."""
    parser = argparse.ArgumentParser(description="Component map for CI and releases.")
    sub = parser.add_subparsers(dest="cmd", required=True)
    p_changed = sub.add_parser("changed")
    p_changed.add_argument("--base")
    p_changed.add_argument("--head")
    p_changed.add_argument("--files", nargs="*")
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
