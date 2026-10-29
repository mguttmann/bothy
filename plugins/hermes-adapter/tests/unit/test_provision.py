# SPDX-License-Identifier: Apache-2.0
"""Provisioning from a local repository: read-only checkout, install from an export."""

from __future__ import annotations

import datetime
import os
import subprocess
import sys
from pathlib import Path

import pytest
from hermes_harness.integrity import IntegrityError, assert_unchanged, snapshot_checkout
from hermes_harness.pin import HermesPin
from hermes_harness.provision import ProvisionError, provision, release

PYPROJECT = """\
[project]
name = "mini"
version = "0.0.0"
requires-python = ">=3.11"
dependencies = []

[tool.uv]
package = false
"""


def _git(cwd: Path, *args: str) -> str:
    env = {
        "PATH": os.environ["PATH"],
        "HOME": str(cwd),
        "GIT_CONFIG_NOSYSTEM": "1",
        "GIT_AUTHOR_NAME": "Alex",
        "GIT_AUTHOR_EMAIL": "alex@example.com",
        "GIT_COMMITTER_NAME": "Alex",
        "GIT_COMMITTER_EMAIL": "alex@example.com",
    }
    return subprocess.run(
        ["git", *args], cwd=cwd, env=env, check=True, capture_output=True, text=True
    ).stdout.strip()


def _repository(tmp_path: Path) -> tuple[Path, str]:
    repo = tmp_path / "upstream"
    repo.mkdir()
    _git(repo, "init", "-q")
    (repo / "pyproject.toml").write_text(PYPROJECT, encoding="utf-8")
    env = {"PATH": os.environ["PATH"], "HOME": str(tmp_path), "UV_OFFLINE": "1"}
    subprocess.run(["uv", "lock", "--quiet"], cwd=repo, env=env, check=True)
    _git(repo, "add", "pyproject.toml", "uv.lock")
    _git(repo, "commit", "-q", "-m", "mini")
    return repo, _git(repo, "rev-parse", "HEAD")


def _pin(repo: Path, commit: str) -> HermesPin:
    return HermesPin(
        repository=repo.as_uri(),
        tag=None,
        commit=commit,
        python=f"{sys.version_info.major}.{sys.version_info.minor}",
        install="uv-lock",
        extras=(),
        recorded_on=datetime.date(2026, 10, 12),
    )


def test_provision_from_a_local_repository(tmp_path: Path) -> None:
    repo, commit = _repository(tmp_path)
    root = tmp_path / "root"
    install = provision(_pin(repo, commit), root, executable="python")
    try:
        assert install.checkout_state.head == commit
        assert install.checkout_state.status == ""
        assert install.checkout.is_relative_to(root.resolve())
        assert install.venv.is_relative_to(root.resolve())
        assert install.export.is_relative_to(root.resolve())
        assert install.executable == install.venv / "bin" / "python"
        assert install.executable.is_absolute()
        assert not os.access(install.checkout / "pyproject.toml", os.W_OK)
        with pytest.raises(PermissionError):
            (install.checkout / "new.txt").write_text("x", encoding="utf-8")
        # The build happened in the export: the checkout has no venv and no build output.
        assert not (install.checkout / ".venv").exists()
        assert (install.export / "pyproject.toml").is_file()
        assert_unchanged(install.checkout_state, snapshot_checkout(install.checkout))
        release(install)
        (install.checkout / "new.txt").write_text("x", encoding="utf-8")
        with pytest.raises(IntegrityError, match=r"new\.txt"):
            assert_unchanged(install.checkout_state, snapshot_checkout(install.checkout))
    finally:
        release(install)


def test_provision_refuses_a_commit_that_does_not_match(tmp_path: Path) -> None:
    repo, commit = _repository(tmp_path)
    (repo / "later.txt").write_text("later\n", encoding="utf-8")
    _git(repo, "add", "later.txt")
    _git(repo, "commit", "-q", "-m", "later")
    other = "0" * 40
    with pytest.raises(ProvisionError):
        provision(_pin(repo, other), tmp_path / "root", executable="python")
    assert commit != other


def test_provision_refuses_a_root_inside_the_repository(tmp_path: Path) -> None:
    repo, commit = _repository(tmp_path)
    inside = Path(__file__).resolve().parent / "not-here"
    with pytest.raises(ProvisionError, match="outside the repository"):
        provision(_pin(repo, commit), inside, executable="python")
    assert not inside.exists()


def test_provision_requires_a_requirements_file_for_that_mode(tmp_path: Path) -> None:
    repo, commit = _repository(tmp_path)
    pin = _pin(repo, commit)
    pin = HermesPin(**{**pin.__dict__, "install": "requirements"})
    with pytest.raises(ProvisionError, match="requirements"):
        provision(pin, tmp_path / "root", executable="python", requirements=tmp_path / "absent.txt")
