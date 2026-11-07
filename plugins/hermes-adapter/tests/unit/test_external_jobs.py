# SPDX-License-Identifier: Apache-2.0
"""The registry of external jobs (09 17.12 point 8; FIX-hermes-external-jobs).

A process a tool call started outside Hermes' registries (a detached supervisor script) registers
itself with its thread, PID and process start time; the adapter reads the records, tells a live
job from a gone one (also when the PID was reused), and stops a job's process group only after it
checked PID and start time. The folder and the records belong to the adapter's user alone.
"""

from __future__ import annotations

import dataclasses
import json
import os
import signal
import stat
import subprocess
import sys
import time
from pathlib import Path
from uuid import uuid4

import pytest
from jobs_kit import kill_quietly, spawn_detached, wait_gone

from bothy_hermes_adapter import jobs
from bothy_hermes_adapter.jobs import JobError, JobRegistry

SRC = Path(jobs.__file__).resolve().parents[1]
SDK = SRC.parents[2] / "sdk" / "python" / "src"


@pytest.fixture
def home(tmp_path: Path) -> Path:
    """A profile home with the adapter's state directory (as the adapter creates it)."""
    profile = tmp_path / "profile"
    (profile / "bothy").mkdir(mode=0o700, parents=True)
    return profile


@pytest.fixture
def registry(home: Path) -> JobRegistry:
    reg = JobRegistry(jobs.jobs_dir(home, "bothy"))
    reg.prepare()
    return reg


def _sleeper(**kwargs: object) -> subprocess.Popen[bytes]:
    return subprocess.Popen(
        [sys.executable, "-c", "import time; time.sleep(600)"],
        **kwargs,  # type: ignore[call-overload]
    )


def test_register_writes_a_private_record_that_reads_back(registry: JobRegistry) -> None:
    thread = uuid4()
    record = registry.register(thread, os.getpid(), "Nightly import", log_path="/x/run.log")
    directory = registry.directory
    assert stat.S_IMODE(directory.stat().st_mode) == 0o700
    path = directory / f"{record.job_id}.json"
    assert stat.S_IMODE(path.stat().st_mode) == 0o600
    assert record.pid_start == jobs.process_start(os.getpid())
    [read] = registry.records()
    assert read == record
    assert (read.thread_id, read.pid, read.title, read.log_path) == (
        thread,
        os.getpid(),
        "Nightly import",
        "/x/run.log",
    )
    assert abs(read.started_at - time.time()) < 60
    assert registry.alive(read) is True
    assert "Nightly import" not in repr(read)
    assert registry.deregister(record.job_id) is True
    assert registry.records() == []
    assert registry.deregister(record.job_id) is False


def test_register_refuses_a_gone_process_and_bad_input(registry: JobRegistry) -> None:
    gone = subprocess.Popen([sys.executable, "-c", "pass"])
    gone.wait()
    with pytest.raises(JobError, match="not running"):
        registry.register(uuid4(), gone.pid, "x")
    for pid in (0, 1, -5):
        with pytest.raises(JobError):
            registry.register(uuid4(), pid, "x")
    with pytest.raises(JobError, match="title"):
        registry.register(uuid4(), os.getpid(), "  ")
    assert registry.records() == []


def test_register_needs_the_adapters_state_directory(tmp_path: Path) -> None:
    reg = JobRegistry(jobs.jobs_dir(tmp_path / "nowhere", "bothy"))
    with pytest.raises(JobError, match="state directory"):
        reg.register(uuid4(), os.getpid(), "x")
    assert not (tmp_path / "nowhere").exists()


def test_a_folder_others_can_read_is_refused(home: Path) -> None:
    folder = jobs.jobs_dir(home, "bothy")
    folder.mkdir(mode=0o755)
    folder.chmod(0o755)
    reg = JobRegistry(folder)
    with pytest.raises(JobError, match="permissions"):
        reg.register(uuid4(), os.getpid(), "x")
    with pytest.raises(JobError, match="permissions"):
        reg.records()


def test_a_linked_folder_is_refused(home: Path, tmp_path: Path) -> None:
    elsewhere = tmp_path / "elsewhere"
    elsewhere.mkdir(mode=0o700)
    jobs.jobs_dir(home, "bothy").symlink_to(elsewhere)
    reg = JobRegistry(jobs.jobs_dir(home, "bothy"))
    with pytest.raises(JobError, match="link"):
        reg.register(uuid4(), os.getpid(), "x")
    with pytest.raises(JobError, match="link"):
        reg.prepare()
    assert list(elsewhere.iterdir()) == []


def test_a_folder_of_another_owner_is_refused(
    registry: JobRegistry, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(jobs, "_owned", lambda st: False)
    with pytest.raises(JobError, match="owner"):
        registry.register(uuid4(), os.getpid(), "x")
    with pytest.raises(JobError, match="owner"):
        registry.records()


def test_records_skip_links_open_modes_foreign_owners_and_garbage(
    registry: JobRegistry, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    good = registry.register(uuid4(), os.getpid(), "good")
    folder = registry.directory
    payload = json.loads((folder / f"{good.job_id}.json").read_text())
    # A link to a record elsewhere.
    target = tmp_path / "planted.json"
    target.write_text(json.dumps(payload | {"job_id": "a" * 32}))
    target.chmod(0o600)
    (folder / f"{'a' * 32}.json").symlink_to(target)
    # A record others can read.
    loose = folder / f"{'b' * 32}.json"
    loose.write_text(json.dumps(payload | {"job_id": "b" * 32}))
    loose.chmod(0o644)
    # Garbage, a wrong type, a name that is no job ID, and an oversized file.
    for name, text in (
        ("c" * 32, "{not json"),
        ("d" * 32, json.dumps(payload | {"job_id": "d" * 32, "pid": "12"})),
        ("e" * 32, json.dumps(payload | {"job_id": "f" * 32})),
        ("note", json.dumps(payload)),
        ("0" * 32, json.dumps(payload | {"job_id": "0" * 32, "title": "x" * 70000})),
    ):
        path = folder / f"{name}.json"
        path.write_text(text)
        path.chmod(0o600)
    assert [r.job_id for r in registry.records()] == [good.job_id]
    # A record of another owner in an own folder.
    monkeypatch.setattr(jobs, "_owned", lambda st: stat.S_ISDIR(st.st_mode))
    assert registry.records() == []


def test_a_reused_pid_is_no_live_job_and_is_never_signalled(registry: JobRegistry) -> None:
    other = _sleeper()
    try:
        record = registry.register(uuid4(), other.pid, "old job")
        # The job ended and the system gave its PID to another process: the start time differs.
        reused = dataclasses.replace(record, pid_start=record.pid_start + "0")
        assert registry.alive(reused) is False
        assert registry.stop(reused, grace=0.2) is False
        assert other.poll() is None
    finally:
        other.kill()
        other.wait()


def test_stop_ends_a_detached_supervisor_and_its_worker(
    registry: JobRegistry, tmp_path: Path
) -> None:
    job = spawn_detached(tmp_path)
    starts = (jobs.process_start(job.supervisor), jobs.process_start(job.worker))
    try:
        assert os.getpgid(job.supervisor) == job.supervisor
        assert os.getpgid(job.worker) == job.supervisor
        parent = subprocess.run(
            ["ps", "-o", "ppid=", "-p", str(job.supervisor)],
            capture_output=True,
            text=True,
            check=True,
        ).stdout.strip()
        assert parent == "1"  # detached: the launcher exited, init adopted the supervisor
        record = registry.register(uuid4(), job.supervisor, "long job")
        assert registry.alive(record) is True
        assert registry.stop(record, grace=5.0) is True
        assert wait_gone(job.supervisor, starts[0])
        assert wait_gone(job.worker, starts[1])
        assert registry.alive(record) is False
    finally:
        kill_quietly(job, starts)


def test_stop_escalates_when_the_job_ignores_sigterm(registry: JobRegistry) -> None:
    stubborn = subprocess.Popen(
        [
            sys.executable,
            "-c",
            "import signal, sys, time\n"
            "signal.signal(signal.SIGTERM, signal.SIG_IGN)\n"
            "print('ready', flush=True)\n"
            "time.sleep(600)",
        ],
        stdout=subprocess.PIPE,
        start_new_session=True,
    )
    try:
        assert stubborn.stdout is not None
        assert stubborn.stdout.readline().strip() == b"ready"
        record = registry.register(uuid4(), stubborn.pid, "stubborn")
        assert registry.stop(record, grace=0.3) is True
        assert stubborn.wait(timeout=10) == -signal.SIGKILL
    finally:
        if stubborn.poll() is None:
            stubborn.kill()
            stubborn.wait()
        if stubborn.stdout is not None:
            stubborn.stdout.close()


def test_stop_signals_only_the_process_when_it_leads_no_group(registry: JobRegistry) -> None:
    leader = _sleeper(process_group=0)
    member = _sleeper(process_group=leader.pid)
    try:
        assert os.getpgid(member.pid) == leader.pid
        record = registry.register(uuid4(), member.pid, "member")
        assert registry.stop(record, grace=2.0) is True
        assert member.wait(timeout=10) == -signal.SIGTERM
        time.sleep(0.2)
        assert leader.poll() is None  # the group it does not lead was left alone
    finally:
        for proc in (leader, member):
            if proc.poll() is None:
                proc.kill()
                proc.wait()


def test_stop_never_signals_the_adapters_own_process(registry: JobRegistry) -> None:
    record = registry.register(uuid4(), os.getpid(), "the gateway itself")
    assert registry.stop(record, grace=0.1) is False


def test_sweep_removes_stale_records_and_keeps_live_ones(registry: JobRegistry) -> None:
    ended = _sleeper(start_new_session=True)
    stale = registry.register(uuid4(), ended.pid, "ended before a gateway restart")
    ended.kill()
    ended.wait()
    live = registry.register(uuid4(), os.getpid(), "live")
    registry.mark_reported(stale.job_id)
    registry.mark_reported(live.job_id)
    # A new process (a gateway restart) reads the same folder.
    again = JobRegistry(registry.directory)
    assert [r.job_id for r in again.sweep()] == [live.job_id]
    assert [r.job_id for r in again.records()] == [live.job_id]
    assert again.reported(live.job_id) is True
    assert again.reported(stale.job_id) is False
    assert sorted(p.name for p in registry.directory.iterdir()) == sorted(
        [f"{live.job_id}.json", f"{live.job_id}.reported"]
    )


def _cli(*args: str, module: str = "bothy_hermes_adapter.jobs") -> subprocess.CompletedProcess[str]:
    """The command line in a fresh Python: the module alone needs no package but its own."""
    path = [str(SRC)] if module.endswith(".jobs") else [str(SRC), str(SDK)]
    env = {"PATH": "/usr/bin:/bin", "PYTHONPATH": os.pathsep.join(path)}
    return subprocess.run(
        [sys.executable, "-m", module, *args],
        capture_output=True,
        text=True,
        env=env,
        timeout=60,
        check=False,
    )


def test_command_line_registers_lists_and_deregisters(home: Path, tmp_path: Path) -> None:
    thread = uuid4()
    worker = _sleeper(start_new_session=True)
    try:
        common = ("--profile-home", str(home), "--platform-key", "bothy")
        out = _cli(
            "register",
            *common,
            "--thread",
            str(thread),
            "--pid",
            str(worker.pid),
            "--title",
            "Build the index",
            "--log",
            str(tmp_path / "run.log"),
        )
        assert out.returncode == 0, out.stderr
        result = json.loads(out.stdout)
        assert result["ok"] is True
        job_id = result["job_id"]
        listed = _cli("list", *common)
        assert listed.returncode == 0, listed.stderr
        [entry] = [json.loads(line) for line in listed.stdout.splitlines()]
        assert (entry["job_id"], entry["thread_id"], entry["alive"]) == (job_id, str(thread), True)
        # The package's own command line names the same commands.
        gone = _cli("job", "deregister", *common, "--job-id", job_id, module="bothy_hermes_adapter")
        assert gone.returncode == 0, gone.stderr
        assert json.loads(gone.stdout) == {"ok": True, "job_id": job_id}
        assert _cli("list", *common).stdout == ""
        # An operator stops a job that has no run, after the PID and start time matched.
        again = json.loads(
            _cli(
                "register",
                *common,
                "--thread",
                str(thread),
                "--pid",
                str(worker.pid),
                "--title",
                "x",
            ).stdout
        )["job_id"]
        halted = _cli("stop", *common, "--job-id", again)
        assert halted.returncode == 0, halted.stderr
        assert json.loads(halted.stdout) == {"ok": True, "job_id": again}
        assert worker.wait(timeout=10) == -signal.SIGTERM
        assert _cli("list", *common).stdout == ""
        assert _cli("stop", *common, "--job-id", again).returncode == 1
        refused = _cli("register", *common, "--thread", "not-a-uuid", "--pid", "1", "--title", "x")
        assert refused.returncode == 2
        assert json.loads(refused.stdout)["ok"] is False
    finally:
        worker.kill()
        worker.wait()
