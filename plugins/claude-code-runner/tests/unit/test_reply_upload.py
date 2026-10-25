# SPDX-License-Identifier: Apache-2.0
"""Staging, part slices and the record of answer attachments (brief M3-12 notes 5 c, 7, 10)."""

from __future__ import annotations

import base64
import hashlib
import json
import os
from pathlib import Path

from bothy_sdk.uploads import content_digest

from claude_code_runner.reply_attachments import (
    DEFAULT_DENIED_PATTERNS,
    ROOT_NAMES,
    Candidate,
    DeniedPatterns,
    Outcome,
    Rejection,
    allowed_roots,
    check_candidate,
)
from claude_code_runner.reply_upload import (
    AttachmentSetRecord,
    StagedFile,
    _FileState,
    part_slices,
    stage_files,
)

CANARY = b"outside-canary-4711"
RUN = "0192d3a0-0000-7000-8000-00000000b001"


def _candidate(tmp_path: Path, name: str, data: bytes) -> tuple[Candidate, Path]:
    project = tmp_path / "projects" / "website"
    project.mkdir(parents=True, exist_ok=True)
    (project / name).write_bytes(data)
    roots = allowed_roots(
        ROOT_NAMES,
        project_dir=project,
        thread_inbox=tmp_path / "state" / "files" / "t",
        run_temp=tmp_path / "state" / "tmp" / RUN,
        state_dir=tmp_path / "state",
        projects_root=tmp_path / "projects",
    )
    result = check_candidate(name, project, roots, DeniedPatterns.of(DEFAULT_DENIED_PATTERNS))
    assert isinstance(result, Candidate)
    return result, project / name


def test_stage_files_detects_swap(tmp_path: Path) -> None:
    outside = tmp_path / "outside.txt"
    outside.write_bytes(CANARY)
    swapped, path = _candidate(tmp_path, "report.md", b"report")
    kept, _ = _candidate(tmp_path, "notes.md", b"notes")
    path.unlink()
    path.symlink_to(outside)  # swapped for a link between the check and the open
    target = tmp_path / "state" / "outgoing" / RUN / "1"
    result = stage_files([kept, swapped], 1000, target)
    assert isinstance(result, tuple)
    assert [o.rejection.code for o in result if o.rejection] == [
        "set_not_sent",
        "changed_during_check",
    ]
    assert not target.exists()  # nothing staged
    # Replaced by another regular file (another inode).
    other, other_path = _candidate(tmp_path, "other.md", b"other")
    other_path.unlink()
    other_path.write_bytes(b"other")
    result = stage_files([other], 1000, target)
    assert isinstance(result, tuple)
    # Grown beyond its checked size, or beyond the server's limit.
    grown, grown_path = _candidate(tmp_path, "grown.md", b"12345")
    with grown_path.open("ab") as fh:
        fh.write(b"678")
    assert isinstance(stage_files([grown], 1000, target), tuple)
    big, _ = _candidate(tmp_path, "big.md", b"x" * 20)
    assert isinstance(stage_files([big], 10, target), tuple)
    # A second link added after the check.
    linked, linked_path = _candidate(tmp_path, "linked.md", b"linked")
    os.link(linked_path, tmp_path / "second-name")
    assert isinstance(stage_files([linked], 1000, target), tuple)


def test_stage_files_copy(tmp_path: Path) -> None:
    data = b"line one\nline two\n"
    candidate, path = _candidate(tmp_path, "report.md", data)
    target = tmp_path / "state" / "outgoing" / RUN / "1"
    staged = stage_files([candidate], 1000, target)
    assert isinstance(staged, list)
    (copy,) = staged
    assert copy == StagedFile(
        "report.md", "report.md", target / "1", len(data), hashlib.sha256(data).hexdigest()
    )
    assert copy.path.read_bytes() == data
    assert copy.path.stat().st_mode & 0o777 == 0o600
    assert target.stat().st_mode & 0o777 == 0o700
    assert (tmp_path / "state" / "outgoing").stat().st_mode & 0o777 == 0o700
    path.write_bytes(b"changed afterwards")
    assert copy.path.read_bytes() == data


def test_part_slices() -> None:
    assert part_slices(30, 10) == [(1, 0, 10), (2, 10, 10), (3, 20, 10)]
    assert part_slices(29, 10) == [(1, 0, 10), (2, 10, 10), (3, 20, 9)]
    assert part_slices(31, 10) == [(1, 0, 10), (2, 10, 10), (3, 20, 10), (4, 30, 1)]
    assert part_slices(1, 10) == [(1, 0, 1)]
    data = b"abc"
    expected = "sha-256=:" + base64.b64encode(hashlib.sha256(data).digest()).decode() + ":"
    assert content_digest(data) == expected


def test_record_roundtrip(tmp_path: Path) -> None:
    path = AttachmentSetRecord.path_for(tmp_path / "state", "ref", RUN, 2)
    assert path == tmp_path / "state" / "sessions" / "ref" / f"{RUN}.attach-2.json"
    record = AttachmentSetRecord(path=path, answer=2, state="staged", limit=3, client_msg_id="m")
    record.files = [
        _FileState(
            announced="out/report.md",
            name="report.md",
            staged=str(tmp_path / "staged"),
            size=5,
            sha256="ab" * 32,
            create_key="k1",
            complete_key="k2",
            file_id="f1",
            part_size=2,
            plan=[{"part_number": 1, "headers": {"Content-Type": "application/octet-stream"}}],
            etags={"1": '"e1"'},
            state="created",
        )
    ]
    record.save()
    assert path.stat().st_mode & 0o777 == 0o600
    assert path.parent.stat().st_mode & 0o777 == 0o700
    assert not [p for p in path.parent.iterdir() if p.name.startswith(".")]  # atomic, no leftovers
    loaded = AttachmentSetRecord.load(path)
    assert loaded is not None
    assert loaded.files == record.files
    assert (loaded.answer, loaded.state, loaded.limit, loaded.client_msg_id) == (
        2,
        "staged",
        3,
        "m",
    )
    loaded.set_outcomes(
        [Outcome("out/report.md", Rejection("upload_refused", "conflict", "quota_exceeded"))]
    )
    loaded.fallback_msg_id = "fb"
    loaded.save()
    again = AttachmentSetRecord.load(path)
    assert again is not None
    assert again.state == "failed"
    assert again.notice() == (
        "Attachments not sent:\n- `report.md`: upload_refused (conflict, quota_exceeded)"
    )
    raw = path.read_text()
    doc = json.loads(raw)
    assert set(doc) == {
        "answer",
        "state",
        "files",
        "outcomes",
        "limit",
        "client_msg_id",
        "fallback_msg_id",
    }
    for word in ("token", "content", "text"):
        assert word not in raw
    assert AttachmentSetRecord.load(tmp_path / "missing.json") is None
    (tmp_path / "broken.json").write_text("{")
    assert AttachmentSetRecord.load(tmp_path / "broken.json") is None
