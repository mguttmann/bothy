from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.command import Command
from ...models.command_register import CommandRegister
from ...models.problem import Problem
from ...types import Response


def _get_kwargs(
    command_id: UUID,
    *,
    body: CommandRegister,
    if_match: str,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    headers["If-Match"] = if_match

    _kwargs: dict[str, Any] = {
        "method": "put",
        "url": "/commands/{command_id}".format(
            command_id=quote(str(command_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Command | Problem:
    if response.status_code == 200:
        response_200 = Command.from_dict(response.json())

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
) -> Response[Command | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    command_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: CommandRegister,
    if_match: str,
) -> Response[Command | Problem]:
    """Replace a command definition

    Args:
        command_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str):
        body (CommandRegister):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Command | Problem]
    """

    kwargs = _get_kwargs(
        command_id=command_id,
        body=body,
        if_match=if_match,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    command_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: CommandRegister,
    if_match: str,
) -> Command | Problem | None:
    """Replace a command definition

    Args:
        command_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str):
        body (CommandRegister):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Command | Problem
    """

    return sync_detailed(
        command_id=command_id,
        client=client,
        body=body,
        if_match=if_match,
    ).parsed


async def asyncio_detailed(
    command_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: CommandRegister,
    if_match: str,
) -> Response[Command | Problem]:
    """Replace a command definition

    Args:
        command_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str):
        body (CommandRegister):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Command | Problem]
    """

    kwargs = _get_kwargs(
        command_id=command_id,
        body=body,
        if_match=if_match,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    command_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: CommandRegister,
    if_match: str,
) -> Command | Problem | None:
    """Replace a command definition

    Args:
        command_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str):
        body (CommandRegister):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Command | Problem
    """

    return (
        await asyncio_detailed(
            command_id=command_id,
            client=client,
            body=body,
            if_match=if_match,
        )
    ).parsed
