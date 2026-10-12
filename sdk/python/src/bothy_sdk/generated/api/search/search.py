import datetime
from http import HTTPStatus
from typing import Any
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.search_hit_page import SearchHitPage
from ...models.search_sort import SearchSort
from ...models.search_type import SearchType
from ...models.thread_state import ThreadState
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    q: str,
    type_: SearchType | Unset = SearchType.MESSAGES,
    sort: SearchSort | Unset = SearchSort.RELEVANCE,
    space_id: list[UUID] | Unset = UNSET,
    channel_id: list[UUID] | Unset = UNSET,
    thread_id: UUID | Unset = UNSET,
    author_id: UUID | Unset = UNSET,
    agent_id: UUID | Unset = UNSET,
    after: datetime.datetime | Unset = UNSET,
    before: datetime.datetime | Unset = UNSET,
    has_file: bool | Unset = UNSET,
    thread_state: ThreadState | Unset = UNSET,
    tag_id: list[UUID] | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["cursor"] = cursor

    params["limit"] = limit

    params["q"] = q

    json_type_: str | Unset = UNSET
    if not isinstance(type_, Unset):
        json_type_ = type_.value

    params["type"] = json_type_

    json_sort: str | Unset = UNSET
    if not isinstance(sort, Unset):
        json_sort = sort.value

    params["sort"] = json_sort

    json_space_id: list[str] | Unset = UNSET
    if not isinstance(space_id, Unset):
        json_space_id = []
        for space_id_item_data in space_id:
            space_id_item = str(space_id_item_data)
            json_space_id.append(space_id_item)

    params["space_id"] = json_space_id

    json_channel_id: list[str] | Unset = UNSET
    if not isinstance(channel_id, Unset):
        json_channel_id = []
        for channel_id_item_data in channel_id:
            channel_id_item = str(channel_id_item_data)
            json_channel_id.append(channel_id_item)

    params["channel_id"] = json_channel_id

    json_thread_id: str | Unset = UNSET
    if not isinstance(thread_id, Unset):
        json_thread_id = str(thread_id)
    params["thread_id"] = json_thread_id

    json_author_id: str | Unset = UNSET
    if not isinstance(author_id, Unset):
        json_author_id = str(author_id)
    params["author_id"] = json_author_id

    json_agent_id: str | Unset = UNSET
    if not isinstance(agent_id, Unset):
        json_agent_id = str(agent_id)
    params["agent_id"] = json_agent_id

    json_after: str | Unset = UNSET
    if not isinstance(after, Unset):
        json_after = after.isoformat()
    params["after"] = json_after

    json_before: str | Unset = UNSET
    if not isinstance(before, Unset):
        json_before = before.isoformat()
    params["before"] = json_before

    params["has_file"] = has_file

    json_thread_state: str | Unset = UNSET
    if not isinstance(thread_state, Unset):
        json_thread_state = thread_state.value

    params["thread_state"] = json_thread_state

    json_tag_id: list[str] | Unset = UNSET
    if not isinstance(tag_id, Unset):
        json_tag_id = []
        for tag_id_item_data in tag_id:
            tag_id_item = str(tag_id_item_data)
            json_tag_id.append(tag_id_item)

    params["tag_id"] = json_tag_id

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/search",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | SearchHitPage:
    if response.status_code == 200:
        response_200 = SearchHitPage.from_dict(response.json())

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
) -> Response[Problem | SearchHitPage]:
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
    q: str,
    type_: SearchType | Unset = SearchType.MESSAGES,
    sort: SearchSort | Unset = SearchSort.RELEVANCE,
    space_id: list[UUID] | Unset = UNSET,
    channel_id: list[UUID] | Unset = UNSET,
    thread_id: UUID | Unset = UNSET,
    author_id: UUID | Unset = UNSET,
    agent_id: UUID | Unset = UNSET,
    after: datetime.datetime | Unset = UNSET,
    before: datetime.datetime | Unset = UNSET,
    has_file: bool | Unset = UNSET,
    thread_state: ThreadState | Unset = UNSET,
    tag_id: list[UUID] | Unset = UNSET,
) -> Response[Problem | SearchHitPage]:
    """Full-text search

     Results are filtered by `message.read`, `thread.read` and `file.read` inside the query; counts
    are not returned.

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        q (str):
        type_ (SearchType | Unset):  Default: SearchType.MESSAGES.
        sort (SearchSort | Unset):  Default: SearchSort.RELEVANCE.
        space_id (list[UUID] | Unset):
        channel_id (list[UUID] | Unset):
        thread_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        author_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        after (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision
            and suffix `Z`.
        before (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision
            and suffix `Z`.
        has_file (bool | Unset):
        thread_state (ThreadState | Unset):
        tag_id (list[UUID] | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | SearchHitPage]
    """

    kwargs = _get_kwargs(
        cursor=cursor,
        limit=limit,
        q=q,
        type_=type_,
        sort=sort,
        space_id=space_id,
        channel_id=channel_id,
        thread_id=thread_id,
        author_id=author_id,
        agent_id=agent_id,
        after=after,
        before=before,
        has_file=has_file,
        thread_state=thread_state,
        tag_id=tag_id,
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
    q: str,
    type_: SearchType | Unset = SearchType.MESSAGES,
    sort: SearchSort | Unset = SearchSort.RELEVANCE,
    space_id: list[UUID] | Unset = UNSET,
    channel_id: list[UUID] | Unset = UNSET,
    thread_id: UUID | Unset = UNSET,
    author_id: UUID | Unset = UNSET,
    agent_id: UUID | Unset = UNSET,
    after: datetime.datetime | Unset = UNSET,
    before: datetime.datetime | Unset = UNSET,
    has_file: bool | Unset = UNSET,
    thread_state: ThreadState | Unset = UNSET,
    tag_id: list[UUID] | Unset = UNSET,
) -> Problem | SearchHitPage | None:
    """Full-text search

     Results are filtered by `message.read`, `thread.read` and `file.read` inside the query; counts
    are not returned.

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        q (str):
        type_ (SearchType | Unset):  Default: SearchType.MESSAGES.
        sort (SearchSort | Unset):  Default: SearchSort.RELEVANCE.
        space_id (list[UUID] | Unset):
        channel_id (list[UUID] | Unset):
        thread_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        author_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        after (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision
            and suffix `Z`.
        before (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision
            and suffix `Z`.
        has_file (bool | Unset):
        thread_state (ThreadState | Unset):
        tag_id (list[UUID] | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | SearchHitPage
    """

    return sync_detailed(
        client=client,
        cursor=cursor,
        limit=limit,
        q=q,
        type_=type_,
        sort=sort,
        space_id=space_id,
        channel_id=channel_id,
        thread_id=thread_id,
        author_id=author_id,
        agent_id=agent_id,
        after=after,
        before=before,
        has_file=has_file,
        thread_state=thread_state,
        tag_id=tag_id,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    q: str,
    type_: SearchType | Unset = SearchType.MESSAGES,
    sort: SearchSort | Unset = SearchSort.RELEVANCE,
    space_id: list[UUID] | Unset = UNSET,
    channel_id: list[UUID] | Unset = UNSET,
    thread_id: UUID | Unset = UNSET,
    author_id: UUID | Unset = UNSET,
    agent_id: UUID | Unset = UNSET,
    after: datetime.datetime | Unset = UNSET,
    before: datetime.datetime | Unset = UNSET,
    has_file: bool | Unset = UNSET,
    thread_state: ThreadState | Unset = UNSET,
    tag_id: list[UUID] | Unset = UNSET,
) -> Response[Problem | SearchHitPage]:
    """Full-text search

     Results are filtered by `message.read`, `thread.read` and `file.read` inside the query; counts
    are not returned.

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        q (str):
        type_ (SearchType | Unset):  Default: SearchType.MESSAGES.
        sort (SearchSort | Unset):  Default: SearchSort.RELEVANCE.
        space_id (list[UUID] | Unset):
        channel_id (list[UUID] | Unset):
        thread_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        author_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        after (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision
            and suffix `Z`.
        before (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision
            and suffix `Z`.
        has_file (bool | Unset):
        thread_state (ThreadState | Unset):
        tag_id (list[UUID] | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | SearchHitPage]
    """

    kwargs = _get_kwargs(
        cursor=cursor,
        limit=limit,
        q=q,
        type_=type_,
        sort=sort,
        space_id=space_id,
        channel_id=channel_id,
        thread_id=thread_id,
        author_id=author_id,
        agent_id=agent_id,
        after=after,
        before=before,
        has_file=has_file,
        thread_state=thread_state,
        tag_id=tag_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    q: str,
    type_: SearchType | Unset = SearchType.MESSAGES,
    sort: SearchSort | Unset = SearchSort.RELEVANCE,
    space_id: list[UUID] | Unset = UNSET,
    channel_id: list[UUID] | Unset = UNSET,
    thread_id: UUID | Unset = UNSET,
    author_id: UUID | Unset = UNSET,
    agent_id: UUID | Unset = UNSET,
    after: datetime.datetime | Unset = UNSET,
    before: datetime.datetime | Unset = UNSET,
    has_file: bool | Unset = UNSET,
    thread_state: ThreadState | Unset = UNSET,
    tag_id: list[UUID] | Unset = UNSET,
) -> Problem | SearchHitPage | None:
    """Full-text search

     Results are filtered by `message.read`, `thread.read` and `file.read` inside the query; counts
    are not returned.

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        q (str):
        type_ (SearchType | Unset):  Default: SearchType.MESSAGES.
        sort (SearchSort | Unset):  Default: SearchSort.RELEVANCE.
        space_id (list[UUID] | Unset):
        channel_id (list[UUID] | Unset):
        thread_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        author_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        after (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision
            and suffix `Z`.
        before (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision
            and suffix `Z`.
        has_file (bool | Unset):
        thread_state (ThreadState | Unset):
        tag_id (list[UUID] | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | SearchHitPage
    """

    return (
        await asyncio_detailed(
            client=client,
            cursor=cursor,
            limit=limit,
            q=q,
            type_=type_,
            sort=sort,
            space_id=space_id,
            channel_id=channel_id,
            thread_id=thread_id,
            author_id=author_id,
            agent_id=agent_id,
            after=after,
            before=before,
            has_file=has_file,
            thread_state=thread_state,
            tag_id=tag_id,
        )
    ).parsed
