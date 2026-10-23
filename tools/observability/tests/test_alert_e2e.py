# SPDX-License-Identifier: Apache-2.0
"""Tests of the runner of the alert path test (brief M7-20 Design note 2) without Docker: the
test rules, the derived Prometheus configuration, the checks of the stub records and of the
output of alert-setup.sh, the overlay and the preflight."""

from __future__ import annotations

import json
import subprocess
from pathlib import Path
from typing import Any

import alert_e2e
import pytest
import yaml


def test_expected_alerts_cover_page_and_ticket() -> None:
    alerts = alert_e2e.expected_alerts()
    assert {a.severity for a in alerts} == {"page", "ticket"}
    for a in alerts:
        assert a.flag() == f"{a.name}|{a.severity}|{a.summary}"
        assert "|" not in a.name


def test_expected_alerts_refuse_rule_without_summary(tmp_path: Path) -> None:
    rules = tmp_path / "rules.yaml"
    rules.write_text(
        yaml.safe_dump({"groups": [{"rules": [{"alert": "X", "labels": {"severity": "page"}}]}]}),
        encoding="utf-8",
    )
    with pytest.raises(alert_e2e.StepError, match="X"):
        alert_e2e.expected_alerts(rules)


def test_prometheus_config_keeps_the_real_routes_and_rules() -> None:
    real = yaml.safe_load(alert_e2e.PROMETHEUS_CONFIG.read_text(encoding="utf-8"))
    cfg = alert_e2e.prometheus_config()
    assert cfg["alerting"] == real["alerting"]
    assert cfg["rule_files"][: len(real["rule_files"])] == real["rule_files"]
    assert cfg["rule_files"][-1] == "/etc/prometheus-e2e/rules.yaml"
    assert cfg["global"]["evaluation_interval"] == alert_e2e.E2E_INTERVAL
    jobs = {j["job_name"]: j for j in cfg["scrape_configs"]}
    assert jobs["alert-stub"]["static_configs"] == [{"targets": [alert_e2e.STUB_TARGET]}]


def note(*alerts: tuple[str, str]) -> str:
    """One recorded notification with (alertname, status) alerts."""
    items = [{"status": s, "labels": {"alertname": n}} for n, s in alerts]
    return json.dumps({"status": "firing", "alerts": items})


def stub_dir(tmp_path: Path, oncall: list[str], heartbeat: list[str]) -> Path:
    for name, lines in (("oncall.jsonl", oncall), ("heartbeat.jsonl", heartbeat)):
        (tmp_path / name).write_text("".join(line + "\n" for line in lines), encoding="utf-8")
    return tmp_path


ALERTS = [
    alert_e2e.ExpectedAlert("AlertPathE2EPage", "page", "p"),
    alert_e2e.ExpectedAlert("AlertPathE2ETicket", "ticket", "t"),
]


def test_check_stubs_passes_on_the_expected_routes(tmp_path: Path) -> None:
    stub = stub_dir(
        tmp_path,
        [note(("AlertPathE2EPage", "firing")), note(("OtherAlert", "firing"))],
        [note(("Watchdog", "firing"))],
    )
    alert_e2e.check_stubs(stub, ALERTS)


@pytest.mark.parametrize(
    ("oncall", "heartbeat", "match"),
    [
        ([], [note(("Watchdog", "firing"))], "no firing AlertPathE2EPage"),
        (
            [note(("AlertPathE2EPage", "firing"), ("AlertPathE2ETicket", "firing"))],
            [note(("Watchdog", "firing"))],
            "AlertPathE2ETicket of level ticket",
        ),
        ([note(("AlertPathE2EPage", "firing"))], [], "no Watchdog"),
        (
            [note(("AlertPathE2EPage", "firing"))],
            [note(("Watchdog", "firing"), ("AlertPathE2EPage", "firing"))],
            "heartbeat stub got a test alert",
        ),
        ([note(("AlertPathE2EPage", "resolved"))], [note(("Watchdog", "firing"))], "no firing"),
    ],
)
def test_check_stubs_names_the_gap(
    tmp_path: Path, oncall: list[str], heartbeat: list[str], match: str
) -> None:
    with pytest.raises(alert_e2e.StepError, match=match):
        alert_e2e.check_stubs(stub_dir(tmp_path, oncall, heartbeat), ALERTS)


def test_missing_record_file_counts_as_empty(tmp_path: Path) -> None:
    assert alert_e2e.alert_names(tmp_path / "absent.jsonl", "firing") == set()


@pytest.mark.parametrize(
    "line",
    [
        "created x sha256:0123456789ab extra",
        "kept x sha256:0123456789ab",
        "created y sha256:0123456789ab",
        "created x sha256:0123",
        "created x https://chat.example.com/api/v1/hooks/a/b",
    ],
)
def test_check_setup_line_refuses_other_output(line: str) -> None:
    with pytest.raises(alert_e2e.StepError):
        alert_e2e.check_setup_line(line, "x", "created")


def test_check_setup_line_accepts_the_form() -> None:
    alert_e2e.check_setup_line("created x sha256:0123456789ab", "x", "created")


class ComposeLoader(yaml.SafeLoader):
    """SafeLoader that reads `!reset` and `!override` as their plain value."""


def _plain(loader: yaml.SafeLoader, node: yaml.Node) -> Any:
    if isinstance(node, yaml.SequenceNode):
        return loader.construct_sequence(node, deep=True)
    if isinstance(node, yaml.MappingNode):
        return loader.construct_mapping(node, deep=True)
    return loader.construct_scalar(node)  # type: ignore[arg-type]


ComposeLoader.add_constructor("!reset", _plain)
ComposeLoader.add_constructor("!override", _plain)


def test_overlay_publishes_nothing_and_stub_runs_as_caller() -> None:
    text = (alert_e2e.TEST_DIR / "compose.alert-e2e.yaml").read_text(encoding="utf-8")
    spec = yaml.load(text, Loader=ComposeLoader)  # noqa: S506 - a SafeLoader with the Compose tags
    services = spec["services"]
    for name, svc in services.items():
        assert not svc.get("ports"), name
    assert services["alert-probe"]["network_mode"] == "service:api"
    assert services["alert-stub"]["networks"] == ["internal"]
    assert "alertmanager" in services
    assert set(services["alertmanager"]) == {"ports"}


def test_stack_args_and_env(tmp_path: Path) -> None:
    env = alert_e2e.stack_env(
        {"PATH": "/bin", "BOTHY_X": "1", "COMPOSE_FILE": "x", "OTEL_Y": "2"},
        tmp_path,
        "img:1",
        "abcd",
    )
    assert "BOTHY_X" not in env
    assert "OTEL_Y" not in env
    assert env["COMPOSE_PROFILES"] == alert_e2e.PROFILES
    assert env["SMOKE_SERVER_IMAGE"] == "img:1"
    stack = alert_e2e.Stack(project="p", env=env)
    args = stack.args("ps")
    assert args[-1] == "ps"
    assert str(alert_e2e.TEST_DIR / "compose.alert-e2e.yaml") in args


def test_workspace_layout(tmp_path: Path) -> None:
    tmp = alert_e2e.workspace({"TMPDIR": str(tmp_path)})
    assert (tmp / "prometheus" / "rules.yaml").read_text(encoding="utf-8") == (
        alert_e2e.RULES.read_text(encoding="utf-8")
    )
    cfg = yaml.safe_load((tmp / "prometheus" / "prometheus.yaml").read_text(encoding="utf-8"))
    assert cfg == alert_e2e.prometheus_config()
    assert (tmp / "secrets-input").stat().st_mode & 0o777 == 0o700


class FakeRun:
    """A runner that answers docker commands from a table."""

    def __init__(self, answers: dict[str, tuple[int, str]]) -> None:
        self.answers = answers
        self.calls: list[list[str]] = []

    def __call__(self, args: list[str], **_: Any) -> subprocess.CompletedProcess[str]:
        self.calls.append(args)
        key = " ".join(args[1:3])
        rc, out = self.answers.get(key, (0, ""))
        return subprocess.CompletedProcess(args, rc, out, "")


def test_preflight_names_the_fix(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr(alert_e2e.shutil, "which", lambda _: "/usr/bin/docker")
    ok = {"version --format": (0, "29.5.2"), "compose version": (0, "2.40.3")}
    alert_e2e.preflight("img", FakeRun(ok))
    with pytest.raises(alert_e2e.PreconditionError, match="too old"):
        alert_e2e.preflight("img", FakeRun({**ok, "version --format": (0, "25.0.1")}))
    with pytest.raises(alert_e2e.PreconditionError, match="compose"):
        alert_e2e.preflight("img", FakeRun({**ok, "compose version": (0, "2.20.0")}))
    with pytest.raises(alert_e2e.PreconditionError, match="make image"):
        alert_e2e.preflight("img", FakeRun({**ok, "image inspect": (1, "")}))
    with pytest.raises(alert_e2e.PreconditionError, match="reachable"):
        alert_e2e.preflight("img", FakeRun({**ok, "version --format": (1, "")}))
    monkeypatch.setattr(alert_e2e.shutil, "which", lambda _: None)
    with pytest.raises(alert_e2e.PreconditionError, match="PATH"):
        alert_e2e.preflight("img", FakeRun(ok))


def test_main_exits_2_on_a_missing_precondition(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr(alert_e2e.shutil, "which", lambda _: None)
    assert alert_e2e.main([], {"PATH": "/bin"}, FakeRun({})) == 2


def test_default_image_is_the_tag_of_make_image() -> None:
    version = (alert_e2e.ROOT / "services" / "server" / "VERSION").read_text().strip()
    assert alert_e2e.default_image() == f"bothy-server:{version}-local"
    makefile = (alert_e2e.ROOT / "Makefile").read_text(encoding="utf-8")
    assert "IMAGE_NAME ?= bothy-server" in makefile
    assert "IMAGE_TAG ?= $(SERVER_VERSION)-local" in makefile


def test_parse_version() -> None:
    assert alert_e2e.parse_version("v29.5.2") == (29, 5, 2)
    assert alert_e2e.parse_version("2.40.3-desktop.1") == (2, 40, 3)
    assert alert_e2e.parse_version("27") == (27, 0, 0)
