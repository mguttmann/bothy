# SPDX-License-Identifier: Apache-2.0
"""The runner of the mobile device E2E test (tools/e2e/mobile_e2e.py, brief M5-11 Design note 3).

The runner talks to the host only through ``mobile_e2e.Host``; these tests give it a fake host
that records every command and answers from a script, so they need no Docker, simulator or
emulator: port selection, the SKIP lines per missing tool, the failure when no platform can run,
and the teardown of everything the run started, also after a failing suite or stack.
"""

from __future__ import annotations

import json
from collections.abc import Callable, Sequence
from dataclasses import dataclass, field
from pathlib import Path

import mobile_e2e
import pytest

IPAD = {"identifier": "dt.ipad", "productFamily": "iPad"}
IPHONE = {"identifier": "dt.iphone", "productFamily": "iPhone"}
RUNTIMES = json.dumps(
    {
        "runtimes": [
            {"identifier": "rt.ios26", "platform": "iOS", "isAvailable": True, "version": "26.0",
             "supportedDeviceTypes": [IPAD]},
            {"identifier": "rt.ios27", "platform": "iOS", "isAvailable": True, "version": "27.0",
             "supportedDeviceTypes": [IPAD, IPHONE]},
            {"identifier": "rt.watch", "platform": "watchOS", "isAvailable": True, "version": "1",
             "supportedDeviceTypes": []},
        ]
    }
)  # fmt: skip


@dataclass
class FakeProc:
    """A started background process: records whether it was stopped.

    The helper of the terminal stack writes its connection file when it starts (unless it
    fails at once) and records on wait whether the runner had written the stop file.
    """

    cmd: list[str]
    pid: int
    stopped: bool = False
    out: Path | None = None
    stop_seen: bool = False
    ended: int | None = None

    def poll(self) -> int | None:
        if self.ended is not None:
            return self.ended
        return 0 if self.stopped else None

    def terminate(self) -> None:
        self.stopped = True

    def kill(self) -> None:
        self.stopped = True

    def wait(self, timeout: float | None = None) -> int:
        if self.out is not None:
            self.stop_seen = Path(f"{self.out}.stop").exists()
        self.stopped = True
        return self.ended or 0


@dataclass
class FakeHost:
    """A host that answers commands from a table of (prefix, rc, stdout) and records them."""

    system: str = "Darwin"
    tools: set[str] = field(
        default_factory=lambda: {"xcrun", "flutter", "adb", "emulator", "docker", "go"}
    )
    # Whether the helper of the terminal stack ends before it wrote its connection file.
    helper_fails: bool = False
    env: dict[str, str] = field(default_factory=dict)
    answers: list[tuple[tuple[str, ...], int, str]] = field(default_factory=list)
    calls: list[list[str]] = field(default_factory=list)
    envs: list[dict[str, str] | None] = field(default_factory=list)
    logs: list[Path | None] = field(default_factory=list)
    procs: list[FakeProc] = field(default_factory=list)
    clock: float = 0.0
    free: Callable[[int], bool] = lambda _port: True

    def which(self, name: str) -> str | None:
        return f"/fake/{name}" if name in self.tools else None

    def run(
        self, cmd: Sequence[str], env: dict[str, str] | None = None, log: Path | None = None
    ) -> tuple[int, str]:
        self.calls.append(list(cmd))
        self.envs.append(env)
        self.logs.append(log)
        for prefix, rc, out in self.answers:
            if tuple(cmd[: len(prefix)]) == prefix:
                return rc, out
        return 0, ""

    def spawn(
        self, cmd: Sequence[str], env: dict[str, str] | None = None, log: Path | None = None
    ) -> FakeProc:
        self.calls.append(list(cmd))
        self.envs.append(env)
        self.logs.append(log)
        proc = FakeProc(list(cmd), pid=1000 + len(self.procs))
        if "^TestClientStack$" in cmd and env is not None:
            proc.out = Path(env["BOTHY_CLIENTSTACK_OUT"])
            if self.helper_fails:
                proc.ended = 1
            else:
                proc.out.write_text(
                    json.dumps(
                        {"mode": env["BOTHY_CLIENTSTACK_MODE"], "published_port": 41443,
                         "control_url": "http://127.0.0.1:41444"}
                    ),
                    encoding="utf-8",
                )  # fmt: skip
        self.procs.append(proc)
        return proc

    def now(self) -> float:
        return self.clock

    def sleep(self, seconds: float) -> None:
        self.clock += seconds

    def port_free(self, port: int) -> bool:
        return self.free(port)

    def ran(self, *prefix: str) -> list[list[str]]:
        return [c for c in self.calls if tuple(c[: len(prefix)]) == prefix]


def booting_host(**kw: object) -> FakeHost:
    """A host with Xcode and the Android SDK whose simulator and emulator come up at once."""
    host = FakeHost(**kw)  # type: ignore[arg-type]
    host.env.setdefault("MOBILE_E2E_AVD", "e2e-avd")
    host.answers += [
        (("xcrun", "--find", "simctl"), 0, "/fake/simctl\n"),
        (("xcrun", "simctl", "list", "runtimes", "-j"), 0, RUNTIMES),
        (("xcrun", "simctl", "create"), 0, "SIM-UDID-1\n"),
        (("adb", "-s"), 0, "1\n"),
    ]
    return host


@pytest.fixture(autouse=True)
def _logs_in_tmp(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    """The runner's logs go to the test's temp dir, never into the repository."""
    monkeypatch.setattr(mobile_e2e, "_log_dir", lambda _host: tmp_path)


# ---- ports -------------------------------------------------------------------------------------


def test_free_port_skips_taken_ports() -> None:
    offered = iter([5000, 41000, 41001])
    port = mobile_e2e.free_port(taken={41000}, bind=lambda: next(offered))
    assert port == 41001


def test_free_port_gives_up_with_an_error() -> None:
    with pytest.raises(RuntimeError, match="free port"):
        mobile_e2e.free_port(taken={41000}, bind=lambda: 41000)


def test_reserved_ports_of_the_host_rules_are_never_chosen() -> None:
    offered = iter([3000, 8080, 5432, 8642, 52000])
    assert mobile_e2e.free_port(bind=lambda: next(offered)) == 52000


def test_console_port_is_even_and_both_ports_free() -> None:
    taken = {5554, 5557}
    port = mobile_e2e.free_console_port(lambda p: p not in taken)
    assert port == 5558


# ---- preflight ---------------------------------------------------------------------------------


def test_skip_lines_per_missing_tool() -> None:
    host = FakeHost(system="Linux", tools={"flutter", "docker"})
    ready, skips = mobile_e2e.preflight(["ios", "android"], host)
    assert ready == []
    assert any(s.startswith("SKIP e2e-mobile ios:") and "macOS" in s for s in skips)
    assert any(s.startswith("SKIP e2e-mobile android:") and "adb" in s for s in skips)


def test_ios_without_simctl_is_skipped_android_runs() -> None:
    host = FakeHost(env={"ANDROID_HOME": "/sdk", "MOBILE_E2E_AVD": "a"})
    host.answers.append((("xcrun", "--find", "simctl"), 1, ""))
    ready, skips = mobile_e2e.preflight(["ios", "android"], host)
    assert ready == ["android"]
    assert skips == [
        "SKIP e2e-mobile ios: xcrun simctl missing (install Xcode with the iOS simulator)"
    ]


def test_android_without_avd_or_serial_is_skipped() -> None:
    host = FakeHost(tools={"adb", "emulator", "flutter"}, system="Linux")
    host.answers.append((("emulator", "-list-avds"), 0, ""))
    ready, skips = mobile_e2e.preflight(["android"], host)
    assert ready == []
    assert "MOBILE_E2E_AVD" in skips[0]


def test_android_takes_the_first_avd() -> None:
    host = FakeHost(tools={"adb", "emulator", "flutter"}, system="Linux")
    host.answers.append((("emulator", "-list-avds"), 0, "first\nsecond\n"))
    ready, _ = mobile_e2e.preflight(["android"], host)
    assert ready == ["android"]
    assert host.env["MOBILE_E2E_AVD"] == "first"


def test_missing_flutter_skips_both() -> None:
    host = FakeHost(tools={"xcrun", "adb", "emulator"}, env={"MOBILE_E2E_AVD": "a"})
    host.answers.append((("xcrun", "--find", "simctl"), 0, ""))
    ready, skips = mobile_e2e.preflight(["ios", "android"], host)
    assert ready == []
    assert all("flutter" in s for s in skips)


def test_unknown_platform_is_an_error() -> None:
    with pytest.raises(ValueError, match="windows"):
        mobile_e2e.platforms_from("ios windows")


def test_unknown_suite_is_an_error() -> None:
    with pytest.raises(ValueError, match="logout"):
        mobile_e2e.suites_from("login logout")
    assert mobile_e2e.suites_from("") == (list(mobile_e2e.WORLD_SUITES), True)
    assert mobile_e2e.suites_from("terminal push") == (["push"], True)


def test_the_terminal_suite_needs_go(capsys: pytest.CaptureFixture[str]) -> None:
    host = booting_host(tools={"xcrun", "flutter", "adb", "emulator", "docker"})
    assert mobile_e2e.run_outer(["android"], host, bind=lambda: 47001) == 2
    assert "go is not on PATH" in capsys.readouterr().out
    assert e2e_calls(host) == []


def test_no_platform_fails_the_run(capsys: pytest.CaptureFixture[str]) -> None:
    host = FakeHost(system="Linux", tools=set())
    assert mobile_e2e.run_outer(["ios", "android"], host) == 1
    out = capsys.readouterr().out
    assert "SKIP e2e-mobile ios:" in out
    assert "e2e-mobile: no platform could run" in out
    assert host.ran("infra/compose/e2e.sh") == []


# ---- the run and its teardown ----------------------------------------------------------------


def e2e_calls(host: FakeHost) -> list[int]:
    return [i for i, c in enumerate(host.calls) if c and c[0].endswith("infra/compose/e2e.sh")]


def test_run_hands_the_devices_and_the_capture_port_to_the_stack() -> None:
    host = booting_host()
    ports = iter([47001, 47001, 47002, 3000, 47003])
    rc = mobile_e2e.run_outer(["ios", "android"], host, bind=lambda: next(ports))
    assert rc == 0
    (i,) = e2e_calls(host)
    env = host.envs[i]
    assert env is not None
    assert env["E2E_OVERLAY"].endswith("infra/compose/test/compose.e2e-mobile.yaml")
    assert env["MOBILE_E2E_FAKEPUSH_PORT"] == "47001"
    assert env["MOBILE_E2E_PATROL_PORTS"] == "47002 47003"
    assert env["MOBILE_E2E_UDID"] == "SIM-UDID-1"
    assert env["MOBILE_E2E_SERIAL"].startswith("emulator-")
    assert env["MOBILE_E2E_READY"] == "ios android"
    # adb of the SDK is on PATH for patrol and the relay.
    assert env["PATH"].split(":")[0] == "/fake"
    assert host.calls[i][-1] == "--inner"
    # The stack's output is kept: on the console and in stack.log.
    assert host.logs[i] == Path(env["MOBILE_E2E_LOG_DIR"]) / "stack.log"


def test_teardown_after_a_failing_stack_or_suite() -> None:
    host = booting_host()
    host.answers.insert(0, (("infra/compose/e2e.sh",), 1, ""))
    host.answers.insert(0, ((str(mobile_e2e.REPO / "infra/compose/e2e.sh"),), 1, ""))
    ports = iter(range(47001, 47100))
    rc = mobile_e2e.run_outer(["ios", "android"], host, bind=lambda: next(ports))
    assert rc == 1
    serial = next(p.cmd for p in host.procs if "-avd" in p.cmd)
    port = serial[serial.index("-port") + 1]
    assert host.ran("adb", "-s", f"emulator-{port}", "emu", "kill")
    (create,) = host.ran("xcrun", "simctl", "create")
    assert create[3].startswith("bothy-e2e-")
    assert create[4:] == ["dt.iphone", "rt.ios27"]
    assert all(p.stopped for p in host.procs)
    assert host.ran("xcrun", "simctl", "shutdown", "SIM-UDID-1")
    assert host.ran("xcrun", "simctl", "delete", "SIM-UDID-1")


def test_teardown_when_the_emulator_never_boots() -> None:
    host = booting_host(env={"MOBILE_E2E_AVD": "a", "MOBILE_E2E_BOOT_TIMEOUT_S": "30"})
    host.answers.insert(0, (("adb", "-s"), 0, "0\n"))
    rc = mobile_e2e.run_outer(["ios", "android"], host, bind=lambda: 47001)
    assert rc == 1
    assert e2e_calls(host) == []
    assert all(p.stopped for p in host.procs)
    assert host.ran("xcrun", "simctl", "delete", "SIM-UDID-1") or not host.ran(
        "xcrun", "simctl", "create"
    )


def test_a_given_simulator_and_emulator_are_not_removed() -> None:
    host = booting_host(env={"MOBILE_E2E_UDID": "MINE", "MOBILE_E2E_SERIAL": "emulator-5554"})
    host.answers.insert(
        0, (("xcrun", "simctl", "list", "devices"), 0, "    iPhone (MINE) (Booted)\n")
    )
    ports = iter(range(47001, 47100))
    assert mobile_e2e.run_outer(["ios", "android"], host, bind=lambda: next(ports)) == 0
    assert not host.ran("xcrun", "simctl", "create")
    assert not host.ran("xcrun", "simctl", "delete")
    assert not host.ran("xcrun", "simctl", "shutdown")
    assert not [p for p in host.procs if "-avd" in p.cmd]
    assert not host.ran("adb", "-s", "emulator-5554", "emu", "kill")


# ---- the inner part: relay and suite per platform -------------------------------------------


def test_inner_runs_relay_and_suite_and_stops_the_relay_after_a_failure(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    host = FakeHost(
        env={
            "MOBILE_E2E_READY": "ios android",
            "MOBILE_E2E_UDID": "SIM",
            "MOBILE_E2E_SERIAL": "emulator-5560",
            "MOBILE_E2E_FAKEPUSH_PORT": "47001",
            "MOBILE_E2E_LOG_DIR": str(tmp_path),
        }
    )
    suite = str(mobile_e2e.REPO / "apps/client/tool/mobile_e2e_suite.sh")
    host.answers.append(((suite, "ios"), 1, ""))
    rc = mobile_e2e.run_inner(host)
    assert rc == 1
    relays = [p for p in host.procs if any(a.endswith("push_relay.py") for a in p.cmd)]
    assert len(relays) == 2
    assert all(p.stopped for p in relays)
    assert "--capture-url" in relays[0].cmd
    assert "http://127.0.0.1:47001" in relays[0].cmd
    assert host.ran(suite, "android")
    out = capsys.readouterr().out
    assert "e2e-mobile ios: FAIL" in out
    assert "e2e-mobile android: PASS" in out


def test_inner_runs_only_the_named_suites(tmp_path: Path) -> None:
    host = FakeHost(
        env={
            "MOBILE_E2E_READY": "android",
            "MOBILE_E2E_SERIAL": "emulator-5560",
            "MOBILE_E2E_FAKEPUSH_PORT": "47001",
            "MOBILE_E2E_SUITES": "push offline",
        }
    )
    assert mobile_e2e.run_inner(host) == 0
    suite = str(mobile_e2e.REPO / "apps/client/tool/mobile_e2e_suite.sh")
    assert host.ran(suite) == [[suite, "android", "push", "offline"]]


def test_inner_needs_its_inputs() -> None:
    host = FakeHost(env={"MOBILE_E2E_READY": "ios"})
    assert mobile_e2e.run_inner(host) == 2


# ---- the terminal suite of M5-13 against the terminal stack ----------------------------------


SUITE = str(mobile_e2e.REPO / "apps/client/tool/mobile_e2e_suite.sh")


def helpers(host: FakeHost) -> list[FakeProc]:
    return [p for p in host.procs if "^TestClientStack$" in p.cmd]


def test_terminal_suite_runs_on_every_platform_against_one_stack() -> None:
    host = booting_host()
    ports = iter(range(47001, 47100))
    assert mobile_e2e.run_outer(["ios", "android"], host, bind=lambda: next(ports)) == 0
    (helper,) = helpers(host)
    i = host.calls.index(helper.cmd)
    env = host.envs[i]
    assert env is not None
    assert env["BOTHY_CLIENTSTACK_MODE"] == "mobile"
    assert helper.cmd[:3] == ["go", "-C", str(mobile_e2e.SERVER)]
    # After e2e.sh: one stack at a time.
    assert e2e_calls(host)[0] < i
    runs = host.ran(SUITE)
    assert [c[1:] for c in runs] == [["ios", "terminal"], ["android", "terminal"]]
    for c in runs:
        suite_env = host.envs[host.calls.index(c)]
        assert suite_env is not None
        assert suite_env["TERMINAL_STACK"] == str(helper.out)
        assert suite_env["MOBILE_E2E_TERMINAL_PORTS"] == "41443 41444"
    assert helper.stop_seen
    assert helper.stopped
    assert helper.out is not None
    assert not helper.out.parent.exists()


def test_the_terminal_stack_is_removed_after_a_failing_suite(
    capsys: pytest.CaptureFixture[str],
) -> None:
    host = booting_host(env={"MOBILE_E2E_SUITES": "terminal"})
    host.answers.insert(0, ((SUITE, "ios", "terminal"), 1, ""))
    ports = iter(range(47001, 47100))
    assert mobile_e2e.run_outer(["ios", "android"], host, bind=lambda: next(ports)) == 1
    # Only the terminal suite was named: no stack of e2e.sh.
    assert e2e_calls(host) == []
    (helper,) = helpers(host)
    assert helper.stop_seen
    assert helper.stopped
    assert helper.out is not None
    assert not helper.out.parent.exists()
    # The other platform still ran, and the devices of the run are removed.
    assert host.ran(SUITE, "android", "terminal")
    assert host.ran("xcrun", "simctl", "delete", "SIM-UDID-1")
    out = capsys.readouterr().out
    assert "e2e-mobile ios terminal: FAIL" in out
    assert "e2e-mobile android terminal: PASS" in out


def test_a_terminal_stack_that_never_got_ready_fails_without_a_suite() -> None:
    host = booting_host(env={"MOBILE_E2E_SUITES": "terminal"}, helper_fails=True)
    ports = iter(range(47001, 47100))
    assert mobile_e2e.run_outer(["android"], host, bind=lambda: next(ports)) == 1
    assert host.ran(SUITE) == []
    (helper,) = helpers(host)
    assert helper.stop_seen
    assert helper.out is not None
    assert not helper.out.parent.exists()
