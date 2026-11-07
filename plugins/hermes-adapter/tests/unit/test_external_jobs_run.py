# SPDX-License-Identifier: Apache-2.0
"""A run holds the external jobs its tool calls registered (09 17.12 point 8).

A detached supervisor a tool call started is not in Hermes' registries; it registers itself
(`bothy_hermes_adapter.jobs`). The run stays running with a ``background_job`` step while the
job lives, ends when the job ends, and stop or an early end stops the job's process group after
checking PID and start time. A job without an open run is posted once, never adopted silently.
The processes here are real: a launcher starts a supervisor in a new session and exits.
"""

from __future__ import annotations

import os
import signal
import subprocess
import sys
import time
from pathlib import Path
from typing import Any
from uuid import UUID, uuid4

import pytest
from adapter_kit import FakeRuntime, Rig, rig
from jobs_kit import Detached, kill_quietly, spawn_detached, wait_gone

from bothy_hermes_adapter import jobs
from bothy_hermes_adapter.handler import EXTERNAL_JOB_STEP
from bothy_hermes_adapter.jobs import JobRegistry
from bothy_hermes_adapter.runtime import TurnOutcome, TurnRequest

pytestmark = pytest.mark.anyio

FAST: dict[str, Any] = {
    "background_poll_interval": 0.02,
    "background_settle": 0.3,
    "background_progress_interval": 0.2,
}
TITLE = "Index the archive"


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


@pytest.fixture
def registry(tmp_path: Path) -> JobRegistry:
    home = tmp_path / "profile"
    (home / "bothy").mkdir(mode=0o700, parents=True)
    reg = JobRegistry(jobs.jobs_dir(home, "bothy"))
    reg.prepare()
    return reg


class Jobs:
    """The detached jobs a test started, killed at the end whatever happened."""

    def __init__(self, tmp: Path) -> None:
        self.tmp = tmp
        self.started: list[tuple[Detached, tuple[str | None, str | None]]] = []

    def spawn(self, on_term: str = "exit") -> Detached:
        job = spawn_detached(self.tmp, on_term)
        self.started.append(
            (job, (jobs.process_start(job.supervisor), jobs.process_start(job.worker)))
        )
        return job

    def starts(self, job: Detached) -> tuple[str | None, str | None]:
        return next(s for j, s in self.started if j == job)

    def cleanup(self) -> None:
        for job, starts in self.started:
            kill_quietly(job, starts)


@pytest.fixture
def detached(tmp_path: Path) -> Any:
    started = Jobs(tmp_path)
    yield started
    started.cleanup()


def _steps(r: Rig, run_id: UUID, kind: str) -> list[dict[str, Any]]:
    return [s for s in r.fake.run(run_id)["steps"].values() if s["kind"] == kind]


def _metric_patches(r: Rig, run_id: UUID, name: str) -> list[Any]:
    values = []
    for request in r.requests("patchRun"):
        if request.path.endswith(str(run_id)):
            values += [
                op["value"] for op in request.json["ops"] if op["path"] == f"/metrics/{name}"
            ]
    return values


def _registering(registry: JobRegistry, detached: Jobs, box: list[Any]) -> Any:
    """A first turn whose tool call starts a detached job that registers itself."""

    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        if len(rt.turns) == 1:
            job = detached.spawn()
            box.append((job, registry.register(turn.thread_id, job.supervisor, TITLE)))
            await rt.events.on_answer(turn.thread_id, "Started the job.", str(turn.message_id))
            return TurnOutcome.COMPLETED
        await rt.events.on_answer(turn.thread_id, "ok", None)
        return TurnOutcome.COMPLETED

    return script


async def test_a_registered_job_holds_the_run_until_it_ends(
    tmp_path: Path, registry: JobRegistry, detached: Jobs
) -> None:
    box: list[Any] = []
    rt = FakeRuntime(script=_registering(registry, detached, box))
    async with rig(tmp_path, runtime=rt, jobs=registry, **FAST) as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: _steps(r, triggered.run_id, "background_job"), "the job step")
        await r.until(lambda: r.fake.messages, "the first answer")

        def progressed() -> bool:
            return int(r.fake.run(triggered.run_id)["metrics"].get("progress_events", 0)) >= 2

        await r.until(progressed, "two progress events")
        assert r.fake.run_state(triggered.run_id).state == "running"
        [step] = _steps(r, triggered.run_id, "background_job")
        assert (step["name"], step["title"], step["status"]) == (
            EXTERNAL_JOB_STEP,
            TITLE,
            "running",
        )
        assert r.fake.run(triggered.run_id)["metrics"].get("background_jobs") == 1
        assert r.handler.tool_allowed(triggered.thread_id) is True
        # The job is covered by its run: nothing is posted about it.
        await r.handler.sweep_jobs()
        assert len(r.fake.messages) == 1
        job, _record = box[0]
        os.killpg(job.supervisor, signal.SIGTERM)  # the job ends by itself
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
    assert r.fake.run_state(triggered.run_id).state == "completed"
    [step] = _steps(r, triggered.run_id, "background_job")
    assert step["status"] == "completed"
    assert step.get("finished_at")
    assert _metric_patches(r, triggered.run_id, "background_jobs") == [1, 0]
    assert [m["run_id"] for m in r.fake.messages] == [str(triggered.run_id)]


async def test_a_deregistered_job_completes_its_step(
    tmp_path: Path, registry: JobRegistry, detached: Jobs
) -> None:
    box: list[Any] = []
    rt = FakeRuntime(script=_registering(registry, detached, box))
    async with rig(tmp_path, runtime=rt, jobs=registry, **FAST) as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: _steps(r, triggered.run_id, "background_job"), "the job step")
        _job, record = box[0]
        # The supervisor removes its record before it reports (its process still runs).
        assert registry.deregister(record.job_id) is True
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
    assert r.fake.run_state(triggered.run_id).state == "completed"
    [step] = _steps(r, triggered.run_id, "background_job")
    assert step["status"] == "completed"


async def test_stop_kills_the_jobs_process_group(
    tmp_path: Path, registry: JobRegistry, detached: Jobs
) -> None:
    box: list[Any] = []
    rt = FakeRuntime(script=_registering(registry, detached, box))
    async with rig(tmp_path, runtime=rt, jobs=registry, **FAST) as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: _steps(r, triggered.run_id, "background_job"), "the job step")
        await r.fake.cancel(triggered.run_id)
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
        job, _record = box[0]
        starts = detached.starts(job)
        assert wait_gone(job.supervisor, starts[0])
        assert wait_gone(job.worker, starts[1])
    view = r.fake.run_state(triggered.run_id)
    assert (view.state, view.state_reason) == ("cancelled", "user_cancelled")


async def test_an_early_end_of_the_run_kills_the_job(
    tmp_path: Path, registry: JobRegistry, detached: Jobs
) -> None:
    box: list[Any] = []

    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        job = detached.spawn()
        box.append(job)
        registry.register(turn.thread_id, job.supervisor, TITLE)
        return TurnOutcome.FAILED

    rt = FakeRuntime(script=script)
    async with rig(tmp_path, runtime=rt, jobs=registry, **FAST) as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
        [job] = box
        starts = detached.starts(job)
        assert wait_gone(job.supervisor, starts[0])
        assert wait_gone(job.worker, starts[1])
    view = r.fake.run_state(triggered.run_id)
    assert (view.state, view.state_reason) == ("failed", "runtime_error")


def _notices(r: Rig) -> list[dict[str, Any]]:
    return [m for m in r.fake.messages if m.get("run_id") is None]


async def test_a_stopping_run_still_covers_a_job_that_ignores_sigterm(
    tmp_path: Path, registry: JobRegistry, detached: Jobs
) -> None:
    box: list[Any] = []

    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        job = detached.spawn("ignore")
        box.append(job)
        registry.register(turn.thread_id, job.supervisor, TITLE)
        await rt.events.on_answer(turn.thread_id, "Started the job.", str(turn.message_id))
        return TurnOutcome.COMPLETED

    rt = FakeRuntime(script=script)
    async with rig(tmp_path, runtime=rt, jobs=registry, **FAST) as r:
        watch = r.handler.start_job_watch()
        try:
            triggered = await r.fake.trigger()
            await r.until(lambda: _steps(r, triggered.run_id, "background_job"), "the job step")
            await r.fake.cancel(triggered.run_id)
            await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
            [job] = box
            starts = detached.starts(job)
            assert wait_gone(job.supervisor, starts[0])
            await r.handler.sweep_jobs()
        finally:
            watch.cancel()
    # The job was being stopped by its run: no notice of a job without a run.
    assert _notices(r) == []
    assert registry.records() == []


async def test_an_ending_run_still_covers_a_job_that_cleans_up_after_sigterm(
    tmp_path: Path, registry: JobRegistry, detached: Jobs
) -> None:
    box: list[Any] = []

    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        job = detached.spawn("slow")
        box.append(job)
        registry.register(turn.thread_id, job.supervisor, TITLE)
        return TurnOutcome.FAILED

    rt = FakeRuntime(script=script)
    async with rig(tmp_path, runtime=rt, jobs=registry, **FAST) as r:
        watch = r.handler.start_job_watch()
        try:
            triggered = await r.fake.trigger()
            await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
            [job] = box
            starts = detached.starts(job)
            assert wait_gone(job.supervisor, starts[0])
            await r.handler.sweep_jobs()
        finally:
            watch.cancel()
    assert _notices(r) == []
    assert registry.records() == []


async def test_a_reused_pid_neither_holds_the_run_nor_is_killed(
    tmp_path: Path, registry: JobRegistry
) -> None:
    other = subprocess.Popen(
        [sys.executable, "-c", "import time; time.sleep(600)"], start_new_session=True
    )

    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        record = registry.register(turn.thread_id, other.pid, TITLE)
        # The registered job ended and the PID now belongs to another process.
        path = registry.directory / f"{record.job_id}.json"
        path.write_text(path.read_text().replace(record.pid_start, record.pid_start + "0"))
        await rt.events.on_answer(turn.thread_id, "Started.", str(turn.message_id))
        return TurnOutcome.FAILED  # an early end stops the run's live jobs, not this one

    try:
        rt = FakeRuntime(script=script)
        async with rig(tmp_path, runtime=rt, jobs=registry, **FAST) as r:
            triggered = await r.fake.trigger()
            await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
            await r.handler.sweep_jobs()
        time.sleep(0.2)
        assert other.poll() is None
        assert registry.records() == []  # the sweep removed the record of the ended job
        assert r.fake.run(triggered.run_id)["metrics"].get("background_jobs") in (None, 0)
    finally:
        other.kill()
        other.wait()


async def test_a_stale_record_after_a_gateway_restart_is_removed_silently(
    tmp_path: Path, registry: JobRegistry
) -> None:
    ended = subprocess.Popen(
        [sys.executable, "-c", "import time; time.sleep(600)"], start_new_session=True
    )
    async with rig(tmp_path, jobs=registry, **FAST) as r:
        first = await r.fake.trigger()
        await r.until(lambda: r.finished(first.run_id), "the first run")
        record = registry.register(first.thread_id, ended.pid, TITLE)
        ended.kill()
        ended.wait()
    before = len(r.fake.messages)
    # The gateway restarted: a new adapter process reads the same folder.
    async with rig(tmp_path, jobs=JobRegistry(registry.directory), **FAST) as again:
        await again.handler.sweep_jobs()
        assert registry.records() == []
        later = await again.fake.trigger(thread_id=first.thread_id)
        await again.until(lambda: again.finished(later.run_id), "the next run")
        assert _steps(again, later.run_id, "background_job") == []
    assert len(r.fake.messages) == before
    assert not (registry.directory / f"{record.job_id}.reported").exists()


async def test_a_live_job_without_an_open_run_is_posted_once(
    tmp_path: Path, registry: JobRegistry, detached: Jobs
) -> None:
    async with rig(tmp_path, jobs=registry, **FAST) as r:
        first = await r.fake.trigger()
        await r.until(lambda: r.finished(first.run_id), "the first run")
        job = detached.spawn()
        # Registered with no run open in the thread (or its run ended at a restart).
        record = registry.register(first.thread_id, job.supervisor, TITLE)
        await r.handler.sweep_jobs()
        notices = [m for m in r.fake.messages if m.get("run_id") is None]
        assert len(notices) == 1
        assert TITLE in notices[0]["content_md"]
        created = r.requests("createMessage")[-1]
        assert created.path.endswith(f"/threads/{first.thread_id}/messages")
        await r.handler.sweep_jobs()
        # A later run of the thread does not adopt it, and its stop leaves it alone.
        later = await r.fake.trigger(thread_id=first.thread_id)
        await r.until(lambda: r.finished(later.run_id), "the later run")
        assert _steps(r, later.run_id, "background_job") == []
    async with rig(tmp_path, jobs=JobRegistry(registry.directory), **FAST) as again:
        await again.handler.sweep_jobs()  # after a restart: still posted only once
    assert len([m for m in r.fake.messages if m.get("run_id") is None]) == 1
    assert registry.alive(record) is True
    assert registry.reported(record.job_id) is True


async def test_a_job_whose_mark_fails_is_still_posted_once(
    tmp_path: Path,
    registry: JobRegistry,
    detached: Jobs,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    def full_disk(job_id: str) -> None:
        raise OSError("no space left on device")

    monkeypatch.setattr(registry, "mark_reported", full_disk)
    async with rig(tmp_path, jobs=registry, **FAST) as r:
        first = await r.fake.trigger()
        await r.until(lambda: r.finished(first.run_id), "the first run")
        registry.register(first.thread_id, detached.spawn().supervisor, TITLE)
        await r.handler.sweep_jobs()
        await r.handler.sweep_jobs()
        assert len([m for m in r.fake.messages if m.get("run_id") is None]) == 1


async def test_the_watch_posts_without_being_called(
    tmp_path: Path, registry: JobRegistry, detached: Jobs
) -> None:
    async with rig(tmp_path, jobs=registry, **FAST) as r:
        first = await r.fake.trigger()
        await r.until(lambda: r.finished(first.run_id), "the first run")
        watch = r.handler.start_job_watch()
        try:
            registry.register(first.thread_id, detached.spawn().supervisor, TITLE)
            await r.until(
                lambda: [m for m in r.fake.messages if m.get("run_id") is None], "the notice"
            )
        finally:
            watch.cancel()


async def test_an_unknown_thread_is_posted_once_and_not_retried(
    tmp_path: Path, registry: JobRegistry, detached: Jobs
) -> None:
    async with rig(tmp_path, jobs=registry, **FAST) as r:
        record = registry.register(uuid4(), detached.spawn().supervisor, TITLE)
        await r.handler.sweep_jobs()
        await r.handler.sweep_jobs()
        refused = [q for q in r.requests("createMessage") if str(record.thread_id) in q.path]
        assert len(refused) == 1
        assert registry.reported(record.job_id) is True
