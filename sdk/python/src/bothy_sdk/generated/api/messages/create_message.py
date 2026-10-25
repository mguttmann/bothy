from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.message import Message
from ...models.message_create import MessageCreate
from ...models.problem import Problem
from ...types import Response


def _get_kwargs(
    thread_id: UUID,
    *,
    body: MessageCreate,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/threads/{thread_id}/messages".format(
            thread_id=quote(str(thread_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Message | Problem:
    if response.status_code == 200:
        response_200 = Message.from_dict(response.json())

        return response_200

    if response.status_code == 201:
        response_201 = Message.from_dict(response.json())

        return response_201

    if response.status_code == 400:
        response_400 = Problem.from_dict(response.json())

        return response_400

    if response.status_code == 401:
        response_401 = Problem.from_dict(response.json())

        return response_401

    if response.status_code == 403:
        response_403 = Problem.from_dict(response.json())

        return response_403

    if response.status_code == 404:
        response_404 = Problem.from_dict(response.json())

        return response_404

    if response.status_code == 409:
        response_409 = Problem.from_dict(response.json())

        return response_409

    if response.status_code == 413:
        response_413 = Problem.from_dict(response.json())

        return response_413

    if response.status_code == 422:
        response_422 = Problem.from_dict(response.json())

        return response_422

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Message | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    thread_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: MessageCreate,
) -> Response[Message | Problem]:
    """Send a message

     Idempotent by `(author, client_msg_id)`: a repeat with the same body returns the original with
    200 and `Idempotent-Replay: true`; with a different body 422 `idempotency_key_reuse`. Sending
    into a closed thread reopens it. Agent invocation and follow-ups are evaluated after storing;
    denial sets `trigger_state = failed` with `trigger_state_reason`. When the message triggers an
    agent while one of its attachments is not final (scan or check still running), the agent
    invocation waits until all attachments are final or until `file.invocation_hold_max` has
    passed; `trigger_state` stays `received` meanwhile. A voice message (`kind = voice`, 12 8.1)
    needs one ready audio attachment: a file whose upload or scan has not ended answers `409
    file_not_ready`, a blocked or failed file `409 file_blocked`, and nothing is stored. A voice
    message that triggers an agent without a transcription path gets `trigger_state = failed`
    with `voice_unsupported` (09, 13); with the server transcript (`voice.transcription_mode =
    service`) the invocation waits until the transcript is `done` or `failed`, at most
    `voice.transcript_hold_max`.

    Args:
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (MessageCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Message | Problem]
    """

    kwargs = _get_kwargs(
        thread_id=thread_id,
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    thread_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: MessageCreate,
) -> Message | Problem | None:
    """Send a message

     Idempotent by `(author, client_msg_id)`: a repeat with the same body returns the original with
    200 and `Idempotent-Replay: true`; with a different body 422 `idempotency_key_reuse`. Sending
    into a closed thread reopens it. Agent invocation and follow-ups are evaluated after storing;
    denial sets `trigger_state = failed` with `trigger_state_reason`. When the message triggers an
    agent while one of its attachments is not final (scan or check still running), the agent
    invocation waits until all attachments are final or until `file.invocation_hold_max` has
    passed; `trigger_state` stays `received` meanwhile. A voice message (`kind = voice`, 12 8.1)
    needs one ready audio attachment: a file whose upload or scan has not ended answers `409
    file_not_ready`, a blocked or failed file `409 file_blocked`, and nothing is stored. A voice
    message that triggers an agent without a transcription path gets `trigger_state = failed`
    with `voice_unsupported` (09, 13); with the server transcript (`voice.transcription_mode =
    service`) the invocation waits until the transcript is `done` or `failed`, at most
    `voice.transcript_hold_max`.

    Args:
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (MessageCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Message | Problem
    """

    return sync_detailed(
        thread_id=thread_id,
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    thread_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: MessageCreate,
) -> Response[Message | Problem]:
    """Send a message

     Idempotent by `(author, client_msg_id)`: a repeat with the same body returns the original with
    200 and `Idempotent-Replay: true`; with a different body 422 `idempotency_key_reuse`. Sending
    into a closed thread reopens it. Agent invocation and follow-ups are evaluated after storing;
    denial sets `trigger_state = failed` with `trigger_state_reason`. When the message triggers an
    agent while one of its attachments is not final (scan or check still running), the agent
    invocation waits until all attachments are final or until `file.invocation_hold_max` has
    passed; `trigger_state` stays `received` meanwhile. A voice message (`kind = voice`, 12 8.1)
    needs one ready audio attachment: a file whose upload or scan has not ended answers `409
    file_not_ready`, a blocked or failed file `409 file_blocked`, and nothing is stored. A voice
    message that triggers an agent without a transcription path gets `trigger_state = failed`
    with `voice_unsupported` (09, 13); with the server transcript (`voice.transcription_mode =
    service`) the invocation waits until the transcript is `done` or `failed`, at most
    `voice.transcript_hold_max`.

    Args:
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (MessageCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Message | Problem]
    """

    kwargs = _get_kwargs(
        thread_id=thread_id,
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    thread_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: MessageCreate,
) -> Message | Problem | None:
    """Send a message

     Idempotent by `(author, client_msg_id)`: a repeat with the same body returns the original with
    200 and `Idempotent-Replay: true`; with a different body 422 `idempotency_key_reuse`. Sending
    into a closed thread reopens it. Agent invocation and follow-ups are evaluated after storing;
    denial sets `trigger_state = failed` with `trigger_state_reason`. When the message triggers an
    agent while one of its attachments is not final (scan or check still running), the agent
    invocation waits until all attachments are final or until `file.invocation_hold_max` has
    passed; `trigger_state` stays `received` meanwhile. A voice message (`kind = voice`, 12 8.1)
    needs one ready audio attachment: a file whose upload or scan has not ended answers `409
    file_not_ready`, a blocked or failed file `409 file_blocked`, and nothing is stored. A voice
    message that triggers an agent without a transcription path gets `trigger_state = failed`
    with `voice_unsupported` (09, 13); with the server transcript (`voice.transcription_mode =
    service`) the invocation waits until the transcript is `done` or `failed`, at most
    `voice.transcript_hold_max`.

    Args:
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (MessageCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Message | Problem
    """

    return (
        await asyncio_detailed(
            thread_id=thread_id,
            client=client,
            body=body,
        )
    ).parsed
