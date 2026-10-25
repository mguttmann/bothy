# SPDX-License-Identifier: Apache-2.0
"""The port `Store`: state and queue of the app (brief M2-50 Design note 9).

The store holds the thread map in both directions (issue ID to thread and forum, thread to
issue ID with the latest issue key), the drafts of ``/issue`` (`IssueDraft`), the Jira objects
the app created (``issue`` or ``comment`` with their ID and time), the seen identifiers with
their time and first answer (``body_digest`` and ``webhook_id``) and the jobs (increasing ID,
kind, parsed input, progress, creation time). A job is written in the same transaction as the
identifier of its request (`Store.admit`), before the request is answered. Seen identifiers,
created objects and drafts older than ``jira_app.dedupe_window`` are pruned at each write; the
thread map stays.

Examples:
    >>> ThreadRef("t", "c").thread_id
    't'
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Literal, Protocol

__all__ = [
    "AdmitResult",
    "IssueDraft",
    "JobRecord",
    "MappedIssue",
    "NewJob",
    "SeenKey",
    "Store",
    "StoreError",
    "StoreLockedError",
    "ThreadRef",
]


class StoreError(Exception):
    """The store failed (disk, database); the endpoints answer ``503``."""


class StoreLockedError(StoreError):
    """Another process holds the lock of the state directory."""


@dataclass(frozen=True)
class SeenKey:
    """An identifier of a request the app already answered.

    Attributes:
        kind: ``body_digest`` (SHA-256 of a Jira body) or ``webhook_id`` (server requests).
        value: The identifier.
    """

    kind: Literal["body_digest", "webhook_id"]
    value: str


@dataclass(frozen=True)
class NewJob:
    """A job to queue.

    Attributes:
        kind: ``jira_event``, ``issue_command``, ``issue_form`` or ``mirror``.
        data: The parsed input (JSON).
    """

    kind: str
    data: dict[str, Any]


@dataclass
class JobRecord:
    """A queued job.

    Attributes:
        id: The increasing ID; the worker takes the smallest first.
        kind: The job kind.
        data: The parsed input.
        progress: What the job did so far and the identifiers of its writes.
        created_at: Creation time, Unix seconds.
    """

    id: int
    kind: str
    data: dict[str, Any]
    progress: dict[str, Any] = field(default_factory=dict)
    created_at: float = 0.0


@dataclass(frozen=True)
class AdmitResult:
    """What `Store.admit` did.

    Attributes:
        kind: ``accepted`` (identifier and job stored), ``duplicate`` (the identifier was seen)
            or ``full`` (``jira_app.max_queued_jobs`` jobs are queued; nothing stored).
        status: The status of the first answer for a duplicate, else the given status.
        job_id: The ID of the queued job, else 0.
    """

    kind: Literal["accepted", "duplicate", "full"]
    status: int
    job_id: int = 0


@dataclass(frozen=True)
class ThreadRef:
    """The thread of an issue.

    Attributes:
        thread_id: The thread.
        channel_id: Its forum.
    """

    thread_id: str
    channel_id: str


@dataclass(frozen=True)
class MappedIssue:
    """The issue of a thread.

    Attributes:
        issue_id: The Jira issue ID.
        issue_key: The latest known issue key.
    """

    issue_id: str
    issue_key: str


@dataclass(frozen=True)
class IssueDraft:
    """An open form of ``/issue`` (``Draft`` is a schema name, so the type is ``IssueDraft``).

    Attributes:
        draft_id: The UUIDv7 in the form's ``custom_id``.
        thread_id: The forum post of the form.
        channel_id: The forum.
        card_message_id: The message of the current form card.
        title: The issue summary to create.
        projects: The projects the form offers.
        created_at: Creation time, Unix seconds.
    """

    draft_id: str
    thread_id: str
    channel_id: str
    card_message_id: str
    title: str
    projects: tuple[str, ...]
    created_at: float = 0.0


class Store(Protocol):
    """State and queue; every method is safe to call from the event loop."""

    async def open(self) -> None:
        """Open the store (and take the lock of the state directory).

        Raises:
            StoreLockedError: Another process holds the lock.
            StoreError: The store cannot be opened.
        """
        ...

    async def close(self) -> None:
        """Close the store and release the lock."""
        ...

    async def admit(
        self, seen: SeenKey, status: int, job: NewJob | None, max_jobs: int
    ) -> AdmitResult:
        """Remember ``seen`` and queue ``job`` in one transaction.

        Args:
            seen: The identifier of the request.
            status: The answer of the request, stored with the identifier.
            job: The job, or ``None`` when there is nothing to do.
            max_jobs: ``jira_app.max_queued_jobs``.

        Returns:
            What was done.
        """
        ...

    async def seen_status(self, seen: SeenKey) -> int | None:
        """Return the first answer of a seen identifier, or ``None``."""
        ...

    async def next_job(self) -> JobRecord | None:
        """Return the oldest queued job."""
        ...

    async def save_progress(self, job: JobRecord) -> None:
        """Store the progress of a job."""
        ...

    async def remove_job(self, job_id: int) -> None:
        """Delete a finished job."""
        ...

    async def thread_of_issue(self, issue_id: str) -> ThreadRef | None:
        """Return the thread of an issue."""
        ...

    async def issue_of_thread(self, thread_id: str) -> MappedIssue | None:
        """Return the issue of a thread."""
        ...

    async def map_thread(self, issue_id: str, issue_key: str, ref: ThreadRef) -> None:
        """Map an issue to a thread in both directions (replacing an older entry)."""
        ...

    async def set_issue_key(self, issue_id: str, issue_key: str) -> None:
        """Keep the latest key of a mapped issue for links."""
        ...

    async def unmap_thread(self, issue_id: str, thread_id: str) -> None:
        """Drop the entry of an issue when it still points to ``thread_id``."""
        ...

    async def record_created(self, kind: str, object_id: str) -> None:
        """Record a Jira object the app created (``issue`` or ``comment``)."""
        ...

    async def is_created(self, kind: str, object_id: str) -> bool:
        """Return whether the app created a Jira object."""
        ...

    async def put_draft(self, draft: IssueDraft) -> None:
        """Store or replace a draft."""
        ...

    async def get_draft(self, draft_id: str) -> IssueDraft | None:
        """Return a draft by its ID."""
        ...

    async def draft_by_message(self, message_id: str) -> IssueDraft | None:
        """Return the draft whose current card is ``message_id``."""
        ...

    async def delete_draft(self, draft_id: str) -> None:
        """Delete a draft."""
        ...
