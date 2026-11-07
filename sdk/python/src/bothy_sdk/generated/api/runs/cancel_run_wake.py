from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.run_wake import RunWake
from ...models.run_wake_cancel import RunWakeCancel
from ...types import UNSET, Response, Unset


def _get_kwargs(
    wake_id: UUID,
    *,
    body: RunWakeCancel,
    idempotency_key: UUID | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(idempotency_key, Unset):
        headers["Idempotency-Key"] = idempotency_key

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/wakes/{wake_id}/cancel".format(
            wake_id=quote(str(wake_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | RunWake:
    if response.status_code == 200:
        response_200 = RunWake.from_dict(response.json())

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
) -> Response[Problem | RunWake]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    wake_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: RunWakeCancel,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[Problem | RunWake]:
    """Cancel a pending wake-up

     Ends a pending wake-up `cancelled` (09 5.8.2): the agent of the wake-up (for example with `bothy-
    wake --cancel` or `usage_cleared`), the wake-up's person, or a person with `manage` on the channel.
    The thread gets the note; a cancel sends no notification (09 5.8.4). A wake-up that already ended
    answers itself without a change. Audited.

    Args:
        wake_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (RunWakeCancel):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | RunWake]
    """

    kwargs = _get_kwargs(
        wake_id=wake_id,
        body=body,
        idempotency_key=idempotency_key,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    wake_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: RunWakeCancel,
    idempotency_key: UUID | Unset = UNSET,
) -> Problem | RunWake | None:
    """Cancel a pending wake-up

     Ends a pending wake-up `cancelled` (09 5.8.2): the agent of the wake-up (for example with `bothy-
    wake --cancel` or `usage_cleared`), the wake-up's person, or a person with `manage` on the channel.
    The thread gets the note; a cancel sends no notification (09 5.8.4). A wake-up that already ended
    answers itself without a change. Audited.

    Args:
        wake_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (RunWakeCancel):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | RunWake
    """

    return sync_detailed(
        wake_id=wake_id,
        client=client,
        body=body,
        idempotency_key=idempotency_key,
    ).parsed


async def asyncio_detailed(
    wake_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: RunWakeCancel,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[Problem | RunWake]:
    """Cancel a pending wake-up

     Ends a pending wake-up `cancelled` (09 5.8.2): the agent of the wake-up (for example with `bothy-
    wake --cancel` or `usage_cleared`), the wake-up's person, or a person with `manage` on the channel.
    The thread gets the note; a cancel sends no notification (09 5.8.4). A wake-up that already ended
    answers itself without a change. Audited.

    Args:
        wake_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (RunWakeCancel):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | RunWake]
    """

    kwargs = _get_kwargs(
        wake_id=wake_id,
        body=body,
        idempotency_key=idempotency_key,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    wake_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: RunWakeCancel,
    idempotency_key: UUID | Unset = UNSET,
) -> Problem | RunWake | None:
    """Cancel a pending wake-up

     Ends a pending wake-up `cancelled` (09 5.8.2): the agent of the wake-up (for example with `bothy-
    wake --cancel` or `usage_cleared`), the wake-up's person, or a person with `manage` on the channel.
    The thread gets the note; a cancel sends no notification (09 5.8.4). A wake-up that already ended
    answers itself without a change. Audited.

    Args:
        wake_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (RunWakeCancel):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | RunWake
    """

    return (
        await asyncio_detailed(
            wake_id=wake_id,
            client=client,
            body=body,
            idempotency_key=idempotency_key,
        )
    ).parsed
