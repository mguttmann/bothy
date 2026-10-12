from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.view_as_snapshot import ViewAsSnapshot
from ...types import Response


def _get_kwargs(
    user_id: UUID,
) -> dict[str, Any]:

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/users/{user_id}/view-as".format(
            user_id=quote(str(user_id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | ViewAsSnapshot:
    if response.status_code == 200:
        response_200 = ViewAsSnapshot.from_dict(response.json())

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
) -> Response[Problem | ViewAsSnapshot]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    user_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem | ViewAsSnapshot]:
    """See the instance as a person sees it

     Owner only, with step-up and audit. Returns the spaces and channels the person sees, with level and
    the end of the view (`authz.view_as_max_duration`). The person's direct messages stay invisible
    (SR6). In M1 the view lives only within this request (07, 9.2).

    Args:
        user_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | ViewAsSnapshot]
    """

    kwargs = _get_kwargs(
        user_id=user_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    user_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Problem | ViewAsSnapshot | None:
    """See the instance as a person sees it

     Owner only, with step-up and audit. Returns the spaces and channels the person sees, with level and
    the end of the view (`authz.view_as_max_duration`). The person's direct messages stay invisible
    (SR6). In M1 the view lives only within this request (07, 9.2).

    Args:
        user_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | ViewAsSnapshot
    """

    return sync_detailed(
        user_id=user_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    user_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem | ViewAsSnapshot]:
    """See the instance as a person sees it

     Owner only, with step-up and audit. Returns the spaces and channels the person sees, with level and
    the end of the view (`authz.view_as_max_duration`). The person's direct messages stay invisible
    (SR6). In M1 the view lives only within this request (07, 9.2).

    Args:
        user_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | ViewAsSnapshot]
    """

    kwargs = _get_kwargs(
        user_id=user_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    user_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Problem | ViewAsSnapshot | None:
    """See the instance as a person sees it

     Owner only, with step-up and audit. Returns the spaces and channels the person sees, with level and
    the end of the view (`authz.view_as_max_duration`). The person's direct messages stay invisible
    (SR6). In M1 the view lives only within this request (07, 9.2).

    Args:
        user_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | ViewAsSnapshot
    """

    return (
        await asyncio_detailed(
            user_id=user_id,
            client=client,
        )
    ).parsed
