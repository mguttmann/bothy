from http import HTTPStatus
from typing import Any
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.grant_page import GrantPage
from ...models.grant_subject_kind import GrantSubjectKind
from ...models.problem import Problem
from ...models.target_kind import TargetKind
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    target_kind: TargetKind | Unset = UNSET,
    target_id: UUID | Unset = UNSET,
    subject_kind: GrantSubjectKind | Unset = UNSET,
    subject_id: UUID | Unset = UNSET,
    include_revoked: bool | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["cursor"] = cursor

    params["limit"] = limit

    json_target_kind: str | Unset = UNSET
    if not isinstance(target_kind, Unset):
        json_target_kind = target_kind.value

    params["target_kind"] = json_target_kind

    json_target_id: str | Unset = UNSET
    if not isinstance(target_id, Unset):
        json_target_id = str(target_id)
    params["target_id"] = json_target_id

    json_subject_kind: str | Unset = UNSET
    if not isinstance(subject_kind, Unset):
        json_subject_kind = subject_kind.value

    params["subject_kind"] = json_subject_kind

    json_subject_id: str | Unset = UNSET
    if not isinstance(subject_id, Unset):
        json_subject_id = str(subject_id)
    params["subject_id"] = json_subject_id

    params["include_revoked"] = include_revoked

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/grants",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> GrantPage | Problem:
    if response.status_code == 200:
        response_200 = GrantPage.from_dict(response.json())

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

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[GrantPage | Problem]:
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
    target_kind: TargetKind | Unset = UNSET,
    target_id: UUID | Unset = UNSET,
    subject_kind: GrantSubjectKind | Unset = UNSET,
    subject_id: UUID | Unset = UNSET,
    include_revoked: bool | Unset = UNSET,
) -> Response[GrantPage | Problem]:
    """List grants of a target or subject

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        target_kind (TargetKind | Unset): `host` is a connected host for the terminal; it has no
            level and only the capability `terminal` (24, 8.1).
        target_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        subject_kind (GrantSubjectKind | Unset):
        subject_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        include_revoked (bool | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[GrantPage | Problem]
    """

    kwargs = _get_kwargs(
        cursor=cursor,
        limit=limit,
        target_kind=target_kind,
        target_id=target_id,
        subject_kind=subject_kind,
        subject_id=subject_id,
        include_revoked=include_revoked,
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
    target_kind: TargetKind | Unset = UNSET,
    target_id: UUID | Unset = UNSET,
    subject_kind: GrantSubjectKind | Unset = UNSET,
    subject_id: UUID | Unset = UNSET,
    include_revoked: bool | Unset = UNSET,
) -> GrantPage | Problem | None:
    """List grants of a target or subject

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        target_kind (TargetKind | Unset): `host` is a connected host for the terminal; it has no
            level and only the capability `terminal` (24, 8.1).
        target_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        subject_kind (GrantSubjectKind | Unset):
        subject_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        include_revoked (bool | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        GrantPage | Problem
    """

    return sync_detailed(
        client=client,
        cursor=cursor,
        limit=limit,
        target_kind=target_kind,
        target_id=target_id,
        subject_kind=subject_kind,
        subject_id=subject_id,
        include_revoked=include_revoked,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    target_kind: TargetKind | Unset = UNSET,
    target_id: UUID | Unset = UNSET,
    subject_kind: GrantSubjectKind | Unset = UNSET,
    subject_id: UUID | Unset = UNSET,
    include_revoked: bool | Unset = UNSET,
) -> Response[GrantPage | Problem]:
    """List grants of a target or subject

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        target_kind (TargetKind | Unset): `host` is a connected host for the terminal; it has no
            level and only the capability `terminal` (24, 8.1).
        target_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        subject_kind (GrantSubjectKind | Unset):
        subject_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        include_revoked (bool | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[GrantPage | Problem]
    """

    kwargs = _get_kwargs(
        cursor=cursor,
        limit=limit,
        target_kind=target_kind,
        target_id=target_id,
        subject_kind=subject_kind,
        subject_id=subject_id,
        include_revoked=include_revoked,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    target_kind: TargetKind | Unset = UNSET,
    target_id: UUID | Unset = UNSET,
    subject_kind: GrantSubjectKind | Unset = UNSET,
    subject_id: UUID | Unset = UNSET,
    include_revoked: bool | Unset = UNSET,
) -> GrantPage | Problem | None:
    """List grants of a target or subject

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        target_kind (TargetKind | Unset): `host` is a connected host for the terminal; it has no
            level and only the capability `terminal` (24, 8.1).
        target_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        subject_kind (GrantSubjectKind | Unset):
        subject_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        include_revoked (bool | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        GrantPage | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            cursor=cursor,
            limit=limit,
            target_kind=target_kind,
            target_id=target_id,
            subject_kind=subject_kind,
            subject_id=subject_id,
            include_revoked=include_revoked,
        )
    ).parsed
