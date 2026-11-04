from http import HTTPStatus
from typing import Any

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.search_assist_query import SearchAssistQuery
from ...models.search_assist_result import SearchAssistResult
from ...types import Response


def _get_kwargs(
    *,
    body: SearchAssistQuery,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/search/assist",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | SearchAssistResult:
    if response.status_code == 200:
        response_200 = SearchAssistResult.from_dict(response.json())

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

    if response.status_code == 503:
        response_503 = Problem.from_dict(response.json())

        return response_503

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Problem | SearchAssistResult]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: SearchAssistQuery,
) -> Response[Problem | SearchAssistResult]:
    """Ask the search assistant

     Smart search (14, 4.10): the server turns `q` and the terms of `history` into search terms,
    runs the thread title and message search with the access filter of `/search`, fuses the hits
    per thread and, when `search.assist.agent_id` names an assistant agent with a live gateway
    connection, sends the candidates the agent may read too as `search.assist_requested` and
    waits up to `search.assist.timeout` for its picks. The answer holds at most
    `search.assist.max_hits` hits: the assistant's picks with a reason first, then full-text
    hits. Without an assistant answer `mode` is `fulltext` and `fallback_reason` says why.
    Nothing is stored; only persons may ask (agents and apps get `403`). `q` longer than
    `search.query_max_chars` or without a term is `400 validation_failed`; more than
    `search.assist.max_turns` turns in `history` is `too_many_items`. A statement timeout is
    `503 unavailable` with reason `search_timeout`.

    Args:
        body (SearchAssistQuery): One question to the search assistant (14, 4.10).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | SearchAssistResult]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    body: SearchAssistQuery,
) -> Problem | SearchAssistResult | None:
    """Ask the search assistant

     Smart search (14, 4.10): the server turns `q` and the terms of `history` into search terms,
    runs the thread title and message search with the access filter of `/search`, fuses the hits
    per thread and, when `search.assist.agent_id` names an assistant agent with a live gateway
    connection, sends the candidates the agent may read too as `search.assist_requested` and
    waits up to `search.assist.timeout` for its picks. The answer holds at most
    `search.assist.max_hits` hits: the assistant's picks with a reason first, then full-text
    hits. Without an assistant answer `mode` is `fulltext` and `fallback_reason` says why.
    Nothing is stored; only persons may ask (agents and apps get `403`). `q` longer than
    `search.query_max_chars` or without a term is `400 validation_failed`; more than
    `search.assist.max_turns` turns in `history` is `too_many_items`. A statement timeout is
    `503 unavailable` with reason `search_timeout`.

    Args:
        body (SearchAssistQuery): One question to the search assistant (14, 4.10).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | SearchAssistResult
    """

    return sync_detailed(
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: SearchAssistQuery,
) -> Response[Problem | SearchAssistResult]:
    """Ask the search assistant

     Smart search (14, 4.10): the server turns `q` and the terms of `history` into search terms,
    runs the thread title and message search with the access filter of `/search`, fuses the hits
    per thread and, when `search.assist.agent_id` names an assistant agent with a live gateway
    connection, sends the candidates the agent may read too as `search.assist_requested` and
    waits up to `search.assist.timeout` for its picks. The answer holds at most
    `search.assist.max_hits` hits: the assistant's picks with a reason first, then full-text
    hits. Without an assistant answer `mode` is `fulltext` and `fallback_reason` says why.
    Nothing is stored; only persons may ask (agents and apps get `403`). `q` longer than
    `search.query_max_chars` or without a term is `400 validation_failed`; more than
    `search.assist.max_turns` turns in `history` is `too_many_items`. A statement timeout is
    `503 unavailable` with reason `search_timeout`.

    Args:
        body (SearchAssistQuery): One question to the search assistant (14, 4.10).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | SearchAssistResult]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: SearchAssistQuery,
) -> Problem | SearchAssistResult | None:
    """Ask the search assistant

     Smart search (14, 4.10): the server turns `q` and the terms of `history` into search terms,
    runs the thread title and message search with the access filter of `/search`, fuses the hits
    per thread and, when `search.assist.agent_id` names an assistant agent with a live gateway
    connection, sends the candidates the agent may read too as `search.assist_requested` and
    waits up to `search.assist.timeout` for its picks. The answer holds at most
    `search.assist.max_hits` hits: the assistant's picks with a reason first, then full-text
    hits. Without an assistant answer `mode` is `fulltext` and `fallback_reason` says why.
    Nothing is stored; only persons may ask (agents and apps get `403`). `q` longer than
    `search.query_max_chars` or without a term is `400 validation_failed`; more than
    `search.assist.max_turns` turns in `history` is `too_many_items`. A statement timeout is
    `503 unavailable` with reason `search_timeout`.

    Args:
        body (SearchAssistQuery): One question to the search assistant (14, 4.10).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | SearchAssistResult
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
        )
    ).parsed
