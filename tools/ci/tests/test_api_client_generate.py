# SPDX-License-Identifier: Apache-2.0
"""Gate G4 for the Dart client: `make -C packages/api_client generate-check` (brief M4-02).

The target regenerates `lib/src/generated/` with the pinned openapi-generator image into a
temporary copy of the package, compares it byte for byte with the checked-in state and analyses
it (docs/DECISIONS.md D-M4-02-1, D-M4-02-2). The end-to-end tests need Docker and the Flutter SDK;
without them they are skipped with the reason, and the report of the build says where they ran.
The negative cases run against a temporary copy of the files the target reads (the package, the
spec and the workspace root files), so the working tree of this repository is never touched.
"""

from __future__ import annotations

import os
import re
import shutil
import subprocess
from pathlib import Path

import pytest

REPO = Path(__file__).resolve().parents[3]
PACKAGE = REPO / "packages" / "api_client"
# Configuration a surrounding `make test` may set must not leak into the make runs below.
_DROPPED = {"MAKEFLAGS", "MAKELEVEL", "MFLAGS", "COMPONENTS", "API_CLIENTS"}
_GENERATED_THREAD = "lib/src/generated/model/thread.dart"


def _docker_ready() -> bool:
    if shutil.which("docker") is None:
        return False
    probe = subprocess.run(
        ["docker", "info", "--format", "{{.ServerVersion}}"],
        capture_output=True,
        text=True,
        check=False,
        timeout=60,
    )
    return probe.returncode == 0


needs_generator = pytest.mark.skipif(
    shutil.which("flutter") is None or not _docker_ready(),
    reason="needs Docker (the pinned generator image) and the Flutter SDK on PATH (brief M4-02)",
)


def _env() -> dict[str, str]:
    return {k: v for k, v in os.environ.items() if k not in _DROPPED}


def _generate_check(package: Path, tmpdir: Path | None = None) -> subprocess.CompletedProcess[str]:
    env = _env()
    if tmpdir is not None:
        env["TMPDIR"] = str(tmpdir)
    return subprocess.run(
        ["make", "-s", "-C", str(package), "generate-check"],
        env=env,
        capture_output=True,
        text=True,
        check=False,
        timeout=1800,
    )


def _status() -> str:
    return subprocess.run(
        ["git", "-C", str(REPO), "status", "--porcelain", "--untracked-files=all"],
        check=True,
        capture_output=True,
        text=True,
    ).stdout


def _copy(tmp_path: Path) -> Path:
    """The files `generate-check` reads, in the layout of the repository."""
    root = tmp_path / "repo"
    package = root / "packages" / "api_client"
    shutil.copytree(
        PACKAGE, package, ignore=shutil.ignore_patterns(".dart_tool", "build", "coverage")
    )
    (root / "docs" / "api").mkdir(parents=True)
    shutil.copy2(REPO / "docs" / "api" / "openapi.yaml", root / "docs" / "api" / "openapi.yaml")
    for name in ("pubspec.yaml", "pubspec.lock", "analysis_options.yaml"):
        shutil.copy2(REPO / name, root / name)
    return package


def _changed(output: str) -> list[str]:
    return re.findall(r"^changed: (\S+)$", output, re.MULTILINE)


def test_generator_image_pinned_by_digest() -> None:
    text = (PACKAGE / "Makefile").read_text(encoding="utf-8")
    image = re.search(r"^GENERATOR_IMAGE := (\S+)$", text, re.MULTILINE)
    assert image, "GENERATOR_IMAGE is not set in packages/api_client/Makefile"
    assert re.fullmatch(
        r"openapitools/openapi-generator-cli:v7\.26\.0@sha256:[0-9a-f]{64}", image.group(1)
    )
    flags = re.search(r"^GENERATOR_DOCKER_FLAGS := (.+)$", text, re.MULTILINE)
    assert flags
    assert "--network none" in flags.group(1)
    script = (PACKAGE / "tool" / "generate.sh").read_text(encoding="utf-8")
    assert '"${docker_flags[@]}"' in script
    # The digest the docs name is the one the Makefile pins.
    digest = image.group(1).split("@", 1)[1]
    assert digest in (REPO / "docs" / "quality" / "toolchain.md").read_text(encoding="utf-8")


@needs_generator
def test_generate_check_passes_on_clean_tree(tmp_path: Path) -> None:
    before = _status()
    # Docker must be able to mount the temporary folders, so they stay below the caller's.
    scratch = Path(os.environ.get("TMPDIR", "/tmp")) / tmp_path.name
    scratch.mkdir()
    try:
        result = _generate_check(PACKAGE, scratch)

        assert result.returncode == 0, result.stdout + result.stderr
        assert "api_client: generated code equals a fresh generation" in result.stdout
        assert _status() == before
        # Every temporary folder of the run is removed on exit.
        assert list(scratch.iterdir()) == []
    finally:
        shutil.rmtree(scratch, ignore_errors=True)


@needs_generator
def test_spec_change_without_regeneration_fails(tmp_path: Path) -> None:
    package = _copy(tmp_path)
    spec = package.parents[1] / "docs" / "api" / "openapi.yaml"
    text = spec.read_text(encoding="utf-8")
    start = text.index("\n    Thread:\n")
    anchor = text.index("\n      properties:\n", start) + len("\n      properties:\n")
    spec.write_text(
        text[:anchor] + "        drift_probe_example:\n          type: string\n" + text[anchor:],
        encoding="utf-8",
    )
    before = (package / _GENERATED_THREAD).read_bytes()

    result = _generate_check(package)

    output = result.stdout + result.stderr
    assert result.returncode != 0, output
    assert _GENERATED_THREAD in _changed(output), output
    assert "make -C packages/api_client generate" in output
    # The gate never writes into the tree it checks.
    assert (package / _GENERATED_THREAD).read_bytes() == before


@needs_generator
def test_hand_edit_of_generated_file_fails(tmp_path: Path) -> None:
    package = _copy(tmp_path)
    target = package / _GENERATED_THREAD
    text = target.read_text(encoding="utf-8")
    # One character: a doc comment line of the generated model gains a space.
    target.write_text(text.replace("/// ", "///  ", 1), encoding="utf-8")

    result = _generate_check(package)

    output = result.stdout + result.stderr
    assert result.returncode != 0, output
    assert _changed(output) == [_GENERATED_THREAD], output
