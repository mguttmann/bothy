# SPDX-License-Identifier: Apache-2.0
"""tools/ci/sandbox.sh runs unreviewed code in a container without root (AUD-CG1/5).

The job `record` of hermes-pin.yml runs the newest upstream commit of Hermes Agent once a day,
in a run on main. As a plain step that code ran as the runner's user, with sudo and the Docker
socket, and could read the credentials of the run, the cache of main among them. The checks run
the real script with a docker program that records its arguments, run the check the script
puts in front of the command on this host (which is no such container, so the command must not
start), and read the real workflow. The run in a real container is the daily run itself and the
trial recorded in docs/decisions.d/AUD-misc-ci-gates-01.md.
"""

from __future__ import annotations

import os
import re
import subprocess
from pathlib import Path
from typing import Any

import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]
SCRIPT = REPO / "tools" / "ci" / "sandbox.sh"
WORKFLOW = REPO / ".github" / "workflows" / "hermes-pin.yml"
COMPONENT = "plugins/hermes-adapter"
SANDBOXED = f"sh tools/ci/sandbox.sh --write {COMPONENT} -- "
# What a `docker run` must never carry here: each gives the command a way to the host.
FORBIDDEN = ("--privileged", "--cap-add", "--pid", "--ipc", "--userns", "--device", "docker.sock")
RECORDER = """#!/bin/sh
cat > /dev/null
printf '%s\\0' "$@" > "$SANDBOX_RECORD/$1.args"
"""


def _call(
    tmp_path: Path, *args: str, env: dict[str, str] | None = None
) -> subprocess.CompletedProcess[str]:
    """Run the real script with a docker program that records the arguments of each call."""
    docker = tmp_path / "docker"
    docker.write_text(RECORDER, encoding="utf-8")
    docker.chmod(0o755)
    uv = tmp_path / "uv"
    uv.write_text("", encoding="utf-8")
    (tmp_path / "record").mkdir(exist_ok=True)
    base = {
        "PATH": os.environ["PATH"],
        "SANDBOX_DOCKER": str(docker),
        "SANDBOX_UV": str(uv),
        "SANDBOX_HOME": str(tmp_path / "home"),
        "SANDBOX_RECORD": str(tmp_path / "record"),
    }
    return subprocess.run(
        ["sh", str(SCRIPT), *args],
        env={**base, **(env or {})},
        capture_output=True,
        text=True,
        check=False,
    )


def _args(tmp_path: Path, call: str) -> list[str]:
    return (tmp_path / "record" / f"{call}.args").read_text(encoding="utf-8").split("\0")[:-1]


def _recorded(tmp_path: Path) -> list[str]:
    done = _call(tmp_path, "--write", COMPONENT, "--", "make", "-C", COMPONENT, "record")
    assert done.returncode == 0, done.stderr
    return _args(tmp_path, "run")


def test_the_container_has_no_root_no_capability_and_no_socket(tmp_path: Path) -> None:
    run = _recorded(tmp_path)
    image = run.index("bothy-sandbox:local")
    options = run[1:image]
    assert options[options.index("--user") + 1] == f"{os.getuid()}:{os.getgid()}"
    assert options[options.index("--cap-drop") + 1] == "ALL"
    assert options[options.index("--security-opt") + 1] == "no-new-privileges"
    assert "--rm" in options
    for word in FORBIDDEN:
        assert not any(word in option for option in options), word
    # No network of the host and no variable of the caller: only the home is set.
    assert "--network" not in options
    assert [options[i + 1] for i, o in enumerate(options) if o == "--env"] == ["HOME=/home/sandbox"]


def test_the_command_can_write_only_the_named_folder_and_its_home(tmp_path: Path) -> None:
    run = _recorded(tmp_path)
    options = run[1 : run.index("bothy-sandbox:local")]
    mounts = [options[i + 1] for i, o in enumerate(options) if o == "--volume"]
    assert mounts == [
        f"{REPO}:/repo:ro",
        f"{tmp_path / 'uv'}:/usr/local/bin/uv:ro",
        f"{tmp_path / 'home'}:/home/sandbox",
        # After the read-only checkout, so it overlays its folder.
        f"{REPO / COMPONENT}:/repo/{COMPONENT}",
    ]
    assert "--mount" not in options
    assert "--volumes-from" not in options


def test_the_command_runs_behind_the_check(tmp_path: Path) -> None:
    run = _recorded(tmp_path)
    image = run.index("bothy-sandbox:local")
    assert run[image + 1 : image + 3] == ["sh", "-c"]
    assert run[image + 4 :] == ["sandbox", "make", "-C", COMPONENT, "record"]
    check = run[image + 3]
    assert check.rstrip().endswith('exec "$@"')


def test_the_image_is_built_for_the_caller_from_the_tracked_file(tmp_path: Path) -> None:
    _recorded(tmp_path)
    build = _args(tmp_path, "build")
    assert build[0] == "build"
    assert build[build.index("--tag") + 1] == "bothy-sandbox:local"
    given = {build[i + 1] for i, word in enumerate(build) if word == "--build-arg"}
    assert given == {f"SANDBOX_UID={os.getuid()}", f"SANDBOX_GID={os.getgid()}"}
    # No build context: the file comes on standard input, nothing of the checkout enters.
    assert build[-1] == "-"
    text = (REPO / "tools" / "ci" / "sandbox.Dockerfile").read_text(encoding="utf-8")
    assert re.search(r"^USER sandbox$", text, re.MULTILINE)
    assert "sudo" not in text.split("FROM", 1)[1]


def test_outside_such_a_container_the_command_does_not_start(tmp_path: Path) -> None:
    """The check the script puts in front of the command, run on this host.

    A test host is no container of the script (root, or new privileges possible, or no
    /proc/self/status at all), so the check must end with 70 before the command.
    """
    run = _recorded(tmp_path)
    check = run[run.index("bothy-sandbox:local") + 3]
    marker = tmp_path / "started"
    done = subprocess.run(
        ["sh", "-c", check, "sandbox", "touch", str(marker)],
        capture_output=True,
        text=True,
        check=False,
    )
    assert done.returncode == 70, done.stderr
    assert "the command did not start" in done.stderr
    assert not marker.exists()


@pytest.mark.parametrize(
    "args",
    [
        [],
        ["--"],
        ["make", "record"],
        ["--write"],
        ["--write", "..", "--", "true"],
        ["--write", "plugins/../.git", "--", "true"],
        ["--write", "/tmp", "--", "true"],
        ["--write", ".", "--", "true"],
        ["--write", "no/such/folder", "--", "true"],
    ],
)
def test_a_wrong_call_starts_nothing(tmp_path: Path, args: list[str]) -> None:
    done = _call(tmp_path, *args)
    assert done.returncode == 2, done.stderr
    assert not list((tmp_path / "record").iterdir())


def test_without_uv_nothing_starts(tmp_path: Path) -> None:
    done = _call(tmp_path, "--", "true", env={"SANDBOX_UV": str(tmp_path / "missing")})
    assert done.returncode == 2
    assert "uv not found" in done.stderr
    assert not list((tmp_path / "record").iterdir())


# --- the workflow ---------------------------------------------------------------------


def _record_job() -> dict[str, Any]:
    job = yaml.safe_load(WORKFLOW.read_text(encoding="utf-8"))["jobs"]["record"]
    assert isinstance(job, dict)
    return job


def _lines(job: dict[str, Any]) -> list[str]:
    runs = "\n".join(str(step.get("run", "")) for step in job["steps"])
    return [line.strip() for line in re.sub(r"\\\n\s*", "", runs).splitlines() if line.strip()]


def test_upstream_code_runs_only_in_the_sandbox() -> None:
    """Every make, pytest and uv call of the job `record` but the repository's own bump."""
    own = "uv run --no-project --python 3.14 plugins/hermes-adapter/tests/follow_main.py"
    starts = re.compile(r"(^|[;&|]\s*)(make|pytest|uv)\b")
    calls = [ln for ln in _lines(_record_job()) if starts.search(ln) and not ln.startswith(own)]
    assert calls == []
    sandboxed = [ln for ln in _lines(_record_job()) if "tools/ci/sandbox.sh" in ln]
    assert sandboxed == [
        SANDBOXED + f"make -C {COMPONENT} record",
        SANDBOXED + f"make -C {COMPONENT} test",
    ]


def test_both_sandbox_calls_share_one_home_outside_the_checkout() -> None:
    steps = [s for s in _record_job()["steps"] if "tools/ci/sandbox.sh" in str(s.get("run", ""))]
    homes = {(s.get("env") or {}).get("SANDBOX_HOME") for s in steps}
    assert homes == {"${{ runner.temp }}/sandbox-home"}


def test_only_regular_fixture_files_leave_the_job() -> None:
    """The folder the sandbox wrote is untrusted: the artifact is a filtered copy outside it."""
    steps = _record_job()["steps"]
    [upload] = [s for s in steps if "upload-artifact" in str(s.get("uses", ""))]
    assert upload["with"]["path"] == "${{ runner.temp }}/hermes-pin-recording/*.json"
    [copy] = [s for s in steps if (s.get("env") or {}).get("TARGET")]
    assert steps.index(copy) < steps.index(upload)
    assert copy["env"] == {
        "SOURCE": f"{COMPONENT}/tests/fixtures/recording",
        "TARGET": "${{ runner.temp }}/hermes-pin-recording",
    }
    run = str(copy["run"])
    for needed in ('[ -f "$path" ]', '[ ! -L "$path" ]', r"[a-z0-9_]+\.json", "realpath"):
        assert needed in run, needed


def _hand_over(tmp_path: Path, prepare: Any) -> subprocess.CompletedProcess[str]:
    """Run the real hand-over step on a checkout whose fixture folder `prepare` filled."""
    [copy] = [s for s in _record_job()["steps"] if (s.get("env") or {}).get("TARGET")]
    work = tmp_path / "work"
    source = work / COMPONENT / "tests" / "fixtures" / "recording"
    source.mkdir(parents=True)
    prepare(source)
    return subprocess.run(
        ["bash", "-e", "-o", "pipefail", "-c", str(copy["run"])],
        cwd=work,
        env={
            "PATH": os.environ["PATH"],
            "GITHUB_WORKSPACE": str(work),
            "SOURCE": copy["env"]["SOURCE"],
            "TARGET": str(tmp_path / "out"),
        },
        capture_output=True,
        text=True,
        check=False,
    )


def test_the_hand_over_copies_the_fixture_files(tmp_path: Path) -> None:
    def prepare(source: Path) -> None:
        (source / "a_1.json").write_text("{}", encoding="utf-8")
        (source / "b.json").write_text("[]", encoding="utf-8")

    done = _hand_over(tmp_path, prepare)
    assert done.returncode == 0, done.stderr + done.stdout
    assert sorted(p.name for p in (tmp_path / "out").iterdir()) == ["a_1.json", "b.json"]


def test_the_hand_over_refuses_a_link_to_a_file_of_the_runner(tmp_path: Path) -> None:
    secret = tmp_path / "secret.json"
    secret.write_text("token", encoding="utf-8")

    def prepare(source: Path) -> None:
        (source / "a.json").write_text("{}", encoding="utf-8")
        (source / "leak.json").symlink_to(secret)

    done = _hand_over(tmp_path, prepare)
    assert done.returncode != 0
    out = tmp_path / "out"
    assert not out.exists() or "leak.json" not in {p.name for p in out.iterdir()}


def test_the_hand_over_refuses_a_fixture_folder_that_is_a_link(tmp_path: Path) -> None:
    elsewhere = tmp_path / "elsewhere"
    elsewhere.mkdir()
    (elsewhere / "event.json").write_text("{}", encoding="utf-8")

    def prepare(source: Path) -> None:
        source.rmdir()
        source.symlink_to(elsewhere, target_is_directory=True)

    done = _hand_over(tmp_path, prepare)
    assert done.returncode != 0
    assert not (tmp_path / "out" / "event.json").exists()


def test_the_hand_over_refuses_another_name(tmp_path: Path) -> None:
    def prepare(source: Path) -> None:
        (source / "Not-A-Fixture.json").write_text("{}", encoding="utf-8")

    assert _hand_over(tmp_path, prepare).returncode != 0
