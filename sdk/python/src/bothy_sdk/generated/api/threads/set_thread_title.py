from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.thread import Thread
from ...models.thread_title_put import ThreadTitlePut
from ...types import Response


def _get_kwargs(
    thread_id: UUID,
    *,
    body: ThreadTitlePut,
    if_match: str,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    headers["If-Match"] = if_match

    _kwargs: dict[str, Any] = {
        "method": "put",
        "url": "/threads/{thread_id}/title".format(
            thread_id=quote(str(thread_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | Thread:
    if response.status_code == 200:
        response_200 = Thread.from_dict(response.json())

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
) -> Response[Problem | Thread]:
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
    body: ThreadTitlePut,
    if_match: str,
) -> Response[Problem | Thread]:
    """Set the title

     Every principal except the thread's operator agent sets `title_source = manual`. The operator agent
    may only replace an `auto` title, which stays `auto`; on a `manual` title it gets `409
    title_manual`.

    Args:
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str):
        body (ThreadTitlePut):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | Thread]
    """

    kwargs = _get_kwargs(
        thread_id=thread_id,
        body=body,
        if_match=if_match,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    thread_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: ThreadTitlePut,
    if_match: str,
) -> Problem | Thread | None:
    """Set the title

     Every principal except the thread's operator agent sets `title_source = manual`. The operator agent
    may only replace an `auto` title, which stays `auto`; on a `manual` title it gets `409
    title_manual`.

    Args:
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str):
        body (ThreadTitlePut):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | Thread
    """

    return sync_detailed(
        thread_id=thread_id,
        client=client,
        body=body,
        if_match=if_match,
    ).parsed


async def asyncio_detailed(
    thread_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: ThreadTitlePut,
    if_match: str,
) -> Response[Problem | Thread]:
    """Set the title

     Every principal except the thread's operator agent sets `title_source = manual`. The operator agent
    may only replace an `auto` title, which stays `auto`; on a `manual` title it gets `409
    title_manual`.

    Args:
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str):
        body (ThreadTitlePut):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | Thread]
    """

    kwargs = _get_kwargs(
        thread_id=thread_id,
        body=body,
        if_match=if_match,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    thread_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: ThreadTitlePut,
    if_match: str,
) -> Problem | Thread | None:
    """Set the title

     Every principal except the thread's operator agent sets `title_source = manual`. The operator agent
    may only replace an `auto` title, which stays `auto`; on a `manual` title it gets `409
    title_manual`.

    Args:
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str):
        body (ThreadTitlePut):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | Thread
    """

    return (
        await asyncio_detailed(
            thread_id=thread_id,
            client=client,
            body=body,
            if_match=if_match,
        )
    ).parsed
