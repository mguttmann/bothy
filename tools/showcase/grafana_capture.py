# SPDX-License-Identifier: Apache-2.0
"""Capture of the Grafana panel plugin with the demo's metrics (brief M7-28 Design note 1).

The demo's collector (smoke overlay) writes the server's metrics as OTLP JSON lines into the
volume `<project>_otel-smoke-out`. This module starts, in a network of its own, a second
collector of the image and version of `infra/compose/compose.yaml` that reads that file
(`otlpjsonfile`) and sends the metrics to a Prometheus with the configuration of the stack
(`infra/observability/prometheus`, OTLP receiver on), and a Grafana with the provisioning and
dashboards of the stack and the plugin build of `plugins/grafana-bothy-live` (the release ZIP of
its VERSION, unpacked). Grafana publishes on a free port of 127.0.0.1 only and lets an anonymous
viewer read; the run captures the dashboard `bothy-live` in kiosk mode in the light and the dark
theme with headless Chromium, then removes the three containers, the network and the files.
Nothing reaches another host, and the containers exist only for the run.
"""

from __future__ import annotations

import os
import re
import subprocess
import urllib.request
import zipfile
from pathlib import Path
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from playwright.sync_api import ViewportSize

from capture import STEP_TIMEOUT_S, Demo, StepError, demo_container, docker, say, until

COMPOSE = Path("infra/compose/compose.yaml")
PLUGIN = Path("plugins/grafana-bothy-live")
DASHBOARD = "/d/bothy-live/bothy-live?orgId=1&kiosk&from=now-30m&to=now"
VIEWPORT: ViewportSize = {"width": 1600, "height": 1000}
# Prometheus evaluates, and Grafana refreshes, on their own clocks: the panels are captured
# once the first query of the dashboard returned series and this long after.
SETTLE_MS = 8_000
COLLECTOR_CONFIG = """receivers:
  otlpjsonfile:
    include: [/in/metrics.jsonl]
    start_at: beginning
    poll_interval: 1s
exporters:
  otlphttp/prometheus:
    metrics_endpoint: http://prometheus:9090/api/v1/otlp/v1/metrics
service:
  telemetry:
    metrics:
      level: none
  pipelines:
    metrics:
      receivers: [otlpjsonfile]
      exporters: [otlphttp/prometheus]
"""


def compose_image(root: Path, service_marker: str) -> str:
    """Return the image of the stack whose reference starts with `service_marker`.

    The default of the variable in compose.yaml is the version the stack pins, for example
    `otel/opentelemetry-collector-contrib:${BOTHY_OTELCOL_VERSION:-0.162.0}`.

    Raises:
        StepError: compose.yaml names no such image.
    """
    text = (root / COMPOSE).read_text(encoding="utf-8")
    m = re.search(r"image:\s*(" + re.escape(service_marker) + r"):\$\{[A-Z_]+:-([^}]+)\}", text)
    if m is None:
        raise StepError(f"grafana: compose.yaml names no image {service_marker}")
    return f"{m.group(1)}:{m.group(2)}"


def plugin_build(root: Path, out: Path) -> Path:
    """Build the plugin's release ZIP and unpack it; return the plugin folder."""
    version = (root / PLUGIN / "VERSION").read_text(encoding="utf-8").strip()
    zips = out / "zip"
    zips.mkdir(parents=True)
    subprocess.run(
        ["make", "-C", str(PLUGIN), "release-build", f"VERSION={version}", f"OUT={zips}"],
        cwd=root,
        check=True,
        capture_output=True,
        text=True,
    )
    with zipfile.ZipFile(zips / f"bothy-live-panel-{version}.zip") as z:
        z.extractall(out / "plugins")
    folder = out / "plugins" / "bothy-live-panel"
    if not (folder / "plugin.json").is_file():
        raise StepError("grafana: the plugin ZIP has no bothy-live-panel/plugin.json")
    return folder


def demo_volume(demo: Demo) -> str:
    """Return the name of the demo's volume of collector output."""
    api = demo_container(demo, "api")
    project = docker(
        "inspect", "--format", '{{index .Config.Labels "com.docker.compose.project"}}', api
    ).stdout.strip()
    return f"{project}_otel-smoke-out"


def capture(root: Path, demo: Demo, work: Path) -> list[tuple[Path, str, str, str]]:
    """Capture the dashboard of the plugin in both themes; returns (file, kind, view, theme)."""
    from playwright.sync_api import sync_playwright  # noqa: PLC0415 - only for this platform

    base = work / "grafana"
    base.mkdir()
    say("grafana: building the panel plugin")
    plugin = plugin_build(root, base)
    (base / "collector.yaml").write_text(COLLECTOR_CONFIG, encoding="utf-8")
    run = f"bothy-showcase-{os.getpid()}"
    grafana = root / "infra/observability/grafana"
    names = [f"{run}-collector", f"{run}-prometheus", f"{run}-grafana"]
    docker("network", "create", run)
    try:
        docker(
            "run", "--detach", "--name", names[1], "--network", run,
            "--network-alias", "prometheus",
            "--volume", f"{root / 'infra/observability/prometheus'}:/etc/prometheus:ro",
            compose_image(root, "prom/prometheus"),
            "--config.file=/etc/prometheus/prometheus.yaml", "--web.enable-otlp-receiver",
            "--storage.tsdb.path=/prometheus",
        )  # fmt: skip
        docker(
            "run", "--detach", "--name", names[0], "--network", run,
            "--volume", f"{demo_volume(demo)}:/in:ro",
            "--volume", f"{base / 'collector.yaml'}:/etc/showcase/collector.yaml:ro",
            compose_image(root, "otel/opentelemetry-collector-contrib"),
            "--config=/etc/showcase/collector.yaml",
        )  # fmt: skip
        docker(
            "run", "--detach", "--name", names[2], "--network", run,
            "--publish", "127.0.0.1::3000",
            "--env", "GF_AUTH_ANONYMOUS_ENABLED=true",
            "--env", "GF_AUTH_ANONYMOUS_ORG_ROLE=Viewer",
            "--env", "GF_AUTH_DISABLE_LOGIN_FORM=true",
            "--env", "GF_ANALYTICS_REPORTING_ENABLED=false",
            "--env", "GF_ANALYTICS_CHECK_FOR_UPDATES=false",
            "--env", "GF_NEWS_NEWS_FEED_ENABLED=false",
            "--env", "GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=bothy-live-panel",
            "--volume", f"{grafana / 'provisioning'}:/etc/grafana/provisioning:ro",
            "--volume", f"{grafana / 'dashboards'}:/var/lib/grafana/dashboards:ro",
            "--volume", f"{plugin}:/var/lib/grafana/plugins/bothy-live-panel:ro",
            compose_image(root, "grafana/grafana"),
        )  # fmt: skip
        port = docker("port", names[2], "3000/tcp").stdout.strip().rsplit(":", 1)[1]
        url = f"http://127.0.0.1:{port}"
        until("Grafana to answer", lambda: _ok(url + "/api/health"), STEP_TIMEOUT_S, 2.0)
        until(
            "the demo's metrics in Prometheus",
            lambda: _has_series(names[1]),
            STEP_TIMEOUT_S,
            5.0,
        )
        out = base / "shots"
        out.mkdir()
        files = []
        with sync_playwright() as pw:
            browser = pw.chromium.launch()
            try:
                for theme in ("light", "dark"):
                    context = browser.new_context(
                        viewport=VIEWPORT, device_scale_factor=1, locale="en-US", timezone_id="UTC"
                    )
                    page = context.new_page()
                    page.goto(f"{url}{DASHBOARD}&theme={theme}")
                    page.wait_for_load_state("networkidle", timeout=STEP_TIMEOUT_S * 1000)
                    page.wait_for_timeout(SETTLE_MS)
                    path = out / f"live-dashboard-{theme}.png"
                    page.screenshot(path=str(path), animations="disabled")
                    files.append((path, "image", "live-dashboard", theme))
                    context.close()
            finally:
                browser.close()
        return files
    finally:
        for name in names:
            docker("rm", "--force", "--volumes", name, check=False)
        docker("network", "rm", run, check=False)


def _ok(url: str) -> bool:
    try:
        with urllib.request.urlopen(url, timeout=5) as r:  # noqa: S310 # nosec B310 - local Grafana
            return bool(r.status == 200)
    except OSError:
        return False


def _has_series(prometheus: str) -> bool:
    """Whether Prometheus holds a series of the demo's server.

    Only the demo's server sends OTLP to this Prometheus, so any `target_info` is its own.
    """
    r = docker(
        "exec", prometheus, "wget", "-qO-",
        "http://127.0.0.1:9090/api/v1/query?query=count(target_info)",
        check=False,
    )  # fmt: skip
    return r.returncode == 0 and '"result":[{' in r.stdout
