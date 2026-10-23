from http import HTTPStatus
from typing import Any
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.expiring_access_page import ExpiringAccessPage
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    space_id: UUID | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["cursor"] = cursor

    params["limit"] = limit

    json_space_id: str | Unset = UNSET
    if not isinstance(space_id, Unset):
        json_space_id = str(space_id)
    params["space_id"] = json_space_id

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/access/expiring",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ExpiringAccessPage | Problem:
    if response.status_code == 200:
        response_200 = ExpiringAccessPage.from_dict(response.json())

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

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[ExpiringAccessPage | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    space_id: UUID | Unset = UNSET,
) -> Response[ExpiringAccessPage | Problem]:
    """List the access that expires soon

     The list "Expiring soon" of 07 section 9.4: grants that are not revoked and memberships that are not
    removed (of groups that are not deleted) whose `expires_at` lies within `authz.expiry_reminder_lead`
    from now, and for owners the API tokens of agents and apps that are not revoked and expire within
    the same lead (never personal tokens, never a token value), ordered by expiry, then kind, then ID.
    "Extend" is a revocation plus a new grant with a new reason (07 section 2.5), a token is renewed by
    creating a new one and revoking the old one. Without `space_id` the instance (owners); with it the
    space (owner or `manage`): only grants on the space and its channels and memberships of groups that
    hold such a grant, no tokens. A space the caller cannot read gives `404`, readable without `manage`
    `403`.

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ExpiringAccessPage | Problem]
    """

    kwargs = _get_kwargs(
        cursor=cursor,
        limit=limit,
        space_id=space_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    space_id: UUID | Unset = UNSET,
) -> ExpiringAccessPage | Problem | None:
    """List the access that expires soon

     The list "Expiring soon" of 07 section 9.4: grants that are not revoked and memberships that are not
    removed (of groups that are not deleted) whose `expires_at` lies within `authz.expiry_reminder_lead`
    from now, and for owners the API tokens of agents and apps that are not revoked and expire within
    the same lead (never personal tokens, never a token value), ordered by expiry, then kind, then ID.
    "Extend" is a revocation plus a new grant with a new reason (07 section 2.5), a token is renewed by
    creating a new one and revoking the old one. Without `space_id` the instance (owners); with it the
    space (owner or `manage`): only grants on the space and its channels and memberships of groups that
    hold such a grant, no tokens. A space the caller cannot read gives `404`, readable without `manage`
    `403`.

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ExpiringAccessPage | Problem
    """

    return sync_detailed(
        client=client,
        cursor=cursor,
        limit=limit,
        space_id=space_id,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    space_id: UUID | Unset = UNSET,
) -> Response[ExpiringAccessPage | Problem]:
    """List the access that expires soon

     The list "Expiring soon" of 07 section 9.4: grants that are not revoked and memberships that are not
    removed (of groups that are not deleted) whose `expires_at` lies within `authz.expiry_reminder_lead`
    from now, and for owners the API tokens of agents and apps that are not revoked and expire within
    the same lead (never personal tokens, never a token value), ordered by expiry, then kind, then ID.
    "Extend" is a revocation plus a new grant with a new reason (07 section 2.5), a token is renewed by
    creating a new one and revoking the old one. Without `space_id` the instance (owners); with it the
    space (owner or `manage`): only grants on the space and its channels and memberships of groups that
    hold such a grant, no tokens. A space the caller cannot read gives `404`, readable without `manage`
    `403`.

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ExpiringAccessPage | Problem]
    """

    kwargs = _get_kwargs(
        cursor=cursor,
        limit=limit,
        space_id=space_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    space_id: UUID | Unset = UNSET,
) -> ExpiringAccessPage | Problem | None:
    """List the access that expires soon

     The list "Expiring soon" of 07 section 9.4: grants that are not revoked and memberships that are not
    removed (of groups that are not deleted) whose `expires_at` lies within `authz.expiry_reminder_lead`
    from now, and for owners the API tokens of agents and apps that are not revoked and expire within
    the same lead (never personal tokens, never a token value), ordered by expiry, then kind, then ID.
    "Extend" is a revocation plus a new grant with a new reason (07 section 2.5), a token is renewed by
    creating a new one and revoking the old one. Without `space_id` the instance (owners); with it the
    space (owner or `manage`): only grants on the space and its channels and memberships of groups that
    hold such a grant, no tokens. A space the caller cannot read gives `404`, readable without `manage`
    `403`.

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ExpiringAccessPage | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            cursor=cursor,
            limit=limit,
            space_id=space_id,
        )
    ).parsed
