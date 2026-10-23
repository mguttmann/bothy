from http import HTTPStatus
from typing import Any
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.permission_hint_kind import PermissionHintKind
from ...models.permission_hint_page import PermissionHintPage
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    space_id: UUID | Unset = UNSET,
    kind: PermissionHintKind | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["cursor"] = cursor

    params["limit"] = limit

    json_space_id: str | Unset = UNSET
    if not isinstance(space_id, Unset):
        json_space_id = str(space_id)
    params["space_id"] = json_space_id

    json_kind: str | Unset = UNSET
    if not isinstance(kind, Unset):
        json_kind = kind.value

    params["kind"] = json_kind

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/access/hints",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> PermissionHintPage | Problem:
    if response.status_code == 200:
        response_200 = PermissionHintPage.from_dict(response.json())

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
) -> Response[PermissionHintPage | Problem]:
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
    kind: PermissionHintKind | Unset = UNSET,
) -> Response[PermissionHintPage | Problem]:
    """List the permission hints

     The list "Permission hints" of 07 section 9.5, computed from the permission state that `authorize`
    uses: `h1_read_horizon` for every agent, person of its trigger set and channel of its read horizon
    the person cannot read (07 section 6), `grant_to_disabled` for grants whose subject is disabled,
    `trigger_blocked` for `trigger` grants whose triggering SR2 or SR3 blocks on every operated channel
    they reach, `agent_personal` for agents in mode `personal` with their trigger set. Ordered by kind
    in this order, then by agent, person and channel, by grant ID or by agent ID. Without `space_id` the
    instance (owners); with it the space (owner or `manage`), limited to its agents, channels and
    grants. A space the caller cannot read gives `404`, readable without `manage` `403`. Hints never
    name content; `labels` carries the names of the principals, groups, spaces and channels of the page.

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        kind (PermissionHintKind | Unset): Kind of a permission hint (07 section 9.5), in the
            order of the list.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[PermissionHintPage | Problem]
    """

    kwargs = _get_kwargs(
        cursor=cursor,
        limit=limit,
        space_id=space_id,
        kind=kind,
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
    kind: PermissionHintKind | Unset = UNSET,
) -> PermissionHintPage | Problem | None:
    """List the permission hints

     The list "Permission hints" of 07 section 9.5, computed from the permission state that `authorize`
    uses: `h1_read_horizon` for every agent, person of its trigger set and channel of its read horizon
    the person cannot read (07 section 6), `grant_to_disabled` for grants whose subject is disabled,
    `trigger_blocked` for `trigger` grants whose triggering SR2 or SR3 blocks on every operated channel
    they reach, `agent_personal` for agents in mode `personal` with their trigger set. Ordered by kind
    in this order, then by agent, person and channel, by grant ID or by agent ID. Without `space_id` the
    instance (owners); with it the space (owner or `manage`), limited to its agents, channels and
    grants. A space the caller cannot read gives `404`, readable without `manage` `403`. Hints never
    name content; `labels` carries the names of the principals, groups, spaces and channels of the page.

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        kind (PermissionHintKind | Unset): Kind of a permission hint (07 section 9.5), in the
            order of the list.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        PermissionHintPage | Problem
    """

    return sync_detailed(
        client=client,
        cursor=cursor,
        limit=limit,
        space_id=space_id,
        kind=kind,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    space_id: UUID | Unset = UNSET,
    kind: PermissionHintKind | Unset = UNSET,
) -> Response[PermissionHintPage | Problem]:
    """List the permission hints

     The list "Permission hints" of 07 section 9.5, computed from the permission state that `authorize`
    uses: `h1_read_horizon` for every agent, person of its trigger set and channel of its read horizon
    the person cannot read (07 section 6), `grant_to_disabled` for grants whose subject is disabled,
    `trigger_blocked` for `trigger` grants whose triggering SR2 or SR3 blocks on every operated channel
    they reach, `agent_personal` for agents in mode `personal` with their trigger set. Ordered by kind
    in this order, then by agent, person and channel, by grant ID or by agent ID. Without `space_id` the
    instance (owners); with it the space (owner or `manage`), limited to its agents, channels and
    grants. A space the caller cannot read gives `404`, readable without `manage` `403`. Hints never
    name content; `labels` carries the names of the principals, groups, spaces and channels of the page.

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        kind (PermissionHintKind | Unset): Kind of a permission hint (07 section 9.5), in the
            order of the list.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[PermissionHintPage | Problem]
    """

    kwargs = _get_kwargs(
        cursor=cursor,
        limit=limit,
        space_id=space_id,
        kind=kind,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    space_id: UUID | Unset = UNSET,
    kind: PermissionHintKind | Unset = UNSET,
) -> PermissionHintPage | Problem | None:
    """List the permission hints

     The list "Permission hints" of 07 section 9.5, computed from the permission state that `authorize`
    uses: `h1_read_horizon` for every agent, person of its trigger set and channel of its read horizon
    the person cannot read (07 section 6), `grant_to_disabled` for grants whose subject is disabled,
    `trigger_blocked` for `trigger` grants whose triggering SR2 or SR3 blocks on every operated channel
    they reach, `agent_personal` for agents in mode `personal` with their trigger set. Ordered by kind
    in this order, then by agent, person and channel, by grant ID or by agent ID. Without `space_id` the
    instance (owners); with it the space (owner or `manage`), limited to its agents, channels and
    grants. A space the caller cannot read gives `404`, readable without `manage` `403`. Hints never
    name content; `labels` carries the names of the principals, groups, spaces and channels of the page.

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        kind (PermissionHintKind | Unset): Kind of a permission hint (07 section 9.5), in the
            order of the list.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        PermissionHintPage | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            cursor=cursor,
            limit=limit,
            space_id=space_id,
            kind=kind,
        )
    ).parsed
