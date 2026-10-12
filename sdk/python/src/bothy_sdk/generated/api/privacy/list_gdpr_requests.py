from http import HTTPStatus
from typing import Any
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.gdpr_request_page import GdprRequestPage
from ...models.gdpr_request_state import GdprRequestState
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    subject_user_id: UUID | Unset = UNSET,
    state: GdprRequestState | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["cursor"] = cursor

    params["limit"] = limit

    json_subject_user_id: str | Unset = UNSET
    if not isinstance(subject_user_id, Unset):
        json_subject_user_id = str(subject_user_id)
    params["subject_user_id"] = json_subject_user_id

    json_state: str | Unset = UNSET
    if not isinstance(state, Unset):
        json_state = state.value

    params["state"] = json_state

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/gdpr-requests",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> GdprRequestPage | Problem:
    if response.status_code == 200:
        response_200 = GdprRequestPage.from_dict(response.json())

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
) -> Response[GdprRequestPage | Problem]:
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
    subject_user_id: UUID | Unset = UNSET,
    state: GdprRequestState | Unset = UNSET,
) -> Response[GdprRequestPage | Problem]:
    """List privacy requests

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        subject_user_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        state (GdprRequestState | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[GdprRequestPage | Problem]
    """

    kwargs = _get_kwargs(
        cursor=cursor,
        limit=limit,
        subject_user_id=subject_user_id,
        state=state,
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
    subject_user_id: UUID | Unset = UNSET,
    state: GdprRequestState | Unset = UNSET,
) -> GdprRequestPage | Problem | None:
    """List privacy requests

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        subject_user_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        state (GdprRequestState | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        GdprRequestPage | Problem
    """

    return sync_detailed(
        client=client,
        cursor=cursor,
        limit=limit,
        subject_user_id=subject_user_id,
        state=state,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    subject_user_id: UUID | Unset = UNSET,
    state: GdprRequestState | Unset = UNSET,
) -> Response[GdprRequestPage | Problem]:
    """List privacy requests

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        subject_user_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        state (GdprRequestState | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[GdprRequestPage | Problem]
    """

    kwargs = _get_kwargs(
        cursor=cursor,
        limit=limit,
        subject_user_id=subject_user_id,
        state=state,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    subject_user_id: UUID | Unset = UNSET,
    state: GdprRequestState | Unset = UNSET,
) -> GdprRequestPage | Problem | None:
    """List privacy requests

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        subject_user_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        state (GdprRequestState | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        GdprRequestPage | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            cursor=cursor,
            limit=limit,
            subject_user_id=subject_user_id,
            state=state,
        )
    ).parsed
