from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.interaction import Interaction
from ...models.interaction_create import InteractionCreate
from ...models.problem import Problem
from ...types import Response


def _get_kwargs(
    component_id: UUID,
    *,
    body: InteractionCreate,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/components/{component_id}/interactions".format(
            component_id=quote(str(component_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Interaction | Problem:
    if response.status_code == 200:
        response_200 = Interaction.from_dict(response.json())

        return response_200

    if response.status_code == 201:
        response_201 = Interaction.from_dict(response.json())

        return response_201

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

    if response.status_code == 422:
        response_422 = Problem.from_dict(response.json())

        return response_422

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Interaction | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    component_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: InteractionCreate,
) -> Response[Interaction | Problem]:
    """Use a button, select or form

     Single-use components accept exactly one interaction; later ones get 409 `component_used`.
    Idempotent by `(principal, client_interaction_id)`.

    Args:
        component_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (InteractionCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Interaction | Problem]
    """

    kwargs = _get_kwargs(
        component_id=component_id,
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    component_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: InteractionCreate,
) -> Interaction | Problem | None:
    """Use a button, select or form

     Single-use components accept exactly one interaction; later ones get 409 `component_used`.
    Idempotent by `(principal, client_interaction_id)`.

    Args:
        component_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (InteractionCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Interaction | Problem
    """

    return sync_detailed(
        component_id=component_id,
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    component_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: InteractionCreate,
) -> Response[Interaction | Problem]:
    """Use a button, select or form

     Single-use components accept exactly one interaction; later ones get 409 `component_used`.
    Idempotent by `(principal, client_interaction_id)`.

    Args:
        component_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (InteractionCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Interaction | Problem]
    """

    kwargs = _get_kwargs(
        component_id=component_id,
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    component_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: InteractionCreate,
) -> Interaction | Problem | None:
    """Use a button, select or form

     Single-use components accept exactly one interaction; later ones get 409 `component_used`.
    Idempotent by `(principal, client_interaction_id)`.

    Args:
        component_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (InteractionCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Interaction | Problem
    """

    return (
        await asyncio_detailed(
            component_id=component_id,
            client=client,
            body=body,
        )
    ).parsed
