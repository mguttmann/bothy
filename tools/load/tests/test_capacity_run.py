# SPDX-License-Identifier: Apache-2.0
"""Unit tests of the runner of the capacity load test (tools/load/capacity_run.py)."""

from __future__ import annotations

import json
import subprocess
from collections.abc import Sequence
from pathlib import Path
from typing import Any

import capacity_run as cr
import portability_run as pr
import pytest

ROOT = Path(__file__).resolve().parents[3]


class Recorder:
    """A fake runner that records every command and answers from a table of prefixes."""

    def __init__(self, answers: dict[str, tuple[int, str]] | None = None) -> None:
        self.calls: list[list[str]] = []
        self.answers = answers or {}

    def __call__(self, args: Sequence[str], **kwargs: Any) -> subprocess.CompletedProcess[str]:
        cmd = list(args)
        self.calls.append(cmd)
        joined = " ".join(cmd)
        for prefix, (code, out) in self.answers.items():
            if joined.startswith(prefix):
                return subprocess.CompletedProcess(cmd, code, out, "")
        return subprocess.CompletedProcess(cmd, 0, "", "")


# ---- values of the specification ---------------------------------------------------------------


def test_read_values_from_the_real_chapters() -> None:
    v = cr.read_values(
        cr.ARCHITECTURE.read_text(encoding="utf-8"), cr.CONFIGURATION.read_text(encoding="utf-8")
    )
    assert v.cpus > 0
    assert v.memory >= 1 << 30
    assert v.message_create_p99.endswith("ms")
    assert v.fanout_p99.endswith("ms")
    assert v.messages_per_minute > 0
    assert "vCPU" in v.profile


def test_table_cells_and_missing_row() -> None:
    text = (
        "| `a.b` | what | test |\n| `a.b` | Load test | `150ms` | target |\n| `c` | x | `1` | y |\n"
    )
    assert cr.table_cells(text, "a.b") == ["a.b", "what", "test"]
    assert cr.table_cells(text, "a.b", "Load test") == ["a.b", "Load test", "150ms", "target"]
    with pytest.raises(pr.StepError, match=r"no table row names a\.c"):
        cr.table_cells(text, "a.c")


def test_read_values_rejects_a_rate_limit_that_is_no_count() -> None:
    arch = (
        "| `perf.reference_profile` | Load test | `4 vCPU, 8 GiB RAM` | x |\n"
        "| `perf.message_create_p99` | Load test | `150ms` | target |\n"
        "| `perf.fanout_local_p99` | Load test | `200ms` | target |\n"
    )
    conf = "| `ratelimit.rl.message.per_minute` | `X` | Server | int | `sixty` | y | active |\n"
    with pytest.raises(pr.StepError, match="no count"):
        cr.read_values(arch, conf)


def test_parse_profile() -> None:
    assert cr.parse_profile("4 vCPU, 8 GiB RAM, SSD") == (4, 8 << 30)
    with pytest.raises(pr.StepError):
        cr.parse_profile("a fast machine")


@pytest.mark.parametrize(
    ("cpus", "mem", "want"),
    [
        (4, 8 << 30, "match"),
        (8, 14 << 30, "emulated"),
        (2, 16 << 30, "below"),
        (8, 4 << 30, "below"),
        (4, int((8 << 30) * 0.95), "match"),  # the kernel keeps a share of the memory
    ],
)
def test_profile_mode(cpus: int, mem: int, want: str) -> None:
    assert cr.profile_mode(cpus, mem, 4, 8 << 30) == want


# ---- the override ------------------------------------------------------------------------------


def test_override_limits_and_seeding() -> None:
    seed = cr.override(True, 4, 8 << 30, "127.0.0.1/32,172.18.0.0/16")
    api = seed["services"]["api"]
    assert api["cpus"] == 4
    assert api["environment"]["GOMAXPROCS"] == "4"
    assert api["environment"]["BOTHY_SERVER__TRUSTED_PROXIES"] == "127.0.0.1/32,172.18.0.0/16"
    assert api["environment"]["BOTHY_RATELIMIT__RL__ADMIN__PER_MINUTE"] == str(cr.SEED_LIMIT)
    assert seed["services"]["postgres"] == {"cpus": 4, "mem_limit": 8 << 30}
    assert seed["services"]["worker"]["environment"] == {"GOMAXPROCS": "4"}
    measured = cr.override(False, 4, 8 << 30, "127.0.0.1/32")
    # The ramp runs with the default rate limits: no class is raised.
    assert not [k for k in measured["services"]["api"]["environment"] if "RATELIMIT" in k]


# ---- measurements ------------------------------------------------------------------------------


def test_prom_queries_cover_the_metrics_of_16_13() -> None:
    q = cr.prom_queries(60)
    assert "bothy_db_event_seq_lock_wait_seconds_bucket[60s]" in q["lock_wait_p99_s"]
    assert "postgresql_backends" in q["backends_max"]
    assert "bothy_gateway_connections_active" in q["gateway_connections_max"]
    assert "bothy_event_fanout_lag_seconds" in q["fanout_lag_max_s"]
    assert q["events_per_s"].endswith("/ 60")


@pytest.mark.parametrize(
    ("body", "want"),
    [
        ('{"status":"success","data":{"result":[{"value":[1,"0.25"]}]}}', 0.25),
        ('{"status":"success","data":{"result":[]}}', None),
        ('{"status":"success","data":{"result":[{"value":[1,"NaN"]}]}}', None),
        ('{"status":"error"}', None),
    ],
)
def test_prom_value(body: str, want: float | None) -> None:
    assert cr.prom_value(body) == want


def test_parse_mem() -> None:
    assert cr.parse_mem("81.5MiB / 7.6GiB") == int(81.5 * (1 << 20))
    assert cr.parse_mem("1.2GiB / 8GiB") == int(1.2 * (1 << 30))
    assert cr.parse_mem("512B / 1GiB") == 512
    with pytest.raises(ValueError, match="no memory value"):
        cr.parse_mem("-- / --")


def test_memory_sampler_peak_per_service_and_window() -> None:
    run = Recorder(
        {
            "docker stats": (
                0,
                "p-api-1\t100MiB / 8GiB\t250.5%\np-clamav-1\t1GiB / 8GiB\t0.1%\n"
                "other\t1B / 1B\t1%\np-api-1\t-- / --\t--\n",
            )
        }
    )
    s = cr.MemorySampler({"p-api-1": "api", "p-clamav-1": "clamav"}, run)
    s.sample()
    assert run.calls[0][:3] == ["docker", "stats", "--no-stream"]
    s.samples.append((s.samples[0][0] + 100, "api", 300 << 20, 390.0))
    t0 = s.samples[0][0]
    assert s.peak("api", t0, t0 + 10) == 100 << 20
    assert s.peak("api", t0, t0 + 200) == 300 << 20
    assert s.peak("clamav", 0, t0 + 1) == 1 << 30
    assert s.peak("worker", 0, t0 + 1) is None
    assert s.cpu_peak("api", t0, t0 + 10) == 250.5
    assert s.cpu_peak("api", t0, t0 + 200) == 390.0
    assert s.cpu_peak("worker", 0, t0 + 1) is None


def test_memory_sampler_start_and_stop() -> None:
    s = cr.MemorySampler({}, Recorder())
    s.start()
    s.stop()
    assert s.samples == []


def test_parse_sizes_counts_the_partitions_of_event_log() -> None:
    rows = "messages|4096000|1024000\nevent_log|0|0\nusers|8192|4096\n"
    tables = cr.parse_sizes(rows, "9000000|3000000\n")
    assert tables[0] == cr.TableSize("event_log", 9000000, 3000000)
    assert [t.name for t in tables] == ["event_log", "messages", "users"]


def test_parse_means() -> None:
    m = cr.parse_means("1000|512.5|3000|300.25|50000000|on|on|18.1\n")
    assert m.messages == 1000
    assert m.event_payload_bytes == 300.25
    assert (m.fsync, m.synchronous_commit, m.postgres_version) == ("on", "on", "18.1")


# ---- the report --------------------------------------------------------------------------------


def _step(index: int, rate: float, missed: list[str] | None = None) -> dict[str, Any]:
    return {
        "index": index,
        "rate": rate,
        "connections": int(rate * 2),
        "window_from": "2026-10-26T12:00:15Z",
        "window_to": "2026-10-26T12:01:15Z",
        "sends": {"goodput_per_s": rate * 0.99, "p99_ns": 80e6},
        "fanout": {"p99_ns": 120e6},
        "missed": missed,
    }


RESULT = {
    "seed": 20261026,
    "params": {"group_size": 20},
    "targets": {"message_create_p99_ms": 150},
    "steps": [_step(0, 100), _step(1, 200), _step(2, 300, ["perf.message_create_p99: p99 x"])],
    "stopped": "step 2 missed a target",
}

CONDITIONS = {
    "commit": "abc",
    "date": "2026-10-26",
    "release": "v0.1.0",
    "profile": "4 vCPU, 8 GiB RAM",
    "mode": "emulated",
    "runtime_cpus": 8,
    "runtime_memory": 14 << 30,
    "cpus": 4,
    "memory": 8 << 30,
    "layout": "api and worker",
    "postgres_version": "18.1",
    "fsync": "on",
    "synchronous_commit": "on",
    "seed": 20261026,
}


def _report() -> dict[str, Any]:
    metrics = [
        cr.StepMetrics(
            1,
            {
                "events_per_s": 410.0,
                "lock_wait_p99_s": 0.004,
                "lock_wait_mean_s": 0.0005,
                "backends_max": 23.0,
                "pool_acquired_max": 12.0,
                "gateway_connections_max": 400.0,
                "fanout_lag_max_s": 0.05,
            },
            {"api": 200 << 20, "worker": 100 << 20, "postgres": 1 << 30},
        )
    ]
    tables = [
        cr.TableSize("event_log", 6_000_000, 2_000_000),
        cr.TableSize("messages", 4_000_000, 1_000_000),
    ]
    means = cr.parse_means("10000|500|30000|250|20000000|on|on|18.1")
    return cr.build_report(RESULT, metrics, tables, means, 900 << 20, CONDITIONS)


def test_capacity_is_the_last_held_and_the_first_missed_step() -> None:
    last, first = cr.capacity(RESULT)
    assert last is not None
    assert last["index"] == 1
    assert first is not None
    assert first["index"] == 2
    held = {"steps": [_step(0, 100)]}
    assert cr.capacity(held)[1] is None


def test_report_has_every_row_of_16_13() -> None:
    chapter = (ROOT / "docs" / "spec" / "16-operations.md").read_text(encoding="utf-8")
    section = chapter.split("## 13. Capacity and resources", 1)[1].split("\n## ", 1)[0]
    bottlenecks = [
        line.split("|")[1].strip().replace("`", "")
        for line in section.splitlines()
        if line.startswith("| ") and not line.startswith("| Bottleneck") and "---" not in line
    ]
    rep = _report()
    got = [r["bottleneck"] for r in rep["rows"]]
    assert len(bottlenecks) == len(got)
    for want in got:
        assert any(b.startswith(want) for b in bottlenecks), want
    lock = rep["rows"][0]["held"]
    assert lock == "p99 4.0 ms, mean 0.50 ms at 410.0 events/s"
    # The first missed step has no server metrics in this fixture.
    assert rep["rows"][0]["missed"] == "p99 n/a, mean n/a at n/a"
    assert rep["rows"][3]["missed"].endswith("commit to dispatch p99 120.0 ms")
    assert rep["throughput"]["last_held_step"] == 1
    assert rep["throughput"]["sustained_messages_per_s"] == pytest.approx(198)
    assert rep["throughput"]["first_missed_step"] == 2
    assert rep["disk"]["index_share"] == pytest.approx(0.15)
    assert rep["disk"]["message_bytes_with_indexes"] == 400.0
    assert rep["disk"]["event_bytes_with_indexes"] == 200.0
    assert rep["rows"][4]["held"] == "peak 900 MiB over the run"


def test_report_without_metrics_says_not_available() -> None:
    means = cr.parse_means("0|0|0|0|0|on|on|18.1")
    rep = cr.build_report({"steps": [_step(0, 100, ["x"])]}, [], [], means, None, CONDITIONS)
    assert rep["throughput"]["sustained_messages_per_s"] is None
    assert rep["rows"][0]["held"] is None
    assert "n/a" in rep["rows"][0]["missed"]
    assert rep["disk"]["index_share"] is None


def test_render_names_profile_commit_and_every_step() -> None:
    text = cr.render(_report())
    assert "Commit abc, 2026-10-26, release v0.1.0" in text
    assert "mode emulated" in text
    assert "First missed step 2" in text
    assert "| Last held step (200 messages/s) | First missed step (300 messages/s) |" in text
    assert text.count("\n| 2 | 300 |") == 1
    assert "`bothy.db.event_seq.lock_wait`" in text
    assert chr(0x2013) not in text
    assert chr(0x2014) not in text


def test_write_report(tmp_path: Path) -> None:
    rep = _report()
    cr.write_report(tmp_path / "r", rep, RESULT)
    data = json.loads((tmp_path / "r" / "capacity-report.json").read_text(encoding="utf-8"))
    assert data["result"]["seed"] == 20261026
    assert (tmp_path / "r" / "capacity-report.md").read_text(encoding="utf-8").startswith("# ")


# ---- the run -----------------------------------------------------------------------------------


def test_go_arch_and_runtime() -> None:
    assert cr.go_arch("aarch64") == "arm64"
    assert cr.go_arch("x86_64") == "amd64"
    run = Recorder({"docker info": (0, "8 15000000000 aarch64\n")})
    assert cr.runtime(run) == (8, 15000000000, "arm64")


def test_build_driver_failure_names_the_step(tmp_path: Path) -> None:
    run = Recorder({"go build": (1, "")})
    with pytest.raises(pr.StepError, match="building bothy-load failed"):
        cr.build_driver(run, tmp_path / "bothy-load", "arm64")
    assert "GOARCH" not in run.calls[0]  # the architecture goes into the environment


def test_main_rejects_an_invalid_interface(capsys: pytest.CaptureFixture[str]) -> None:
    assert cr.main({"PORTABILITY_TAG": "latest"}, Recorder()) == 2
    assert "PORTABILITY_TAG" in capsys.readouterr().err


def test_commit_of_the_checkout() -> None:
    assert cr.commit(Recorder({"git": (0, "0123456789ab\n")})) == "0123456789ab"
    assert cr.commit(Recorder({"git": (128, "")})) == "unknown"


def _installation(tmp_path: Path, run: Recorder) -> pr.Installation:
    inst = pr.Installation(tmp_path, "fs", "bothy-load-test", {}, run)
    inst.compose_dir.mkdir(parents=True)
    (inst.compose_dir / ".env").write_text("COMPOSE_PROFILES=direct,scan\n", encoding="utf-8")
    return inst


def test_metrics_overlay_starts_prometheus_and_the_collector_only(tmp_path: Path) -> None:
    run = Recorder()
    inst = _installation(tmp_path, run)
    cr.metrics_overlay(inst)
    assert inst.profiles == "direct,scan,observability"
    assert inst.env["BOTHY_OTEL_EXPORTER"] == "local"
    up = run.calls[-1]
    assert up[:3] == ["docker", "compose", "up"]
    assert up[-2:] == ["prometheus", "otel-collector"]
    assert "loki" not in up
    assert "tempo" not in up


def test_apply_writes_the_override_and_restarts_the_servers(tmp_path: Path) -> None:
    run = Recorder()
    inst = _installation(tmp_path, run)
    path = tmp_path / "override.json"
    cr.apply(inst, path, cr.override(False, 4, 8 << 30, "127.0.0.1/32"))
    assert json.loads(path.read_text(encoding="utf-8"))["services"]["api"]["cpus"] == 4
    assert run.calls[-1][-3:] == ["postgres", "api", "worker"]


def test_queries_of_the_installation(tmp_path: Path) -> None:
    body = '{"status":"success","data":{"result":[{"value":[1,"3"]}]}}'
    run = Recorder(
        {
            "docker compose exec -T prometheus": (0, body),
            "docker compose exec -T postgres": (0, "a|1|0\n"),
            "docker compose ps": (
                0,
                '{"Name":"p-api-1","Service":"api"}\n{"Name":"p-x","Service":"x"}',
            ),
        }
    )
    inst = _installation(tmp_path, run)
    assert cr.prometheus(inst, "up", 1.5) == 3.0
    assert "time=1.500" in run.calls[-1][-1]
    assert cr.sql(inst, "SELECT 1") == "a|1|0\n"
    assert cr.containers(inst) == {"p-api-1": "api"}
    result = {"steps": [_step(0, 100)]}
    sampler = cr.MemorySampler({}, run)
    metrics = cr.step_metrics(inst, result, sampler)
    assert metrics[0].prom["events_per_s"] == 3.0
    assert metrics[0].memory == {"api": None, "worker": None, "postgres": None, "driver": None}


# ---- the orchestration with fakes --------------------------------------------------------------

VALUES = cr.Values("4 vCPU, 8 GiB RAM", 4, 8 << 30, "150ms", "200ms", 60)


def _ctx(tmp_path: Path, run: Recorder, environ: dict[str, str] | None = None) -> pr.Context:
    iface = pr.Interface("v0.0.0", "0.0.0", "release-download", tmp_path)
    release = pr.Release(
        tmp_path / "install.sh",
        tmp_path / "b.tgz",
        "127.0.0.1:5000",
        "0.0.0",
        ("bothy-server:0.0.0",),
    )
    return pr.Context(iface, tmp_path, release, "", None, environ or {}, run)


def test_install_stack_prepares_the_driver(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    run = Recorder(
        {
            "docker network inspect": (0, "172.30.0.0/16 \n"),
            "docker compose port api 8080": (0, "127.0.0.1:34567\n"),
        }
    )
    verified = [pr.Verified("127.0.0.1:5000/bothy-server:0.0.0", "sha256:" + "0" * 64, "id")]
    monkeypatch.setattr(pr, "verify_images", lambda *a, **k: verified)
    monkeypatch.setattr(pr, "install", lambda ctx, inst: "en_CODE")
    inst = _installation(tmp_path / "fs", run)
    out = tmp_path / "out"
    out.mkdir()
    stack = cr.Stack(inst, out, tmp_path / "override.json", rates="20,40")
    cr.install_stack(_ctx(tmp_path, run), stack, VALUES, "arm64")
    assert (out / "setup.out").read_text(encoding="utf-8") == "en_CODE\n"
    assert stack.trusted == "127.0.0.1/32,::1/128,172.30.0.0/16"
    assert inst.env["COMPOSE_FILE"] == f"compose.yaml:{stack.override}"
    assert inst.env["BOTHY_HTTP_PUBLISH_PORT"] == "34567"
    seed = json.loads(stack.override.read_text(encoding="utf-8"))
    assert "BOTHY_RATELIMIT__RL__ADMIN__PER_MINUTE" in seed["services"]["api"]["environment"]
    d = stack.driver
    assert d[d.index("--base-url") + 1] == "http://127.0.0.1:34567"
    assert d[d.index("--entrypoint") + 2] == "127.0.0.1:5000/bothy-server:0.0.0"
    assert d[-2:] == ["--rates", "20,40"]
    assert d[d.index("--name") + 1] == "bothy-load-test-driver"


def test_drive_seeds_restarts_and_reads_the_result(tmp_path: Path) -> None:
    run = Recorder()
    inst = _installation(tmp_path / "fs", run)
    out = tmp_path / "out"
    out.mkdir()
    script = (
        f'touch "{out}/seeded"; while [ ! -f "{out}/measure" ]; do sleep 0.05; done; '
        f'echo \'{{"seed": 1, "steps": []}}\' > "{out}/result.json"'
    )
    stack = cr.Stack(inst, out, tmp_path / "override.json", trusted="127.0.0.1/32")
    stack.driver = ["sh", "-c", script]
    result, sampler = cr.drive(stack, VALUES, tmp_path / "driver.log")
    assert result == {"seed": 1, "steps": []}
    measured = json.loads(stack.override.read_text(encoding="utf-8"))
    assert not [k for k in measured["services"]["api"]["environment"] if "RATELIMIT" in k]
    assert sampler.containers == {"bothy-load-test-driver": "driver"}


def test_drive_fails_when_the_driver_fails(tmp_path: Path) -> None:
    inst = _installation(tmp_path / "fs", Recorder())
    out = tmp_path / "out"
    out.mkdir()
    stack = cr.Stack(inst, out, tmp_path / "override.json")
    stack.driver = ["sh", "-c", f'touch "{out}/seeded"; exit 3']
    with pytest.raises(pr.StepError, match="exit code 3"):
        cr.drive(stack, VALUES, tmp_path / "driver.log")


def _fake_run(monkeypatch: pytest.MonkeyPatch, fail: bool = False) -> None:
    def install_stack(ctx: Any, stack: Any, values: Any, goarch: str) -> None:
        if fail:
            raise pr.StepError("install.sh ended with exit code 1")

    monkeypatch.setattr(cr, "install_stack", install_stack)
    monkeypatch.setattr(cr, "drive", lambda stack, values, log: (RESULT, cr.MemorySampler({})))
    monkeypatch.setattr(cr, "step_metrics", lambda inst, result, sampler: [])
    answers = {
        cr.SIZES_SQL: "messages|4000|1000\n",
        cr.EVENT_LOG_SQL: "0|0\n",
        cr.MEANS_SQL: "10|500|30|250|20000|on|on|18.1\n",
    }
    monkeypatch.setattr(cr, "sql", lambda inst, q: answers[q])


def test_measure_writes_the_report_and_cleans_up(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    _fake_run(monkeypatch)
    run = Recorder({"docker info": (0, "8 15000000000 aarch64\n"), "git": (0, "abc\n")})
    report = cr.measure(_ctx(tmp_path, run), VALUES, tmp_path, tmp_path / "report")
    assert report["conditions"]["mode"] == "emulated"
    assert report["conditions"]["commit"] == "abc"
    assert (tmp_path / "report" / "capacity-report.md").is_file()
    # The cleanup looks for leftovers of the project (install.sh never ran here).
    assert any("label=com.docker.compose.project=" in " ".join(c) for c in run.calls)


def test_measure_diagnoses_and_cleans_up_after_a_failure(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    _fake_run(monkeypatch, fail=True)
    run = Recorder({"docker info": (0, "8 15000000000 aarch64\n")})
    with pytest.raises(pr.StepError, match=r"install\.sh"):
        cr.measure(_ctx(tmp_path, run), VALUES, tmp_path, tmp_path / "report")
    assert "load: FAILED" in capsys.readouterr().err
    assert any("label=com.docker.compose.project=" in " ".join(c) for c in run.calls)


def test_main_runs_and_fails(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    rel = tmp_path / "rel"
    rel.mkdir()
    (rel / "install.sh").write_text("", encoding="utf-8")
    (rel / "bothy-compose-0.0.0.tar.gz").write_text("", encoding="utf-8")
    env = {
        "PORTABILITY_TAG": "v0.0.0",
        "PORTABILITY_INSTALL_SOURCE": "release-download",
        "PORTABILITY_RELEASE_DIR": str(rel),
        "LOAD_REPORT_DIR": str(tmp_path / "report"),
    }
    monkeypatch.setattr(pr, "preflight", lambda run: None)
    monkeypatch.setattr(pr, "fetch_release", lambda iface, workdir: None)
    monkeypatch.setattr(cr.shutil, "which", lambda name: "/usr/bin/go")
    monkeypatch.setattr(
        cr, "measure", lambda *a: {"throughput": {"sustained_messages_per_s": 300.0}}
    )
    assert cr.main(env, Recorder()) == 0

    def broken(*a: Any) -> dict[str, Any]:
        raise pr.StepError("bothy-load ended with exit code 1")

    monkeypatch.setattr(cr, "measure", broken)
    assert cr.main(env, Recorder()) == 1
    monkeypatch.setattr(cr.shutil, "which", lambda name: None)
    assert cr.main(env, Recorder()) == 2
