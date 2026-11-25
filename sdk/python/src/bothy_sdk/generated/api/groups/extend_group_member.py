from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.group_member import GroupMember
from ...models.group_member_extend import GroupMemberExtend
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    group_id: UUID,
    user_id: UUID,
    *,
    body: GroupMemberExtend,
    idempotency_key: UUID | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(idempotency_key, Unset):
        headers["Idempotency-Key"] = str(idempotency_key)

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/groups/{group_id}/members/{user_id}/extend".format(
            group_id=quote(str(group_id), safe=""),
            user_id=quote(str(user_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> GroupMember | Problem:
    if response.status_code == 200:
        response_200 = GroupMember.from_dict(response.json())

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
) -> Response[GroupMember | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    group_id: UUID,
    user_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: GroupMemberExtend,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[GroupMember | Problem]:
    """Extend a time-limited membership

     Gives the time-limited membership of the person that is in effect a later `expires_at`, on the same
    membership, so access never lapses (07 9.4). A membership without an expiry is `400
    validation_failed` at `/expires_at` with `not_time_limited`, an expiry not after the current one
    with `not_later`; a person without a membership in effect is `404`; a group with `source =
    oidc_claim` is `409 conflict` with `claim_managed_group`.

    Args:
        group_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        user_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (GroupMemberExtend): The later expiry of a time-limited membership in effect
            (`extendGroupMember`).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[GroupMember | Problem]
    """

    kwargs = _get_kwargs(
        group_id=group_id,
        user_id=user_id,
        body=body,
        idempotency_key=idempotency_key,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    group_id: UUID,
    user_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: GroupMemberExtend,
    idempotency_key: UUID | Unset = UNSET,
) -> GroupMember | Problem | None:
    """Extend a time-limited membership

     Gives the time-limited membership of the person that is in effect a later `expires_at`, on the same
    membership, so access never lapses (07 9.4). A membership without an expiry is `400
    validation_failed` at `/expires_at` with `not_time_limited`, an expiry not after the current one
    with `not_later`; a person without a membership in effect is `404`; a group with `source =
    oidc_claim` is `409 conflict` with `claim_managed_group`.

    Args:
        group_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        user_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (GroupMemberExtend): The later expiry of a time-limited membership in effect
            (`extendGroupMember`).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        GroupMember | Problem
    """

    return sync_detailed(
        group_id=group_id,
        user_id=user_id,
        client=client,
        body=body,
        idempotency_key=idempotency_key,
    ).parsed


async def asyncio_detailed(
    group_id: UUID,
    user_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: GroupMemberExtend,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[GroupMember | Problem]:
    """Extend a time-limited membership

     Gives the time-limited membership of the person that is in effect a later `expires_at`, on the same
    membership, so access never lapses (07 9.4). A membership without an expiry is `400
    validation_failed` at `/expires_at` with `not_time_limited`, an expiry not after the current one
    with `not_later`; a person without a membership in effect is `404`; a group with `source =
    oidc_claim` is `409 conflict` with `claim_managed_group`.

    Args:
        group_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        user_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (GroupMemberExtend): The later expiry of a time-limited membership in effect
            (`extendGroupMember`).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[GroupMember | Problem]
    """

    kwargs = _get_kwargs(
        group_id=group_id,
        user_id=user_id,
        body=body,
        idempotency_key=idempotency_key,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    group_id: UUID,
    user_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: GroupMemberExtend,
    idempotency_key: UUID | Unset = UNSET,
) -> GroupMember | Problem | None:
    """Extend a time-limited membership

     Gives the time-limited membership of the person that is in effect a later `expires_at`, on the same
    membership, so access never lapses (07 9.4). A membership without an expiry is `400
    validation_failed` at `/expires_at` with `not_time_limited`, an expiry not after the current one
    with `not_later`; a person without a membership in effect is `404`; a group with `source =
    oidc_claim` is `409 conflict` with `claim_managed_group`.

    Args:
        group_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        user_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (GroupMemberExtend): The later expiry of a time-limited membership in effect
            (`extendGroupMember`).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        GroupMember | Problem
    """

    return (
        await asyncio_detailed(
            group_id=group_id,
            user_id=user_id,
            client=client,
            body=body,
            idempotency_key=idempotency_key,
        )
    ).parsed
