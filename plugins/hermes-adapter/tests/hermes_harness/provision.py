# SPDX-License-Identifier: Apache-2.0
"""Provisioning of the pinned Hermes Agent version in a throwaway directory.

:func:`provision` works only inside ``root``, a fresh temporary directory outside the
repository:

1. Checkout: ``git init``, ``git fetch --depth 1 <repository> <commit>``,
   ``git checkout --detach FETCH_HEAD``; ``HEAD`` must equal the pinned commit. Then every
   write bit of the checkout is removed, so any write attempt fails loudly.
2. Install from an export, never from the checkout: ``git archive <commit>`` unpacked into
   ``root/build``, because build backends write ``build/`` and metadata into the source tree.
   With ``install = "uv-lock"`` Hermes' own ``uv.lock`` is used (``uv sync --locked
   --no-dev``) into ``root/venv``; with ``install = "requirements"`` a hash-pinned list is
   installed first and the export without dependencies after it.
3. Snapshots of the checkout and of the installed files, compared again after the run.

Provisioning needs the network for the pinned downloads; it runs with the allowlisted
provisioning environment (no proxy, a throwaway ``HOME``, the caller's uv cache and Python
directories so repeated runs do not download again).

Example:
    >>> install = provision(load_pin(PIN_FILE), Path(tempfile.mkdtemp()))
    >>> try:
    ...     run(install.executable)
    ... finally:
    ...     release(install)
"""

from __future__ import annotations

import shutil
import subprocess
import tarfile
from collections.abc import Mapping
from dataclasses import dataclass
from pathlib import Path

from hermes_harness.environment import ThrowawayHomes, build_provisioning_environment
from hermes_harness.integrity import (
    CheckoutState,
    installed_digest,
    make_read_only,
    restore_writable,
    snapshot_checkout,
)
from hermes_harness.limits import PROVISION_DEADLINE_S
from hermes_harness.pin import HermesPin

#: The repository this harness belongs to; a temporary root must lie outside it.
REPO_ROOT = Path(__file__).resolve().parents[4]
#: Hash-pinned dependency list, used only when the pinned commit has no ``uv.lock``.
REQUIREMENTS_FILE = Path(__file__).resolve().parents[1] / "hermes_requirements.txt"


class ProvisionError(RuntimeError):
    """A provisioning step failed, or the result does not match the pin."""


@dataclass(frozen=True)
class HermesInstall:
    """A provisioned Hermes version.

    Attributes:
        root: The temporary root.
        checkout: The read-only git checkout of the pinned commit.
        export: The ``git archive`` export the environment was installed from.
        venv: The throwaway virtual environment.
        executable: The Hermes CLI inside the venv, by absolute path.
        checkout_state: Snapshot of the checkout after provisioning.
        installed: Digest of the installed files after provisioning.
    """

    root: Path
    checkout: Path
    export: Path
    venv: Path
    executable: Path
    checkout_state: CheckoutState
    installed: str


def _tool(name: str) -> Path:
    found = shutil.which(name)
    if found is None:
        raise ProvisionError(f"{name} is required for provisioning and was not found on PATH")
    return Path(found)


def _run(command: list[str], *, cwd: Path, env: Mapping[str, str], what: str) -> str:
    try:
        result = subprocess.run(
            command,
            cwd=cwd,
            env=dict(env),
            capture_output=True,
            text=True,
            timeout=PROVISION_DEADLINE_S,
            check=False,
        )
    except subprocess.TimeoutExpired as exc:
        raise ProvisionError(f"{what}: exceeded the provisioning deadline") from exc
    if result.returncode != 0:
        tail = (result.stderr or result.stdout).strip().splitlines()[-5:]
        raise ProvisionError(f"{what} failed ({result.returncode}): " + " | ".join(tail))
    return result.stdout


def _uv_dirs(uv: Path) -> tuple[Path, Path]:
    # Asked with the caller's environment on purpose: only the two directory names are used.
    cache = subprocess.run([str(uv), "cache", "dir"], capture_output=True, text=True, check=True)
    python = subprocess.run([str(uv), "python", "dir"], capture_output=True, text=True, check=True)
    return Path(cache.stdout.strip()), Path(python.stdout.strip())


def provision(
    pin: HermesPin,
    root: Path,
    *,
    executable: str = "hermes",
    requirements: Path = REQUIREMENTS_FILE,
) -> HermesInstall:
    """Check out, export and install the pinned version below ``root``.

    Args:
        pin: The pinned version.
        root: A fresh temporary directory outside the repository.
        executable: Name of the CLI in the venv's ``bin`` directory.
        requirements: The hash-pinned list for ``install = "requirements"``.

    Returns:
        The installation with its snapshots.

    Raises:
        ProvisionError: ``root`` lies inside the repository, a tool is missing, a step fails,
            ``HEAD`` differs from the pinned commit or the executable is missing.
    """
    root = root.resolve()
    if root.is_relative_to(REPO_ROOT):
        raise ProvisionError(f"{root}: the temporary root must lie outside the repository")
    if pin.install == "requirements" and not requirements.is_file():
        raise ProvisionError(f"install = requirements needs {requirements}")
    git, uv = _tool("git"), _tool("uv")
    uv_cache, uv_python = _uv_dirs(uv)
    root.mkdir(parents=True, exist_ok=True)
    homes = ThrowawayHomes.create(root / "provisioning")
    env = build_provisioning_environment(
        homes, uv_cache_dir=uv_cache, uv_python_dir=uv_python, tool_dirs=[git.parent, uv.parent]
    )

    checkout = root / "checkout"
    checkout.mkdir()
    _run([str(git), "init", "-q"], cwd=checkout, env=env, what="git init")
    _run(
        [str(git), "fetch", "-q", "--depth", "1", pin.repository, pin.commit],
        cwd=checkout,
        env=env,
        what="git fetch of the pinned commit",
    )
    _run(
        [str(git), "checkout", "-q", "--detach", "FETCH_HEAD"],
        cwd=checkout,
        env=env,
        what="git checkout",
    )
    head = _run(
        [str(git), "rev-parse", "HEAD"], cwd=checkout, env=env, what="git rev-parse"
    ).strip()
    if head != pin.commit:
        raise ProvisionError(f"checkout HEAD {head} differs from the pinned commit {pin.commit}")
    make_read_only(checkout)

    export = root / "build"
    export.mkdir()
    archive = root / "export.tar"
    with archive.open("wb") as handle:
        result = subprocess.run(
            [str(git), "archive", "--format=tar", pin.commit],
            cwd=checkout,
            env=env,
            stdout=handle,
            stderr=subprocess.PIPE,
            timeout=PROVISION_DEADLINE_S,
            check=False,
        )
    if result.returncode != 0:
        raise ProvisionError(f"git archive failed ({result.returncode})")
    with tarfile.open(archive) as tar:
        tar.extractall(export, filter="data")
    archive.unlink()

    venv = root / "venv"
    if pin.install == "uv-lock":
        command = [str(uv), "sync", "--locked", "--no-dev", "--python", pin.python]
        for extra in pin.extras:
            command += ["--extra", extra]
        _run(command, cwd=export, env={**env, "UV_PROJECT_ENVIRONMENT": str(venv)}, what="uv sync")
    else:
        _run(
            [str(uv), "venv", "-q", str(venv), "--python", pin.python],
            cwd=export,
            env=env,
            what="uv venv",
        )
        python = str(venv / "bin" / "python")
        _run(
            [
                str(uv),
                "pip",
                "install",
                "--python",
                python,
                "--require-hashes",
                "-r",
                str(requirements),
            ],
            cwd=export,
            env=env,
            what="uv pip install of the hash-pinned requirements",
        )
        _run(
            [str(uv), "pip", "install", "--python", python, "--no-deps", "-e", str(export)],
            cwd=export,
            env=env,
            what="uv pip install of the export",
        )
    cli = venv / "bin" / executable
    if not cli.exists():
        raise ProvisionError(f"{cli} is missing after the install")
    return HermesInstall(
        root=root,
        checkout=checkout,
        export=export,
        venv=venv,
        executable=cli,
        checkout_state=snapshot_checkout(checkout),
        installed=installed_digest(venv),
    )


def release(install: HermesInstall) -> None:
    """Restore the write bits of the checkout, so the temporary root can be removed.

    Args:
        install: The installation from :func:`provision`.
    """
    if install.checkout.exists():
        restore_writable(install.checkout)
