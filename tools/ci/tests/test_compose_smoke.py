# SPDX-License-Identifier: Apache-2.0
"""The Compose smoke test (brief M1-73, plan P-06): infra/compose/smoke.sh, the test overlay
infra/compose/test/compose.smoke.yaml, test/smoke.env and the collector overlay test/otel/file.yaml.

The preflight tests run smoke.sh with a fake `docker` on PATH, so they need no Docker; the
overlay tests read the YAML with a loader that registers the Compose tags `!reset` and
`!override`; the release test runs `make release-build`.
"""

from __future__ import annotations

import os
import re
import shutil
import subprocess
import tarfile
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import yaml

REPO = Path(__file__).resolve().parents[3]
COMPOSE_DIR = REPO / "infra" / "compose"
SMOKE_SH = COMPOSE_DIR / "smoke.sh"
OVERLAY = COMPOSE_DIR / "test" / "compose.smoke.yaml"
SMOKE_ENV = COMPOSE_DIR / "test" / "smoke.env"
FILE_OVERLAY = COMPOSE_DIR / "test" / "otel" / "file.yaml"
PGTEST_IMAGE = REPO / "services" / "server" / "internal" / "platform" / "pg" / "pgtest" / "image.go"
DOCKERFILE = REPO / "services" / "server" / "Dockerfile"
SERVER_SERVICES = ("secrets-init", "migrate", "api", "worker")


# ---- YAML with the Compose tags -----------------------------------------------------------


@dataclass(frozen=True)
class Tagged:
    """A value with a Compose merge tag (`!reset` or `!override`)."""

    tag: str
    value: Any


class ComposeLoader(yaml.SafeLoader):
    """SafeLoader that keeps `!reset` and `!override` as `Tagged` values."""


def _tagged(loader: yaml.SafeLoader, node: yaml.Node) -> Tagged:
    if isinstance(node, yaml.MappingNode):
        value: Any = loader.construct_mapping(node, deep=True)
    elif isinstance(node, yaml.SequenceNode):
        value = loader.construct_sequence(node, deep=True)
    else:
        value = loader.construct_scalar(node)  # type: ignore[arg-type]
    return Tagged(node.tag, value)


ComposeLoader.add_constructor("!reset", _tagged)
ComposeLoader.add_constructor("!override", _tagged)


def load(path: Path) -> dict[str, Any]:
    data = yaml.load(path.read_text(encoding="utf-8"), Loader=ComposeLoader)  # noqa: S506
    assert isinstance(data, dict)
    return data


def untag(value: Any) -> Any:
    return value.value if isinstance(value, Tagged) else value


def test_loader_registers_both_tags() -> None:
    data = yaml.load("a: !reset []\nb: !override {x: 1}\n", Loader=ComposeLoader)  # noqa: S506
    assert data == {"a": Tagged("!reset", []), "b": Tagged("!override", {"x": 1})}


# Interpolation messages (`${VAR:?message}`) may differ between the files; the variable and
# its default may not.
_MESSAGE = re.compile(r"(\$\{[A-Z0-9_]+):\?[^}]*\}")


def normalized(env: dict[str, Any]) -> dict[str, str]:
    return {k: _MESSAGE.sub(r"\1:?}", str(v)) for k, v in env.items()}


def dropped_keys() -> list[str]:
    """The keys the overlay header documents as rejected by the M1 server."""
    return re.findall(r"^#\s+(BOTHY_[A-Z0-9_]+)\s+\S", OVERLAY.read_text(encoding="utf-8"), re.M)


# ---- preflight with a fake docker ---------------------------------------------------------

FAKE_DOCKER = """#!/bin/sh
printf '%s\\n' "$*" >> "$FAKE_LOG"
case "$1" in
  compose) printf 'env SMOKE_SERVER_IMAGE=%s\\n' "${SMOKE_SERVER_IMAGE:-}" >> "$FAKE_LOG" ;;
esac
case "$1 $2" in
  "info "*) exit 0 ;;
  "version --format") printf '%s\\n' "$FAKE_ENGINE"; exit 0 ;;
  "compose version") printf '%s\\n' "$FAKE_COMPOSE"; exit 0 ;;
  "image inspect") exit "$FAKE_IMAGE_RC" ;;
esac
exit 0
"""


def _default_image() -> str:
    makefile = (REPO / "Makefile").read_text(encoding="utf-8")
    name = re.search(r"^IMAGE_NAME \?= (\S+)$", makefile, re.M)
    assert name is not None
    version = (REPO / "services" / "server" / "VERSION").read_text(encoding="utf-8").strip()
    return f"{name.group(1)}:{version}-local"


def run_smoke(
    tmp_path: Path,
    *,
    docker: bool = True,
    engine: str = "29.5.2",
    compose: str = "2.40.3",
    image_rc: int = 0,
) -> tuple[subprocess.CompletedProcess[str], str]:
    """Run smoke.sh with a fake docker; return the result and the docker calls."""
    bin_dir = tmp_path / "bin"
    bin_dir.mkdir()
    log = tmp_path / "docker.log"
    log.touch()
    if docker:
        fake = bin_dir / "docker"
        fake.write_text(FAKE_DOCKER, encoding="utf-8")
        fake.chmod(0o755)
        path = f"{bin_dir}:/usr/bin:/bin"
    else:
        # Only the tools the script needs before its docker check, so a docker of the host
        # (CI runners have /usr/bin/docker) cannot be found.
        for tool in ("bash", "dirname", "sed", "tr", "cat"):
            found = shutil.which(tool)
            assert found is not None, tool
            (bin_dir / tool).symlink_to(found)
        path = str(bin_dir)
    env = {
        "PATH": path,
        "HOME": str(tmp_path),
        "TMPDIR": str(tmp_path),
        "FAKE_LOG": str(log),
        "FAKE_ENGINE": engine,
        "FAKE_COMPOSE": compose,
        "FAKE_IMAGE_RC": str(image_rc),
        # Exported variables of these prefixes must not reach the stack.
        "BOTHY_PUBLIC_URL": "https://other.example.com",
        "COMPOSE_PROJECT_NAME": "other",
    }
    result = subprocess.run(
        [str(SMOKE_SH)], env=env, capture_output=True, text=True, check=False, timeout=120
    )
    return result, log.read_text(encoding="utf-8")


def test_missing_image_fails_with_hint(tmp_path: Path) -> None:
    result, calls = run_smoke(tmp_path, image_rc=1)
    assert result.returncode == 2, result.stderr
    image = _default_image()
    assert f"server image '{image}' not found locally" in result.stderr
    assert "make image" in result.stderr
    assert f"image inspect {image}" in calls
    assert "compose up" not in calls
    assert "pull" not in calls
    assert "build" not in calls


def test_missing_docker_fails(tmp_path: Path) -> None:
    result, _ = run_smoke(tmp_path, docker=False)
    assert result.returncode == 2
    assert "docker is not on PATH" in result.stderr


def test_old_compose_fails(tmp_path: Path) -> None:
    result, calls = run_smoke(tmp_path, compose="v2.25.9")
    assert result.returncode == 2
    assert "Docker Compose v2.25.9 is too old; 2.26.0 or newer" in result.stderr
    assert "image inspect" not in calls


def test_old_engine_fails(tmp_path: Path) -> None:
    result, _ = run_smoke(tmp_path, engine="25.0.5")
    assert result.returncode == 2
    assert "Docker Engine 25.0.5 is too old; 26.0.0 or newer" in result.stderr


def test_new_versions_with_suffix_pass_the_preflight(tmp_path: Path) -> None:
    """A suffix like -desktop.1 and a leading v are ignored; the run then starts."""
    result, calls = run_smoke(tmp_path, engine="26.0.0", compose="v2.26.0-desktop.1")
    assert "too old" not in result.stderr
    assert "compose" in calls.split("image inspect", 1)[1]
    # The default image reaches the overlay's ${SMOKE_SERVER_IMAGE:?} through the environment.
    assert f"env SMOKE_SERVER_IMAGE={_default_image()}" in calls
    # The fake answers every compose call with nothing; the run stops after the start step.
    assert result.returncode == 1


# ---- the overlay --------------------------------------------------------------------------


def test_overlay_server_services_use_smoke_image() -> None:
    services = load(OVERLAY)["services"]
    for name in SERVER_SERVICES:
        assert services[name]["image"] == "${SMOKE_SERVER_IMAGE:?set by smoke.sh}", name
        assert services[name]["pull_policy"] == "never", name


def test_overlay_postgres_pin_equals_pgtest() -> None:
    match = re.search(r'^const Image = "([^"]+)"$', PGTEST_IMAGE.read_text(encoding="utf-8"), re.M)
    assert match is not None
    pin = match.group(1)
    assert re.fullmatch(r"postgres:18\.[0-9]+-[a-z]+@sha256:[0-9a-f]{64}", pin)
    services = load(OVERLAY)["services"]
    assert services["postgres"]["image"] == pin
    assert services["otel-out-init"]["image"] == pin


def test_overlay_publishes_no_ports() -> None:
    services = load(OVERLAY)["services"]
    assert services["api"]["ports"] == Tagged("!reset", [])
    assert services["otel-collector"]["ports"] == Tagged("!reset", [])
    others = [n for n, s in services.items() if "ports" in s and n not in ("api", "otel-collector")]
    assert others == []
    assert services["smoke-probe"]["network_mode"] == "service:api"
    assert services["smoke-scan"]["network_mode"] == "none"
    env = untag(services["api"]["environment"])
    assert env["BOTHY_SERVER__LISTEN_ADDR"] == "127.0.0.1:8080"


def test_overlay_env_only_removes_keys() -> None:
    base = load(COMPOSE_DIR / "compose.yaml")
    overlay = load(OVERLAY)
    dropped = dropped_keys()
    assert dropped, "the overlay documents no dropped key"
    server_env = base["x-server-env"]
    for key in dropped:
        assert key in server_env, f"{key} is documented as dropped but not in x-server-env"
    expected_anchor = {k: v for k, v in server_env.items() if k not in dropped}
    assert normalized(overlay["x-smoke-env"]) == normalized(expected_anchor)

    for name in ("migrate", "api", "worker"):
        installed = base["services"][name]["environment"]
        want = {k: v for k, v in installed.items() if k not in dropped}
        got = untag(overlay["services"][name]["environment"])
        assert isinstance(overlay["services"][name]["environment"], Tagged), name
        if name == "api":
            assert got.pop("BOTHY_SERVER__LISTEN_ADDR") == "127.0.0.1:8080"
            want.pop("BOTHY_SERVER__LISTEN_ADDR")
        assert normalized(got) == normalized(want), name


def test_backup_is_never_started() -> None:
    backup = load(OVERLAY)["services"]["backup"]
    assert backup == {"profiles": ["smoke-disabled"]}
    profiles = _env_file()["COMPOSE_PROFILES"].split(",")
    assert "smoke-disabled" not in profiles
    assert profiles == ["direct", "smoke"]


def _env_file() -> dict[str, str]:
    values: dict[str, str] = {}
    for line in SMOKE_ENV.read_text(encoding="utf-8").splitlines():
        if line.strip() and not line.startswith("#"):
            key, _, value = line.partition("=")
            values[key] = value
    return values


def test_smoke_env_has_no_secrets_and_staging() -> None:
    values = _env_file()
    assert values["BOTHY_ENVIRONMENT"] == "staging"
    assert values["BOTHY_PUBLIC_URL"] == "https://chat.example.com"
    assert set(values) == {
        "COMPOSE_PROFILES",
        "BOTHY_REGISTRY",
        "BOTHY_VERSION",
        "BOTHY_PUBLIC_URL",
        "BOTHY_ENVIRONMENT",
        "BOTHY_OTEL_EXPORTER",
        "BOTHY_OTEL_EXPORTER_2",
        "BOTHY_TRACE_SAMPLE_PERCENT",
        "BOTHY_OTEL_METRIC_EXPORT_INTERVAL_MS",
    }
    secretish = re.compile(r"SECRET|PASSWORD|TOKEN|KEY|AUTH", re.I)
    assert not [k for k in values if secretish.search(k)]
    # The overlay passes the canary through without a value of its own.
    probe_env = load(OVERLAY)["services"]["smoke-probe"]["environment"]
    assert "SMOKE_CANARY" in probe_env


def _seconds(text: str) -> float:
    match = re.fullmatch(r"([0-9.]+)(ms|s)", str(text))
    assert match is not None, text
    return float(match.group(1)) / (1000 if match.group(2) == "ms" else 1)


def test_settle_covers_collector_waits() -> None:
    spec = load(FILE_OVERLAY)
    decision = _seconds(spec["processors"]["tail_sampling"]["decision_wait"])
    batch = _seconds(spec["processors"]["batch"]["timeout"])
    flush = max(_seconds(e["flush_interval"]) for e in spec["exporters"].values())
    script = SMOKE_SH.read_text(encoding="utf-8")
    match = re.search(r'^settle="\$\{SMOKE_TELEMETRY_SETTLE_S:-([0-9]+)\}"$', script, re.M)
    assert match is not None
    assert float(match.group(1)) >= decision + batch + flush


def test_dockerfile_production_stage_last_and_smoke_target_exists() -> None:
    text = DOCKERFILE.read_text(encoding="utf-8")
    froms = re.findall(r"^FROM (?:--platform=\S+ )?(\S+)(?: AS (\S+))?$", text, re.M)
    names = [alias for _, alias in froms]
    assert ("build", "smoke-build") in froms
    assert "smoke" in names
    production_ref, production_alias = froms[-1]
    assert production_alias == ""
    smoke_ref = next(ref for ref, alias in froms if alias == "smoke")
    assert smoke_ref == production_ref
    assert names.index("smoke-build") < names.index("smoke") < len(froms) - 1
    production = text.rsplit("\nFROM ", 1)[1]
    assert 'CMD ["serve"]' in production
    assert "bothy-smoke" not in production
    smoke = text.split(" AS smoke\n", 1)[1].split("\nFROM ", 1)[0]
    assert "USER 65532:65532" in smoke
    assert 'ENTRYPOINT ["/usr/local/bin/bothy-smoke"]' in smoke


# ---- release bundle -----------------------------------------------------------------------

_DROPPED = {"MAKEFLAGS", "MAKELEVEL", "MFLAGS", "VERSION", "OUT"}


def test_release_bundle_excludes_smoke_overlay(tmp_path: Path) -> None:
    tracked = subprocess.run(
        ["git", "-C", str(REPO), "ls-files", "--error-unmatch", str(OVERLAY.relative_to(REPO))],
        capture_output=True,
        text=True,
        check=False,
    )
    assert tracked.returncode == 0, "the overlay is not tracked; the test would pass vacuously"
    version = (REPO / "VERSION").read_text(encoding="utf-8").strip()
    env = {k: v for k, v in os.environ.items() if k not in _DROPPED}
    result = subprocess.run(
        ["make", "-s", "-C", str(REPO), "release-build", f"VERSION={version}", f"OUT={tmp_path}"],
        env=env,
        capture_output=True,
        text=True,
        check=False,
        timeout=300,
    )
    assert result.returncode == 0, result.stdout + result.stderr
    with tarfile.open(tmp_path / f"bothy-compose-{version}.tar.gz") as archive:
        members = archive.getnames()
    prefix = f"bothy-{version}/"
    assert prefix + "infra/compose/compose.yaml" in members
    assert [m for m in members if m.startswith(prefix + "infra/compose/test")] == []
