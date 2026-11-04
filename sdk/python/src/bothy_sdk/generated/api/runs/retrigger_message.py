from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.message import Message
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    message_id: UUID,
    *,
    idempotency_key: UUID | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(idempotency_key, Unset):
        headers["Idempotency-Key"] = idempotency_key

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/messages/{message_id}/retrigger".format(
            message_id=quote(str(message_id), safe=""),
        ),
    }

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Message | Problem:
    if response.status_code == 200:
        response_200 = Message.from_dict(response.json())

        return response_200

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
    message_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[Message | Problem]:
    """Trigger a failed message again

     Runs the trigger decision (09 3.4) again for an own message whose `trigger_state` is `failed` with
    `trigger_state_reason` `project_not_ready`, `operator_cannot_serve_project` or
    `operator_not_in_channel`, after the cause was fixed (a runner set as operator, a folder created),
    so the person does not have to write the message again (09 3.7, addendum of 2026-11-04). Only the
    author may do this (`403 forbidden` with `reason` `not_self` otherwise); a message that is not
    readable is `404`; another state or reason, or a deleted message, is `409 invalid_state_transition`.
    When the message's thread is a thread whose fixed operator is not the channel's effective operator
    and never completed a run in it, the thread takes the channel's effective operator first (an
    exception to R-DM-12, recorded as `thread.updated`). Then the acceptance runs as for a new message:
    `agent.invoke` in request mode for the caller (authz audits a denial), and the message's
    `trigger_state` and `trigger_state_reason` follow it (`received` and a queued run, or `failed` with
    the reason of now). The answer is the message after the decision. Repeating the request after a
    successful retrigger answers `409 invalid_state_transition`, because the message is no longer
    `failed`.

    Args:
        message_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Message | Problem]
    """

    kwargs = _get_kwargs(
        message_id=message_id,
        idempotency_key=idempotency_key,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    message_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    idempotency_key: UUID | Unset = UNSET,
) -> Message | Problem | None:
    """Trigger a failed message again

     Runs the trigger decision (09 3.4) again for an own message whose `trigger_state` is `failed` with
    `trigger_state_reason` `project_not_ready`, `operator_cannot_serve_project` or
    `operator_not_in_channel`, after the cause was fixed (a runner set as operator, a folder created),
    so the person does not have to write the message again (09 3.7, addendum of 2026-11-04). Only the
    author may do this (`403 forbidden` with `reason` `not_self` otherwise); a message that is not
    readable is `404`; another state or reason, or a deleted message, is `409 invalid_state_transition`.
    When the message's thread is a thread whose fixed operator is not the channel's effective operator
    and never completed a run in it, the thread takes the channel's effective operator first (an
    exception to R-DM-12, recorded as `thread.updated`). Then the acceptance runs as for a new message:
    `agent.invoke` in request mode for the caller (authz audits a denial), and the message's
    `trigger_state` and `trigger_state_reason` follow it (`received` and a queued run, or `failed` with
    the reason of now). The answer is the message after the decision. Repeating the request after a
    successful retrigger answers `409 invalid_state_transition`, because the message is no longer
    `failed`.

    Args:
        message_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Message | Problem
    """

    return sync_detailed(
        message_id=message_id,
        client=client,
        idempotency_key=idempotency_key,
    ).parsed


async def asyncio_detailed(
    message_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[Message | Problem]:
    """Trigger a failed message again

     Runs the trigger decision (09 3.4) again for an own message whose `trigger_state` is `failed` with
    `trigger_state_reason` `project_not_ready`, `operator_cannot_serve_project` or
    `operator_not_in_channel`, after the cause was fixed (a runner set as operator, a folder created),
    so the person does not have to write the message again (09 3.7, addendum of 2026-11-04). Only the
    author may do this (`403 forbidden` with `reason` `not_self` otherwise); a message that is not
    readable is `404`; another state or reason, or a deleted message, is `409 invalid_state_transition`.
    When the message's thread is a thread whose fixed operator is not the channel's effective operator
    and never completed a run in it, the thread takes the channel's effective operator first (an
    exception to R-DM-12, recorded as `thread.updated`). Then the acceptance runs as for a new message:
    `agent.invoke` in request mode for the caller (authz audits a denial), and the message's
    `trigger_state` and `trigger_state_reason` follow it (`received` and a queued run, or `failed` with
    the reason of now). The answer is the message after the decision. Repeating the request after a
    successful retrigger answers `409 invalid_state_transition`, because the message is no longer
    `failed`.

    Args:
        message_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Message | Problem]
    """

    kwargs = _get_kwargs(
        message_id=message_id,
        idempotency_key=idempotency_key,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    message_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    idempotency_key: UUID | Unset = UNSET,
) -> Message | Problem | None:
    """Trigger a failed message again

     Runs the trigger decision (09 3.4) again for an own message whose `trigger_state` is `failed` with
    `trigger_state_reason` `project_not_ready`, `operator_cannot_serve_project` or
    `operator_not_in_channel`, after the cause was fixed (a runner set as operator, a folder created),
    so the person does not have to write the message again (09 3.7, addendum of 2026-11-04). Only the
    author may do this (`403 forbidden` with `reason` `not_self` otherwise); a message that is not
    readable is `404`; another state or reason, or a deleted message, is `409 invalid_state_transition`.
    When the message's thread is a thread whose fixed operator is not the channel's effective operator
    and never completed a run in it, the thread takes the channel's effective operator first (an
    exception to R-DM-12, recorded as `thread.updated`). Then the acceptance runs as for a new message:
    `agent.invoke` in request mode for the caller (authz audits a denial), and the message's
    `trigger_state` and `trigger_state_reason` follow it (`received` and a queued run, or `failed` with
    the reason of now). The answer is the message after the decision. Repeating the request after a
    successful retrigger answers `409 invalid_state_transition`, because the message is no longer
    `failed`.

    Args:
        message_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Message | Problem
    """

    return (
        await asyncio_detailed(
            message_id=message_id,
            client=client,
            idempotency_key=idempotency_key,
        )
    ).parsed
