# SPDX-License-Identifier: Apache-2.0
"""Staging, upload and sending of answer attachments (brief M3-12 notes 5 c, 7, 9 to 11).

`send_answer_with_attachments` is the step the answer path runs for an answer with
``ATTACH:`` lines: it reads the server's limits (``getInstance``), checks the set with
`claude_code_runner.reply_attachments`, copies the accepted files into private staging copies
(`stage_files`), uploads them one after the other through the upload session of M3-03
(`upload_set`) and sends the answer with ``attachment_file_ids``; when any step fails it sends
the answer without attachments and with the notice: all or none (09 18.10 point 4). Every
request goes through the SDK client inside the run's binding (07 SR9).

An answer above ``message_max_content_bytes`` (brief M4-47) goes the same way with its text cut
by `bothy_sdk.long_answers.split_for_limit`: the rest becomes one more staged file
``answer.txt`` after the ``ATTACH:`` files (`read_content_limit` reads the limit once the text is
longer than `LIMIT_READ_FLOOR_BYTES`), so a long answer stays one message (01 6.4).

`AttachmentSetRecord` keeps the progress of one answer's set in
``<runner.state_dir>/sessions/<session ref>/<run_id>.attach-<answer>.json`` (mode ``0600``,
written atomically): every ``Idempotency-Key`` before its request, every ``file_id`` right
after its ``201``, every confirmed part, and the ``client_msg_id`` of a fallback before it is
sent. A runner that adopts the turn after a crash resumes the set from it, so no file is
uploaded twice and the answer appears once. The record never holds a token, a file content or
the answer text.

Log lines carry codes and IDs only, never a file name, a path or a content (SEC-LOG-4).

Examples:
    >>> AttachmentSetRecord.path_for(Path("/s"), "ref", "run", 1).name
    'run.attach-1.json'
"""

from __future__ import annotations

import asyncio
import contextlib
import hashlib
import json
import logging
import os
import shutil
import stat
from collections.abc import Awaitable, Callable, Mapping, Sequence
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any
from uuid import UUID

import httpx
from bothy_sdk.client import Client
from bothy_sdk.errors import (
    InvocationTokenInvalid,
    ProblemError,
    RateLimitedError,
    RequestInProgress,
    ServerError,
    UnauthenticatedError,
)
from bothy_sdk.generated.api.admin import get_instance
from bothy_sdk.generated.api.files import (
    abort_upload,
    complete_upload,
    create_upload,
    get_upload,
)
from bothy_sdk.generated.models import (
    File,
    InstanceInfo,
    UploadComplete,
    UploadCreate,
    UploadPartResult,
    UploadSession,
    UploadState,
)
from bothy_sdk.ids import uuid7
from bothy_sdk.long_answers import OVERFLOW_NAME, split_for_limit
from bothy_sdk.uploads import upload_part

from claude_code_runner.config import RunnerConfig
from claude_code_runner.reply_attachments import (
    Candidate,
    DeniedPatterns,
    Limits,
    Outcome,
    Rejection,
    SetVerdict,
    allowed_roots,
    build_notice,
    check_candidate,
    check_set,
    with_notice,
)
from claude_code_runner.statedir import ensure_private_dir, write_private

__all__ = [
    "LIMIT_READ_FLOOR_BYTES",
    "STATUS_LINE",
    "AttachmentSetRecord",
    "ReplyContext",
    "RunEnded",
    "SendResult",
    "StagedFile",
    "outgoing_dir",
    "read_content_limit",
    "remove_run_files",
    "run_temp_dir",
    "send_answer_with_attachments",
    "stage_files",
    "upload_set",
]

_LOG = logging.getLogger("claude_code_runner")

STATUS_LINE = "uploading_attachments"
"""``/status_line`` of the run while a set uploads (09 5.3, note 7)."""

LIMIT_READ_FLOOR_BYTES = 4096
"""An answer text up to this many UTF-8 bytes is sent without reading ``getInstance`` first
(D-M4-47-6): far below the default ``message.max_content_bytes``, so a short answer costs no
request; a longer one reads the limit before it is sent."""

_COPY_CHUNK = 1024 * 1024
_FORBIDDEN = 403
_REFUSALS_403 = frozenset({"feature_disabled", "forbidden"})


def _log(event: str, **ids: object) -> None:
    _LOG.info(json.dumps({"event": event, **{k: str(v) for k, v in ids.items()}}))


def run_temp_dir(state_dir: Path, run_id: UUID | str) -> Path:
    """The run's temporary directory, the ``TMPDIR`` of its turns (note 3 c).

    Args:
        state_dir: ``runner.state_dir``.
        run_id: The run.

    Returns:
        ``<runner.state_dir>/tmp/<run_id>``.
    """
    return state_dir / "tmp" / str(run_id)


def outgoing_dir(state_dir: Path, run_id: UUID | str) -> Path:
    """The staging directory of a run's answer attachments (note 5 c).

    Args:
        state_dir: ``runner.state_dir``.
        run_id: The run.

    Returns:
        ``<runner.state_dir>/outgoing/<run_id>``.
    """
    return state_dir / "outgoing" / str(run_id)


def remove_run_files(state_dir: Path, run_id: UUID | str) -> None:
    """Remove the run's temporary and staging directories when the run ends (note 11).

    Leftovers of a crash fall to the retention sweep. Symbolic links inside are removed, never
    followed.

    Args:
        state_dir: ``runner.state_dir``.
        run_id: The run.
    """
    for directory in (run_temp_dir(state_dir, run_id), outgoing_dir(state_dir, run_id)):
        shutil.rmtree(directory, ignore_errors=True)


# A state of the run, not an error of the runner: no "Error" suffix.
class RunEnded(Exception):  # noqa: N818
    """The server ended the run while its answer was on the way (note 7 f)."""


# ---- note 5 c: staging ---------------------------------------------------------------------


@dataclass(frozen=True)
class StagedFile:
    """The private copy of one accepted file.

    Attributes:
        announced: The path of its ``ATTACH:`` line.
        name: The upload name (last component of the announced path).
        path: The staged copy.
        size: Its byte count.
        sha256: Its lowercase hex SHA-256.
    """

    announced: str
    name: str
    path: Path
    size: int
    sha256: str


def _copy(candidate: Candidate, target: Path, max_bytes: int) -> tuple[int, str] | None:
    """Copy one checked file into ``target``; ``None`` when it changed since the check."""
    flags = os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC
    try:
        fd = os.open(candidate.real_path, flags)
    except OSError:
        return None
    try:
        info = os.fstat(fd)
        if (
            not stat.S_ISREG(info.st_mode)
            or info.st_dev != candidate.device
            or info.st_ino != candidate.inode
            or info.st_nlink > 1
        ):
            return None
        digest = hashlib.sha256()
        count = 0
        out = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
        with os.fdopen(out, "wb") as sink:
            while True:
                chunk = os.read(fd, _COPY_CHUNK)
                if not chunk:
                    break
                count += len(chunk)
                if count > max_bytes or count > candidate.size:
                    return None
                digest.update(chunk)
                sink.write(chunk)
        if count != candidate.size:
            return None
        return count, digest.hexdigest()
    except OSError:
        return None
    finally:
        os.close(fd)


def stage_files(
    accepted: Sequence[Candidate], max_upload_bytes: int, directory: Path
) -> list[StagedFile] | tuple[Outcome, ...]:
    """Copy every accepted file into ``directory`` right after the checks (note 5 c).

    Each file is opened with ``O_NOFOLLOW``; its device, inode and type must equal the
    ``stat`` of the check, it must still have one link, and the copy must have exactly the
    checked size and at most ``max_upload_bytes`` bytes. The upload reads only the copy, so a
    turn that changes the file afterwards cannot change what is sent, and the SHA-256 of
    ``createUpload`` describes the bytes sent.

    Args:
        accepted: The candidates of a set without rejection, in position order.
        max_upload_bytes: ``InstanceLimits.file_max_upload_bytes``.
        directory: ``<runner.state_dir>/outgoing/<run_id>/<answer>`` (created ``0700``).

    Returns:
        The staged files in position order, or, when one file changed, the outcomes of the set
        (that file ``changed_during_check``, every other ``set_not_sent``); then nothing is
        left in ``directory``.
    """
    ensure_private_dir(directory)
    staged: list[StagedFile] = []
    for position, candidate in enumerate(accepted, start=1):
        target = directory / str(position)
        with contextlib.suppress(FileNotFoundError):
            target.unlink()  # a copy of an earlier attempt of the same answer
        copied = _copy(candidate, target, max_upload_bytes)
        if copied is None:
            shutil.rmtree(directory, ignore_errors=True)
            return tuple(
                Outcome(
                    c.announced,
                    Rejection("changed_during_check" if c is candidate else "set_not_sent"),
                )
                for c in accepted
            )
        staged.append(StagedFile(candidate.announced, candidate.name, target, *copied))
    return staged


def part_slices(size: int, part_size: int) -> list[tuple[int, int, int]]:
    """The part number, offset and length of each part of a file (12 3.2).

    Args:
        size: The file size.
        part_size: ``UploadSession.part_size_bytes``.

    Returns:
        ``(part_number, offset, length)`` per part; the last part holds the rest.

    Examples:
        >>> part_slices(25, 10)
        [(1, 0, 10), (2, 10, 10), (3, 20, 5)]
    """
    count = max(1, -(-size // part_size))
    return [
        (n, (n - 1) * part_size, min(part_size, size - (n - 1) * part_size))
        for n in range(1, count + 1)
    ]


# ---- note 10 a: the record -----------------------------------------------------------------


@dataclass
class _FileState:
    announced: str
    name: str
    staged: str
    size: int
    sha256: str
    create_key: str | None = None
    complete_key: str | None = None
    file_id: str | None = None
    part_size: int | None = None
    plan: list[dict[str, Any]] = field(default_factory=list)
    etags: dict[str, str] = field(default_factory=dict)
    state: str = "staged"


@dataclass
class AttachmentSetRecord:
    """The persisted progress of one answer's attachment set (note 10 a).

    Attributes:
        path: Where the record lives.
        answer: The index of the answer in the run, from 1.
        state: ``staged``, ``uploaded``, ``failed`` (the set will not be attached; the
            fallback is due) or ``fallback``.
        files: Per staged file its keys, ``file_id``, part plan, confirmed parts and state
            (``staged``, ``created``, ``completed``, ``failed``).
        outcomes: The outcome per announced file of a failed set (announced path, code,
            problem, reason).
        limit: ``message_max_attachments`` for the notice (``None`` when unknown).
        client_msg_id: The answer's ``client_msg_id`` (of the turn record).
        fallback_msg_id: The ``client_msg_id`` of the answer without attachments, stored before
            it is sent.
    """

    path: Path
    answer: int
    state: str
    files: list[_FileState] = field(default_factory=list)
    outcomes: list[dict[str, Any]] = field(default_factory=list)
    limit: int | None = None
    client_msg_id: str = ""
    fallback_msg_id: str | None = None

    @staticmethod
    def path_for(state_dir: Path, session_ref: str, run_id: UUID | str, answer: int) -> Path:
        """The record file of an answer.

        Args:
            state_dir: ``runner.state_dir``.
            session_ref: The Claude Code session of the run.
            run_id: The run.
            answer: The index of the answer, from 1.

        Returns:
            ``<state_dir>/sessions/<ref>/<run_id>.attach-<answer>.json``.
        """
        return state_dir / "sessions" / session_ref / f"{run_id}.attach-{answer}.json"

    def save(self) -> None:
        """Write the record atomically with mode ``0600``."""
        ensure_private_dir(self.path.parent)
        doc = {
            "answer": self.answer,
            "state": self.state,
            "files": [vars(f) for f in self.files],
            "outcomes": self.outcomes,
            "limit": self.limit,
            "client_msg_id": self.client_msg_id,
            "fallback_msg_id": self.fallback_msg_id,
        }
        write_private(self.path, json.dumps(doc), atomic=True)

    @classmethod
    def load(cls, path: Path) -> AttachmentSetRecord | None:
        """Read a record; ``None`` when there is none or it cannot be read.

        Args:
            path: The record file.

        Returns:
            The record.
        """
        try:
            doc = json.loads(path.read_text(encoding="utf-8"))
            files = [_FileState(**f) for f in doc.get("files", [])]
            limit = doc.get("limit")
            return cls(
                path=path,
                answer=int(doc["answer"]),
                state=str(doc["state"]),
                files=files,
                outcomes=list(doc.get("outcomes", [])),
                limit=limit if isinstance(limit, int) else None,
                client_msg_id=str(doc.get("client_msg_id", "")),
                fallback_msg_id=doc.get("fallback_msg_id"),
            )
        except (OSError, ValueError, KeyError, TypeError):
            return None

    def set_outcomes(self, outcomes: Sequence[Outcome]) -> None:
        """Store the outcomes of a failed set and mark it ``failed``."""
        self.state = "failed"
        self.outcomes = [
            {
                "announced": o.announced,
                "code": (o.rejection or Rejection("set_not_sent")).code,
                "problem": o.rejection.problem if o.rejection else None,
                "reason": o.rejection.reason if o.rejection else None,
            }
            for o in outcomes
        ]

    def notice(self) -> str:
        """The notice of the stored outcomes (note 8)."""
        outcomes = [
            Outcome(
                str(o["announced"]), Rejection(str(o["code"]), o.get("problem"), o.get("reason"))
            )
            for o in self.outcomes
        ]
        return build_notice(outcomes, self.limit)


# ---- the context of one answer -------------------------------------------------------------


@dataclass(frozen=True)
class SendResult:
    """What became of an answer with ``ATTACH:`` lines.

    Attributes:
        problem: ``None`` once the answer is stored on the server, else the problem code (the
            run then ends ``adapter_error`` as for any refused answer, M2-34).
        attached: How many files the stored answer carries.
        stopped: A stop came first: nothing was sent.
    """

    problem: str | None
    attached: int = 0
    stopped: bool = False


Deliver = Callable[[str, UUID, Sequence[UUID]], Awaitable[tuple[str | None, ProblemError | None]]]
"""Send an answer through the outbox: text, ``client_msg_id`` and ``attachment_file_ids``;
returns ``None`` or the problem code, and the refusal when the server refused it now."""


@dataclass
class ReplyContext:
    """What the answer path hands to `send_answer_with_attachments`.

    Attributes:
        client: The SDK client; the caller is inside the run's binding (07 SR9).
        config: The runner configuration.
        run_id: The run.
        thread_id: Its thread.
        channel_id: Its channel (``createUpload`` goes there).
        session_ref: The Claude Code session (the record lives in its directory).
        cwd: The turn's working directory (the resolved project).
        deliver: Sends the answer through the outbox.
        set_status: Sets (text) or removes (``None``) ``/status_line`` and sends it.
        stopped: Whether a stop request came.
        track: Called with the task of a running upload (``None`` after it), so a stop can
            cancel the request in flight.
        ids: Run, thread and channel IDs for log lines.
    """

    client: Client
    config: RunnerConfig
    run_id: UUID
    thread_id: UUID
    channel_id: UUID
    session_ref: str
    cwd: Path | None
    deliver: Deliver
    set_status: Callable[[str | None], Awaitable[None]]
    stopped: Callable[[], bool]
    track: Callable[[asyncio.Task[Any] | None], None]
    ids: Mapping[str, Any]


def _ended(err: ProblemError) -> bool:
    """``401``, or ``403 invocation_token_invalid`` with ``run_inactive`` (note 7 f)."""
    if isinstance(err, UnauthenticatedError):
        return True
    return isinstance(err, InvocationTokenInvalid) and err.reason == "run_inactive"


async def _limits(ctx: ReplyContext) -> Limits | None:
    """``InstanceLimits`` read once per answer (note 6 a); ``None`` when the read failed."""
    try:
        info = await ctx.client.call_as(InstanceInfo, get_instance.asyncio_detailed)
    except ProblemError as err:
        if _ended(err):
            raise RunEnded from None
        _log("attachments_limits_unavailable", code=err.code, **ctx.ids)
        return None
    except (httpx.TransportError, TypeError):
        _log("attachments_limits_unavailable", code="transport_error", **ctx.ids)
        return None
    return Limits(info.limits.message_max_attachments, info.limits.file_max_upload_bytes)


async def read_content_limit(client: Client, ids: Mapping[str, Any]) -> int | None:
    """``message_max_content_bytes`` of ``getInstance`` (M4-47); ``None`` when the read failed.

    Args:
        client: The SDK client inside the run's binding.
        ids: Run, thread and channel IDs for log lines.

    Returns:
        The limit in UTF-8 bytes, or ``None`` (the answer then goes out as it is).

    Raises:
        RunEnded: The server ended the run (note 7 f).
    """
    try:
        info = await client.call_as(InstanceInfo, get_instance.asyncio_detailed)
    except ProblemError as err:
        if _ended(err):
            raise RunEnded from None
        _log("content_limit_unavailable", code=err.code, **ids)
        return None
    except (httpx.TransportError, TypeError):
        _log("content_limit_unavailable", code="transport_error", **ids)
        return None
    return int(info.limits.message_max_content_bytes)


def _check(ctx: ReplyContext, paths: Sequence[str], limits: Limits | None) -> SetVerdict:
    cfg = ctx.config
    roots = allowed_roots(
        cfg.attach_allowed_roots,
        project_dir=ctx.cwd,
        thread_inbox=cfg.state_dir / "files" / str(ctx.thread_id),
        run_temp=run_temp_dir(cfg.state_dir, ctx.run_id),
        state_dir=cfg.state_dir,
        projects_root=cfg.projects_root,
    )
    patterns = DeniedPatterns.of(cfg.attach_denied_patterns)
    checked = [(p, check_candidate(p, ctx.cwd, roots, patterns)) for p in paths]
    return check_set(checked, limits)


def _with_overflow(
    outcomes: Sequence[Outcome], overflow: str | None, code: str = "set_not_sent"
) -> list[Outcome]:
    """The outcomes of a set that is not sent, with the rest of a long answer last (M4-47)."""
    rest = [Outcome(OVERFLOW_NAME, Rejection(code))] if overflow is not None else []
    return [*outcomes, *rest]


def _stage_overflow(
    overflow: str, directory: Path, position: int, limits: Limits
) -> _FileState | None:
    """Write the rest of a long answer as the staged file ``position`` (M4-47).

    Returns:
        Its state, or ``None`` above ``file_max_upload_bytes``.
    """
    data = overflow.encode("utf-8")
    if not data or len(data) > limits.max_upload_bytes:
        return None
    target = directory / str(position)
    with contextlib.suppress(FileNotFoundError):
        target.unlink()  # a copy of an earlier attempt of the same answer
    write_private(target, data)
    digest = hashlib.sha256(data).hexdigest()
    return _FileState(OVERFLOW_NAME, OVERFLOW_NAME, str(target), len(data), digest)


async def _prepare(
    ctx: ReplyContext,
    paths: Sequence[str],
    answer: int,
    client_msg_id: UUID,
    overflow: str | None = None,
) -> AttachmentSetRecord:
    """Read the limits, check the set and stage it (notes 5 and 6); the record is stored.

    The rest of a long answer (``overflow``, M4-47) takes the last place of the set, so the
    ``ATTACH:`` files may use one place less.
    """
    cfg = ctx.config
    path = AttachmentSetRecord.path_for(cfg.state_dir, ctx.session_ref, ctx.run_id, answer)
    limits = await _limits(ctx)
    room = limits
    if limits is not None and overflow is not None:
        room = Limits(limits.max_attachments - 1, limits.max_upload_bytes)
    verdict = _check(ctx, paths, room)
    record = AttachmentSetRecord(
        path=path,
        answer=answer,
        state="staged",
        limit=limits.max_attachments if limits is not None else None,
        client_msg_id=str(client_msg_id),
    )
    if not verdict.ok or limits is None:
        code = "limits_unavailable" if limits is None else "set_not_sent"
        record.set_outcomes(_with_overflow(verdict.outcomes, overflow, code))
        _log("attachments_rejected", count=len(verdict.outcomes), **ctx.ids)
    else:
        directory = outgoing_dir(cfg.state_dir, ctx.run_id) / str(answer)
        staged = stage_files(verdict.accepted, limits.max_upload_bytes, directory)
        if isinstance(staged, tuple):
            record.set_outcomes(_with_overflow(staged, overflow))
            _log("attachments_changed_during_check", **ctx.ids)
        else:
            files = [_FileState(s.announced, s.name, str(s.path), s.size, s.sha256) for s in staged]
            rest = (
                _stage_overflow(overflow, directory, len(files) + 1, limits)
                if overflow is not None
                else None
            )
            if overflow is not None and rest is None:
                shutil.rmtree(directory, ignore_errors=True)
                record.set_outcomes(
                    [
                        *(Outcome(f.announced, Rejection("set_not_sent")) for f in files),
                        Outcome(OVERFLOW_NAME, Rejection("too_large")),
                    ]
                )
                _log("answer_rest_too_large", **ctx.ids)
            else:
                record.files = [*files, *([rest] if rest is not None else [])]
    record.save()
    return record


# ---- note 7: the upload --------------------------------------------------------------------


class _FileFailed(Exception):  # noqa: N818 - the end of one file's upload
    def __init__(self, rejection: Rejection) -> None:
        super().__init__(rejection.code)
        self.rejection = rejection


def _refusal(err: ProblemError) -> Rejection:
    """A final problem of an upload request as the file's outcome (note 7 e)."""
    if err.reason == "storage_full":
        return Rejection("upload_refused", err.code, err.reason)  # a final 503 (note 7 e)
    if isinstance(err, ServerError | RateLimitedError | RequestInProgress):
        return Rejection("upload_failed")
    return Rejection("upload_refused", err.code, err.reason)


async def _call(ctx: ReplyContext, operation: Callable[[], Awaitable[Any]]) -> Any:
    """One request of the set; a stop before it means no request (note 10 d)."""
    if ctx.stopped():
        raise asyncio.CancelledError
    try:
        return await operation()
    except ProblemError as err:
        if _ended(err):
            raise RunEnded from None
        _log("attachment_upload_refused", code=err.code, **ctx.ids)
        raise _FileFailed(_refusal(err)) from None
    except httpx.TransportError:
        _log("attachment_upload_failed", code="transport_error", **ctx.ids)
        raise _FileFailed(Rejection("upload_failed")) from None


async def _create(ctx: ReplyContext, record: AttachmentSetRecord, f: _FileState) -> None:
    if f.create_key is None:
        f.create_key = str(uuid7())
        record.save()  # the key before its request (note 10 a)
    body = UploadCreate(name=f.name, size_bytes=f.size, sha256=f.sha256)
    key = f.create_key

    async def run() -> UploadSession:
        with ctx.client.idempotency_key(key):
            return await ctx.client.call_as(
                UploadSession, create_upload.asyncio_detailed, channel_id=ctx.channel_id, body=body
            )

    session = await _call(ctx, run)
    f.file_id = str(session.file_id)
    f.state = "created"
    f.part_size = int(session.part_size_bytes)
    f.plan = [
        {"part_number": p.part_number, "headers": {h.name: h.value for h in p.headers}}
        for p in session.parts
    ]
    record.save()  # the file_id right after its 201 (note 10 a)
    if str(session.mode.value) != "direct":
        _log("upload_mode_unsupported", **ctx.ids)
        raise _FileFailed(Rejection("upload_failed"))


async def _confirmed(ctx: ReplyContext, record: AttachmentSetRecord, f: _FileState) -> None:
    """After an adoption: the parts the server confirmed (``getUpload``, note 10 b)."""
    file_id = UUID(str(f.file_id))

    async def run() -> UploadState:
        return await ctx.client.call_as(UploadState, get_upload.asyncio_detailed, file_id=file_id)

    state = await _call(ctx, run)
    for part in state.completed_parts:
        f.etags[str(part.part_number)] = part.etag
    record.save()


def _read_slice(path: Path, offset: int, length: int) -> bytes:
    """One part of a staged copy (off the event loop)."""
    with path.open("rb") as fh:
        fh.seek(offset)
        return fh.read(length)


async def _upload_file(ctx: ReplyContext, record: AttachmentSetRecord, f: _FileState) -> None:
    resumed = f.file_id is not None
    if not resumed:
        await _create(ctx, record, f)
    elif f.state != "completed":
        await _confirmed(ctx, record, f)
    if f.state == "completed":
        return
    staged = Path(f.staged)
    if not staged.is_file() or f.part_size is None:
        raise _FileFailed(Rejection("upload_failed"))
    file_id = UUID(str(f.file_id))
    for number, offset, length in part_slices(f.size, f.part_size):
        if str(number) in f.etags:
            continue
        plan = next((p for p in f.plan if p["part_number"] == number), None)
        if plan is None:
            raise _FileFailed(Rejection("upload_failed"))
        data = await asyncio.to_thread(_read_slice, staged, offset, length)
        headers = dict(plan["headers"])

        async def put(n: int = number, d: bytes = data, h: dict[str, str] = headers) -> Any:
            return await upload_part(ctx.client, file_id, n, d, h)

        result = await _call(ctx, put)
        f.etags[str(number)] = result.etag
        record.save()
    if f.complete_key is None:
        f.complete_key = str(uuid7())
        record.save()
    parts = [
        UploadPartResult(part_number=int(n), etag=e)
        for n, e in sorted(f.etags.items(), key=lambda x: int(x[0]))
    ]
    key = f.complete_key

    async def complete() -> File:
        with ctx.client.idempotency_key(key):
            return await ctx.client.call_as(
                File,
                complete_upload.asyncio_detailed,
                file_id=file_id,
                body=UploadComplete(parts=parts),
            )

    await _call(ctx, complete)
    f.state = "completed"
    record.save()


async def _abort_open(ctx: ReplyContext, record: AttachmentSetRecord) -> None:
    """``abortUpload`` for every open session of the set (best effort, codes only)."""
    for f in record.files:
        if f.file_id is None or f.state == "completed":
            continue
        try:
            await ctx.client.call(abort_upload.asyncio_detailed, file_id=UUID(f.file_id))
        except ProblemError as err:
            _log("attachment_abort_failed", code=err.code, **ctx.ids)
        except httpx.TransportError:
            _log("attachment_abort_failed", code="transport_error", **ctx.ids)
        f.state = "failed"
    record.save()


async def upload_set(ctx: ReplyContext, record: AttachmentSetRecord) -> list[UUID] | None:
    """Upload the staged files of a set, one after the other (note 7).

    Files go in position order, parts in order, every request through the client inside the
    run's binding. A failure of any file ends the set: the failing file keeps its reason,
    every other file gets ``set_not_sent``, the open session of the failing file is aborted
    and no further file starts; the record is then ``failed``.

    Args:
        ctx: The answer's context.
        record: The record of the set (``staged``, or partly uploaded after an adoption).

    Returns:
        The ``file_id`` values in position order, or ``None`` when the set failed.

    Raises:
        RunEnded: The server ended the run (note 7 f).
        asyncio.CancelledError: A stop came (the caller aborts the open sessions).
    """
    for index, f in enumerate(record.files):
        try:
            await _upload_file(ctx, record, f)
        except _FileFailed as failed:
            await _abort_open(ctx, record)
            record.set_outcomes(
                [
                    Outcome(g.announced, failed.rejection if i == index else None)
                    for i, g in enumerate(record.files)
                ]
            )
            record.save()
            return None
    record.state = "uploaded"
    record.save()
    return [UUID(str(f.file_id)) for f in record.files]


# ---- note 9: sending -----------------------------------------------------------------------


def _attachments_refused(err: ProblemError | None) -> bool:
    """Whether ``createMessage`` refused only the attachments (note 9 c)."""
    if err is None:
        return False
    if err.status == _FORBIDDEN and err.code in _REFUSALS_403:
        return True
    if err.code == "conflict" and err.reason == "file_not_attachable":
        return True
    return (
        err.code == "validation_failed"
        and bool(err.errors)
        and all(str(e.pointer or "").startswith("/attachment_file_ids") for e in err.errors)
    )


def _cleanup(ctx: ReplyContext, record: AttachmentSetRecord) -> None:
    """Remove the staged copies and the record once the answer is settled (note 11)."""
    shutil.rmtree(outgoing_dir(ctx.config.state_dir, ctx.run_id) / str(record.answer), True)
    with contextlib.suppress(FileNotFoundError):
        record.path.unlink()


async def _upload_tracked(ctx: ReplyContext, record: AttachmentSetRecord) -> list[UUID] | None:
    """`upload_set` as a task a stop can cancel; the open sessions are aborted then."""
    _no_stop(ctx)  # a stop during getInstance or staging: not even the status line
    await ctx.set_status(STATUS_LINE)
    task = asyncio.ensure_future(upload_set(ctx, record))
    ctx.track(task)
    try:
        # A stop cancels the task, not this caller: wait for it without taking its result.
        await asyncio.wait({task})
        if task.cancelled():
            await _abort_open(ctx, record)
            return None
        return task.result()
    finally:
        if not task.done():
            task.cancel()  # this caller itself was cancelled (the runner stops)
        ctx.track(None)
        with contextlib.suppress(Exception):
            await ctx.set_status(None)


async def send_answer_with_attachments(  # noqa: PLR0913 - the answer and its rest
    ctx: ReplyContext,
    text: str,
    paths: Sequence[str],
    answer: int,
    client_msg_id: UUID,
    *,
    overflow: str | None = None,
    limit: int | None = None,
) -> SendResult:
    """Check, stage, upload and send one answer with ``ATTACH:`` lines (note 9).

    A set that uploaded completely goes out with ``attachment_file_ids`` in position order
    under the answer's ``client_msg_id``. A rejected or failed set, or an answer whose
    attachments the server refuses (``403 feature_disabled``, ``403 forbidden``, ``409
    conflict`` with ``file_not_attachable``, ``400 validation_failed`` on
    ``/attachment_file_ids`` only), goes out without them, with the notice and a new
    ``client_msg_id`` stored first. A stop sends nothing. An existing record (after an
    adoption) is resumed instead of starting again.

    Args:
        ctx: The answer's context.
        text: The answer without ``TITLE:`` and ``ATTACH:`` lines (may be empty).
        paths: The paths of the ``ATTACH:`` lines in line order.
        answer: The index of the answer in the run, from 1.
        client_msg_id: The answer's ``client_msg_id`` of the turn record.
        overflow: The rest of an answer above ``message_max_content_bytes`` (M4-47), sent as
            the last file ``answer.txt`` of the set; ``text`` is then the head.
        limit: ``message_max_content_bytes`` when the answer was cut to it; the answer without
            its files then shortens its head so head and notice fit the limit together.

    Returns:
        What became of the answer.

    Raises:
        RunEnded: The server ended the run; nothing more is sent (note 7 f).
    """
    cfg = ctx.config
    path = AttachmentSetRecord.path_for(cfg.state_dir, ctx.session_ref, ctx.run_id, answer)
    record = AttachmentSetRecord.load(path)
    settled = True
    try:
        _no_stop(ctx)
        if record is None:
            record = await _prepare(ctx, paths, answer, client_msg_id, overflow)
        if record.state == "staged":
            ids = await _upload_tracked(ctx, record)
            _no_stop(ctx)
            if ids is None and record.state != "failed":
                record.set_outcomes(
                    [Outcome(f.announced, Rejection("upload_failed")) for f in record.files]
                )
                record.save()
        if record.state == "uploaded":
            sent = await _send_attached(ctx, record, text)
            if sent is not None:
                return sent
        return await _send_fallback(ctx, record, text, limit)
    except _Stopped:
        return SendResult(None, stopped=True)
    except asyncio.CancelledError:
        # The runner itself stops: the record stays for the runner that adopts the turn.
        settled = False
        raise
    finally:
        if record is not None and settled:
            _cleanup(ctx, record)


# A stop request, not an error: no "Error" suffix.
class _Stopped(Exception):  # noqa: N818
    """A stop came: nothing more is sent (note 10 d)."""


def _no_stop(ctx: ReplyContext) -> None:
    if ctx.stopped():
        raise _Stopped


async def _send_attached(
    ctx: ReplyContext, record: AttachmentSetRecord, text: str
) -> SendResult | None:
    """Send the answer with its uploaded files; ``None`` when the server refused them."""
    _no_stop(ctx)  # the stop wins also between the last upload and createMessage
    ids = [UUID(str(f.file_id)) for f in record.files]
    problem, refusal = await ctx.deliver(text, UUID(record.client_msg_id), ids)
    if problem is None:
        _log("answer_attachments_sent", count=len(ids), **ctx.ids)
        return SendResult(None, attached=len(ids))
    if refusal is not None and _ended(refusal):
        raise RunEnded
    if not _attachments_refused(refusal):
        return SendResult(problem)
    _log("answer_attachments_refused", code=problem, **ctx.ids)
    reason = refusal.reason if refusal is not None else None
    record.set_outcomes(
        [Outcome(f.announced, Rejection("not_attached", problem, reason)) for f in record.files]
    )
    record.save()
    return None


async def _send_fallback(
    ctx: ReplyContext, record: AttachmentSetRecord, text: str, limit: int | None = None
) -> SendResult:
    """Send the answer without attachments and with the notice (note 9 c and d).

    A head cut to ``limit`` (M4-47) is shortened further, so head and notice fit together.
    """
    if record.fallback_msg_id is None:
        record.fallback_msg_id = str(uuid7())
        record.save()  # a new id before the send: the old id with another body is 422
    _no_stop(ctx)
    notice = record.notice()
    body = with_notice(text, notice)
    if limit is not None and len(body.encode("utf-8")) > limit:
        room = len(with_notice("x", notice).encode("utf-8")) - 1
        head, _ = split_for_limit(text, max(1, limit - room))
        body = with_notice(head, notice)
    problem, refusal = await ctx.deliver(body, UUID(record.fallback_msg_id), ())
    if refusal is not None and _ended(refusal):
        raise RunEnded
    return SendResult(problem)
