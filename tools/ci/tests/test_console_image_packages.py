# SPDX-License-Identifier: Apache-2.0
"""The console stage of services/server/Dockerfile copies the lib/ and the asset and font
directories of every workspace package the console uses (brief M6-02, docs/spec/18-release.md 7
rule 7), directly or through another package, and Dockerfile.dockerignore lets them into the build
context. A package dependency added to the console or to one of its packages without these lines
fails the image build only in CI (`flutter build web` cannot read the package or its assets); this
test fails first, on every pull request.
"""

from __future__ import annotations

from pathlib import Path

import yaml

REPO = Path(__file__).resolve().parents[3]
DOCKERFILE = REPO / "services" / "server" / "Dockerfile"
DOCKERIGNORE = REPO / "services" / "server" / "Dockerfile.dockerignore"


def _pubspec(path: Path) -> dict[str, object]:
    data = yaml.safe_load(path.read_text(encoding="utf-8"))
    assert isinstance(data, dict), path
    return data


def _dependencies(pubspec: Path) -> set[str]:
    deps = _pubspec(pubspec).get("dependencies") or {}
    assert isinstance(deps, dict), pubspec
    return set(deps)


def console_packages() -> set[str]:
    """Workspace packages under packages/ the console needs at run time, transitively."""
    found: set[str] = set()
    todo = list(_dependencies(REPO / "apps" / "console" / "pubspec.yaml"))
    while todo:
        name = todo.pop()
        pubspec = REPO / "packages" / name / "pubspec.yaml"
        if name in found or not pubspec.is_file():
            continue
        found.add(name)
        todo.extend(_dependencies(pubspec))
    return found


def _directories(name: str) -> set[str]:
    """lib/ plus the top directory of every asset and font file the package's pubspec declares."""
    flutter = _pubspec(REPO / "packages" / name / "pubspec.yaml").get("flutter") or {}
    assert isinstance(flutter, dict), name
    paths = [str(a) for a in flutter.get("assets") or []]
    for family in flutter.get("fonts") or []:
        paths += [str(f["asset"]) for f in family.get("fonts") or []]
    return {"lib"} | {path.split("/", 1)[0] for path in paths}


def required() -> list[tuple[str, str]]:
    return sorted((name, d) for name in console_packages() for d in _directories(name))


def test_console_packages_found() -> None:
    # The walk itself: the console's direct packages and one reached only through admin_ui.
    packages = console_packages()
    assert {"admin_ui", "design_system", "l10n", "bothy_viz"} <= packages
    assert _directories("design_system") == {"lib", "assets", "fonts"}


def test_console_stage_copies_every_package() -> None:
    lines = {line.strip() for line in DOCKERFILE.read_text(encoding="utf-8").splitlines()}
    missing = [
        f"packages/{n}/{d}"
        for n, d in required()
        if f"COPY packages/{n}/{d} packages/{n}/{d}" not in lines
    ]
    assert missing == [], f"the console stage of {DOCKERFILE.name} lacks: {missing}"


def test_build_context_admits_every_package() -> None:
    lines = {line.strip() for line in DOCKERIGNORE.read_text(encoding="utf-8").splitlines()}
    missing = [f"packages/{n}/{d}" for n, d in required() if f"!packages/{n}/{d}/" not in lines]
    assert missing == [], f"{DOCKERIGNORE.name} keeps out: {missing}"
