# SPDX-License-Identifier: Apache-2.0
"""In-memory `ChatPort` of the unit tests, with the signing key of the server's requests.

Threads and messages live in memory; ``createThread`` is idempotent by
``first_message.client_msg_id`` and ``createMessage`` by ``client_msg_id`` (the server answers
the original on a repeat, 04 5.1). Faults are queued per operation as `ProblemError` values or
transport errors and raised by the next call. `FakeChat.sign` signs a request body with the
seed whose public key `webhook_keys` returns, the way the server signs requests to
``interactions_url``.
"""

from __future__ import annotations

import asyncio
import json
from dataclasses import dataclass, field
from datetime import UTC, datetime
from typing import Any
from uuid import UUID

import httpx
from bothy_sdk.errors import ProblemError, problem_error
from bothy_sdk.generated.models.instance_limits import InstanceLimits
from bothy_sdk.generated.models.interaction_respond import InteractionRespond
from bothy_sdk.generated.models.message import Message
from bothy_sdk.generated.models.message_create import MessageCreate
from bothy_sdk.generated.models.principal_ref import PrincipalRef
from bothy_sdk.ids import uuid7
from bothy_sdk.testing import sign_webhook
from bothy_sdk.testing.webhooks import webhook_public_key
from bothy_sdk.webhooks import parse_webhook_public_key
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey

APP_ID = "0198a4f2-6c1e-7a3b-9d2e-000000000a99"
ALEX_ID = "0198a4f2-6c1e-7a3b-9d2e-000000000a01"
SEED = bytes(range(1, 33))
OTHER_SEED = bytes(range(101, 133))

DEFAULT_LIMITS = {
    "message_max_content_bytes": 262144,
    "message_max_attachments": 10,
    "file_max_upload_bytes": 104857600,
    "thread_title_max_chars": 200,
    "voice_max_duration_ms": 900000,
}


def problem(status: int, code: str, **extra: Any) -> ProblemError:
    """A problem error as the SDK raises it for an answer."""
    return problem_error(status, {"code": code, "title": code, "status": status, **extra})


def principal(pid: str, kind: str, name: str) -> dict[str, Any]:
    """A `PrincipalRef` as JSON."""
    return {"id": pid, "kind": kind, "handle": name.lower(), "display_name": name}


def message_json(
    *,
    thread_id: str,
    channel_id: str,
    content: str,
    author: dict[str, Any],
    origin: str = "human",
    kind: str = "text",
    message_id: str | None = None,
    client_msg_id: str | None = None,
    components: list[Any] | None = None,
    **extra: Any,
) -> dict[str, Any]:
    """A `Message` as JSON with every required member."""
    return {
        "id": message_id or str(uuid7()),
        "thread_id": thread_id,
        "channel_id": channel_id,
        "thread_seq": 1,
        "author": author,
        "client_msg_id": client_msg_id or str(uuid7()),
        "origin": origin,
        "kind": kind,
        "content_md": content,
        "content_format": 1,
        "silent": False,
        "attachments": [],
        "mention_ids": [],
        "reactions": [],
        "pinned": False,
        "components": components or [],
        "version": 1,
        "created_at": datetime.now(UTC).isoformat().replace("+00:00", "Z"),
        **extra,
    }


@dataclass
class FakeThread:
    """A forum post of the fake."""

    thread_id: str
    channel_id: str
    title: str
    messages: list[dict[str, Any]] = field(default_factory=list)


@dataclass
class ChatCall:
    """One call of the port."""

    operation: str
    args: dict[str, Any]


class FakeChat:
    """The in-memory chat port."""

    def __init__(self, limits: dict[str, int] | None = None) -> None:
        self.threads: dict[str, FakeThread] = {}
        self.calls: list[ChatCall] = []
        self.responses: list[tuple[str, dict[str, Any], str]] = []
        self.faults: dict[str, list[Exception]] = {}
        self.after_faults: dict[str, list[Exception]] = {}
        self.gate: asyncio.Event | None = None
        self.full_messages: dict[str, dict[str, Any]] = {}
        self.limit_values = dict(limits or DEFAULT_LIMITS)
        self.limit_calls = 0
        self._by_msg_id: dict[str, tuple[str, dict[str, Any]]] = {}
        self._threads_by_msg: dict[str, str] = {}

    # -- faults and signing ---------------------------------------------------------------

    def fail(self, operation: str, *errors: Exception) -> None:
        """Queue errors for the next calls of an operation (``createThread`` and so on)."""
        self.faults.setdefault(operation, []).extend(errors)

    def fail_after(self, operation: str, *errors: Exception) -> None:
        """Queue errors raised after the next calls of an operation took effect (lost answers)."""
        self.after_faults.setdefault(operation, []).extend(errors)

    def _after(self, operation: str) -> None:
        queue = self.after_faults.get(operation)
        if queue:
            raise queue.pop(0)

    def _fault(self, operation: str) -> None:
        queue = self.faults.get(operation)
        if queue:
            raise queue.pop(0)

    def sign(
        self, body: bytes, webhook_id: str, *, timestamp: int, seed: bytes = SEED
    ) -> dict[str, str]:
        """The headers of a request of the server to the app."""
        return {
            "Content-Type": "application/json",
            "webhook-id": webhook_id,
            "webhook-timestamp": str(timestamp),
            "webhook-signature": sign_webhook(
                webhook_id=webhook_id, timestamp=timestamp, body=body, seeds=[seed]
            ),
        }

    # -- ChatPort --------------------------------------------------------------------------

    async def me(self) -> PrincipalRef:
        self.calls.append(ChatCall("getMe", {}))
        self._fault("getMe")
        return PrincipalRef.from_dict(principal(APP_ID, "app", "Jira"))

    async def limits(self) -> InstanceLimits:
        self.calls.append(ChatCall("getInstance", {}))
        self.limit_calls += 1
        self._fault("getInstance")
        return InstanceLimits.from_dict(self.limit_values)

    async def webhook_keys(self) -> list[Ed25519PublicKey]:
        self._fault("getWebhookKeys")
        return [parse_webhook_public_key(webhook_public_key(SEED))]

    async def create_forum_thread(
        self, channel_id: str, title: str, first_message: MessageCreate, idempotency_key: str
    ) -> str:
        body = first_message.to_dict()
        self.calls.append(
            ChatCall(
                "createThread",
                {
                    "channel_id": channel_id,
                    "title": title,
                    "first_message": body,
                    "idempotency_key": idempotency_key,
                },
            )
        )
        self._fault("createThread")
        if self.gate is not None:
            await self.gate.wait()
        msg_id = str(body["client_msg_id"])
        known = self._threads_by_msg.get(msg_id)
        if known is not None:
            return known
        thread_id = str(uuid7())
        thread = FakeThread(thread_id, channel_id, title)
        self.threads[thread_id] = thread
        self._threads_by_msg[msg_id] = thread_id
        self._store(thread, body)
        self._after("createThread")
        return thread_id

    def _store(self, thread: FakeThread, body: dict[str, Any]) -> dict[str, Any]:
        msg_id = str(body["client_msg_id"])
        known = self._by_msg_id.get(msg_id)
        if known is not None:
            return known[1]
        stored = message_json(
            thread_id=thread.thread_id,
            channel_id=thread.channel_id,
            content=str(body.get("content_md", "")),
            author=principal(APP_ID, "app", "Jira"),
            origin="app",
            kind=str(body.get("kind", "text")),
            client_msg_id=msg_id,
            components=[],
        )
        stored["sent"] = body
        thread.messages.append(stored)
        self._by_msg_id[msg_id] = (thread.thread_id, stored)
        return stored

    async def post_message(self, thread_id: str, message: MessageCreate) -> Message:
        body = message.to_dict()
        self.calls.append(ChatCall("createMessage", {"thread_id": thread_id, "message": body}))
        self._fault("createMessage")
        thread = self.threads.get(thread_id)
        if thread is None:
            raise problem(404, "not_found")
        stored = self._store(thread, body)
        self._after("createMessage")
        visible = {k: v for k, v in stored.items() if k != "sent"}
        return Message.from_dict(visible)

    async def get_message(self, message_id: str) -> Message:
        self.calls.append(ChatCall("getMessage", {"message_id": message_id}))
        self._fault("getMessage")
        found = self.full_messages.get(message_id)
        if found is None:
            raise problem(404, "not_found")
        return Message.from_dict(found)

    async def respond_interaction(
        self, interaction_id: str, body: InteractionRespond, idempotency_key: str
    ) -> None:
        payload = body.to_dict()
        self.calls.append(
            ChatCall(
                "respondToInteraction",
                {"interaction_id": interaction_id, "body": payload, "key": idempotency_key},
            )
        )
        self._fault("respondToInteraction")
        self.responses.append((interaction_id, payload, idempotency_key))

    # -- views ------------------------------------------------------------------------------

    def calls_of(self, operation: str) -> list[ChatCall]:
        """The calls of one operation."""
        return [c for c in self.calls if c.operation == operation]

    def thread_titled(self, prefix: str) -> FakeThread | None:
        """The first thread whose title starts with ``prefix``."""
        for thread in self.threads.values():
            if thread.title.startswith(prefix):
                return thread
        return None

    def card_of(self, thread: FakeThread) -> dict[str, Any] | None:
        """The last card message of a thread, as sent."""
        cards = [m for m in thread.messages if m["kind"] == "card"]
        return cards[-1]["sent"] if cards else None

    def message_id_of_card(self, thread: FakeThread) -> str:
        """The message ID of the last card of a thread."""
        cards = [m for m in thread.messages if m["kind"] == "card"]
        return str(cards[-1]["id"])


def envelope(event_type: str, payload: dict[str, Any], *, channel_id: str) -> dict[str, Any]:
    """The envelope of a server event (the members the app reads plus ``sync_epoch``)."""
    return {
        "id": str(uuid7()),
        "type": event_type,
        "schema_version": 1,
        "occurred_at": datetime.now(UTC).isoformat().replace("+00:00", "Z"),
        "scope": {"kind": "channel", "id": channel_id},
        "resource": {"kind": "message", "id": str(uuid7())},
        "sync_epoch": str(uuid7()),
        "payload": payload,
    }


def body_of(event: dict[str, Any]) -> bytes:
    """The JSON body of an event."""
    return json.dumps(event).encode("utf-8")


def transport_error() -> Exception:
    """A network failure after the SDK's retries."""
    return httpx.ConnectError("connection refused")


def is_uuid(text: str) -> bool:
    """Whether ``text`` is a UUID."""
    try:
        UUID(text)
    except ValueError:
        return False
    return True
