# SPDX-License-Identifier: Apache-2.0
"""Tests for the gate `make api-clients` (G4, docs/spec/06-api.md 15) and the SDK drift checks.

The recipe runs against throwaway client directories: `API_CLIENTS` names them with their gate
component, and each lives in its own temporary git repository, so that "tracked manifest" means
what it means in the real repository. The end-to-end tests copy a real SDK directory into a
temporary repository, change a copy of the spec and run the real `generate-check` there; the
working tree of this repository is never touched.
"""

from __future__ import annotations

import os
import re
import shutil
import subprocess
from pathlib import Path

import pytest

REPO = Path(__file__).resolve().parents[3]
# The test may run under `make test` and in a shell or CI job that sets configuration the
# Makefile reads with `?=`; neither may leak into the make runs below.
_DROPPED = {"MAKEFLAGS", "MAKELEVEL", "MFLAGS", "COMPONENTS", "API_CLIENTS", "SPEC", "TOOLS"}
_SKIP = re.compile(r"^\s*SKIP\s+api-clients\s+(.*)$", re.MULTILINE)
_TARGET_FAILING = "generate-check:\n\t@echo generate-check ran in $(CURDIR)\n\t@exit 3\n"
_TARGET_PASSING = "generate-check:\n\t@echo generate-check ran in $(CURDIR)\n"


def _env() -> dict[str, str]:
    return {k: v for k, v in os.environ.items() if k not in _DROPPED}


def make(
    *args: str, cwd: Path = REPO, env: dict[str, str] | None = None
) -> subprocess.CompletedProcess[str]:
    """Run make; the test itself may run under `make test`."""
    return subprocess.run(
        ["make", "-s", "-C", str(cwd), *args],
        env=_env() if env is None else env,
        capture_output=True,
        text=True,
        check=False,
        timeout=600,
    )


def _git(repo: Path, *args: str) -> str:
    return subprocess.run(
        ["git", "-C", str(repo), *args], check=True, capture_output=True, text=True
    ).stdout


def _commit_all(repo: Path) -> None:
    _git(repo, "init", "-q", "-b", "main")
    _git(repo, "config", "commit.gpgsign", "false")
    _git(repo, "config", "core.hooksPath", "/dev/null")
    _git(repo, "add", "-A")
    _git(repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "x")


def _client(root: Path, name: str, files: dict[str, str]) -> Path:
    directory = root / name
    directory.mkdir(parents=True)
    for rel, text in files.items():
        (directory / rel).write_text(text, encoding="utf-8")
    return directory


@pytest.fixture
def clients(tmp_path: Path) -> Path:
    """A git repository for throwaway client directories (committed by each test)."""
    root = tmp_path / "clients"
    root.mkdir()
    return root


def _api_clients(*entries: tuple[Path, str], components: str = "all") -> list[str]:
    value = " ".join(f"{d}:{c}" for d, c in entries)
    return ["api-clients", f"API_CLIENTS={value}", f"COMPONENTS={components}"]


def test_client_without_manifest_is_skipped(clients: Path) -> None:
    empty = _client(clients, "empty", {"README.md": "no code yet\n"})
    untracked = _client(clients, "untracked", {})
    _commit_all(clients)
    (untracked / "pyproject.toml").write_text("[project]\n", encoding="utf-8")
    missing = clients / "missing"

    result = make(*_api_clients((empty, "sdk-python"), (untracked, "apps"), (missing, "apps")))

    assert result.returncode == 0, result.stderr
    skips = _SKIP.findall(result.stdout)
    assert skips == [
        f"{empty} has no tracked manifest yet",
        f"{untracked} has no tracked manifest yet",
        f"{missing} has no tracked manifest yet",
    ]


@pytest.mark.parametrize(
    ("manifest", "makefile"),
    [
        ("pyproject.toml", None),
        ("package.json", "generate:\n\t@true\n"),
        ("pubspec.yaml", "check:\n\t@true\n"),
    ],
)
def test_client_with_manifest_but_without_generate_check_fails(
    clients: Path, manifest: str, makefile: str | None
) -> None:
    files = {manifest: "{}\n"}
    if makefile is not None:
        files["Makefile"] = makefile
    client = _client(clients, "client", files)
    _commit_all(clients)

    result = make(*_api_clients((client, "sdk-typescript")))

    assert result.returncode != 0
    assert f"api-clients: {client} has a manifest but no Makefile with the target" in result.stderr
    assert "generate-check" in result.stderr
    assert "docs/spec/23-quality.md" in result.stderr


def test_failing_generate_check_fails_the_gate(clients: Path) -> None:
    first = _client(clients, "first", {"package.json": "{}\n", "Makefile": _TARGET_FAILING})
    second = _client(clients, "second", {"package.json": "{}\n", "Makefile": _TARGET_PASSING})
    _commit_all(clients)

    result = make(*_api_clients((first, "sdk-typescript"), (second, "sdk-python")))

    assert result.returncode != 0
    assert f"generate-check ran in {first}" in result.stdout
    # The gate stops at the first failing client.
    assert f"generate-check ran in {second}" not in result.stdout


def test_passing_generate_check_passes(clients: Path) -> None:
    first = _client(clients, "first", {"pyproject.toml": "\n", "Makefile": _TARGET_PASSING})
    second = _client(clients, "second", {"pubspec.yaml": "\n", "Makefile": _TARGET_PASSING})
    _commit_all(clients)

    result = make(*_api_clients((first, "sdk-python"), (second, "apps")))

    assert result.returncode == 0, result.stderr
    assert f"generate-check ran in {first}" in result.stdout
    assert f"generate-check ran in {second}" in result.stdout
    assert _SKIP.findall(result.stdout) == []


def test_git_dir_of_a_hook_does_not_hide_the_manifest(clients: Path, tmp_path: Path) -> None:
    # git exports GIT_DIR to hooks (absolute in a linked worktree) and the pre-push hook runs
    # `make check`; the gate must still find the tracked manifests of each client directory.
    client = _client(clients, "client", {"package.json": "{}\n", "Makefile": _TARGET_PASSING})
    _commit_all(clients)
    outer = tmp_path / "outer"
    outer.mkdir()
    _git(outer, "init", "-q", "-b", "main")
    env = {**_env(), "GIT_DIR": str(outer / ".git"), "GIT_WORK_TREE": str(outer)}

    result = make(*_api_clients((client, "sdk-typescript")), env=env)

    assert result.returncode == 0, result.stderr
    assert f"generate-check ran in {client}" in result.stdout
    assert _SKIP.findall(result.stdout) == []


def test_unselected_component_is_skipped(clients: Path) -> None:
    py = _client(clients, "py", {"pyproject.toml": "\n", "Makefile": _TARGET_FAILING})
    ts = _client(clients, "ts", {"package.json": "{}\n", "Makefile": _TARGET_FAILING})
    _commit_all(clients)

    result = make(*_api_clients((py, "sdk-python"), (ts, "sdk-typescript"), components="server"))

    assert result.returncode == 0, result.stderr
    assert "generate-check ran" not in result.stdout
    assert _SKIP.findall(result.stdout) == [
        f"{py}: component sdk-python not selected",
        f"{ts}: component sdk-typescript not selected",
    ]


def test_unselected_real_sdks_are_skipped() -> None:
    result = make("api-clients", "COMPONENTS=server")

    assert result.returncode == 0, result.stderr
    skips = _SKIP.findall(result.stdout)
    assert "sdk/python: component sdk-python not selected" in skips
    assert "sdk/typescript: component sdk-typescript not selected" in skips
    assert "codegen-check" not in result.stdout + result.stderr


def test_default_client_list_names_every_client() -> None:
    printed = make("-n", "api-clients", "COMPONENTS=none").stdout
    for entry in (
        "sdk/python:sdk-python",
        "sdk/typescript:sdk-typescript",
        "packages/api_client:apps",
    ):
        assert entry in printed


# ---- end to end: a spec change without regeneration fails generate-check (G4) ------------

_COPY_IGNORE = shutil.ignore_patterns(
    ".venv", "node_modules", "coverage", "coverage.xml", ".coverage", "__pycache__", "*_cache"
)
_GENERATED_THREAD = {
    "python": "src/bothy_sdk/generated/models/thread.py",
    "typescript": "src/generated/openapi.ts",
}


def _spec_with_new_thread_member(target: Path) -> Path:
    text = (REPO / "docs" / "api" / "openapi.yaml").read_text(encoding="utf-8")
    start = text.index("\n    Thread:\n")
    anchor = text.index("\n      properties:\n", start) + len("\n      properties:\n")
    changed = (
        text[:anchor] + "        drift_probe_example:\n          type: string\n" + text[anchor:]
    )
    target.write_text(changed, encoding="utf-8")
    return target


_STRAY = {
    "python": "src/bothy_sdk/generated/stray_example.py",
    "typescript": "src/generated/stray_example.ts",
}


def _sdk_copy(tmp_path: Path, sdk: str, extra: dict[str, str] | None = None) -> Path:
    """Copy sdk/<sdk> into a temporary git repository (with the repository .gitignore)."""
    source = REPO / "sdk" / sdk
    if (
        sdk == "typescript"
        and not (source / "node_modules" / ".bin" / "openapi-typescript").exists()
    ):
        pytest.fail("sdk/typescript/node_modules is missing; run `make bootstrap-ci`")
    repo = tmp_path / "repo"
    repo.mkdir()
    shutil.copy2(REPO / ".gitignore", repo / ".gitignore")
    copy = repo / "sdk" / sdk
    shutil.copytree(source, copy, ignore=_COPY_IGNORE, symlinks=True)
    if sdk == "typescript":
        (copy / "node_modules").symlink_to(source / "node_modules", target_is_directory=True)
    for rel, text in (extra or {}).items():
        (copy / rel).write_text(text, encoding="utf-8")
    _commit_all(repo)
    return copy


@pytest.mark.parametrize("sdk", ["python", "typescript"])
def test_g4_file_the_generator_does_not_write_fails(tmp_path: Path, sdk: str) -> None:
    # A checked-in file inside the generated directory that a fresh generation does not
    # produce is drift too: it would escape lint, coverage and Sonar as "generated".
    before = _git(REPO, "status", "--porcelain", "--untracked-files=all")
    text = "x = 1\n" if sdk == "python" else "export const x = 1;\n"
    copy = _sdk_copy(tmp_path, sdk, {_STRAY[sdk]: text})

    result = make(
        "generate-check",
        f"TOOLS={REPO / 'tools'}",
        f"SPEC={REPO / 'docs/api/openapi.yaml'}",
        cwd=copy,
    )

    output = result.stdout + result.stderr
    assert result.returncode != 0, output
    assert f"removed: {_STRAY[sdk]}" in output.splitlines(), output
    assert _git(REPO, "status", "--porcelain", "--untracked-files=all") == before


@pytest.mark.parametrize("sdk", ["python", "typescript"])
def test_g4_spec_change_without_regeneration_fails(tmp_path: Path, sdk: str) -> None:
    source = REPO / "sdk" / sdk
    if (
        sdk == "typescript"
        and not (source / "node_modules" / ".bin" / "openapi-typescript").exists()
    ):
        pytest.fail("sdk/typescript/node_modules is missing; run `make bootstrap-ci`")
    before = _git(REPO, "status", "--porcelain", "--untracked-files=all")
    repo = tmp_path / "repo"
    repo.mkdir()
    shutil.copy2(REPO / ".gitignore", repo / ".gitignore")
    copy = repo / "sdk" / sdk
    shutil.copytree(source, copy, ignore=_COPY_IGNORE, symlinks=True)
    if sdk == "typescript":
        (copy / "node_modules").symlink_to(source / "node_modules", target_is_directory=True)
    _commit_all(repo)
    spec = _spec_with_new_thread_member(tmp_path / "openapi.yaml")

    result = make("generate-check", f"SPEC={spec}", f"TOOLS={REPO / 'tools'}", cwd=copy)

    output = result.stdout + result.stderr
    assert result.returncode != 0, output
    assert "codegen-check: generated code in" in output
    changed = re.findall(r"^\s*changed: (\S+)$", output, re.MULTILINE)
    assert _GENERATED_THREAD[sdk] in changed, output
    assert "drift_probe_example" in (copy / _GENERATED_THREAD[sdk]).read_text(encoding="utf-8")
    assert _git(REPO, "status", "--porcelain", "--untracked-files=all") == before
