# SPDX-License-Identifier: Apache-2.0
"""Component map of the monorepo: CI path filter and release tag resolution.

Gate components group code by tool chain. `make gates COMPONENTS=...` runs the
component-specific gates only for the listed ones; repository-wide gates always run.
A change outside every known area selects `all` (fail safe).

Release components map a tag `<component>/v<semver>` (or `v<semver>` for the whole
product) to a directory with its own VERSION file (docs/spec/18-release.md).

Usage:
  components.py changed --base <sha> --head <sha>   print components for `make COMPONENTS=`
  components.py changed --files <path>...
  components.py release --tag <tag> [--github-output]
                                                    print release info as JSON, or as
                                                    key=value lines for $GITHUB_OUTPUT
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
    "apps": ("apps/", "packages/", "design/"),
    "sdk-python": ("sdk/python/",),
    "sdk-typescript": ("sdk/typescript/",),
    "plugins": ("plugins/",),
}
# The API contract feeds generated server stubs and every client.
API_PREFIX = "docs/api/"
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
_SEMVER = re.compile(
    r"^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)"
    r"(?:-((?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*))*))?"
    r"(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$"
)


class ReleaseError(Exception):
    """The tag does not describe a releasable component state."""


def changed(files: list[str]) -> list[str]:
    selected: set[str] = set()
    for path in files:
        owners = [c for c, prefixes in GATE_COMPONENTS.items() if path.startswith(prefixes)]
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
    if not names:
        return "none"
    return " ".join(sorted(names))


def release_info(tag: str, repo: Path) -> dict[str, Any]:
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
    match = _SEMVER.match(version)
    return {
        "component": component,
        "version": version,
        "path": directory,
        "gate_components": gate,
        "prerelease": bool(match and match.group(4)),
    }


def _diff_files(base: str, head: str) -> list[str]:
    out = subprocess.run(
        ["git", "diff", "--name-only", f"{base}...{head}"],
        check=True,
        capture_output=True,
        text=True,
    ).stdout
    return [line for line in out.splitlines() if line]


def main(argv: list[str] | None = None) -> int:
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
