# SPDX-License-Identifier: Apache-2.0
"""The desktop E2E stack (brief M4-34): infra/compose/e2e.sh and its overlay.

The script runs with a fake `docker`, `uv` and `curl` on PATH (the real `openssl` makes the test
CA), so the tests need no Docker: the preflight messages, the sanitized environment the Compose
calls see, the cleanup of what the run created and the removal command of a kept stack. The
overlay tests read test/compose.e2e.yaml (it uses no Compose merge tag, so the safe loader
reads it).
"""

from __future__ import annotations

import os
import re
import shutil
import subprocess
from pathlib import Path
from typing import Any

import yaml

REPO = Path(__file__).resolve().parents[3]
COMPOSE_DIR = REPO / "infra" / "compose"
E2E_SH = COMPOSE_DIR / "e2e.sh"
OVERLAY = COMPOSE_DIR / "test" / "compose.e2e.yaml"
E2E_ENV = COMPOSE_DIR / "test" / "e2e.env"

# Answers like docker: every call is logged with the variables the stack reads; `compose ps`
# names no container, so the run stops at the check of secrets-init with exit code 1.
FAKE_DOCKER = """#!/bin/sh
printf '%s\\n' "$*" >> "$FAKE_LOG"
case "$1" in
  compose) printf 'env BOTHY_PUBLIC_URL=%s E2E_PORT=%s COMPOSE_PROJECT_NAME=%s SMOKE_KEEP=%s\\n' \\
    "${BOTHY_PUBLIC_URL:-}" "${E2E_PORT:-}" "${COMPOSE_PROJECT_NAME:-}" "${SMOKE_KEEP:-}" \\
    >> "$FAKE_LOG" ;;
esac
# FAKE_FULL=1 plays a healthy stack: the one-shot containers exist and exit 0, the Fake-Agent
# reports both agents ready, so the run reaches the command.
if [ "${FAKE_FULL:-0}" = 1 ]; then
  case "$*" in
    *" ps --all --quiet "*) printf 'c0ffee\n'; exit 0 ;;
    *" logs --no-color e2e-agent"*)
      printf 'e2e-agent: ready support-agent\ne2e-agent: ready stream-agent\n'; exit 0 ;;
    "inspect --format"*) printf '0\n'; exit 0 ;;
  esac
fi
case "$1 $2" in
  "info "*) exit 0 ;;
  "version --format") printf '29.5.2\\n'; exit 0 ;;
  "compose version") printf '2.40.3\\n'; exit 0 ;;
  "image inspect") exit "$FAKE_IMAGE_RC" ;;
esac
exit 0
"""

# uv: the free port of `uv run` and the requirements of `uv export`.
FAKE_UV = """#!/bin/sh
case "$1" in
  run) printf '54321\\n' ;;
  export) printf 'attrs==26.1.0 \\\\\\n    --hash=sha256:00\\n' ;;
esac
exit 0
"""

FAKE_CURL = "#!/bin/sh\nexit 0\n"

# The tools the script needs from the host besides the fakes.
HOST_TOOLS = (
    "bash", "dirname", "sed", "tr", "cat", "cut", "od", "id", "mktemp", "chmod", "mkdir",
    "rm", "date", "sleep", "openssl", "env", "basename",
)  # fmt: skip


def load(path: Path) -> dict[str, Any]:
    """The YAML document of path."""
    data = yaml.safe_load(path.read_text(encoding="utf-8"))
    assert isinstance(data, dict)
    return data


def run_e2e(
    tmp_path: Path,
    *,
    fakes: tuple[str, ...] = ("docker", "uv", "curl"),
    image_rc: int = 0,
    extra_env: dict[str, str] | None = None,
    command: list[str] | None = None,
) -> tuple[subprocess.CompletedProcess[str], str, Path]:
    """Run e2e.sh with the fakes; return the result, the docker calls and the temp root."""
    bin_dir = tmp_path / "bin"
    bin_dir.mkdir()
    scripts = {"docker": FAKE_DOCKER, "uv": FAKE_UV, "curl": FAKE_CURL}
    for name in fakes:
        fake = bin_dir / name
        fake.write_text(scripts[name], encoding="utf-8")
        fake.chmod(0o755)
    for tool in HOST_TOOLS:
        found = shutil.which(tool)
        assert found is not None, tool
        (bin_dir / tool).symlink_to(found)
    log = tmp_path / "docker.log"
    log.touch()
    root = tmp_path / "tmp"
    root.mkdir()
    env = {
        "PATH": str(bin_dir),
        "HOME": str(tmp_path),
        "TMPDIR": str(root),
        "FAKE_LOG": str(log),
        "FAKE_IMAGE_RC": str(image_rc),
        # Exported variables of these prefixes must not reach the stack.
        "BOTHY_PUBLIC_URL": "https://other.example.com",
        "COMPOSE_PROJECT_NAME": "other",
        "SMOKE_KEEP": "1",
        **(extra_env or {}),
    }
    result = subprocess.run(
        [str(E2E_SH), *(command or [])],
        env=env,
        capture_output=True,
        text=True,
        check=False,
        timeout=120,
    )
    return result, log.read_text(encoding="utf-8"), root


def test_missing_docker_fails(tmp_path: Path) -> None:
    result, _, _ = run_e2e(tmp_path, fakes=("uv", "curl"))
    assert result.returncode == 2
    assert "docker is not on PATH" in result.stderr


def test_missing_image_names_make_image(tmp_path: Path) -> None:
    result, calls, root = run_e2e(tmp_path, image_rc=1)
    assert result.returncode == 2, result.stderr
    assert "not found locally" in result.stderr
    assert "make image" in result.stderr
    assert "compose up" not in calls
    assert list(root.iterdir()) == [], "the preflight creates nothing"


def test_missing_uv_names_make_bootstrap(tmp_path: Path) -> None:
    result, calls, _ = run_e2e(tmp_path, fakes=("docker", "curl"))
    assert result.returncode == 2
    assert "uv is not on PATH" in result.stderr
    assert "make bootstrap" in result.stderr
    assert "compose" not in calls.split("image inspect", 1)[1]


def test_bad_wait_input_fails(tmp_path: Path) -> None:
    result, _, _ = run_e2e(tmp_path, extra_env={"E2E_WAIT_TIMEOUT_S": "soon"})
    assert result.returncode == 2
    assert "E2E_WAIT_TIMEOUT_S must be a whole number" in result.stderr


def test_sanitized_environment_and_cleanup(tmp_path: Path) -> None:
    result, calls, root = run_e2e(tmp_path)
    # The fake names no one-shot container: the run fails after the start, then cleans up.
    assert result.returncode == 1, result.stderr
    assert "no container of secrets-init" in result.stderr
    lines = calls.splitlines()
    # The variables of every call of the stack (the line after it; not `compose version`).
    compose_env = [
        lines[i + 1] for i, line in enumerate(lines) if line.startswith("compose --project-name")
    ]
    assert compose_env, "no compose call"
    for line in compose_env:
        assert "BOTHY_PUBLIC_URL=https://localhost:54321 " in line, line
        assert "E2E_PORT=54321 " in line, line
        assert "COMPOSE_PROJECT_NAME= " in line, line
        assert line.endswith("SMOKE_KEEP="), line
    project = re.search(r"--project-name (bothy-e2e-[0-9a-f]{8})", calls)
    assert project is not None
    run_id = project.group(1).removeprefix("bothy-e2e-")
    assert f"--project-name {project.group(1)}" in calls
    assert "--env-file " + str(COMPOSE_DIR / "test" / "e2e.env") in calls
    assert re.search(r"compose .* down --volumes --remove-orphans --rmi local", calls)
    assert f"image rm bothy-smoke:{run_id} bothy-e2e-agent:{run_id}" in calls
    assert list(root.iterdir()) == [], "the temp directory of the run is removed"
    assert "e2e: FAIL" in result.stderr


def test_the_command_sees_its_inputs_and_not_the_shell(tmp_path: Path) -> None:
    seen = tmp_path / "seen.txt"
    command = tmp_path / "command.sh"
    command.write_text(
        "#!/bin/sh\n"
        f"printf 'device=%s\\nconfig=%s\\npublic=%s\\n' \"${{E2E_DEVICE:-}}\" "
        f'"${{E2E_CONFIG:-}}" "${{BOTHY_PUBLIC_URL:-}}" > {seen}\n',
        encoding="utf-8",
    )
    command.chmod(0o755)
    result, _, _ = run_e2e(
        tmp_path,
        extra_env={"FAKE_FULL": "1", "E2E_DEVICE": "flutter-tester"},
        command=[str(command)],
    )
    assert result.returncode == 0, result.stderr
    assert "e2e: PASS" in result.stderr
    lines = dict(line.split("=", 1) for line in seen.read_text(encoding="utf-8").splitlines())
    # The suite's device choice reaches the command (make e2e-desktop E2E_DEVICE=...).
    assert lines["device"] == "flutter-tester"
    assert lines["config"].endswith("/out/e2e.json")
    # The developer's BOTHY_PUBLIC_URL does not; the stack's own address does.
    assert lines["public"] == "https://localhost:54321"


def test_build_input_builds_the_overlay_services_and_cleanup_removes_their_images(
    tmp_path: Path,
) -> None:
    # The mobile E2E run (brief M5-11) builds the E2E server image and the fake push providers
    # of its overlay; every image tagged with the run ID goes at the cleanup.
    overlay = COMPOSE_DIR / "test" / "compose.id-t16.yaml"
    result, calls, _ = run_e2e(
        tmp_path,
        extra_env={"FAKE_FULL": "1", "E2E_OVERLAY": str(overlay), "E2E_BUILD": "api fakepush"},
    )
    assert result.returncode == 0, result.stderr
    version = (REPO / "services" / "server" / "VERSION").read_text(encoding="utf-8").strip()
    assert re.search(rf"compose .* build --quiet --build-arg VERSION={version} api fakepush", calls)
    project = re.search(r"--project-name bothy-e2e-([0-9a-f]{8})", calls)
    assert project is not None
    assert f"image ls --filter reference=*:{project.group(1)}" in calls


def test_build_input_takes_only_service_names(tmp_path: Path) -> None:
    result, calls, _ = run_e2e(tmp_path, extra_env={"E2E_BUILD": "api; rm -rf x"})
    assert result.returncode == 2
    assert "E2E_BUILD" in result.stderr
    assert "compose --project-name" not in calls


def test_keep_prints_a_working_removal(tmp_path: Path) -> None:
    result, calls, root = run_e2e(tmp_path, extra_env={"E2E_KEEP": "1"})
    assert result.returncode == 1
    assert "down --volumes" not in calls
    kept = list(root.iterdir())
    assert len(kept) == 1
    assert kept[0].name.startswith("bothy-e2e.")
    # The overlay requires its variables for down as well, so the command sets them.
    removal = next(line for line in result.stderr.splitlines() if "env SMOKE_SERVER_IMAGE=" in line)
    for name in (
        "E2E_PORT=54321",
        "E2E_CERTS=",
        "E2E_AGENT_CONTEXT=",
        "SMOKE_OUT=",
        "E2E_UID_GID=",
    ):
        assert name in removal, name
    assert "test/compose.e2e.yaml" in removal
    # The test CA and the requirements were written before the stack started.
    assert (kept[0] / "certs" / "ca.pem").is_file()
    assert (kept[0] / "certs" / "leaf.key").is_file()
    assert not (kept[0] / "certs" / "ca.key").exists(), "the CA key goes once the leaf is signed"
    assert (kept[0] / "agent" / "requirements.txt").read_text(encoding="utf-8").startswith("attrs")


def test_leaf_names_localhost_and_loopback(tmp_path: Path) -> None:
    _, _, root = run_e2e(tmp_path, extra_env={"E2E_KEEP": "1"})
    certs = next(root.iterdir()) / "certs"
    text = subprocess.run(
        ["openssl", "x509", "-in", str(certs / "leaf.pem"), "-noout", "-text"],
        capture_output=True,
        text=True,
        check=True,
    ).stdout
    assert "DNS:localhost" in text
    assert "IP Address:127.0.0.1" in text
    verify = subprocess.run(
        ["openssl", "verify", "-CAfile", str(certs / "ca.pem"), str(certs / "leaf.pem")],
        capture_output=True,
        text=True,
        check=False,
    )
    assert verify.returncode == 0, verify.stderr


# ---- the overlay --------------------------------------------------------------------------


def test_overlay_publishes_only_the_tls_front_on_loopback() -> None:
    services = load(OVERLAY)["services"]
    assert services["api"]["ports"] == [
        "127.0.0.1:${E2E_PORT:?set by e2e.sh}:${E2E_PORT:?set by e2e.sh}"
    ]
    others = [n for n, s in services.items() if "ports" in s and n != "api"]
    assert others == []
    for name in ("e2e-tls", "e2e-world", "e2e-agent"):
        assert services[name]["network_mode"] == "service:api", name
        assert services[name]["profiles"] == ["e2e"], name
        assert services[name]["read_only"] is True, name
        assert services[name]["cap_drop"] == ["ALL"], name


def test_overlay_images_are_pinned_by_digest() -> None:
    services = load(OVERLAY)["services"]
    assert re.fullmatch(r"caddy:[0-9.]+-alpine@sha256:[0-9a-f]{64}", services["e2e-tls"]["image"])
    inline = services["e2e-agent"]["build"]["dockerfile_inline"]
    assert re.search(r"^FROM python:[0-9.]+-slim-trixie@sha256:[0-9a-f]{64}$", inline, re.M)
    assert "--require-hashes" in inline


def test_overlay_keys_exist_in_the_configuration_reference() -> None:
    reference = (REPO / "docs" / "operations" / "configuration.md").read_text(encoding="utf-8")
    services = load(OVERLAY)["services"]
    env = services["api"]["environment"]
    assert env == services["worker"]["environment"]
    for key in env:
        assert f"`{key}`" in reference, key


def test_e2e_env_adds_the_profile_and_no_secret() -> None:
    lines = [
        line
        for line in E2E_ENV.read_text(encoding="utf-8").splitlines()
        if line and not line.startswith("#")
    ]
    assert lines == ["COMPOSE_PROFILES=direct,smoke,e2e"]


def test_script_is_executable_and_bash_32_compatible() -> None:
    assert os.access(E2E_SH, os.X_OK)
    text = E2E_SH.read_text(encoding="utf-8")
    # bash 3.2 of macOS: no associative arrays, no ${var,,}, no mapfile.
    for construct in ("declare -A", ",,}", "mapfile", "readarray"):
        assert construct not in text, construct
