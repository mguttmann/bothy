from http import HTTPStatus
from typing import Any

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.thread_title_hit_list import ThreadTitleHitList
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    limit: int | Unset = UNSET,
    q: str,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["limit"] = limit

    params["q"] = q

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/search/thread-titles",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | ThreadTitleHitList:
    if response.status_code == 200:
        response_200 = ThreadTitleHitList.from_dict(response.json())

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
) -> Response[Problem | ThreadTitleHitList]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    limit: int | Unset = UNSET,
    q: str,
) -> Response[Problem | ThreadTitleHitList]:
    """Prefix search over thread titles

     For the quick switcher (14, 4.9): words shorter than `search.prefix_min_chars` are dropped; same
    access filter as `/search`, newest activity first.

    Args:
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        q (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | ThreadTitleHitList]
    """

    kwargs = _get_kwargs(
        limit=limit,
        q=q,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    limit: int | Unset = UNSET,
    q: str,
) -> Problem | ThreadTitleHitList | None:
    """Prefix search over thread titles

     For the quick switcher (14, 4.9): words shorter than `search.prefix_min_chars` are dropped; same
    access filter as `/search`, newest activity first.

    Args:
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        q (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | ThreadTitleHitList
    """

    return sync_detailed(
        client=client,
        limit=limit,
        q=q,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    limit: int | Unset = UNSET,
    q: str,
) -> Response[Problem | ThreadTitleHitList]:
    """Prefix search over thread titles

     For the quick switcher (14, 4.9): words shorter than `search.prefix_min_chars` are dropped; same
    access filter as `/search`, newest activity first.

    Args:
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        q (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | ThreadTitleHitList]
    """

    kwargs = _get_kwargs(
        limit=limit,
        q=q,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    limit: int | Unset = UNSET,
    q: str,
) -> Problem | ThreadTitleHitList | None:
    """Prefix search over thread titles

     For the quick switcher (14, 4.9): words shorter than `search.prefix_min_chars` are dropped; same
    access filter as `/search`, newest activity first.

    Args:
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        q (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | ThreadTitleHitList
    """

    return (
        await asyncio_detailed(
            client=client,
            limit=limit,
            q=q,
        )
    ).parsed
