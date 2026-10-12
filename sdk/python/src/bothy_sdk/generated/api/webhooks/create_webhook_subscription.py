from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.webhook_subscription_create import WebhookSubscriptionCreate
from ...models.webhook_subscription_created import WebhookSubscriptionCreated
from ...types import UNSET, Response, Unset


def _get_kwargs(
    app_id: UUID,
    *,
    body: WebhookSubscriptionCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(idempotency_key, Unset):
        headers["Idempotency-Key"] = idempotency_key

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/apps/{app_id}/webhooks".format(
            app_id=quote(str(app_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | WebhookSubscriptionCreated:
    if response.status_code == 201:
        response_201 = WebhookSubscriptionCreated.from_dict(response.json())

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
) -> Response[Problem | WebhookSubscriptionCreated]:
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
    body: WebhookSubscriptionCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[Problem | WebhookSubscriptionCreated]:
    """Subscribe a URL to events

     The host must be in `webhook.allowed_hosts`; otherwise 400 `webhook_host_not_allowed`.

    Args:
        app_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (WebhookSubscriptionCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | WebhookSubscriptionCreated]
    """

    kwargs = _get_kwargs(
        app_id=app_id,
        body=body,
        idempotency_key=idempotency_key,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    app_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: WebhookSubscriptionCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> Problem | WebhookSubscriptionCreated | None:
    """Subscribe a URL to events

     The host must be in `webhook.allowed_hosts`; otherwise 400 `webhook_host_not_allowed`.

    Args:
        app_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (WebhookSubscriptionCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | WebhookSubscriptionCreated
    """

    return sync_detailed(
        app_id=app_id,
        client=client,
        body=body,
        idempotency_key=idempotency_key,
    ).parsed


async def asyncio_detailed(
    app_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: WebhookSubscriptionCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[Problem | WebhookSubscriptionCreated]:
    """Subscribe a URL to events

     The host must be in `webhook.allowed_hosts`; otherwise 400 `webhook_host_not_allowed`.

    Args:
        app_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (WebhookSubscriptionCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | WebhookSubscriptionCreated]
    """

    kwargs = _get_kwargs(
        app_id=app_id,
        body=body,
        idempotency_key=idempotency_key,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    app_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: WebhookSubscriptionCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> Problem | WebhookSubscriptionCreated | None:
    """Subscribe a URL to events

     The host must be in `webhook.allowed_hosts`; otherwise 400 `webhook_host_not_allowed`.

    Args:
        app_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (WebhookSubscriptionCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | WebhookSubscriptionCreated
    """

    return (
        await asyncio_detailed(
            app_id=app_id,
            client=client,
            body=body,
            idempotency_key=idempotency_key,
        )
    ).parsed
