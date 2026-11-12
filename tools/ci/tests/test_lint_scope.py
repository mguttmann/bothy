# SPDX-License-Identifier: Apache-2.0
"""The scope of ShellCheck, bandit and mypy follows the tracked files, not a list kept by hand.

`make lint` once named the folders ShellCheck and bandit read, and `tools/pyproject.toml` the
folders of mypy; a new script or tool folder outside those lists was checked by nothing
(docs/decisions.d/AUD-root-01.md). ShellCheck now reads every tracked shell script
(tools/ci/shell_scripts.py), and the tests hold the bandit list of the Makefile and the mypy list
of tools/pyproject.toml to the tool folders that git tracks.
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
import tomllib
from pathlib import Path

import shell_scripts
from git_fixtures import commit, init_repo

REPO = Path(__file__).resolve().parents[3]
_DROPPED = {"MAKEFLAGS", "MAKELEVEL", "MFLAGS", "COMPONENTS", "AREAS"}

_FAKE = """#!{python}
import json, os, sys
with open(os.environ["FAKE_LOG"], "a", encoding="utf-8") as fh:
    fh.write(json.dumps(sys.argv[1:]) + "\\n")
"""


def _tracked() -> list[str]:
    out = subprocess.run(
        ["git", "ls-files", "-z"], cwd=REPO, check=True, capture_output=True, text=True
    ).stdout
    return [p for p in out.split("\0") if p]


def _first_line(path: Path) -> bytes:
    try:
        with path.open("rb") as fh:
            return fh.readline(200)
    except OSError:
        return b""


def _is_shell_script(path: str) -> bool:
    """The rule of the gate, written out again: the suffix `.sh` or a shell in the `#!` line."""
    if path.endswith(".sh"):
        return True
    first = _first_line(REPO / path)
    if not first.startswith(b"#!"):
        return False
    words = [word.rsplit(b"/", 1)[-1] for word in first[2:].split()]
    if words[:1] == [b"env"]:
        words = words[1:]
    return words[:1] in ([b"sh"], [b"bash"], [b"dash"], [b"ksh"])


def test_shell_scripts_are_known_by_name_or_by_their_first_line(tmp_path: Path) -> None:
    init_repo(tmp_path)
    files = {
        "a/by-name.sh": "echo no shebang, sourced by another script\n",
        "a/stub": "#!/bin/sh\nexit 0\n",
        "b/with space": "#!/usr/bin/env bash\nexit 0\n",
        "b/flags": "#! /bin/bash -eu\nexit 0\n",
        "b/dash": "#!/usr/bin/dash\nexit 0\n",
        "c/tool.py": "#!/usr/bin/env python3\nprint()\n",
        "c/fish": "#!/usr/bin/env fish\necho\n",
        "c/notes.txt": "sh is mentioned here\n",
        "d/zsh-not-read": "#!/bin/zsh\necho\n",
    }
    for name, text in files.items():
        path = tmp_path / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
    (tmp_path / "c" / "image.bin").write_bytes(b"\x89PNG\r\n\x00\xff")
    commit(tmp_path, "scripts")
    (tmp_path / "untracked.sh").write_text("#!/bin/sh\n", encoding="utf-8")
    assert shell_scripts.tracked(tmp_path) == [
        "a/by-name.sh",
        "a/stub",
        "b/dash",
        "b/flags",
        "b/with space",
    ]


def test_a_tracked_file_that_is_gone_is_left_out(tmp_path: Path) -> None:
    """A deleted but not yet committed script must not make the linter fail on a missing file."""
    init_repo(tmp_path)
    (tmp_path / "gone.sh").write_text("#!/bin/sh\n", encoding="utf-8")
    (tmp_path / "kept.sh").write_text("#!/bin/sh\n", encoding="utf-8")
    commit(tmp_path, "scripts")
    (tmp_path / "gone.sh").unlink()
    assert shell_scripts.tracked(tmp_path) == ["kept.sh"]


def _make_env(tmp: Path) -> tuple[dict[str, str], Path, Path]:
    log = tmp / "calls.jsonl"
    bin_dir = tmp / "bin"
    bin_dir.mkdir()
    fake = bin_dir / "shellcheck"
    fake.write_text(_FAKE.format(python=sys.executable), encoding="utf-8")
    fetch = bin_dir / "fetch"
    fetch.write_text(f"#!/bin/sh\necho {fake}\n", encoding="utf-8")
    for exe in (fake, fetch):
        exe.chmod(0o755)
    env = {k: v for k, v in os.environ.items() if k not in _DROPPED}
    env["FAKE_LOG"] = str(log)
    return env, fetch, log


def _lint_commands(*variables: str, env: dict[str, str] | None = None) -> list[str]:
    """The commands of `make lint` as `make -n` prints them, continuation lines joined."""
    printed = subprocess.run(
        ["make", "-n", "-C", str(REPO), "lint", "COMPONENTS=all", "AREAS=all", *variables],
        env=env or {k: v for k, v in os.environ.items() if k not in _DROPPED},
        check=True,
        capture_output=True,
        text=True,
    ).stdout
    commands: list[str] = []
    for line in printed.splitlines():
        if commands and (commands[-1].endswith("\\") or line.startswith("\t")):
            commands[-1] += "\n" + line
        else:
            commands.append(line)
    return commands


def test_make_lint_gives_every_tracked_shell_script_to_shellcheck(tmp_path: Path) -> None:
    """The real recipe line, run with a recording ShellCheck, against the tracked files."""
    env, fetch, log = _make_env(tmp_path)
    commands = _lint_commands(f"FETCH={fetch}", env=env)
    [command] = [c for c in commands if "shell_scripts.py --shellcheck" in c]
    done = subprocess.run(
        ["bash", "-euo", "pipefail", "-c", command],
        cwd=REPO,
        env=env,
        check=False,
        capture_output=True,
        text=True,
    )
    assert done.returncode == 0, done.stdout + done.stderr
    [checked] = [json.loads(line) for line in log.read_text(encoding="utf-8").splitlines()]
    expected = [path for path in _tracked() if _is_shell_script(path)]
    assert sorted(checked) == sorted(expected)
    # Scripts that ship or run on another host, and a stub without the suffix.
    for script in (
        "services/server/images/bothy-postgres/entrypoint.sh",
        "infra/host/linux/postinstall.sh",
        "infra/observability/alertmanager/start.sh",
        "infra/compose/install.sh",
        "infra/compose/test/install/stubs/docker",
    ):
        assert script in checked


def _tool_folders() -> list[str]:
    """Folders directly under tools/ with tracked Python code outside their tests."""
    folders = set()
    for path in _tracked():
        parts = Path(path).parts
        if len(parts) > 2 and parts[0] == "tools" and path.endswith(".py") and "tests" not in parts:
            folders.add(parts[1])
    return sorted(folders)


def test_the_repository_has_tool_folders() -> None:
    assert {"ci", "distribution", "design-tokens", "testing"} <= set(_tool_folders())


def test_bandit_reads_every_tool_folder() -> None:
    [command] = [c for c in _lint_commands() if " bandit -q -c tools/pyproject.toml " in c]
    words = [word.rstrip(";") for word in command.split()]
    scanned = {word.removeprefix("tools/") for word in words if word.startswith("tools/")}
    scanned.discard("pyproject.toml")
    assert sorted(scanned) == _tool_folders()


# Tool folders mypy does not read, each with its reason.
NOT_TYPE_CHECKED = {
    "design-tokens": "a standalone generator on the Python standard library (PEP 723 header of "
    "tokens.py) with its own gate `make tokens` and its own ruff.toml; it is no package of the "
    "tool environment and was never written for strict mypy",
}


def test_mypy_reads_every_tool_folder() -> None:
    config = tomllib.loads((REPO / "tools" / "pyproject.toml").read_text(encoding="utf-8"))
    expected = [folder for folder in _tool_folders() if folder not in NOT_TYPE_CHECKED]
    assert sorted(config["tool"]["mypy"]["files"]) == expected
    assert set(NOT_TYPE_CHECKED) <= set(_tool_folders()), "an exception without its folder"
