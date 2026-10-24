# SPDX-License-Identifier: Apache-2.0
"""Fake-Hermes kit: the pinned Hermes Agent in a subprocess with the adapter (Design note 20).

Pin, provisioning and isolation are M2-32's (``hermes_harness``): the session fixture `world`
provisions the pinned version once into a temporary root outside the repository, adds exactly
what the adapter needs to that throwaway environment (``attrs`` and ``coverage`` at the locked
versions, and a ``.pth`` file with the source trees of the SDK and the adapter) and takes the
integrity snapshots after that. `HermesRig` gives one test a throwaway Hermes home configured
by the adapter's own installer, the fake model of M2-32 with fake credentials, the egress
recorder, the SDK's FakeServer on ``127.0.0.1`` with a wall clock, and the gateway process group
under the macOS sandbox; it stops the group by its ID and moves the coverage data of the
subprocess next to the main data file, so ``platform.py`` and ``hermes_config.py`` count.

Without the pinned Hermes the tests fail, they never skip (P-09 applied).
"""

from __future__ import annotations

import asyncio
import contextlib
import dataclasses
import json
import shutil
import subprocess
import tomllib
from collections.abc import AsyncIterator, Callable, Iterator, Mapping, Sequence
from dataclasses import dataclass, field
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any

import pytest
import yaml
from bothy_sdk.testing._patience import eventually
from bothy_sdk.testing.doubles import FakeClock
from bothy_sdk.testing.fakeserver import FakeServer
from hermes_harness import fake_model as _fake_model
from hermes_harness.egress import EgressRecorder
from hermes_harness.environment import ThrowawayHomes, build_environment, interpreter_dirs
from hermes_harness.fake_model import (
    CONTEXT_LENGTH,
    MODEL_NAME,
    FakeCredentials,
    FakeModel,
    ScriptTurn,
)
from hermes_harness.fixtures import make_root
from hermes_harness.gateway import HermesGateway
from hermes_harness.integrity import (
    CheckoutState,
    installed_digest,
    restore_writable,
    snapshot_checkout,
)
from hermes_harness.pin import PIN_FILE, HermesPin, load_pin
from hermes_harness.provision import HermesInstall, provision, release
from hermes_harness.sandbox import sandbox_profile, wrap_command

COMPONENT = Path(__file__).resolve().parents[1]
REPO = COMPONENT.parents[1]
SDK_SRC = REPO / "sdk" / "python" / "src"
ADAPTER_SRC = COMPONENT / "src"
PLATFORM_KEY = "bothy"
#: Distributions the adapter adds to the Hermes environment (11 11.3): nothing else may change.
ADDED = ("attrs", "coverage")
#: Gateway starts before a run gives up: Hermes' fixed wait for its plugin discovery can end
#: before the adapter's platform is registered on a loaded machine (D-M2-33-14).
_START_ATTEMPTS = 3
_NO_PLATFORM = "No messaging platforms enabled."
_CREDENTIAL_NAMES = {"anthropic_api_key": "ANTHROPIC_API_KEY", "anthropic_oauth": "ANTHROPIC_TOKEN"}


class WallClock(FakeClock):
    """The FakeServer's clock in wall time: tokens fit the subprocess's real clock.

    Sleeps still wait for `advance`, so no redelivery or grace timer fires by itself.
    """

    def __init__(self) -> None:
        super().__init__(datetime.now(UTC))

    def now(self) -> datetime:
        return datetime.now(UTC) + timedelta(seconds=self.monotonic())


def _locked_versions(names: Sequence[str]) -> dict[str, str]:
    lock = tomllib.loads((COMPONENT / "uv.lock").read_text(encoding="utf-8"))
    found = {p["name"]: p["version"] for p in lock["package"] if p["name"] in names}
    missing = sorted(set(names) - set(found))
    if missing:
        raise AssertionError(f"not in uv.lock: {missing}")
    return found


def _freeze(python: Path) -> dict[str, str]:
    script = (
        "import importlib.metadata as m, json\n"
        "print(json.dumps({d.metadata['Name'].lower(): d.version for d in m.distributions()}))"
    )
    out = subprocess.run(
        [str(python), "-c", script], capture_output=True, text=True, check=True, env=_bare_env()
    )
    data: dict[str, str] = json.loads(out.stdout)
    return data


def newest_release(pin: HermesPin) -> HermesPin:
    """The newest release tag of the pinned repository (``--hermes-version=latest``, 09 17.1).

    An unpinned download: a manual switch, never part of ``make check``.
    """
    git = shutil.which("git")
    if git is None:
        raise AssertionError("git is required")
    out = subprocess.run(
        [git, "ls-remote", "--tags", pin.repository],
        capture_output=True,
        text=True,
        check=True,
        env={**_bare_env(), "GIT_TERMINAL_PROMPT": "0"},
    ).stdout
    commits: dict[str, str] = {}
    for line in out.splitlines():
        commit, _, ref = line.partition("\t")
        tag = ref.removeprefix("refs/tags/")
        if tag.endswith("^{}"):  # the commit an annotated tag points to wins
            commits[tag[:-3]] = commit
        else:
            commits.setdefault(tag, commit)
    tags = []
    for tag, commit in commits.items():
        numbers = tag.lstrip("v").split(".")
        if all(n.isdigit() for n in numbers):
            tags.append((tuple(int(n) for n in numbers), tag, commit))
    if not tags:
        raise AssertionError("no release tag found")
    _, tag, commit = max(tags)
    return dataclasses.replace(pin, tag=tag, commit=commit)


def _bare_env() -> dict[str, str]:
    return {"PATH": "/usr/bin:/bin", "LANG": "C.UTF-8", "PYTHONDONTWRITEBYTECODE": "1"}


@dataclass
class HermesWorld:
    """The provisioned pinned version with the adapter's additions."""

    install: HermesInstall
    root: Path
    python: Path
    before: dict[str, str]
    after: dict[str, str]
    checkout: CheckoutState
    digest: str
    coverage_rc: Path
    coverage_dir: Path
    count: int = 0

    def next_dir(self) -> Path:
        self.count += 1
        return self.root / f"{self.count:02d}"


#: Coverage of the Hermes processes: started from a ``.pth`` file when ``COVERAGE_PROCESS_START``
#: is set, and saved before ``os._exit``, which the gateway ends with (atexit never runs there).
_COVERAGE_STARTUP = """\
# SPDX-License-Identifier: Apache-2.0
\"\"\"Coverage of a Hermes process of the Fake-Hermes tests (written by the test harness).\"\"\"
import os

import coverage

# The coverage distribution may start measuring from its own .pth file first; then
# process_startup returns None and keeps the running instance on itself.
_cov = coverage.process_startup() or getattr(coverage.process_startup, "coverage", None)
if _cov is not None:
    _exit = os._exit

    def _saving_exit(code, _exit=_exit, _cov=_cov):
        try:
            _cov.stop()
            _cov.save()
        finally:
            _exit(code)

    os._exit = _saving_exit
"""


def _site_packages(venv: Path) -> Path:
    found = sorted(venv.glob("lib/python*/site-packages"))
    if not found:
        raise AssertionError("no site-packages in the throwaway venv")
    return found[0]


def _add_adapter(install: HermesInstall, root: Path) -> tuple[Path, Path]:
    uv = shutil.which("uv")
    if uv is None:
        raise AssertionError("uv is required")
    python = install.venv / "bin" / "python"
    versions = _locked_versions(ADDED)
    pins = [f"{name}=={version}" for name, version in versions.items()]
    cache = subprocess.run([uv, "cache", "dir"], capture_output=True, text=True, check=True)
    env = {**_bare_env(), "HOME": str(root / "uvhome"), "UV_CACHE_DIR": cache.stdout.strip()}
    subprocess.run(
        [uv, "pip", "install", "--python", str(python), "--no-deps", "--quiet", *pins],
        check=True,
        env=env,
        capture_output=True,
    )
    site = _site_packages(install.venv)
    # Byte-compile the export and the environment once: every Hermes process runs with
    # PYTHONDONTWRITEBYTECODE, and compiling hundreds of modules at each start can outlast
    # Hermes' own fixed wait for its plugin discovery on a loaded machine, after which the
    # gateway starts without the adapter's platform.
    subprocess.run(
        [str(python), "-m", "compileall", "-q", "-j", "0", str(install.export), str(site)],
        check=False,
        env=_bare_env(),
        capture_output=True,
    )
    (site / "bothy_sources.pth").write_text(f"{SDK_SRC}\n{ADAPTER_SRC}\n", encoding="utf-8")
    (site / "bothy_coverage_startup.py").write_text(_COVERAGE_STARTUP, encoding="utf-8")
    (site / "bothy_coverage.pth").write_text("import bothy_coverage_startup\n", encoding="utf-8")
    coverage_dir = root / "cov"
    coverage_dir.mkdir()
    rc = root / "coveragerc"
    rc.write_text(
        "[run]\nbranch = True\nparallel = True\n"
        f"data_file = {coverage_dir}/.coverage\n"
        "source_pkgs = bothy_hermes_adapter\n",
        encoding="utf-8",
    )
    return rc, coverage_dir


def provision_world(request: pytest.FixtureRequest) -> Iterator[HermesWorld]:
    """Provision the pinned version once and add the adapter (fails without network)."""
    version = request.config.getoption("--hermes-version")
    if version not in ("pin", "latest"):
        raise AssertionError("--hermes-version is pin or latest")
    pin = load_pin(PIN_FILE)
    if version == "latest":
        pin = newest_release(pin)
    root = make_root()
    install: HermesInstall | None = None
    try:
        install = provision(pin, root / "i")
        python = install.venv / "bin" / "python"
        before = _freeze(python)
        rc, coverage_dir = _add_adapter(install, root)
        after = _freeze(python)
        yield HermesWorld(
            install=install,
            root=root,
            python=python,
            before=before,
            after=after,
            checkout=snapshot_checkout(install.checkout),
            digest=installed_digest(install.venv),
            coverage_rc=rc,
            coverage_dir=coverage_dir,
        )
    finally:
        if install is not None:
            release(install)
        elif (root / "i" / "checkout").exists():
            restore_writable(root / "i" / "checkout")
        shutil.rmtree(root, ignore_errors=True)


def collect_coverage(world: HermesWorld) -> None:
    """Move the subprocess's coverage data next to the main data file (pytest-cov combines)."""
    for data in world.coverage_dir.glob(".coverage.*"):
        shutil.move(str(data), str(COMPONENT / f".coverage.hermes.{data.name.split('.', 2)[2]}"))


class _RawHandler(_fake_model._Handler):
    """The fake model's handler that also keeps the raw JSON body of every model request.

    The bodies stay in this process (fake credentials never reach a body: they travel in
    headers); the AG-20 test reads the system part and the messages Hermes sent.
    """

    def read_body(self) -> bytes:
        raw = super().read_body()
        owner: RawFakeModel = self.server.owner  # type: ignore[attr-defined]
        if raw:
            with contextlib.suppress(ValueError):
                owner.raw.append(json.loads(raw))
        return raw


class RawFakeModel(FakeModel):
    """`FakeModel` of M2-32 plus the raw request bodies (``raw``)."""

    def __init__(self, *args: Any, **kwargs: Any) -> None:
        super().__init__(*args, **kwargs)
        self.raw: list[dict[str, Any]] = []
        self._server.RequestHandlerClass = _RawHandler


@dataclass
class Profile:
    """How one throwaway Hermes home is configured."""

    busy_input_mode: str = "steer"
    provider: str = "anthropic"
    dotenv: Mapping[str, str] = field(default_factory=lambda: {"anthropic_api_key": "api_key"})
    toolsets: Sequence[str] = ("todo", "clarify")
    config: Mapping[str, Any] = field(default_factory=dict)
    login_file: bool = False


@dataclass
class HermesRig:
    """One gateway with the adapter, its fake model and the FakeServer."""

    world: HermesWorld
    fake: FakeServer
    model: RawFakeModel
    creds: FakeCredentials
    homes: ThrowawayHomes
    home: Path
    egress: EgressRecorder
    gateway: HermesGateway | None = None
    env: dict[str, str] = field(default_factory=dict)
    installed_output: str = ""

    async def until(self, predicate: Callable[[], Any], what: str = "the condition") -> None:
        async def step() -> None:
            if self.gateway is not None and not self.gateway.running():
                raise AssertionError("the gateway exited:\n" + gateway_log(self))
            await asyncio.sleep(0.05)

        await eventually(predicate, what, step=step)

    def requests(self, operation_id: str) -> list[Any]:
        return [r for r in self.fake.requests if r.operation_id == operation_id]

    def finished(self, run_id: Any) -> bool:
        return self.fake.run_state(run_id).state in ("completed", "failed", "cancelled")

    def model_requests(self, scripted: bool | None = None) -> list[dict[str, Any]]:
        out = self.model.requests()
        if scripted is None:
            return out
        return [r for r in out if bool(r.get("scripted")) == scripted]

    def write_dotenv(self, values: Mapping[str, str]) -> None:
        """Set the credential lines of the profile's ``.env`` (fake credentials only).

        Args:
            values: ``anthropic_api_key`` and ``anthropic_oauth`` to ``api_key`` or ``oauth``.
        """
        path = self.home / ".env"
        names = set(_CREDENTIAL_NAMES.values())
        kept = []
        if path.exists():
            kept = [
                line
                for line in path.read_text(encoding="utf-8").splitlines()
                if line.split("=", 1)[0] not in names
            ]
        for name, kind in values.items():
            secret = self.creds.api_key if kind == "api_key" else self.creds.oauth_token
            kept.append(f"{_CREDENTIAL_NAMES[name]}={secret}")
        path.write_text("\n".join(kept) + "\n", encoding="utf-8")

    def installer(self, *args: str) -> subprocess.CompletedProcess[str]:
        """Run ``python -m bothy_hermes_adapter`` with the profile's Python and environment."""
        return subprocess.run(
            [str(self.world.python), "-m", "bothy_hermes_adapter", *args],
            env=self.env,
            cwd=self.homes.work,
            capture_output=True,
            text=True,
            check=False,
        )

    def state_files(self) -> list[Path]:
        state = self.home / PLATFORM_KEY
        return sorted(p for p in state.rglob("*") if p.is_file()) if state.exists() else []


def _base_config(model: FakeModel, profile: Profile) -> dict[str, Any]:
    if profile.provider == "anthropic":
        model_cfg: dict[str, Any] = {"provider": "anthropic", "base_url": model.anthropic_base_url}
    else:
        model_cfg = {"provider": "custom", "base_url": model.openai_base_url}
    model_cfg |= {"default": MODEL_NAME, "context_length": CONTEXT_LENGTH}
    config: dict[str, Any] = {
        "model": model_cfg,
        "platform_toolsets": {PLATFORM_KEY: list(profile.toolsets)},
        "tools": {"tool_search": {"enabled": "off"}},
        "streaming": {"enabled": True},
        "security": {"allow_lazy_installs": False, "tirith_enabled": False},
        "updates": {"check": False},
        "model_catalog": {"enabled": False},
        "models_dev": {"url": model.models_dev_url},
        "auth": {"adopt_external_logins": False},
    }
    for key, value in profile.config.items():
        config[key] = value
    return config


@contextlib.asynccontextmanager
async def hermes_rig(
    world: HermesWorld,
    script: Sequence[ScriptTurn],
    *,
    profile: Profile | None = None,
    texts: Sequence[str] = (),
    billing_mode: str = "organization",
    tool_class: str = "chat",
    start: bool = True,
) -> AsyncIterator[HermesRig]:
    """Start the fake model, the FakeServer and (with ``start``) the gateway with the adapter."""
    profile = profile or Profile()
    creds = FakeCredentials.generate()
    model = RawFakeModel(script, creds, scenario_texts=list(texts))
    egress = EgressRecorder()
    fake = FakeServer(
        clock=WallClock(),
        dpop=False,
        billing_mode=billing_mode,
        tool_class=tool_class,
        timings={"agent.invocation_token_ttl": 3600},
    )
    homes = ThrowawayHomes.create(world.next_dir())
    home = homes.hermes("h")
    model.start()
    egress.start()
    await fake.start()
    rig = HermesRig(world, fake, model, creds, homes, home, egress)
    try:
        (home / "config.yaml").write_text(
            yaml.safe_dump(_base_config(model, profile), sort_keys=True), encoding="utf-8"
        )
        token = home / "token"
        token.write_text(fake.credentials.api_token.reveal(), encoding="utf-8")
        token.chmod(0o600)
        rig.write_dotenv(profile.dotenv)
        if profile.login_file:
            login = homes.home / ".claude" / ".credentials.json"
            login.parent.mkdir(parents=True)
            login.write_text(
                json.dumps(
                    {
                        "claudeAiOauth": {
                            "accessToken": creds.oauth_token,
                            "refreshToken": "fake-refresh",
                            "expiresAt": 4102444800000,
                        }
                    }
                ),
                encoding="utf-8",
            )
        # An empty managed scope: a system-wide /etc/hermes of the machine must not reach the
        # throwaway profile (Hermes reads it unless HERMES_MANAGED_DIR names another directory).
        managed = homes.root / "managed"
        managed.mkdir()
        rig.env = build_environment(
            homes,
            "h",
            egress.url,
            {
                "COVERAGE_PROCESS_START": str(world.coverage_rc),
                "HERMES_MANAGED_DIR": str(managed),
            },
            venv_bin=world.install.venv / "bin",
        )
        installed = rig.installer(
            "install",
            "--profile-home",
            str(home),
            "--server-url",
            fake.base_url,
            "--instance-id",
            str(fake.instance_id),
            "--token-file",
            str(token),
            "--busy-input-mode",
            profile.busy_input_mode,
        )
        if installed.returncode != 0:
            raise AssertionError(f"install failed: {installed.stderr[-2000:]}")
        rig.installed_output = installed.stdout
        if start:
            await _start_gateway(rig)
        yield rig
    finally:
        if rig.gateway is not None:
            rig.gateway.stop()
        collect_coverage(world)
        await fake.stop()
        for server in (model, egress):
            server.close()


async def _start_gateway(rig: HermesRig) -> None:
    world = rig.world
    profile_file = rig.homes.root / "sandbox.sb"
    python_home = (world.install.venv / "bin" / "python").resolve().parents[1]
    readable = [*interpreter_dirs(), world.install.root, python_home, SDK_SRC, ADAPTER_SRC]
    ports = [rig.model.port, rig.egress.port, int(rig.fake.origin.rsplit(":", 1)[1])]
    profile_file.write_text(
        sandbox_profile(rig.homes.root.parent, readable=readable, ports=ports), encoding="utf-8"
    )
    command = wrap_command(
        [str(world.install.executable), "gateway", "run", "--force"], profile_file
    )
    log = rig.homes.root / "gateway.log"
    for attempt in range(1, _START_ATTEMPTS + 1):
        rig.gateway = HermesGateway(command, env=rig.env, cwd=rig.homes.work, log=log)
        rig.gateway.start()

        def started() -> bool:
            identified = any(f.get("op") == "identify" for f in rig.fake.frames)
            text = log.read_text(encoding="utf-8", errors="replace") if log.exists() else ""
            return identified or _NO_PLATFORM in text

        try:
            await rig.until(started, "the adapter's identify")
        except AssertionError as exc:
            raise AssertionError(f"{exc}\n{gateway_log(rig)}") from None
        if any(f.get("op") == "identify" for f in rig.fake.frames):
            break
        # Hermes waits a fixed time for its own plugin discovery and starts without the plugin's
        # platform when a loaded machine needs longer; that start never connects, so start again.
        rig.gateway.stop()
        log.write_text("", encoding="utf-8")
        if attempt == _START_ATTEMPTS:
            raise AssertionError("Hermes started without the adapter's platform every time")
    await rig.until(
        lambda: rig.fake.heartbeats or any(f.get("op") == "heartbeat" for f in rig.fake.frames),
        "the first heartbeat",
    )


def gateway_log(rig: HermesRig) -> str:
    """The gateway's output (for a failure message)."""
    if rig.gateway is None or not rig.gateway.log.exists():
        return ""
    return rig.gateway.log.read_text(encoding="utf-8", errors="replace")[-12000:]


def credential_kinds(rig: HermesRig) -> list[str]:
    """The credential kind of each scripted request the fake model received."""
    return [str(r["credential"]) for r in rig.model_requests(scripted=True)]
