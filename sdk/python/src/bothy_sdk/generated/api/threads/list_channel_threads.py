from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.forum_sort import ForumSort
from ...models.problem import Problem
from ...models.thread_page import ThreadPage
from ...models.thread_state import ThreadState
from ...types import UNSET, Response, Unset


def _get_kwargs(
    channel_id: UUID,
    *,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    state: ThreadState | Unset = UNSET,
    tag_id: UUID | Unset = UNSET,
    sort: ForumSort | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["cursor"] = cursor

    params["limit"] = limit

    json_state: str | Unset = UNSET
    if not isinstance(state, Unset):
        json_state = state.value

    params["state"] = json_state

    json_tag_id: str | Unset = UNSET
    if not isinstance(tag_id, Unset):
        json_tag_id = str(tag_id)
    params["tag_id"] = json_tag_id

    json_sort: str | Unset = UNSET
    if not isinstance(sort, Unset):
        json_sort = sort.value

    params["sort"] = json_sort

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/channels/{channel_id}/threads".format(
            channel_id=quote(str(channel_id), safe=""),
        ),
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | ThreadPage:
    if response.status_code == 200:
        response_200 = ThreadPage.from_dict(response.json())

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
) -> Response[Problem | ThreadPage]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    channel_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    state: ThreadState | Unset = UNSET,
    tag_id: UUID | Unset = UNSET,
    sort: ForumSort | Unset = UNSET,
) -> Response[Problem | ThreadPage]:
    """List threads or forum posts of a channel

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        state (ThreadState | Unset):
        tag_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        sort (ForumSort | Unset): Sort order of forum posts.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | ThreadPage]
    """

    kwargs = _get_kwargs(
        channel_id=channel_id,
        cursor=cursor,
        limit=limit,
        state=state,
        tag_id=tag_id,
        sort=sort,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    channel_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    state: ThreadState | Unset = UNSET,
    tag_id: UUID | Unset = UNSET,
    sort: ForumSort | Unset = UNSET,
) -> Problem | ThreadPage | None:
    """List threads or forum posts of a channel

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        state (ThreadState | Unset):
        tag_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        sort (ForumSort | Unset): Sort order of forum posts.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | ThreadPage
    """

    return sync_detailed(
        channel_id=channel_id,
        client=client,
        cursor=cursor,
        limit=limit,
        state=state,
        tag_id=tag_id,
        sort=sort,
    ).parsed


async def asyncio_detailed(
    channel_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    state: ThreadState | Unset = UNSET,
    tag_id: UUID | Unset = UNSET,
    sort: ForumSort | Unset = UNSET,
) -> Response[Problem | ThreadPage]:
    """List threads or forum posts of a channel

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        state (ThreadState | Unset):
        tag_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        sort (ForumSort | Unset): Sort order of forum posts.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | ThreadPage]
    """

    kwargs = _get_kwargs(
        channel_id=channel_id,
        cursor=cursor,
        limit=limit,
        state=state,
        tag_id=tag_id,
        sort=sort,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    channel_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    state: ThreadState | Unset = UNSET,
    tag_id: UUID | Unset = UNSET,
    sort: ForumSort | Unset = UNSET,
) -> Problem | ThreadPage | None:
    """List threads or forum posts of a channel

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        state (ThreadState | Unset):
        tag_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        sort (ForumSort | Unset): Sort order of forum posts.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | ThreadPage
    """

    return (
        await asyncio_detailed(
            channel_id=channel_id,
            client=client,
            cursor=cursor,
            limit=limit,
            state=state,
            tag_id=tag_id,
            sort=sort,
        )
    ).parsed
