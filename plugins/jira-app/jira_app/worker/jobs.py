# SPDX-License-Identifier: Apache-2.0
"""The four jobs of the app and the handling of the server's answers.

Brief M2-50 Design notes 9 to 14:

- ``jira_event``: one message per Jira event into the thread of its issue, or a new forum post
  for an unknown issue ID; a thread out of reach (``404``) is replaced once.
- ``issue_command``: ``/issue`` opens its own forum post with a form card and stores a draft.
- ``issue_form``: the submitted form creates the issue in Jira and answers in the thread.
- ``mirror``: a person's reply in a linked thread becomes a Jira comment.

Every write to the server gets its ``client_msg_id`` or ``Idempotency-Key`` stored in the job's
progress before the first attempt, so a rerun repeats the same request and the server answers
the original. A write to Jira is marked ``attempted`` before it is sent and never repeated; a
rerun that finds ``attempted`` without a stored result treats the outcome as unknown.
`JobRunner.run` answers ``done`` (the job ends) or ``retry`` (transient: the job stays queued
and the worker pauses ``sdk.retry_backoff_cap``).

Examples:
    >>> TEXTS["expired"]
    'This form expired. Run /issue again.'
"""

from __future__ import annotations

from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from typing import Any, TypeVar
from uuid import UUID

import httpx
from bothy_sdk.components import FormField, form
from bothy_sdk.errors import ProblemError
from bothy_sdk.generated.models.instance_limits import InstanceLimits
from bothy_sdk.generated.models.interaction_respond import InteractionRespond
from bothy_sdk.generated.models.interaction_respond_state import InteractionRespondState
from bothy_sdk.generated.models.interaction_response import InteractionResponse
from bothy_sdk.generated.models.message_create import MessageCreate
from bothy_sdk.generated.models.message_create_kind import MessageCreateKind
from bothy_sdk.ids import uuid7
from bothy_sdk.settings import SdkSettings

from jira_app.chat.port import ChatPort
from jira_app.config import AppConfig, ConfigError
from jira_app.domain.mirror import echo_of
from jira_app.domain.render import (
    RenderLimits,
    browse_link,
    clean_title,
    render_event,
    thread_title,
)
from jira_app.jira.adf import issue_description, mirror_comment
from jira_app.jira.client import JiraClient, JiraResult
from jira_app.jira.events import JiraEvent
from jira_app.log import AppLog
from jira_app.store.base import IssueDraft, JobRecord, Store, ThreadRef
from jira_app.worker.worker import JobOutcome

__all__ = ["FORM_PREFIX", "TEXTS", "JobDeps", "JobRunner"]

FORM_PREFIX = "issue:"
"""The prefix of the form's ``custom_id``; the draft ID follows."""

TEXTS: dict[str, str] = {
    "expired": "This form expired. Run /issue again.",
    "created": "Created issue {key}",
    "refused": "Jira refused the issue (status {status}).",
    "unknown": "Jira did not confirm the issue. Check the project before you try again.",
    "not_sent": "The app could not send the issue to Jira. Try again later.",
    "choose_project": "Choose one of the listed projects.",
    "new_issue_title": "New issue",
    "first_message": "New Jira issue. Fill in the form below.",
    "card_title": "New Jira issue",
    "card_text": "Describe the issue and submit the form.",
    "confirmation": "Created Jira issue {link}.",
}
"""The texts the person sees (English, without mentions; Design notes 11 and 12)."""

_T = TypeVar("_T")
_NOT_FOUND = 404
_CONFLICT = 409


@dataclass(frozen=True)
class JobDeps:
    """The dependencies of `JobRunner`.

    Attributes:
        config: The configuration.
        store: State and queue.
        chat: The port to the server.
        jira: The Jira client.
        log: The app's logger.
        now: Wall time in Unix seconds.
        limits: The current limits of ``getInstance``.
        refresh_limits: Reloads the limits from ``getInstance``.
        on_unauthenticated: Called on ``401``: the endpoints answer ``503`` until ``getMe``
            succeeds again.
        settings: The SDK settings (``component.custom_id_max_chars``,
            ``component.form_max_fields``).
    """

    config: AppConfig
    store: Store
    chat: ChatPort
    jira: JiraClient
    log: AppLog
    now: Callable[[], float]
    limits: Callable[[], InstanceLimits]
    refresh_limits: Callable[[], Awaitable[None]]
    on_unauthenticated: Callable[[], None]
    settings: SdkSettings


class _JobEndedError(Exception):
    """Ends a job early with its log line already written."""


class _CallFailedError(Exception):
    """A server call failed; carries the operation for the log."""

    def __init__(self, operation: str, error: ProblemError) -> None:
        super().__init__(operation)
        self.operation = operation
        self.error = error


def _field_codes(err: ProblemError) -> str:
    return ",".join(e.code for e in err.errors)


def _too_long(err: ProblemError) -> bool:
    return (
        err.status == 400
        and err.code == "validation_failed"
        and any(e.code == "too_long" for e in err.errors)
    )


def _on_title(err: ProblemError) -> bool:
    return (
        err.status == 400
        and err.code == "validation_failed"
        and any(e.pointer == "/title" for e in err.errors)
    )


def _text(value: Any) -> str | None:
    return value if isinstance(value, str) else None


class JobRunner:
    """Runs one job; the worker calls it in creation order, one at a time.

    Args:
        deps: Configuration, store, ports, logger and clock.
    """

    def __init__(self, deps: JobDeps) -> None:
        """Create the runner; see the class docstring."""
        self._d = deps

    async def run(self, job: JobRecord) -> JobOutcome:
        """Run ``job`` until it ends or meets a transient failure.

        Args:
            job: The queued job; its progress is saved after each step.

        Returns:
            ``done`` when the job ends (also after a permanent failure, logged once),
            ``retry`` when it stays queued.
        """
        base: dict[str, Any] = {"job_id": job.id, "job_kind": job.kind}
        try:
            if job.kind == "jira_event":
                await self._jira_event(job)
            elif job.kind == "issue_command":
                await self._issue_command(job)
            elif job.kind == "issue_form":
                await self._issue_form(job)
            elif job.kind == "mirror":
                await self._mirror(job)
            else:
                self._d.log.error("job.unknown_kind", **base)
        except Exception as exc:
            return self._failed(exc, base)
        return "done"

    def _failed(self, exc: Exception, base: dict[str, Any]) -> JobOutcome:
        log = self._d.log
        if isinstance(exc, _JobEndedError):
            return "done"
        if isinstance(exc, httpx.TransportError):
            log.warn("job.transient", **base, reason="transport")
            return "retry"
        if isinstance(exc, ConfigError):
            log.error("job.secret_file", **base, reason="secret_file")
            return "done"
        if not isinstance(exc, _CallFailedError):
            log.error("job.failed", **base, reason="internal")
            return "done"
        err = exc.error
        fields = {**base, "operation": exc.operation, "status": err.status, "code": err.code}
        if err.status == 429 or err.status >= 500:
            log.warn("job.transient", **fields, reason="server_unavailable")
            return "retry"
        if err.status == 401:
            log.error("job.unauthenticated", **fields, reason="unauthenticated")
            self._d.on_unauthenticated()
            return "done"
        if err.status == 403 and err.code == "scope_missing":
            log.warn(
                "job.scope_missing",
                **base,
                operation=exc.operation,
                status=err.status,
                reason="scope_missing",
            )
            return "done"
        if err.status == 422 and err.code == "idempotency_key_reuse":
            log.error("job.idempotency_key_reuse", **fields, reason="idempotency_key_reuse")
            return "done"
        if err.status == 400:
            log.warn(
                "job.refused",
                **{**fields, "code": _field_codes(err) or err.code},
                reason="validation_failed",
            )
            return "done"
        log.warn("job.refused", **fields, reason=err.reason or err.code)
        return "done"

    # -- helpers --------------------------------------------------------------------------

    async def _call(self, operation: str, call: Awaitable[_T]) -> _T:
        try:
            return await call
        except ProblemError as exc:
            raise _CallFailedError(operation, exc) from exc

    async def _ensure(self, job: JobRecord, key: str) -> str:
        """Store a fresh identifier in the progress before the step that uses it."""
        existing = _text(job.progress.get(key))
        if existing is not None:
            return existing
        value = str(uuid7())
        job.progress[key] = value
        await self._d.store.save_progress(job)
        return value

    async def _set(self, job: JobRecord, **values: Any) -> None:
        job.progress.update(values)
        await self._d.store.save_progress(job)

    def _render_limits(self) -> RenderLimits:
        limits = self._d.limits()
        return RenderLimits(limits.message_max_content_bytes, limits.thread_title_max_chars)

    async def _create_thread(
        self,
        job: JobRecord,
        channel_id: str,
        title: Callable[[], str],
        bare: str,
        content: Callable[[], str],
    ) -> str:
        """Create a forum post; the ``400`` retries of Design note 11 included."""
        use_bare = job.progress.get("bare_title") is True
        refreshed = job.progress.get("limits_refreshed") is True
        while True:
            msg_id = await self._ensure(job, "thread_msg_id")
            key = await self._ensure(job, "thread_idem")
            try:
                return await self._call(
                    "createThread",
                    self._d.chat.create_forum_thread(
                        channel_id,
                        bare if use_bare else title(),
                        MessageCreate(client_msg_id=UUID(msg_id), content_md=content()),
                        key,
                    ),
                )
            except _CallFailedError as exc:
                again = False
                if _too_long(exc.error) and not refreshed:
                    await self._d.refresh_limits()
                    refreshed = again = True
                elif _on_title(exc.error) and not use_bare:
                    use_bare = again = True
                if not again:
                    raise
                # A refused request created nothing; the next attempt gets new identifiers.
                await self._set(
                    job,
                    bare_title=use_bare,
                    limits_refreshed=refreshed,
                    thread_msg_id=str(uuid7()),
                    thread_idem=str(uuid7()),
                )

    async def _post(
        self, job: JobRecord, slot: str, thread_id: str, content: Callable[[], str]
    ) -> None:
        """Post a text message once per progress slot; one retry after ``too_long``."""
        if job.progress.get(f"{slot}_done") is True:
            return
        refreshed = job.progress.get(f"{slot}_refreshed") is True
        while True:
            msg_id = await self._ensure(job, f"{slot}_msg_id")
            try:
                await self._call(
                    "createMessage",
                    self._d.chat.post_message(
                        thread_id, MessageCreate(client_msg_id=UUID(msg_id), content_md=content())
                    ),
                )
            except _CallFailedError as exc:
                if _too_long(exc.error) and not refreshed:
                    await self._d.refresh_limits()
                    refreshed = True
                    await self._set(
                        job, **{f"{slot}_refreshed": True, f"{slot}_msg_id": str(uuid7())}
                    )
                    continue
                raise
            await self._set(job, **{f"{slot}_done": True})
            return

    # -- jira_event -----------------------------------------------------------------------

    async def _jira_event(self, job: JobRecord) -> None:
        event = JiraEvent.from_json(job.data["event"])
        d = self._d
        fields: dict[str, Any] = {
            "job_id": job.id,
            "job_kind": job.kind,
            "event": event.event,
            "issue_id": event.issue_id,
        }
        echo = echo_of(event)
        if echo is not None and await d.store.is_created(*echo):
            d.log.info("jira.event.skipped", **fields, reason="own_jira_object")
            return
        project = d.config.project(event.project)
        if project is None:
            d.log.warn("jira.event.skipped", **fields, reason="project_not_configured")
            return
        content = lambda: render_event(event, d.config.jira_base_url, self._render_limits())  # noqa: E731
        mapped = await d.store.thread_of_issue(event.issue_id)
        if mapped is not None and job.progress.get("replaced") is not True:
            try:
                await self._post(job, "event", mapped.thread_id, content)
            except _CallFailedError as exc:
                if exc.error.status != _NOT_FOUND:
                    raise
                await d.store.unmap_thread(event.issue_id, mapped.thread_id)
                await self._set(job, replaced=True)
                d.log.warn(
                    "jira.thread.replaced",
                    **fields,
                    thread_id=mapped.thread_id,
                    reason="thread_unreachable",
                )
            else:
                await d.store.set_issue_key(event.issue_id, event.issue_key)
                d.log.info("jira.event.posted", **fields, thread_id=mapped.thread_id)
                return
        thread_id = await self._create_thread(
            job,
            project.channel_id,
            lambda: thread_title(
                event.issue_key, event.summary, self._d.limits().thread_title_max_chars
            ),
            event.issue_key,
            content,
        )
        await d.store.map_thread(
            event.issue_id, event.issue_key, ThreadRef(thread_id, project.channel_id)
        )
        d.log.info(
            "jira.event.thread_created",
            **fields,
            thread_id=thread_id,
            channel_id=project.channel_id,
        )

    # -- issue_command --------------------------------------------------------------------

    async def _issue_command(self, job: JobRecord) -> None:
        d = self._d
        channel_id = str(job.data["channel_id"])
        fields: dict[str, Any] = {"job_id": job.id, "job_kind": job.kind, "channel_id": channel_id}
        projects = tuple(p.project for p in d.config.projects_of_forum(channel_id))
        if not projects:
            d.log.warn("issue.command.ignored", **fields, reason="channel_not_mapped")
            return
        draft_id = await self._ensure(job, "draft_id")
        raw_title = str(job.data.get("title", ""))

        def title() -> str:
            cleaned = clean_title(raw_title, d.limits().thread_title_max_chars)
            return cleaned or TEXTS["new_issue_title"]

        thread_id = _text(job.progress.get("thread_id"))
        if thread_id is None:
            thread_id = await self._create_thread(
                job,
                channel_id,
                title,
                TEXTS["new_issue_title"],
                lambda: TEXTS["first_message"],
            )
            await self._set(job, thread_id=thread_id)
        card_id = await self._post_card(job, "card", thread_id, draft_id, projects)
        await d.store.put_draft(
            IssueDraft(draft_id, thread_id, channel_id, card_id, title(), projects, d.now())
        )
        d.log.info("issue.command.form_posted", **fields, thread_id=thread_id)

    async def _post_card(
        self, job: JobRecord, slot: str, thread_id: str, draft_id: str, projects: tuple[str, ...]
    ) -> str:
        """Post the form card of a draft once per progress slot; return its message ID."""
        done = _text(job.progress.get(f"{slot}_message_id"))
        if done is not None:
            return done
        msg_id = await self._ensure(job, f"{slot}_msg_id")
        fields = [FormField("description", "Description", "textarea", required=False)]
        if len(projects) > 1:
            fields.append(
                FormField("project", "Project", "choice", required=True, choices=projects)
            )
        component = form(
            f"{FORM_PREFIX}{draft_id}",
            TEXTS["card_title"],
            fields,
            single_use=True,
            restricted_to="any",
            settings=self._d.settings,
        )
        message = MessageCreate(
            client_msg_id=UUID(msg_id),
            kind=MessageCreateKind.CARD,
            content_md=TEXTS["card_text"],
            components=[component],
        )
        posted = await self._call("createMessage", self._d.chat.post_message(thread_id, message))
        posted_id = str(posted.id)
        await self._set(job, **{f"{slot}_message_id": posted_id})
        return posted_id

    async def _respond(
        self, job: JobRecord, interaction_id: str, state: InteractionRespondState, text: str
    ) -> None:
        """``respondToInteraction`` once; a ``409`` (already answered) counts as done."""
        if job.progress.get("respond_done") is True:
            return
        key = await self._ensure(job, "respond_idem")
        body = InteractionRespond(state=state, response=InteractionResponse(text=text))
        try:
            await self._call(
                "respondToInteraction", self._d.chat.respond_interaction(interaction_id, body, key)
            )
        except _CallFailedError as exc:
            if exc.error.status != _CONFLICT:
                raise
        await self._set(job, respond_done=True)

    # -- issue_form -----------------------------------------------------------------------

    async def _find_draft(self, data: dict[str, Any]) -> IssueDraft | None:
        store = self._d.store
        custom_id = str(data.get("custom_id", ""))
        draft = None
        if custom_id.startswith(FORM_PREFIX):
            draft = await store.get_draft(custom_id[len(FORM_PREFIX) :])
        if draft is None:
            draft = await store.draft_by_message(str(data.get("message_id", "")))
        return draft

    async def _issue_form(self, job: JobRecord) -> None:
        d = self._d
        data = job.data
        interaction_id = str(data["interaction_id"])
        fields: dict[str, Any] = {"job_id": job.id, "job_kind": job.kind}
        draft = await self._find_draft(data)
        if draft is None:
            await self._respond(
                job, interaction_id, InteractionRespondState.REJECTED, TEXTS["expired"]
            )
            d.log.info("issue.form.expired", **fields, reason="draft_unknown")
            return
        fields["thread_id"] = draft.thread_id
        raw_values = data.get("values")
        values: dict[str, Any] = raw_values if isinstance(raw_values, dict) else {}
        chosen = values.get("project")
        project = (
            draft.projects[0]
            if len(draft.projects) == 1
            else (chosen if isinstance(chosen, str) and chosen in draft.projects else None)
        )
        entry = d.config.project(project) if project is not None else None
        if entry is None:
            await self._respond(
                job, interaction_id, InteractionRespondState.REJECTED, TEXTS["choose_project"]
            )
            d.log.info("issue.form.refused", **fields, reason="project_not_offered")
            return
        stored = job.progress.get("jira_result")
        if isinstance(stored, dict):
            result = JiraResult.from_json(stored)
        elif job.progress.get("jira") == "attempted":
            result = JiraResult("unknown", reason="restart")
            await self._set(job, jira_result=result.to_json())
        else:
            description = values.get("description")
            text = description if isinstance(description, str) else ""
            await self._set(job, jira="attempted")
            result = await d.jira.create_issue(
                entry.project, entry.issue_type, draft.title, issue_description(text)
            )
            await self._set(job, jira_result=result.to_json())
        if result.kind == "created":
            await self._created(job, interaction_id, draft, result, fields)
            return
        if result.kind in ("refused", "not_sent"):
            text = (
                TEXTS["refused"].format(status=result.status)
                if result.kind == "refused"
                else TEXTS["not_sent"]
            )
            await self._respond(job, interaction_id, InteractionRespondState.FAILED, text)
            card_id = await self._post_card(
                job, "retry_card", draft.thread_id, draft.draft_id, draft.projects
            )
            await d.store.put_draft(
                IssueDraft(
                    draft.draft_id,
                    draft.thread_id,
                    draft.channel_id,
                    card_id,
                    draft.title,
                    draft.projects,
                    draft.created_at,
                )
            )
            extra = (
                {"status": result.status} if result.kind == "refused" else {"reason": result.reason}
            )
            d.log.warn("issue.form.refused", **fields, **extra, outcome=result.kind)
            return
        await self._respond(job, interaction_id, InteractionRespondState.FAILED, TEXTS["unknown"])
        await d.store.delete_draft(draft.draft_id)
        extra = {"status": result.status} if result.status else {}
        d.log.warn(
            "issue.form.unknown",
            **fields,
            **extra,
            outcome="unknown",
            reason=result.reason or "jira_outcome_unknown",
        )

    async def _created(
        self,
        job: JobRecord,
        interaction_id: str,
        draft: IssueDraft,
        result: JiraResult,
        fields: dict[str, Any],
    ) -> None:
        d = self._d
        await d.store.record_created("issue", result.object_id)
        await d.store.map_thread(
            result.object_id, result.key, ThreadRef(draft.thread_id, draft.channel_id)
        )
        await self._respond(
            job,
            interaction_id,
            InteractionRespondState.ACKNOWLEDGED,
            TEXTS["created"].format(key=result.key),
        )
        link = browse_link(d.config.jira_base_url, result.key)
        await self._post(
            job, "confirm", draft.thread_id, lambda: TEXTS["confirmation"].format(link=link)
        )
        await d.store.delete_draft(draft.draft_id)
        d.log.info("issue.form.created", **fields, issue_id=result.object_id, outcome="created")

    # -- mirror ---------------------------------------------------------------------------

    async def _mirror(self, job: JobRecord) -> None:
        d = self._d
        data = job.data
        issue_id = str(data["issue_id"])
        fields: dict[str, Any] = {
            "job_id": job.id,
            "job_kind": job.kind,
            "issue_id": issue_id,
            "thread_id": str(data.get("thread_id", "")),
        }
        if job.progress.get("jira") == "attempted":
            d.log.warn("mirror.unknown", **fields, outcome="unknown", reason="jira_outcome_unknown")
            return
        content = _text(job.progress.get("content"))
        if content is None:
            content = str(data.get("content", ""))
            if data.get("truncated") is True:
                message = await self._call(
                    "getMessage", d.chat.get_message(str(data["message_id"]))
                )
                content = message.content_md
                await self._set(job, content=content)
        await self._set(job, jira="attempted")
        result = await d.jira.create_comment(
            issue_id, mirror_comment(str(data.get("display_name", "")), content)
        )
        if result.kind == "created":
            await d.store.record_created("comment", result.object_id)
            d.log.info("mirror.created", **fields, outcome="created")
            return
        if result.status:
            d.log.warn("mirror.failed", **fields, outcome=result.kind, status=result.status)
        else:
            d.log.warn(
                "mirror.failed",
                **fields,
                outcome=result.kind,
                reason=result.reason or "jira_outcome_unknown",
            )
