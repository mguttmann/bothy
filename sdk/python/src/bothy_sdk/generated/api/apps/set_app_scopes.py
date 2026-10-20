from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.app import App
from ...models.app_scopes_put import AppScopesPut
from ...models.problem import Problem
from ...types import Response


def _get_kwargs(
    app_id: UUID,
    *,
    body: AppScopesPut,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "put",
        "url": "/apps/{app_id}/scopes".format(
            app_id=quote(str(app_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> App | Problem:
    if response.status_code == 200:
        response_200 = App.from_dict(response.json())

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

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[App | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    app_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: AppScopesPut,
) -> Response[App | Problem]:
    """Change granted scopes and subscriptions

     Owner only with a fresh step-up; the consent for new scopes and events. `granted_scopes` is a subset
    of the scopes of the manifest; `event_subscriptions` absent keeps the current ones the new scopes
    still allow, present it replaces them (each needs `events:subscribe` and the scope of the event). A
    removed scope acts on the next request of every token; a newly granted scope needs a new token,
    because a token keeps the scopes it was created with. Equal lists answer `200` without effect; an
    uninstalled app is `409 conflict`.

    Args:
        app_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (AppScopesPut):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[App | Problem]
    """

    kwargs = _get_kwargs(
        app_id=app_id,
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    app_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: AppScopesPut,
) -> App | Problem | None:
    """Change granted scopes and subscriptions

     Owner only with a fresh step-up; the consent for new scopes and events. `granted_scopes` is a subset
    of the scopes of the manifest; `event_subscriptions` absent keeps the current ones the new scopes
    still allow, present it replaces them (each needs `events:subscribe` and the scope of the event). A
    removed scope acts on the next request of every token; a newly granted scope needs a new token,
    because a token keeps the scopes it was created with. Equal lists answer `200` without effect; an
    uninstalled app is `409 conflict`.

    Args:
        app_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (AppScopesPut):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        App | Problem
    """

    return sync_detailed(
        app_id=app_id,
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    app_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: AppScopesPut,
) -> Response[App | Problem]:
    """Change granted scopes and subscriptions

     Owner only with a fresh step-up; the consent for new scopes and events. `granted_scopes` is a subset
    of the scopes of the manifest; `event_subscriptions` absent keeps the current ones the new scopes
    still allow, present it replaces them (each needs `events:subscribe` and the scope of the event). A
    removed scope acts on the next request of every token; a newly granted scope needs a new token,
    because a token keeps the scopes it was created with. Equal lists answer `200` without effect; an
    uninstalled app is `409 conflict`.

    Args:
        app_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (AppScopesPut):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[App | Problem]
    """

    kwargs = _get_kwargs(
        app_id=app_id,
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    app_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: AppScopesPut,
) -> App | Problem | None:
    """Change granted scopes and subscriptions

     Owner only with a fresh step-up; the consent for new scopes and events. `granted_scopes` is a subset
    of the scopes of the manifest; `event_subscriptions` absent keeps the current ones the new scopes
    still allow, present it replaces them (each needs `events:subscribe` and the scope of the event). A
    removed scope acts on the next request of every token; a newly granted scope needs a new token,
    because a token keeps the scopes it was created with. Equal lists answer `200` without effect; an
    uninstalled app is `409 conflict`.

    Args:
        app_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (AppScopesPut):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        App | Problem
    """

    return (
        await asyncio_detailed(
            app_id=app_id,
            client=client,
            body=body,
        )
    ).parsed
