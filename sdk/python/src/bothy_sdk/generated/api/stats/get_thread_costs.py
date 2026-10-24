from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.cost_breakdown import CostBreakdown
from ...models.problem import Problem
from ...types import Response


def _get_kwargs(
    thread_id: UUID,
) -> dict[str, Any]:

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/threads/{thread_id}/costs".format(
            thread_id=quote(str(thread_id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> CostBreakdown | Problem:
    if response.status_code == 200:
        response_200 = CostBreakdown.from_dict(response.json())

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
) -> Response[CostBreakdown | Problem]:
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
) -> Response[CostBreakdown | Problem]:
    """Costs of a thread per run

     Valid cost entries of the thread (entries no correction supersedes), whatever channel they were
    booked in, per run (by `run_id` ascending, unpaginated) and summed per billing mode and currency
    without conversion. `tokens` are always present; amounts of runs in mode `personal` (their `costs`
    and their totals) appear only when the server key `billing.show_personal_estimates` is on (10,
    12.2). Never a person: costs stand per run (20, 9). Without read on the thread the same 404 as for
    an unknown thread. A thread without entries answers empty lists.

    Args:
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[CostBreakdown | Problem]
    """

    kwargs = _get_kwargs(
        thread_id=thread_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    thread_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> CostBreakdown | Problem | None:
    """Costs of a thread per run

     Valid cost entries of the thread (entries no correction supersedes), whatever channel they were
    booked in, per run (by `run_id` ascending, unpaginated) and summed per billing mode and currency
    without conversion. `tokens` are always present; amounts of runs in mode `personal` (their `costs`
    and their totals) appear only when the server key `billing.show_personal_estimates` is on (10,
    12.2). Never a person: costs stand per run (20, 9). Without read on the thread the same 404 as for
    an unknown thread. A thread without entries answers empty lists.

    Args:
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        CostBreakdown | Problem
    """

    return sync_detailed(
        thread_id=thread_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    thread_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[CostBreakdown | Problem]:
    """Costs of a thread per run

     Valid cost entries of the thread (entries no correction supersedes), whatever channel they were
    booked in, per run (by `run_id` ascending, unpaginated) and summed per billing mode and currency
    without conversion. `tokens` are always present; amounts of runs in mode `personal` (their `costs`
    and their totals) appear only when the server key `billing.show_personal_estimates` is on (10,
    12.2). Never a person: costs stand per run (20, 9). Without read on the thread the same 404 as for
    an unknown thread. A thread without entries answers empty lists.

    Args:
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[CostBreakdown | Problem]
    """

    kwargs = _get_kwargs(
        thread_id=thread_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    thread_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> CostBreakdown | Problem | None:
    """Costs of a thread per run

     Valid cost entries of the thread (entries no correction supersedes), whatever channel they were
    booked in, per run (by `run_id` ascending, unpaginated) and summed per billing mode and currency
    without conversion. `tokens` are always present; amounts of runs in mode `personal` (their `costs`
    and their totals) appear only when the server key `billing.show_personal_estimates` is on (10,
    12.2). Never a person: costs stand per run (20, 9). Without read on the thread the same 404 as for
    an unknown thread. A thread without entries answers empty lists.

    Args:
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        CostBreakdown | Problem
    """

    return (
        await asyncio_detailed(
            thread_id=thread_id,
            client=client,
        )
    ).parsed
