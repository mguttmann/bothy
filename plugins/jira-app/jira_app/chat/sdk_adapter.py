# SPDX-License-Identifier: Apache-2.0
"""`ChatPort` on the SDK `Client` (brief M2-50 Design note 10).

``getMe``, ``getInstance``, ``createThread``, ``createMessage``, ``getMessage`` and
``respondToInteraction`` go through the SDK `bothy_sdk.Client` with the app's token; the client
retries transient failures within ``sdk.retry_max_attempts``. Writes carry their stored
``client_msg_id`` or ``Idempotency-Key`` (through `bothy_sdk.Client.idempotency_key`, which the
SDK uses instead of a key of its own), so a retry or a rerun after a crash repeats the same
request.

The keys of ``getWebhookKeys`` come through one `bothy_sdk.webhooks.WebhookKeyCache` on a
generated client without the app's token: the operation is public (``security: []``) and no app
scope of 05 2.6 covers it, so the server would refuse the app's token.

Examples:
    >>> from bothy_sdk.generated.api.messages import create_message
    >>> callable(create_message.asyncio_detailed)
    True
"""

from __future__ import annotations

from uuid import UUID

from bothy_sdk.client import Client
from bothy_sdk.generated.api.admin import get_instance
from bothy_sdk.generated.api.agent_runtime import respond_to_interaction
from bothy_sdk.generated.api.me import get_me
from bothy_sdk.generated.api.messages import create_message, get_message
from bothy_sdk.generated.api.threads import create_thread
from bothy_sdk.generated.client import Client as CoreClient
from bothy_sdk.generated.models.instance_info import InstanceInfo
from bothy_sdk.generated.models.instance_limits import InstanceLimits
from bothy_sdk.generated.models.interaction_respond import InteractionRespond
from bothy_sdk.generated.models.me import Me
from bothy_sdk.generated.models.message import Message
from bothy_sdk.generated.models.message_create import MessageCreate
from bothy_sdk.generated.models.principal_ref import PrincipalRef
from bothy_sdk.generated.models.thread_create import ThreadCreate
from bothy_sdk.generated.models.thread_created import ThreadCreated
from bothy_sdk.webhooks import WebhookKeyCache
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey

__all__ = ["SdkChatPort"]


class SdkChatPort:
    """`jira_app.chat.port.ChatPort` over the SDK.

    Args:
        client: The SDK client with the app's token.
        public: A generated client of the same server without a token, for ``getWebhookKeys``.
    """

    def __init__(self, client: Client, public: CoreClient) -> None:
        """Create the adapter; see the class docstring."""
        self._client = client
        self._public = public
        self._keys = WebhookKeyCache(public, clock=client.clock, settings=client.settings)

    async def aclose(self) -> None:
        """Close the connections of both clients."""
        await self._client.aclose()
        await self._public.get_async_httpx_client().aclose()

    async def me(self) -> PrincipalRef:
        """``getMe``; see `jira_app.chat.port.ChatPort.me`."""
        return (await self._client.call_as(Me, get_me.asyncio_detailed)).principal

    async def limits(self) -> InstanceLimits:
        """``getInstance``; see `jira_app.chat.port.ChatPort.limits`."""
        return (await self._client.call_as(InstanceInfo, get_instance.asyncio_detailed)).limits

    async def webhook_keys(self) -> list[Ed25519PublicKey]:
        """The cached keys; see `jira_app.chat.port.ChatPort.webhook_keys`."""
        return await self._keys.keys()

    async def create_forum_thread(
        self, channel_id: str, title: str, first_message: MessageCreate, idempotency_key: str
    ) -> str:
        """``createThread``; see `jira_app.chat.port.ChatPort.create_forum_thread`."""
        with self._client.idempotency_key(idempotency_key):
            created = await self._client.call_as(
                ThreadCreated,
                create_thread.asyncio_detailed,
                channel_id=UUID(channel_id),
                body=ThreadCreate(title=title, first_message=first_message),
            )
        return str(created.thread.id)

    async def post_message(self, thread_id: str, message: MessageCreate) -> Message:
        """``createMessage``; see `jira_app.chat.port.ChatPort.post_message`."""
        return await self._client.call_as(
            Message, create_message.asyncio_detailed, thread_id=UUID(thread_id), body=message
        )

    async def get_message(self, message_id: str) -> Message:
        """``getMessage``; see `jira_app.chat.port.ChatPort.get_message`."""
        return await self._client.call_as(
            Message, get_message.asyncio_detailed, message_id=UUID(message_id)
        )

    async def respond_interaction(
        self, interaction_id: str, body: InteractionRespond, idempotency_key: str
    ) -> None:
        """``respondToInteraction``; see `jira_app.chat.port.ChatPort.respond_interaction`."""
        with self._client.idempotency_key(idempotency_key):
            await self._client.call(
                respond_to_interaction.asyncio_detailed,
                interaction_id=UUID(interaction_id),
                body=body,
            )
