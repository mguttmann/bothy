# SPDX-License-Identifier: Apache-2.0
"""The platform suite workflow of the host service (docs/decisions.d/AUD-misc-misc-01.md).

`.github/workflows/host-platform.yml` runs the platform suite of `services/host` with
administrator rights on throwaway GitHub-hosted runners. `bothy-host` refuses to start when a
session user can write a folder on the way to its program folder (HD10 `helper_untrusted`, HD11
`program_location_untrusted`, docs/spec/24-terminal.md 2.2). The hosted Ubuntu image opens `/opt`
to every user while it is built, and both the program folder of the product on Linux and the
folder of the suite lie below it, so every test that started the service failed there. The
workflow therefore gives such a folder the owner and mode of a stock system before the suite.

The tests read the places from their sources (the install plan of the product, the suite and the
workflow), so a place that moves takes the step with it.
"""

from __future__ import annotations

import re
from pathlib import Path
from typing import Any

import check_runners as cr

REPO = Path(__file__).resolve().parents[3]
WORKFLOW = REPO / ".github" / "workflows" / "host-platform.yml"
INSTALL_PLAN = REPO / "services" / "host" / "internal" / "install" / "plan.go"
SUITE_STEPS = REPO / "services" / "host" / "test" / "platform" / "steps_test.go"

LINUX_ONLY = "runner.os == 'Linux'"
_PROGRAM_DIR = re.compile(r'^\s*LinuxProgramDir\s*=\s*"(/[^"]+)"', re.MULTILINE)
_SUITE_DIR = re.compile(r'filepath\.Join\("(/[^"]+)", name\)')
_PLATFORM_OUT = re.compile(r"\bPLATFORM_OUT=(/\S+)")


def steps() -> list[dict[str, Any]]:
    """The steps of the one job of the workflow."""
    doc = cr.load_yaml(WORKFLOW.read_text(encoding="utf-8"))
    assert isinstance(doc, dict)
    (job,) = doc["jobs"].values()
    found = job["steps"]
    assert isinstance(found, list)
    return found


def suite_step(found: list[dict[str, Any]]) -> int:
    """The position of the step that runs the platform suite on Linux and macOS."""
    (index,) = [
        i
        for i, step in enumerate(found)
        if "platform-test" in str(step.get("run", "")) and "sudo" in str(step.get("run", ""))
    ]
    return index


def top_folder(path: str) -> str:
    """The folder directly below `/` on the way to `path`: `top_folder("/opt/x/bin") == "/opt"`."""
    return "/" + path.strip("/").split("/", 1)[0]


def linux_places() -> dict[str, str]:
    """The Linux folders the service must trust, by the source that names them."""
    program = _PROGRAM_DIR.search(INSTALL_PLAN.read_text(encoding="utf-8"))
    suite = _SUITE_DIR.search(SUITE_STEPS.read_text(encoding="utf-8"))
    run = str(steps()[suite_step(steps())]["run"])
    bundle = _PLATFORM_OUT.search(run)
    moved = "a place moved; adjust the pattern that reads it"
    assert program, moved
    assert suite, moved
    assert bundle, moved
    return {
        "install plan (LinuxProgramDir)": program.group(1),
        "platform suite (programDir)": suite.group(1),
        "workflow (PLATFORM_OUT)": bundle.group(1),
    }


def resets(run: str, folder: str) -> bool:
    """Whether the shell text gives `folder` itself to root with mode 0755, as root."""
    owner = re.search(rf"\bsudo chown root:root {re.escape(folder)}(?=\s|$)", run)
    mode = re.search(rf"\bsudo chmod 0?755 {re.escape(folder)}(?=\s|$)", run)
    return bool(owner and mode)


def test_the_places_are_read_from_their_sources() -> None:
    places = linux_places()
    assert places["install plan (LinuxProgramDir)"] == "/opt/bothy-host/bin"
    assert {top_folder(p) for p in places.values()} == {"/opt"}


def test_linux_gives_the_folders_above_the_programs_back_to_root_before_the_suite() -> None:
    found = steps()
    before = found[: suite_step(found)]
    linux = [str(s.get("run", "")) for s in before if LINUX_ONLY in str(s.get("if", ""))]
    for source, place in linux_places().items():
        folder = top_folder(place)
        assert any(resets(run, folder) for run in linux), (
            f"{source} uses {place}: no Linux step before the platform suite gives {folder} "
            "to root with mode 0755; the hosted Ubuntu image opens it to every user and "
            "bothy-host then refuses to start (helper_untrusted, program_location_untrusted)"
        )


def test_the_reset_runs_only_on_linux_and_touches_the_folder_alone() -> None:
    for step in steps():
        run = str(step.get("run", ""))
        if "chown root:root" not in run and "chmod 0755" not in run and "chmod 755" not in run:
            continue
        assert LINUX_ONLY in str(step.get("if", "")), step.get("name")
        # Not recursive: the tool cache of the runner below /opt keeps its owner.
        assert not re.search(r"\bch(?:own|mod)\s+-\S*R", run), step.get("name")


def test_resets_matches_only_the_folder_itself() -> None:
    assert resets("sudo chown root:root /opt && sudo chmod 0755 /opt", "/opt")
    assert resets("sudo chown root:root /opt\nsudo chmod 755 /opt\n", "/opt")
    assert not resets("sudo chown root:root /opt/x && sudo chmod 0755 /opt/x", "/opt")
    assert not resets("sudo chmod 0755 /opt", "/opt")
    assert not resets("chown root:root /opt && chmod 0755 /opt", "/opt")
    assert not resets("sudo chown root:root /opt && sudo chmod 0777 /opt", "/opt")
