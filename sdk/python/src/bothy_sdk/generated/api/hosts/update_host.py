from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.host import Host
from ...models.host_update import HostUpdate
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    host_id: UUID,
    *,
    body: HostUpdate,
    idempotency_key: UUID | Unset = UNSET,
    if_match: str,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(idempotency_key, Unset):
        headers["Idempotency-Key"] = idempotency_key

    headers["If-Match"] = if_match

    _kwargs: dict[str, Any] = {
        "method": "patch",
        "url": "/hosts/{host_id}".format(
            host_id=quote(str(host_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Host | Problem:
    if response.status_code == 200:
        response_200 = Host.from_dict(response.json())

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

    if response.status_code == 412:
        response_412 = Problem.from_dict(response.json())

        return response_412

    if response.status_code == 428:
        response_428 = Problem.from_dict(response.json())

        return response_428

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Host | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    host_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: HostUpdate,
    idempotency_key: UUID | Unset = UNSET,
    if_match: str,
) -> Response[Host | Problem]:
    """Change name or assignee of a host

     A new `assigned_user_id` ends the sessions of people who no longer qualify under SR10
    (`access_revoked`). Needs step-up.

    Args:
        host_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str):
        body (HostUpdate): Members that are absent stay unchanged. To remove the assignee send
            `clear_assigned_user` (no JSON null, ADR 0053).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Host | Problem]
    """

    kwargs = _get_kwargs(
        host_id=host_id,
        body=body,
        idempotency_key=idempotency_key,
        if_match=if_match,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    host_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: HostUpdate,
    idempotency_key: UUID | Unset = UNSET,
    if_match: str,
) -> Host | Problem | None:
    """Change name or assignee of a host

     A new `assigned_user_id` ends the sessions of people who no longer qualify under SR10
    (`access_revoked`). Needs step-up.

    Args:
        host_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str):
        body (HostUpdate): Members that are absent stay unchanged. To remove the assignee send
            `clear_assigned_user` (no JSON null, ADR 0053).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Host | Problem
    """

    return sync_detailed(
        host_id=host_id,
        client=client,
        body=body,
        idempotency_key=idempotency_key,
        if_match=if_match,
    ).parsed


async def asyncio_detailed(
    host_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: HostUpdate,
    idempotency_key: UUID | Unset = UNSET,
    if_match: str,
) -> Response[Host | Problem]:
    """Change name or assignee of a host

     A new `assigned_user_id` ends the sessions of people who no longer qualify under SR10
    (`access_revoked`). Needs step-up.

    Args:
        host_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str):
        body (HostUpdate): Members that are absent stay unchanged. To remove the assignee send
            `clear_assigned_user` (no JSON null, ADR 0053).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Host | Problem]
    """

    kwargs = _get_kwargs(
        host_id=host_id,
        body=body,
        idempotency_key=idempotency_key,
        if_match=if_match,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    host_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: HostUpdate,
    idempotency_key: UUID | Unset = UNSET,
    if_match: str,
) -> Host | Problem | None:
    """Change name or assignee of a host

     A new `assigned_user_id` ends the sessions of people who no longer qualify under SR10
    (`access_revoked`). Needs step-up.

    Args:
        host_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str):
        body (HostUpdate): Members that are absent stay unchanged. To remove the assignee send
            `clear_assigned_user` (no JSON null, ADR 0053).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Host | Problem
    """

    return (
        await asyncio_detailed(
            host_id=host_id,
            client=client,
            body=body,
            idempotency_key=idempotency_key,
            if_match=if_match,
        )
    ).parsed
