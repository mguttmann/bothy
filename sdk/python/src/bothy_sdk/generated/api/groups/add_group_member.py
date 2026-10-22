from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.group_member import GroupMember
from ...models.group_member_add import GroupMemberAdd
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    group_id: UUID,
    *,
    body: GroupMemberAdd,
    idempotency_key: UUID | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(idempotency_key, Unset):
        headers["Idempotency-Key"] = idempotency_key

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/groups/{group_id}/members".format(
            group_id=quote(str(group_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> GroupMember | Problem:
    if response.status_code == 201:
        response_201 = GroupMember.from_dict(response.json())

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
) -> Response[GroupMember | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    group_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: GroupMemberAdd,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[GroupMember | Problem]:
    """Add a member

     Optional `resolve_i1` runs the guided flow (05 rule 1.6, 10 section 11): the listed agents are
    authorized with `agent.billing_mode.set` and switched to `organization` first, in the same
    transaction as the change; I1 is checked before the commit, and either both parts commit or none.

    Args:
        group_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (GroupMemberAdd):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[GroupMember | Problem]
    """

    kwargs = _get_kwargs(
        group_id=group_id,
        body=body,
        idempotency_key=idempotency_key,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    group_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: GroupMemberAdd,
    idempotency_key: UUID | Unset = UNSET,
) -> GroupMember | Problem | None:
    """Add a member

     Optional `resolve_i1` runs the guided flow (05 rule 1.6, 10 section 11): the listed agents are
    authorized with `agent.billing_mode.set` and switched to `organization` first, in the same
    transaction as the change; I1 is checked before the commit, and either both parts commit or none.

    Args:
        group_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (GroupMemberAdd):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        GroupMember | Problem
    """

    return sync_detailed(
        group_id=group_id,
        client=client,
        body=body,
        idempotency_key=idempotency_key,
    ).parsed


async def asyncio_detailed(
    group_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: GroupMemberAdd,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[GroupMember | Problem]:
    """Add a member

     Optional `resolve_i1` runs the guided flow (05 rule 1.6, 10 section 11): the listed agents are
    authorized with `agent.billing_mode.set` and switched to `organization` first, in the same
    transaction as the change; I1 is checked before the commit, and either both parts commit or none.

    Args:
        group_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (GroupMemberAdd):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[GroupMember | Problem]
    """

    kwargs = _get_kwargs(
        group_id=group_id,
        body=body,
        idempotency_key=idempotency_key,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    group_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: GroupMemberAdd,
    idempotency_key: UUID | Unset = UNSET,
) -> GroupMember | Problem | None:
    """Add a member

     Optional `resolve_i1` runs the guided flow (05 rule 1.6, 10 section 11): the listed agents are
    authorized with `agent.billing_mode.set` and switched to `organization` first, in the same
    transaction as the change; I1 is checked before the commit, and either both parts commit or none.

    Args:
        group_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (GroupMemberAdd):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        GroupMember | Problem
    """

    return (
        await asyncio_detailed(
            group_id=group_id,
            client=client,
            body=body,
            idempotency_key=idempotency_key,
        )
    ).parsed
