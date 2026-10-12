from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.incoming_webhook_post import IncomingWebhookPost
from ...models.incoming_webhook_result import IncomingWebhookResult
from ...models.problem import Problem
from ...types import Response


def _get_kwargs(
    webhook_id: UUID,
    token: str,
    *,
    body: IncomingWebhookPost,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/hooks/{webhook_id}/{token}".format(
            webhook_id=quote(str(webhook_id), safe=""),
            token=quote(str(token), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> IncomingWebhookResult | Problem:
    if response.status_code == 201:
        response_201 = IncomingWebhookResult.from_dict(response.json())

        return response_201

    if response.status_code == 400:
        response_400 = Problem.from_dict(response.json())

        return response_400

    if response.status_code == 404:
        response_404 = Problem.from_dict(response.json())

        return response_404

    if response.status_code == 413:
        response_413 = Problem.from_dict(response.json())

        return response_413

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[IncomingWebhookResult | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    webhook_id: UUID,
    token: str,
    *,
    client: AuthenticatedClient | Client,
    body: IncomingWebhookPost,
) -> Response[IncomingWebhookResult | Problem]:
    """Post a message through an incoming webhook

     The token in the path authenticates the call; servers redact it in logs. Messages have `origin =
    app` and never trigger agents. Unknown or revoked webhooks give 404.

    Args:
        webhook_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        token (str):
        body (IncomingWebhookPost):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[IncomingWebhookResult | Problem]
    """

    kwargs = _get_kwargs(
        webhook_id=webhook_id,
        token=token,
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    webhook_id: UUID,
    token: str,
    *,
    client: AuthenticatedClient | Client,
    body: IncomingWebhookPost,
) -> IncomingWebhookResult | Problem | None:
    """Post a message through an incoming webhook

     The token in the path authenticates the call; servers redact it in logs. Messages have `origin =
    app` and never trigger agents. Unknown or revoked webhooks give 404.

    Args:
        webhook_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        token (str):
        body (IncomingWebhookPost):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        IncomingWebhookResult | Problem
    """

    return sync_detailed(
        webhook_id=webhook_id,
        token=token,
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    webhook_id: UUID,
    token: str,
    *,
    client: AuthenticatedClient | Client,
    body: IncomingWebhookPost,
) -> Response[IncomingWebhookResult | Problem]:
    """Post a message through an incoming webhook

     The token in the path authenticates the call; servers redact it in logs. Messages have `origin =
    app` and never trigger agents. Unknown or revoked webhooks give 404.

    Args:
        webhook_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        token (str):
        body (IncomingWebhookPost):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[IncomingWebhookResult | Problem]
    """

    kwargs = _get_kwargs(
        webhook_id=webhook_id,
        token=token,
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    webhook_id: UUID,
    token: str,
    *,
    client: AuthenticatedClient | Client,
    body: IncomingWebhookPost,
) -> IncomingWebhookResult | Problem | None:
    """Post a message through an incoming webhook

     The token in the path authenticates the call; servers redact it in logs. Messages have `origin =
    app` and never trigger agents. Unknown or revoked webhooks give 404.

    Args:
        webhook_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        token (str):
        body (IncomingWebhookPost):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        IncomingWebhookResult | Problem
    """

    return (
        await asyncio_detailed(
            webhook_id=webhook_id,
            token=token,
            client=client,
            body=body,
        )
    ).parsed
