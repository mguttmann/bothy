# SPDX-License-Identifier: Apache-2.0
"""Loading and running the recording scenarios against the provisioned Hermes version.

A scenario (``tests/recording/scenarios/<name>.json``) names its point of 09 17.10, the
provider and credential sources of its throwaway Hermes home, the probe flags, extra
configuration, the model script and the steps. :class:`ScenarioRunner` gives every scenario
its own throwaway homes, fake model, egress recorder and control channel, starts the gateway
in its own process group (under the sandbox on macOS), runs the steps, waits a settle time,
checks for listeners and stops the group. Nothing of a run reaches a real model, a real
credential or another Hermes installation.

Example:
    >>> runner = ScenarioRunner(install, root=tmp_root)
    >>> result = runner.run(load_scenario(SCENARIO_DIR / "tool_and_text.json"))
"""

from __future__ import annotations

import copy
import json
import shutil
import sys
import threading
import time
from collections.abc import Iterable, Mapping
from concurrent.futures import ThreadPoolExecutor
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Final

import yaml

from hermes_harness.control import ControlServer
from hermes_harness.egress import EgressRecorder
from hermes_harness.environment import ThrowawayHomes, build_environment, interpreter_dirs
from hermes_harness.fake_model import (
    CONTEXT_LENGTH,
    MODEL_NAME,
    FakeCredentials,
    FakeModel,
    ScriptTurn,
)
from hermes_harness.gateway import GatewayError, HermesGateway, unexpected_listeners
from hermes_harness.limits import (
    COMMAND_POLL_S,
    GATEWAY_START_DEADLINE_S,
    PARALLEL_SCENARIOS,
    SETTLE_S,
    STEP_DEADLINE_S,
)
from hermes_harness.provision import HermesInstall
from hermes_harness.sandbox import sandbox_profile, wrap_command

TESTS_DIR = Path(__file__).resolve().parents[1]
SCENARIO_DIR = TESTS_DIR / "recording" / "scenarios"
PROBE_DIR = TESTS_DIR / "recording" / "probe_plugin"
PLATFORM = "recording_probe"
#: Where the pinned version discovers user plugins, relative to the Hermes home.
PLUGIN_PLACE = Path("plugins") / PLATFORM
#: The people of the scenarios: key, user ID, display name (white label).
PEOPLE: Mapping[str, tuple[str, str]] = {
    "alex": ("u-alex", "Alex"),
    "sam": ("u-sam", "Sam"),
    "robin": ("u-robin", "Robin"),
}
CHANNEL_ID = "c-general"
HOME_CHANNEL_ID = "c-notices"
#: API hosts of model providers; an attempt to reach one fails the run.
#: API hosts of the providers the scenarios configure (Anthropic and the OpenAI-compatible
#: route); an attempt to reach one means a base URL the harness missed and fails the run.
PROVIDER_HOSTS = frozenset(
    {"api.anthropic.com", "console.anthropic.com", "platform.claude.com", "api.openai.com"}
)
#: Calls that a timer repeats while a turn runs; they do not count as activity.
TIMER_CALLS = frozenset({"send_typing", "stop_typing"})
#: The longest Unix socket path the pinned version binds below the Hermes home is its loop-tick
#: socket, named after the PID (at most 5 digits on macOS, 7 on Linux). Hermes v0.22 binds it
#: only up to 100 bytes (its own margin below ``sun_path``) and listens on TCP loopback for a
#: longer path, which a recording refuses as an opened listener.
_PID_DIGITS = 5 if sys.platform == "darwin" else 7
_SOCKET_SUFFIX = "/state/gateway.loop-tick." + "9" * _PID_DIGITS + ".sock"
_SUN_PATH_MAX = 100
_CREDENTIAL_NAMES = {
    "anthropic_api_key": "ANTHROPIC_API_KEY",
    "anthropic_oauth": "ANTHROPIC_TOKEN",
}
_STEPS = frozenset(
    {
        "inject",
        "wait_held",
        "release",
        "wait_for",
        "remember_session",
        "credential_type",
        "set_dotenv",
        "interrupt",
        "describe",
        "wait_quiet",
        "confirm",
    }
)


#: Surfaces of 09 17.2, 17.3 and 17.9 whose presence the probe reports (owner.attribute).
SURFACES: Final = (
    "ctx.register_platform",
    "ctx.register_hook",
    "config.extra",
    "entry.env_enablement_fn",
    "entry.apply_yaml_config_fn",
    "entry.cron_deliver_env_var",
    "entry.standalone_sender_fn",
    "entry.max_message_length",
    "entry.allowed_users_env",
    "entry.allow_all_env",
    "base.connect",
    "base.disconnect",
    "base.send",
    "base.edit_message",
    "base.supports_draft_streaming",
    "base.send_draft",
    "base.supports_status_text",
    "base.set_status_text",
    "base.format_tool_event",
    "base.render_message_event",
    "base.send_typing",
    "base.stop_typing",
    "base.send_image",
    "base.send_document",
    "base.send_voice",
    "base.send_video",
    "base.send_clarify",
    "base._send_exec_approval_prompt",
    "base.send_slash_confirm",
    "base.send_model_picker",
    "base.send_choice_picker",
    "base.on_processing_start",
    "base.on_processing_complete",
    "base.get_chat_info",
    "base.build_source",
    "base.handle_message",
    "base.MAX_MESSAGE_LENGTH",
    "base.splits_long_messages",
    "base.enforces_own_access_policy",
    "base.authorization_is_upstream",
    "base.cancel_session_processing",
    "base.interrupt_session_activity",
)


def root_fits(root: Path) -> bool:
    """Whether the Hermes homes below ``root`` keep their Unix socket paths within sun_path.

    The scenario homes are ``<root>/<nn>/hermes/h``; see :data:`_SOCKET_SUFFIX`.
    """
    return len(str(root) + "/00/hermes/h" + _SOCKET_SUFFIX) <= _SUN_PATH_MAX


class ScenarioError(RuntimeError):
    """A scenario file is invalid or a step failed."""


@dataclass(frozen=True)
class Scenario:
    """One recording scenario (see the module docstring for the file format)."""

    name: str
    point: str
    provider: str
    credentials: Mapping[str, Any]
    probe: Mapping[str, Any]
    config: Mapping[str, Any]
    allowlist: tuple[str, ...]
    texts: Mapping[str, str]
    model_script: tuple[ScriptTurn, ...]
    steps: tuple[Mapping[str, Any], ...]


def load_scenario(path: Path) -> Scenario:
    """Read and validate one scenario file.

    Raises:
        ScenarioError: A field is missing, the name differs from the file name, a step is
            unknown or references an unknown text or person.
    """
    data = json.loads(path.read_text(encoding="utf-8"))
    for name in ("scenario", "point", "provider", "texts", "model_script", "steps"):
        if name not in data:
            raise ScenarioError(f"{path.name}: field {name!r} is missing")
    if data["scenario"] != path.stem:
        raise ScenarioError(f"{path.name}: scenario name differs from the file name")
    if data["provider"] not in {"openai", "anthropic"}:
        raise ScenarioError(f"{path.name}: unknown provider {data['provider']!r}")
    texts = dict(data["texts"])
    for step in data["steps"]:
        if step.get("do") not in _STEPS:
            raise ScenarioError(f"{path.name}: unknown step {step.get('do')!r}")
        if "user" in step and step["user"] not in PEOPLE:
            raise ScenarioError(f"{path.name}: unknown person {step['user']!r}")
        if "text" in step and step["text"] not in texts:
            raise ScenarioError(f"{path.name}: unknown text {step['text']!r}")
    allowlist = tuple(data.get("allowlist", ()))
    if any(person not in PEOPLE for person in allowlist):
        raise ScenarioError(f"{path.name}: unknown person in allowlist")
    return Scenario(
        name=data["scenario"],
        point=data["point"],
        provider=data["provider"],
        credentials=dict(data.get("credentials", {})),
        probe=dict(data.get("probe", {})),
        config=dict(data.get("config", {})),
        allowlist=allowlist,
        texts=texts,
        model_script=tuple(ScriptTurn.from_dict(t) for t in data["model_script"]),
        steps=tuple(data["steps"]),
    )


#: The point whose scenarios record the tool progress itself; they order it with latches.
PROGRESS_POINT: Final = "progress_hooks"


def timing_problems(scenario: Scenario) -> list[str]:
    """Reasons why a scenario's call pattern would depend on stream timing.

    The pinned stream consumer drafts only the deltas it drains before the end of the
    stream, and the tool progress task polls its queue on its own clock; an unpaced answer
    or a progress bubble outside the progress scenarios therefore races the answer and makes
    the pattern flaky. Every text turn is paced, and a scenario with tool calls switches the
    tool progress of the probe platform off unless it records the progress itself.

    Args:
        scenario: The scenario to check.

    Returns:
        One reason per problem, empty when the scenario is timing independent.
    """
    problems = [
        f"turn {number}: text turn is not paced"
        for number, turn in enumerate(scenario.model_script, start=1)
        if turn.text and not turn.paced
    ]
    has_tools = any(turn.tool_calls for turn in scenario.model_script)
    platforms = dict(scenario.config.get("display", {})).get("platforms", {})
    progress = dict(platforms.get(PLATFORM, {})).get("tool_progress")
    if has_tools and scenario.point != PROGRESS_POINT and progress != "off":
        problems.append(f"tool calls without display.platforms.{PLATFORM}.tool_progress: off")
    return problems


def load_scenarios(directory: Path = SCENARIO_DIR) -> list[Scenario]:
    """All scenarios of a directory, sorted by name."""
    return [load_scenario(path) for path in sorted(directory.glob("*.json"))]


@dataclass
class ScenarioResult:
    """The raw recording of one scenario (normalized later by :mod:`hermes_harness.calls`).

    Attributes:
        name: Scenario name.
        root: The scenario's temporary directory.
        calls: Probe calls in arrival order.
        model_requests: Request shapes of the fake model in arrival order.
        commands: ``{"command", "result"}`` of every command step.
        egress: Attempted destinations ``(host:port, phase)``.
        store_calls: Calls of the credential store and service shims.
        listeners: Listening sockets of the gateway process group after ``connect``.
        failures: Reasons that fail the run.
        discovery: Where the probe was installed, relative to the Hermes home.
        sessions: Session IDs remembered by ``remember_session`` steps.
    """

    name: str
    root: Path
    calls: list[dict[str, Any]] = field(default_factory=list)
    model_requests: list[dict[str, Any]] = field(default_factory=list)
    commands: list[dict[str, Any]] = field(default_factory=list)
    egress: list[tuple[str, str]] = field(default_factory=list)
    store_calls: list[str] = field(default_factory=list)
    listeners: list[str] = field(default_factory=list)
    failures: list[str] = field(default_factory=list)
    discovery: str = ""
    sessions: dict[str, str] = field(default_factory=dict)


def _deep_merge(base: dict[str, Any], extra: Mapping[str, Any]) -> dict[str, Any]:
    merged = copy.deepcopy(base)
    for key, value in extra.items():
        if isinstance(value, Mapping) and isinstance(merged.get(key), dict):
            merged[key] = _deep_merge(merged[key], value)
        else:
            merged[key] = copy.deepcopy(value)
    return merged


class _Run:
    """State of one running scenario."""

    def __init__(
        self,
        scenario: Scenario,
        install: HermesInstall,
        root: Path,
        probe_dir: Path,
        credentials: FakeCredentials,
    ) -> None:
        self.scenario = scenario
        self.install = install
        self.homes = ThrowawayHomes.create(root)
        self.hermes_home = self.homes.hermes("h")
        if len(str(self.hermes_home) + _SOCKET_SUFFIX) > _SUN_PATH_MAX:
            raise ScenarioError(
                f"temporary root {root} is too long for the Unix sockets of the Hermes home; "
                "use a shorter TMPDIR"
            )
        self.creds = credentials
        self.model = FakeModel(
            scenario.model_script, credentials, scenario_texts=list(scenario.texts.values())
        )
        self.control = ControlServer()
        self.egress = EgressRecorder()
        self.result = ScenarioResult(name=scenario.name, root=root)
        self.probe_dir = probe_dir
        self.mark = 0

    # ---- throwaway configuration -------------------------------------------------------
    def _secret(self, kind: str) -> str:
        return self.creds.api_key if kind == "api_key" else self.creds.oauth_token

    def _config(self) -> dict[str, Any]:
        scenario = self.scenario
        model: dict[str, Any]
        if scenario.provider == "anthropic":
            model = {"provider": "anthropic", "base_url": self.model.anthropic_base_url}
        else:
            model = {"provider": "custom", "base_url": self.model.openai_base_url}
            if scenario.credentials.get("config_api_key"):
                model["api_key"] = self._secret(scenario.credentials["config_api_key"])
        model |= {"default": MODEL_NAME, "context_length": CONTEXT_LENGTH}
        extra: dict[str, Any] = {
            "control_url": self.control.url,
            "command_poll_s": COMMAND_POLL_S,
            "supports_draft_streaming": bool(scenario.probe.get("supports_draft_streaming")),
            "supports_status_text": bool(scenario.probe.get("supports_status_text")),
            "enforces_own_access_policy": bool(scenario.probe.get("enforces_own_access_policy")),
            "authorization_is_upstream": bool(scenario.probe.get("authorization_is_upstream")),
        }
        if scenario.probe.get("dm_policy"):
            extra["dm_policy"] = scenario.probe["dm_policy"]
        base: dict[str, Any] = {
            "model": model,
            "plugins": {"enabled": [PLATFORM]},
            "platform_toolsets": {PLATFORM: ["todo"]},
            "tools": {"tool_search": {"enabled": "off"}},
            "streaming": {"enabled": True},
            "security": {"allow_lazy_installs": False, "tirith_enabled": False},
            "updates": {"check": False},
            "model_catalog": {"enabled": False},
            "models_dev": {"url": self.model.models_dev_url},
            "gateway": {
                "platforms": {
                    PLATFORM: {
                        "enabled": True,
                        "home_channel": {
                            "platform": PLATFORM,
                            "chat_id": HOME_CHANNEL_ID,
                            "name": "Notices",
                        },
                        "extra": extra,
                    }
                }
            },
        }
        return _deep_merge(base, scenario.config)

    def write_dotenv(self, values: Mapping[str, str | None]) -> None:
        lines = []
        if self.scenario.allowlist:
            people = ",".join(PEOPLE[p][0] for p in self.scenario.allowlist)
            lines.append(f"RECORDING_PROBE_ALLOWED_USERS={people}")
        for name, kind in values.items():
            if kind:
                lines.append(f"{_CREDENTIAL_NAMES[name]}={self._secret(kind)}")
        (self.hermes_home / ".env").write_text("\n".join(lines) + "\n", encoding="utf-8")

    def prepare(self) -> HermesGateway:
        plugin = self.hermes_home / PLUGIN_PLACE
        shutil.copytree(self.probe_dir, plugin, ignore=shutil.ignore_patterns("__pycache__"))
        self.result.discovery = PLUGIN_PLACE.as_posix()
        (self.hermes_home / "config.yaml").write_text(
            yaml.safe_dump(self._config(), sort_keys=True), encoding="utf-8"
        )
        self.write_dotenv(self.scenario.credentials.get("dotenv", {}))
        if self.scenario.credentials.get("login_file"):
            login = self.homes.home / ".claude" / ".credentials.json"
            login.parent.mkdir(parents=True)
            payload = {
                "claudeAiOauth": {
                    "accessToken": self.creds.oauth_token,
                    "refreshToken": "fake-refresh-" + self.creds.oauth_token[-12:],
                    "expiresAt": 4102444800000,
                }
            }
            login.write_text(json.dumps(payload), encoding="utf-8")
        extra_env = {
            _CREDENTIAL_NAMES[name]: self._secret(kind)
            for name, kind in dict(self.scenario.credentials.get("env", {})).items()
        }
        env = build_environment(
            self.homes, "h", self.egress.url, extra_env, venv_bin=self.install.venv / "bin"
        )
        profile = self.homes.root / "sandbox.sb"
        readable = [*interpreter_dirs(), self.install.root, self._python_home()]
        ports = [self.model.port, self.control.port, self.egress.port]
        profile.write_text(
            sandbox_profile(self.homes.root, readable=readable, ports=ports), encoding="utf-8"
        )
        # The pinned venv and export live in the shared root. The profile does not stop writes
        # there; the integrity check after the run finds any change to them.
        command = wrap_command([str(self.install.executable), "gateway", "run", "--force"], profile)
        return HermesGateway(
            command, env=env, cwd=self.homes.work, log=self.homes.root / "gateway.log"
        )

    def _python_home(self) -> Path:
        python = (self.install.venv / "bin" / "python").resolve()
        return python.parents[1]

    # ---- readiness and steps -----------------------------------------------------------
    def wait_ready(self, gateway: HermesGateway) -> None:
        """Wait for ``connect``, then until the gateway's startup restore let inbound through."""
        gateway.wait_connected(self.control)
        end = time.monotonic() + GATEWAY_START_DEADLINE_S
        while time.monotonic() < end:
            if not gateway.running():
                raise GatewayError(f"gateway exited before it was ready (see {gateway.log})")
            command = self.control.push("ready")
            if (self.control.result(command) or {}).get("ready") is True:
                return
            time.sleep(COMMAND_POLL_S * 4)
        raise GatewayError(f"gateway not ready within the start deadline (see {gateway.log})")

    def _message(self, step: Mapping[str, Any], text: str) -> dict[str, Any]:
        user_id, user_name = PEOPLE[step.get("user", "alex")]
        return {
            "chat_id": CHANNEL_ID,
            "thread_id": step.get("thread", "t-1"),
            "user_id": user_id,
            "user_name": user_name,
            "message_id": step["message"],
            "text": text,
        }

    def _text(self, step: Mapping[str, Any]) -> str:
        if "command" in step:
            text = str(step["command"])
            for key, value in self.scenario.texts.items():
                text = text.replace("{text:" + key + "}", value)
            for key, value in self.result.sessions.items():
                text = text.replace("{session:" + key + "}", value)
            return text
        return self.scenario.texts[step["text"]]

    def _command(self, name: str, **args: Any) -> Any:
        command = self.control.push(name, **args)
        result = self.control.result(command)
        self.result.commands.append({"command": name, "result": result})
        if isinstance(result, dict) and "error" in result:
            raise ScenarioError(f"command {name} failed in the probe: {result['error']}")
        return result

    def step(self, step: Mapping[str, Any]) -> None:
        action = step["do"]
        if action == "inject":
            self.mark = self.control.mark()
            self._command("inject", **self._message(step, self._text(step)))
        elif action == "interrupt":
            way = step.get("way", "command")
            args = self._message(step, self._text(step)) if way == "command" else {}
            args["way"] = way
            args["thread_id"] = step.get("thread", "t-1")
            self._command("interrupt", **args)
        elif action == "wait_held":
            self.model.wait_held(step["latch"])
        elif action == "release":
            self.model.release(step["latch"])
        elif action == "wait_for":
            contains = self.scenario.texts.get(step.get("text", ""), None)

            message = step.get("message")

            def matches(args: dict[str, Any]) -> bool:
                if contains is not None and contains not in json.dumps(args):
                    return False
                if message is not None and (args.get("event") or {}).get("message_id") != message:
                    return False
                return all(args.get(k) == v for k, v in dict(step.get("args", {})).items())

            after = 0 if step.get("from_start") else self.mark
            self.control.wait_for(step["method"], matches, after=after, deadline_s=STEP_DEADLINE_S)
        elif action == "remember_session":
            hooks = [c for c in self.control.calls() if c["method"] == "hook:pre_llm_call"]
            if not hooks or not hooks[-1]["args"].get("session_id"):
                raise ScenarioError("remember_session: no pre_llm_call hook with a session ID yet")
            self.result.sessions[step["as"]] = str(hooks[-1]["args"]["session_id"])
        elif action == "credential_type":
            self._command("credential_type")
        elif action == "set_dotenv":
            self.write_dotenv(step.get("values", {}))
        elif action == "confirm":
            self._command("confirm", choice=step.get("choice", "once"))
        elif action == "wait_quiet":
            self.wait_quiet()
        elif action == "describe":
            self._command("describe", surfaces=list(SURFACES))

    def wait_quiet(self) -> None:
        """Wait until neither the probe (timer calls aside) nor the fake model saw anything new.

        Raises:
            ScenarioError: Activity did not stop within the step deadline.
        """
        end = time.monotonic() + STEP_DEADLINE_S
        last = (-1, -1)
        while time.monotonic() < end:
            active = [c for c in self.control.calls() if c["method"] not in TIMER_CALLS]
            now = (len(active), len(self.model.requests()))
            if now == last:
                return
            last = now
            time.sleep(SETTLE_S)
        raise ScenarioError("activity did not stop within the step deadline")

    def finish(self) -> None:
        self.result.model_requests = self.model.requests()
        self.result.egress = self.egress.seen()
        self.result.store_calls = self.homes.store_calls()
        self.result.failures += self.model.failures()
        for destination, phase in self.result.egress:
            host = destination.rsplit(":", 1)[0]
            if host in PROVIDER_HOSTS:
                self.result.failures.append(f"model provider contacted: {destination} ({phase})")
        unexpected = unexpected_listeners(self.result.listeners)
        if unexpected:
            self.result.failures.append(f"Hermes opened listeners: {unexpected}")

    def close(self) -> None:
        for server in (self.model, self.control, self.egress):
            server.close()


class ScenarioRunner:
    """Run scenarios against one provisioned Hermes version."""

    def __init__(self, install: HermesInstall, *, root: Path, probe_dir: Path = PROBE_DIR) -> None:
        """Prepare the runner.

        Args:
            install: The provisioned version.
            root: Directory for the scenario roots (inside the temporary root).
            probe_dir: The probe plugin to install into each Hermes home.
        """
        self.install = install
        self.root = root
        self.probe_dir = probe_dir
        self._lock = threading.Lock()
        self._dirs: dict[str, str] = {}

    def run(self, scenario: Scenario) -> ScenarioResult:
        """Run one scenario in its own throwaway homes and gateway process group."""
        run = _Run(
            scenario,
            self.install,
            self.root / self._short(scenario.name),
            self.probe_dir,
            FakeCredentials.generate(),
        )
        gateway: HermesGateway | None = None
        try:
            run.model.start()
            run.control.start()
            run.egress.start()
            gateway = run.prepare()
            gateway.start()
            run.wait_ready(gateway)
            run.result.listeners = gateway.listening_sockets()
            run.egress.phase = "turn"
            for step in scenario.steps:
                run.step(step)
            time.sleep(SETTLE_S)
            # Again after the turns: a socket opened during a turn must not go unseen.
            late = [s for s in gateway.listening_sockets() if s not in run.result.listeners]
            run.result.listeners += late
        except (GatewayError, ScenarioError, TimeoutError) as exc:
            run.result.failures.append(f"{type(exc).__name__}: {exc}")
        finally:
            # The call pattern ends with the turns; the shutdown's calls depend on signal timing.
            run.result.calls = run.control.calls()
            run.egress.phase = "shutdown"
            if gateway is not None:
                try:
                    gateway.stop()
                except GatewayError as exc:
                    run.result.failures.append(str(exc))
            # Judged only after the stop: Hermes' shutdown work (memory extraction, session end
            # hooks) can still reach the fake model, the egress recorder or a store tool.
            run.finish()
            run.close()
        return run.result

    def _short(self, name: str) -> str:
        # Short directory names keep the Unix socket paths of the Hermes home within sun_path.
        with self._lock:
            if name not in self._dirs:
                self._dirs[name] = f"{len(self._dirs):02d}"
            return self._dirs[name]

    def run_all(self, scenarios: Iterable[Scenario]) -> dict[str, ScenarioResult]:
        """Run scenarios, :data:`~hermes_harness.limits.PARALLEL_SCENARIOS` at a time."""
        items = list(scenarios)
        for scenario in items:
            self._short(scenario.name)
        with ThreadPoolExecutor(max_workers=PARALLEL_SCENARIOS) as pool:
            results = list(pool.map(self.run, items))
        return {result.name: result for result in results}
