# SPDX-License-Identifier: Apache-2.0
"""The app's port to the server (brief M2-50 Design note 10).

The few operations of the public API the app uses, typed with the generated models of
`bothy_sdk.generated` (no own type repeats a schema, ADR 0026 "Confirmation").
`jira_app.chat.sdk_adapter.SdkChatPort` implements it on the SDK `Client`; unit tests use an
in-memory implementation. Failures are the SDK's errors: `bothy_sdk.ProblemError` (and its
classes) for an answer, ``httpx.TransportError`` when the network failed after the SDK's
retries.

Examples:
    >>> from bothy_sdk.generated.models.message_create import MessageCreate
    >>> MessageCreate.__name__
    'MessageCreate'
"""

from __future__ import annotations

from typing import Protocol

from bothy_sdk.generated.models.instance_limits import InstanceLimits
from bothy_sdk.generated.models.interaction_respond import InteractionRespond
from bothy_sdk.generated.models.message import Message
from bothy_sdk.generated.models.message_create import MessageCreate
from bothy_sdk.generated.models.principal_ref import PrincipalRef
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey

__all__ = ["ChatPort"]


class ChatPort(Protocol):
    """The operations of the public API the app uses."""

    async def me(self) -> PrincipalRef:
        """``getMe``: the app's own principal.

        Returns:
            The principal.
        """
        ...

    async def limits(self) -> InstanceLimits:
        """``getInstance``: the limits of the instance.

        Returns:
            The limits.
        """
        ...

    async def webhook_keys(self) -> list[Ed25519PublicKey]:
        """The trusted ``v1a`` keys of ``getWebhookKeys`` (the SDK's ``WebhookKeyCache``).

        Returns:
            The keys.
        """
        ...

    async def create_forum_thread(
        self, channel_id: str, title: str, first_message: MessageCreate, idempotency_key: str
    ) -> str:
        """``createThread`` in a forum with a first message, in one call.

        Args:
            channel_id: The forum.
            title: The title, already cleaned and cut.
            first_message: The first message; its ``client_msg_id`` makes the call idempotent.
            idempotency_key: ``Idempotency-Key`` stored in the job before the first attempt.

        Returns:
            The ID of the new thread (the original on a replay).
        """
        ...

    async def post_message(self, thread_id: str, message: MessageCreate) -> Message:
        """``createMessage`` into a thread.

        Args:
            thread_id: The thread.
            message: The message with its stored ``client_msg_id``.

        Returns:
            The message (the original on a replay).
        """
        ...

    async def get_message(self, message_id: str) -> Message:
        """``getMessage``: the full message.

        Args:
            message_id: The message.

        Returns:
            The message.
        """
        ...

    async def respond_interaction(
        self, interaction_id: str, body: InteractionRespond, idempotency_key: str
    ) -> None:
        """``respondToInteraction``.

        Args:
            interaction_id: The interaction.
            body: State and response text.
            idempotency_key: ``Idempotency-Key`` stored in the job before the first attempt.
        """
        ...
