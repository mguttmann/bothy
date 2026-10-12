import datetime
from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...types import UNSET, Response


def _get_kwargs(
    channel_id: UUID,
    *,
    from_: datetime.datetime,
    to: datetime.datetime,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    json_from_ = from_.isoformat()
    params["from"] = json_from_

    json_to = to.isoformat()
    params["to"] = json_to

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/channels/{channel_id}/export".format(
            channel_id=quote(str(channel_id), safe=""),
        ),
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem:
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
) -> Response[Problem]:
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
    from_: datetime.datetime,
    to: datetime.datetime,
) -> Response[Problem]:
    """Export the history of a channel

     Streams one `Message` per line in `thread_seq` order per thread. Requires step-up.

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        from_ (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        to (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem]
    """

    kwargs = _get_kwargs(
        channel_id=channel_id,
        from_=from_,
        to=to,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    channel_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    from_: datetime.datetime,
    to: datetime.datetime,
) -> Problem | None:
    """Export the history of a channel

     Streams one `Message` per line in `thread_seq` order per thread. Requires step-up.

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        from_ (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        to (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem
    """

    return sync_detailed(
        channel_id=channel_id,
        client=client,
        from_=from_,
        to=to,
    ).parsed


async def asyncio_detailed(
    channel_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    from_: datetime.datetime,
    to: datetime.datetime,
) -> Response[Problem]:
    """Export the history of a channel

     Streams one `Message` per line in `thread_seq` order per thread. Requires step-up.

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        from_ (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        to (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem]
    """

    kwargs = _get_kwargs(
        channel_id=channel_id,
        from_=from_,
        to=to,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    channel_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    from_: datetime.datetime,
    to: datetime.datetime,
) -> Problem | None:
    """Export the history of a channel

     Streams one `Message` per line in `thread_seq` order per thread. Requires step-up.

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        from_ (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        to (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem
    """

    return (
        await asyncio_detailed(
            channel_id=channel_id,
            client=client,
            from_=from_,
            to=to,
        )
    ).parsed
