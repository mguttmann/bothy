from http import HTTPStatus
from typing import Any

import httpx

from ...client import AuthenticatedClient, Client
from ...models.computer_session_list import ComputerSessionList
from ...models.computer_session_state import ComputerSessionState
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    state: ComputerSessionState | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    json_state: str | Unset = UNSET
    if not isinstance(state, Unset):
        json_state = state.value

    params["state"] = json_state

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/computer/sessions",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ComputerSessionList | Problem:
    if response.status_code == 200:
        response_200 = ComputerSessionList.from_dict(response.json())

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
) -> Response[ComputerSessionList | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    state: ComputerSessionState | Unset = UNSET,
) -> Response[ComputerSessionList | Problem]:
    """List one's own computer sessions

     The computer sessions the caller triggered as an active owner, newest first, at most
    `api.page_size_max` (docs/spec/27-computer-use.md 12.2, 07 2.15 item 5). Metadata only; never an
    image, the token or its `jti`.

    Args:
        state (ComputerSessionState | Unset): State of a computer session (docs/spec/27-computer-
            use.md 8.1).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ComputerSessionList | Problem]
    """

    kwargs = _get_kwargs(
        state=state,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    state: ComputerSessionState | Unset = UNSET,
) -> ComputerSessionList | Problem | None:
    """List one's own computer sessions

     The computer sessions the caller triggered as an active owner, newest first, at most
    `api.page_size_max` (docs/spec/27-computer-use.md 12.2, 07 2.15 item 5). Metadata only; never an
    image, the token or its `jti`.

    Args:
        state (ComputerSessionState | Unset): State of a computer session (docs/spec/27-computer-
            use.md 8.1).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ComputerSessionList | Problem
    """

    return sync_detailed(
        client=client,
        state=state,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    state: ComputerSessionState | Unset = UNSET,
) -> Response[ComputerSessionList | Problem]:
    """List one's own computer sessions

     The computer sessions the caller triggered as an active owner, newest first, at most
    `api.page_size_max` (docs/spec/27-computer-use.md 12.2, 07 2.15 item 5). Metadata only; never an
    image, the token or its `jti`.

    Args:
        state (ComputerSessionState | Unset): State of a computer session (docs/spec/27-computer-
            use.md 8.1).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ComputerSessionList | Problem]
    """

    kwargs = _get_kwargs(
        state=state,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    state: ComputerSessionState | Unset = UNSET,
) -> ComputerSessionList | Problem | None:
    """List one's own computer sessions

     The computer sessions the caller triggered as an active owner, newest first, at most
    `api.page_size_max` (docs/spec/27-computer-use.md 12.2, 07 2.15 item 5). Metadata only; never an
    image, the token or its `jti`.

    Args:
        state (ComputerSessionState | Unset): State of a computer session (docs/spec/27-computer-
            use.md 8.1).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ComputerSessionList | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            state=state,
        )
    ).parsed
