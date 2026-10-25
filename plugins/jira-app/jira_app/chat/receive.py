# SPDX-License-Identifier: Apache-2.0
"""The three endpoints after the checks of `jira_app.web` (brief M2-50 Design notes 4 to 7, 13).

- ``POST /jira/webhook``: ``X-Hub-Signature`` first (``401``), then the JSON (``400``), the
  duplicate check by the SHA-256 of the body (``204``), the event list (``204`` or ``400``),
  the echo check of the app's own Jira objects, and in one store transaction the seen
  identifier and the job (``202``).
- ``POST /chat/interactions``: the SDK's `bothy_sdk.webhooks.verify_webhook` with the keys of
  ``getWebhookKeys`` (``401``), the duplicate check by ``webhook-id``; ``command.invoked`` of
  ``issue`` and ``interaction.created`` queue a job (``202``),
  ``command.autocomplete_requested`` answers ``200`` with ``{"choices": []}``; anything else
  ``202`` with Warn reason ``unknown_request``. In a forum the server has no reply thread for a
  ``200`` (``no_thread``) and Jira calls do not fit into ``app.interaction_timeout``.
- ``POST /chat/events``: the same verification; ``message.created`` passes the mirror decision
  and queues ``mirror`` (``202``) or answers ``204``; any other type ``204``. ``sync_epoch`` is
  ignored: the app keeps no ``seq`` state.

A full queue, a failing store or a secret file that fails its check answers ``503``.

Examples:
    >>> digest_of(b"{}")[:16]
    '44136fa355b3678a'
"""

from __future__ import annotations

import hashlib
import json
from collections.abc import Callable
from dataclasses import dataclass
from typing import Any

from bothy_sdk.clock import Clock
from bothy_sdk.webhooks import (
    VerifiedWebhook,
    WebhookConfigError,
    WebhookVerificationError,
    verify_webhook,
)

from jira_app.chat.port import ChatPort
from jira_app.config import AppConfig, ConfigError
from jira_app.domain.mirror import echo_of, mirror_decision
from jira_app.jira.events import Ignored, Invalid, parse_event
from jira_app.jira.signature import HEADER, verify_signature
from jira_app.log import AppLog
from jira_app.store.base import NewJob, SeenKey, Store, StoreError
from jira_app.web import RawRequest, Reply, RouteHandler

__all__ = [
    "ReceiveContext",
    "decode_json",
    "digest_of",
    "events_route",
    "interactions_route",
    "jira_route",
]


@dataclass(frozen=True)
class ReceiveContext:
    """What the endpoints need.

    Attributes:
        config: The configuration.
        store: State and queue.
        chat: The port (only its webhook keys are used here).
        webhook_secret: Reads the Jira webhook secret at each use (with the file check).
        own_principal_id: The app's principal ID from ``getMe``.
        wake: Called after a job was queued.
        clock: The time source of ``webhook-timestamp``.
        log: The app's logger.
    """

    config: AppConfig
    store: Store
    chat: ChatPort
    webhook_secret: Callable[[], bytes]
    own_principal_id: Callable[[], str]
    wake: Callable[[], None]
    clock: Clock
    log: AppLog


class _DuplicateMemberError(ValueError):
    pass


def _pairs(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    out: dict[str, Any] = {}
    for key, value in pairs:
        if key in out:
            raise _DuplicateMemberError(key)
        out[key] = value
    return out


def _constant(name: str) -> Any:
    raise ValueError(name)


def decode_json(body: bytes) -> Any:
    """Decode a JSON body strictly.

    Args:
        body: The raw body.

    Returns:
        The decoded value.

    Raises:
        ValueError: Invalid UTF-8, invalid JSON, ``NaN`` or ``Infinity``, or a duplicate member
            name (Design note 4).
    """
    try:
        return json.loads(body.decode("utf-8"), object_pairs_hook=_pairs, parse_constant=_constant)
    except RecursionError as exc:
        raise ValueError("too deeply nested") from exc


def digest_of(body: bytes) -> str:
    """Return the SHA-256 of a body in hex; its first 16 digits are the log's ``delivery_id``.

    Args:
        body: The raw body.

    Returns:
        64 lower-case hexadecimal digits.
    """
    return hashlib.sha256(body).hexdigest()


async def _admit(
    context: ReceiveContext,
    seen: SeenKey,
    status: int,
    job: NewJob | None,
    fields: dict[str, Any],
) -> Reply:
    log = context.log
    try:
        result = await context.store.admit(seen, status, job, context.config.max_queued_jobs)
    except StoreError:
        log.error("store.failed", **fields, status=503, reason="store")
        return Reply(503)
    if result.kind == "duplicate":
        log.info("delivery.duplicate", **fields, status=result.status, reason="duplicate")
        return Reply(result.status)
    if result.kind == "full":
        log.warn("delivery.queue_full", **fields, status=503, reason="queue_full")
        return Reply(503)
    if job is not None:
        context.wake()
        log.info(
            "delivery.accepted", **fields, status=status, job_id=result.job_id, job_kind=job.kind
        )
    else:
        log.info("delivery.accepted", **fields, status=status)
    return Reply(status)


def jira_route(context: ReceiveContext) -> RouteHandler:
    """Return the handler of ``POST /jira/webhook``.

    Args:
        context: Configuration, store, secret source and logger.

    Returns:
        The route handler.
    """

    async def handle(request: RawRequest) -> Reply:
        log = context.log
        try:
            secret = context.webhook_secret()
        except ConfigError:
            log.error("jira.delivery.refused", status=503, reason="secret_file")
            return Reply(503)
        reason = verify_signature(secret, request.body, request.headers.getall(HEADER, []))
        if reason is not None:
            log.warn("jira.delivery.refused", status=401, reason=reason)
            return Reply(401)
        digest = digest_of(request.body)
        fields: dict[str, Any] = {"delivery_id": digest[:16]}
        try:
            payload = decode_json(request.body)
        except ValueError:
            log.warn("jira.delivery.refused", **fields, status=400, reason="invalid_json")
            return Reply(400)
        seen = SeenKey("body_digest", digest)
        try:
            first = await context.store.seen_status(seen)
        except StoreError:
            log.error("store.failed", **fields, status=503, reason="store")
            return Reply(503)
        if first is not None:
            log.info("jira.delivery.duplicate", **fields, status=204, reason="duplicate")
            return Reply(204)
        parsed = parse_event(payload, {p.project for p in context.config.projects})
        if isinstance(parsed, Invalid):
            log.warn(
                "jira.delivery.refused",
                **fields,
                event=parsed.event,
                status=400,
                reason="invalid_payload",
            )
            return Reply(400)
        if isinstance(parsed, Ignored):
            fields["event"] = parsed.event
            log.info("jira.delivery.ignored", **fields, status=204, reason=parsed.reason)
            return await _admit(context, seen, 204, None, fields)
        fields.update(event=parsed.event, issue_id=parsed.issue_id)
        echo = echo_of(parsed)
        try:
            own = echo is not None and await context.store.is_created(*echo)
        except StoreError:
            log.error("store.failed", **fields, status=503, reason="store")
            return Reply(503)
        if own:
            log.info("jira.delivery.ignored", **fields, status=204, reason="own_jira_object")
            return await _admit(context, seen, 204, None, fields)
        job = NewJob("jira_event", {"event": parsed.to_json()})
        return await _admit(context, seen, 202, job, fields)

    return handle


async def _verify(context: ReceiveContext, request: RawRequest) -> VerifiedWebhook | Reply:
    log = context.log
    try:
        keys = await context.chat.webhook_keys()
    except Exception:
        log.error("chat.request.refused", status=503, reason="webhook_keys")
        return Reply(503)
    try:
        return verify_webhook(request.body, request.headers, keys=keys, clock=context.clock)
    except WebhookVerificationError as exc:
        log.warn("chat.request.refused", status=401, reason="verification", code=exc.code)
        return Reply(401)
    except WebhookConfigError:
        log.error("chat.request.refused", status=503, reason="webhook_config")
        return Reply(503)


def _decode_envelope(
    context: ReceiveContext, verified: VerifiedWebhook, body: bytes
) -> tuple[str, dict[str, Any]] | Reply:
    try:
        data = decode_json(body)
    except ValueError:
        context.log.warn(
            "chat.request.refused", webhook_id=verified.id, status=400, reason="invalid_json"
        )
        return Reply(400)
    if not isinstance(data, dict):
        context.log.warn(
            "chat.request.refused", webhook_id=verified.id, status=400, reason="invalid_payload"
        )
        return Reply(400)
    kind = data.get("type")
    payload = data.get("payload")
    return (
        kind if isinstance(kind, str) else "",
        payload if isinstance(payload, dict) else {},
    )


def _option(payload: dict[str, Any], name: str) -> str:
    options = payload.get("options")
    for option in options if isinstance(options, list) else []:
        if isinstance(option, dict) and option.get("name") == name:
            value = option.get("value")
            return value if isinstance(value, str) else ""
    return ""


def interactions_route(context: ReceiveContext) -> RouteHandler:
    """Return the handler of ``POST /chat/interactions``.

    Args:
        context: Configuration, store, keys, clock and logger.

    Returns:
        The route handler.
    """

    async def handle(request: RawRequest) -> Reply:
        verified = await _verify(context, request)
        if isinstance(verified, Reply):
            return verified
        decoded = _decode_envelope(context, verified, request.body)
        if isinstance(decoded, Reply):
            return decoded
        kind, payload = decoded
        fields: dict[str, Any] = {"event": kind, "webhook_id": verified.id}
        log = context.log
        if kind == "command.autocomplete_requested":
            log.info("chat.autocomplete.answered", **fields, status=200)
            return Reply(200, {"choices": []})
        seen = SeenKey("webhook_id", verified.id)
        if kind == "command.invoked" and payload.get("command_name") == "issue":
            channel = payload.get("channel_id")
            if not isinstance(channel, str):
                log.warn("chat.request.refused", **fields, status=400, reason="invalid_payload")
                return Reply(400)
            job = NewJob(
                "issue_command", {"channel_id": channel.lower(), "title": _option(payload, "title")}
            )
            return await _admit(context, seen, 202, job, fields)
        if kind == "interaction.created":
            interaction = payload.get("interaction")
            item = interaction if isinstance(interaction, dict) else {}
            interaction_id, message_id = item.get("id"), item.get("message_id")
            if not isinstance(interaction_id, str) or not isinstance(message_id, str):
                log.warn("chat.request.refused", **fields, status=400, reason="invalid_payload")
                return Reply(400)
            values = item.get("values")
            custom_id = item.get("custom_id")
            job = NewJob(
                "issue_form",
                {
                    "interaction_id": interaction_id,
                    "custom_id": custom_id if isinstance(custom_id, str) else "",
                    "message_id": message_id.lower(),
                    "values": values if isinstance(values, dict) else {},
                },
            )
            return await _admit(context, seen, 202, job, fields)
        log.warn("chat.request.ignored", **fields, status=202, reason="unknown_request")
        return await _admit(context, seen, 202, None, fields)

    return handle


def events_route(context: ReceiveContext) -> RouteHandler:
    """Return the handler of ``POST /chat/events``.

    Args:
        context: Configuration, store, keys, own principal, clock and logger.

    Returns:
        The route handler.
    """

    async def handle(request: RawRequest) -> Reply:
        verified = await _verify(context, request)
        if isinstance(verified, Reply):
            return verified
        decoded = _decode_envelope(context, verified, request.body)
        if isinstance(decoded, Reply):
            return decoded
        kind, payload = decoded
        fields: dict[str, Any] = {"event": kind, "webhook_id": verified.id}
        log = context.log
        seen = SeenKey("webhook_id", verified.id)
        if kind != "message.created":
            log.info("chat.event.ignored", **fields, status=204, reason="unsupported")
            return await _admit(context, seen, 204, None, fields)
        message = payload.get("message")
        author = message.get("author") if isinstance(message, dict) else None
        if (
            not isinstance(message, dict)
            or not isinstance(message.get("id"), str)
            or not isinstance(message.get("thread_id"), str)
            or not isinstance(author, dict)
        ):
            log.warn("chat.request.refused", **fields, status=400, reason="invalid_payload")
            return Reply(400)
        thread_id = str(message["thread_id"]).lower()
        fields["thread_id"] = thread_id
        try:
            issue = await context.store.issue_of_thread(thread_id)
        except StoreError:
            log.error("store.failed", **fields, status=503, reason="store")
            return Reply(503)
        reason = mirror_decision(message, context.own_principal_id(), lambda _: issue is not None)
        if reason is not None or issue is None:
            log.info("chat.event.ignored", **fields, status=204, reason=reason)
            return await _admit(context, seen, 204, None, fields)
        content = message.get("content_md")
        name = author.get("display_name")
        job = NewJob(
            "mirror",
            {
                "message_id": str(message["id"]).lower(),
                "thread_id": thread_id,
                "issue_id": issue.issue_id,
                "display_name": name if isinstance(name, str) else "",
                "content": content if isinstance(content, str) else "",
                "truncated": message.get("content_truncated") is True,
            },
        )
        return await _admit(context, seen, 202, job, fields)

    return handle
