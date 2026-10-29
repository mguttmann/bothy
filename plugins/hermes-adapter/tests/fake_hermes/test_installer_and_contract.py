# SPDX-License-Identifier: Apache-2.0
"""The installer and the contract with the pinned Hermes (09 17.2, 17.5, 17.7; RK-12; 11 11.3).

The stub contract compares every signature of ``typings/`` with the pinned version, the toolset
table and the module list are held equal to it, the adapter's additions change no package of the
Hermes environment, and the installer's commands are run with the profile's own Python.
"""

from __future__ import annotations

import ast
import json
import re
import subprocess
import tomllib
from typing import Any

import pytest
import yaml
from fake_hermes_kit import (
    ADDED,
    COMPONENT,
    HermesWorld,
    Profile,
    _bare_env,
    _start_gateway,
    gateway_log,
    hermes_rig,
)
from hermes_harness.fake_model import ScriptTurn
from hermes_modules import HERMES_TOP_LEVEL

from bothy_hermes_adapter.installer import CONTROL_SENDER
from bothy_hermes_adapter.tool_class import TOOLSET_CLASSES

pytestmark = pytest.mark.anyio

TYPINGS = COMPONENT / "typings"
#: The SDK's runtime requirements as ranges (sdk/python/pyproject.toml).
SDK_MINIMUMS = {
    "cryptography": (50, 0, 0),
    "httpx": (0, 28, 1),
    "pyjwt": (2, 13, 0),
    "websockets": (15, 0, 1),
    "attrs": (26, 1, 0),
}


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


def _run(world: HermesWorld, script: str) -> Any:
    out = subprocess.run(
        [str(world.python), "-c", script],
        capture_output=True,
        text=True,
        check=False,
        env={**_bare_env(), "HOME": str(world.root / "probe-home")},
    )
    assert out.returncode == 0, out.stderr[-3000:]
    return json.loads(out.stdout.strip().splitlines()[-1])


def _stub_signatures() -> dict[str, dict[str, Any]]:
    """``module:qualname`` to parameter names and kinds and whether it is a coroutine."""
    found: dict[str, dict[str, Any]] = {}
    for path in sorted(TYPINGS.rglob("*.pyi")):
        rel = path.relative_to(TYPINGS).with_suffix("")
        module = ".".join(p for p in rel.parts if p != "__init__")
        tree = ast.parse(path.read_text(encoding="utf-8"))

        def visit(body: list[ast.stmt], prefix: str, mod: str = module) -> None:
            for node in body:
                if isinstance(node, ast.ClassDef):
                    visit(node.body, f"{prefix}{node.name}.")
                elif isinstance(node, ast.FunctionDef | ast.AsyncFunctionDef):
                    if any(
                        isinstance(d, ast.Name) and d.id == "property" for d in node.decorator_list
                    ):
                        found[f"{mod}:{prefix}{node.name}"] = {"property": True}
                        continue
                    args = node.args
                    params = [a.arg for a in args.posonlyargs + args.args]
                    kw = [a.arg for a in args.kwonlyargs]
                    found[f"{mod}:{prefix}{node.name}"] = {
                        "params": params,
                        "kwonly": kw,
                        "varkw": args.kwarg is not None,
                        "async": isinstance(node, ast.AsyncFunctionDef),
                    }

        visit(tree.body, "")
    return found


def test_hermes_stubs_match_pinned_version(world: HermesWorld) -> None:
    stubs = _stub_signatures()
    script = (
        "import importlib, inspect, json\n"
        f"wanted = json.loads({json.dumps(json.dumps(sorted(stubs)))})\n"
        "out = {}\n"
        "for key in wanted:\n"
        "    mod, qual = key.split(':')\n"
        "    obj = importlib.import_module(mod)\n"
        "    for part in qual.split('.'):\n"
        "        static = inspect.isclass(obj)\n"
        "        obj = inspect.getattr_static(obj, part) if static else getattr(obj, part)\n"
        "    if isinstance(obj, property):\n"
        "        out[key] = {'property': True}\n"
        "        continue\n"
        "    fn = getattr(obj, '__func__', obj)\n"
        "    sig = inspect.signature(fn)\n"
        "    ps = list(sig.parameters.values())\n"
        "    out[key] = {\n"
        "        'params': [p.name for p in ps\n"
        "                   if p.kind in (p.POSITIONAL_ONLY, p.POSITIONAL_OR_KEYWORD)],\n"
        "        'kwonly': [p.name for p in ps if p.kind is p.KEYWORD_ONLY],\n"
        "        'varkw': any(p.kind is p.VAR_KEYWORD for p in ps),\n"
        "        'async': inspect.iscoroutinefunction(fn),\n"
        "    }\n"
        "print(json.dumps(out))\n"
    )
    real = _run(world, script)
    differences = {k: (stubs[k], real.get(k)) for k in stubs if stubs[k] != real.get(k)}
    assert differences == {}


def test_tool_class_table_covers_pinned_toolsets(world: HermesWorld) -> None:
    names = _run(world, "import json, toolsets; print(json.dumps(sorted(toolsets.TOOLSETS)))")
    assert sorted(TOOLSET_CLASSES) == names


def test_hermes_top_level_matches_pinned_version(world: HermesWorld) -> None:
    names = _run(
        world,
        "import json, importlib.metadata as m\n"
        "d = m.distribution('hermes-agent')\n"
        "print(json.dumps(sorted((d.read_text('top_level.txt') or '').split())))",
    )
    assert sorted(HERMES_TOP_LEVEL) == names


def test_sdk_dependencies_fit_pinned_hermes(world: HermesWorld) -> None:
    changed = {
        name: (world.before[name], world.after.get(name))
        for name in world.before
        if world.before[name] != world.after.get(name)
    }
    assert changed == {}
    added = sorted(set(world.after) - set(world.before))
    assert added == sorted(ADDED)
    for name, minimum in SDK_MINIMUMS.items():
        version = tuple(int(p) for p in world.after[name].split(".")[:3])
        assert version >= minimum, name


def _lowest_python(requires: str) -> tuple[int, int]:
    found = re.search(r">=\s*3\.([0-9]+)", requires)
    assert found is not None, requires
    return (3, int(found.group(1)))


def test_adapter_admits_every_python_of_pinned_hermes(world: HermesWorld) -> None:
    # Hermes admits older Pythons than its lock installs (v0.22: >=3.11 for installs that still
    # update on an old Python, the lock only 3.14); the adapter must load in every one of them.
    hermes = tomllib.loads((world.install.export / "pyproject.toml").read_text(encoding="utf-8"))
    adapter = tomllib.loads((COMPONENT / "pyproject.toml").read_text(encoding="utf-8"))
    oldest = _lowest_python(hermes["project"]["requires-python"])
    assert _lowest_python(adapter["project"]["requires-python"]) <= oldest


async def test_installer_busy_mode_and_switch(world: HermesWorld) -> None:
    script = [ScriptTurn(text=("Fine.",), paced=False)]
    profile = Profile(dotenv={"anthropic_api_key": "api_key", "anthropic_oauth": "oauth"})
    async with hermes_rig(world, script, profile=profile, start=False) as rig:
        config = yaml.safe_load((rig.home / "config.yaml").read_text(encoding="utf-8"))
        assert config["display"]["busy_input_mode"] == "steer"
        section = config["gateway"]["platforms"]["bothy"]
        assert section["extra"]["dm_policy"] == "allowlist"
        assert section["extra"]["group_allow_admin_from"] == [CONTROL_SENDER]
        assert config["display"]["platforms"]["bothy"]["tool_progress"] == "off"
        assert "bothy" in config["plugins"]["enabled"]
        token = str(rig.home / "token")
        base = [
            "--profile-home",
            str(rig.home),
            "--server-url",
            rig.fake.base_url,
            "--instance-id",
            str(rig.fake.instance_id),
            "--token-file",
            token,
        ]
        refused = rig.installer("install", *base, "--busy-input-mode", "interrupt")
        assert refused.returncode == 2
        assert "interrupt is refused" in refused.stderr
        other_key = rig.installer(
            "install", *base, "--busy-input-mode", "queue", "--platform-key", "other"
        )
        assert other_key.returncode == 2
        queue = rig.installer("install", *base, "--busy-input-mode", "queue")
        assert queue.returncode == 0, queue.stderr
        config = yaml.safe_load((rig.home / "config.yaml").read_text(encoding="utf-8"))
        assert config["display"]["busy_input_mode"] == "queue"
        again = rig.installer("install", *base, "--busy-input-mode", "steer")
        assert again.returncode == 0
        # personal: the API key goes, Hermes's allow list holds exactly the credential holder.
        holder = str(rig.fake.robin_id)
        personal = rig.installer(
            "switch-mode",
            "personal",
            "--profile-home",
            str(rig.home),
            "--credential-holder",
            holder,
        )
        assert personal.returncode == 0, personal.stderr
        assert "predicted mode: personal" in personal.stdout
        env = (rig.home / ".env").read_text(encoding="utf-8")
        assert "ANTHROPIC_API_KEY=" not in env
        allowed = [line for line in env.splitlines() if line.startswith("BOTHY_ALLOWED_USERS=")]
        assert len(allowed) == 1
        assert allowed[0].split("=", 1)[1].strip("'\"") == holder
        # The prediction equals the credential Hermes then uses for a turn.
        rig.fake.world.billing_mode = "personal"
        await _start_gateway(rig)
        pid = rig.gateway.pid if rig.gateway else None
        triggered = await rig.fake.trigger("hello", author="robin")
        try:
            await rig.until(lambda: rig.finished(triggered.run_id), "the end of the run")
        except AssertionError:
            raise AssertionError(gateway_log(rig)) from None
        used = [
            str(r["credential"])
            for r in rig.model_requests()
            if r["route"] == "anthropic.messages" and r.get("tools")
        ]
        assert used
        assert set(used) == {"oauth"}
        # Layer 3 (09 17.7): Hermes's allow list still refuses every other person, even with a
        # valid token (the adapter declares no upstream authorization next to the list); the
        # model never sees the turn.
        before = len(rig.model_requests(scripted=True))
        stranger = await rig.fake.trigger("hello", author="sam")
        try:
            await rig.until(lambda: rig.finished(stranger.run_id), "the end of the run")
        except AssertionError:
            raise AssertionError(gateway_log(rig)) from None
        assert f"Unauthorized user: {rig.fake.sam_id}" in gateway_log(rig)
        assert len(rig.model_requests(scripted=True)) == before
        # organization needs the API key; the switch empties the OAuth variables and the list.
        no_key = rig.installer("switch-mode", "organization", "--profile-home", str(rig.home))
        assert no_key.returncode == 2
        assert "ANTHROPIC_API_KEY" in no_key.stderr
        rig.write_dotenv({"anthropic_api_key": "api_key", "anthropic_oauth": "oauth"})
        organization = rig.installer("switch-mode", "organization", "--profile-home", str(rig.home))
        assert organization.returncode == 0, organization.stderr
        assert "predicted mode: organization" in organization.stdout
        env = (rig.home / ".env").read_text(encoding="utf-8")
        assert "ANTHROPIC_TOKEN=" not in env
        assert "BOTHY_ALLOWED_USERS=" not in env
        assert "restart" in organization.stdout
        check = rig.installer("check", "--profile-home", str(rig.home))
        assert check.returncode == 0
        assert "predicted mode: organization" in check.stdout
        # No process was restarted.
        assert rig.gateway is not None
        assert rig.gateway.running()
        assert rig.gateway.pid == pid


async def test_installer_refuses_a_login_file_for_organization(world: HermesWorld) -> None:
    profile = Profile(login_file=True)
    async with hermes_rig(world, [], profile=profile, start=False) as rig:
        result = rig.installer("switch-mode", "organization", "--profile-home", str(rig.home))
        assert result.returncode == 2
        assert "login file" in result.stderr
