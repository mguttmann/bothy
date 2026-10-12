from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.message_page import MessagePage
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    thread_id: UUID,
    *,
    before_seq: int | Unset = UNSET,
    after_seq: int | Unset = UNSET,
    around_seq: int | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["before_seq"] = before_seq

    params["after_seq"] = after_seq

    params["around_seq"] = around_seq

    params["limit"] = limit

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/threads/{thread_id}/messages".format(
            thread_id=quote(str(thread_id), safe=""),
        ),
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> MessagePage | Problem:
    if response.status_code == 200:
        response_200 = MessagePage.from_dict(response.json())

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
) -> Response[MessagePage | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    thread_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    before_seq: int | Unset = UNSET,
    after_seq: int | Unset = UNSET,
    around_seq: int | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> Response[MessagePage | Problem]:
    """Read thread history

     At most one of `before_seq`, `after_seq`, `around_seq`. Without any, returns the newest page.
    Gaps in `thread_seq` mean missing data, tombstones fill deleted positions.

    Args:
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        before_seq (int | Unset):
        after_seq (int | Unset):
        around_seq (int | Unset):
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[MessagePage | Problem]
    """

    kwargs = _get_kwargs(
        thread_id=thread_id,
        before_seq=before_seq,
        after_seq=after_seq,
        around_seq=around_seq,
        limit=limit,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    thread_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    before_seq: int | Unset = UNSET,
    after_seq: int | Unset = UNSET,
    around_seq: int | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> MessagePage | Problem | None:
    """Read thread history

     At most one of `before_seq`, `after_seq`, `around_seq`. Without any, returns the newest page.
    Gaps in `thread_seq` mean missing data, tombstones fill deleted positions.

    Args:
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        before_seq (int | Unset):
        after_seq (int | Unset):
        around_seq (int | Unset):
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        MessagePage | Problem
    """

    return sync_detailed(
        thread_id=thread_id,
        client=client,
        before_seq=before_seq,
        after_seq=after_seq,
        around_seq=around_seq,
        limit=limit,
    ).parsed


async def asyncio_detailed(
    thread_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    before_seq: int | Unset = UNSET,
    after_seq: int | Unset = UNSET,
    around_seq: int | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> Response[MessagePage | Problem]:
    """Read thread history

     At most one of `before_seq`, `after_seq`, `around_seq`. Without any, returns the newest page.
    Gaps in `thread_seq` mean missing data, tombstones fill deleted positions.

    Args:
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        before_seq (int | Unset):
        after_seq (int | Unset):
        around_seq (int | Unset):
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[MessagePage | Problem]
    """

    kwargs = _get_kwargs(
        thread_id=thread_id,
        before_seq=before_seq,
        after_seq=after_seq,
        around_seq=around_seq,
        limit=limit,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    thread_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    before_seq: int | Unset = UNSET,
    after_seq: int | Unset = UNSET,
    around_seq: int | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> MessagePage | Problem | None:
    """Read thread history

     At most one of `before_seq`, `after_seq`, `around_seq`. Without any, returns the newest page.
    Gaps in `thread_seq` mean missing data, tombstones fill deleted positions.

    Args:
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        before_seq (int | Unset):
        after_seq (int | Unset):
        around_seq (int | Unset):
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        MessagePage | Problem
    """

    return (
        await asyncio_detailed(
            thread_id=thread_id,
            client=client,
            before_seq=before_seq,
            after_seq=after_seq,
            around_seq=around_seq,
            limit=limit,
        )
    ).parsed
