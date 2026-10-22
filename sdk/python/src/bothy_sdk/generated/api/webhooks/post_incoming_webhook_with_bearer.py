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
    *,
    body: IncomingWebhookPost,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/hooks/{webhook_id}".format(
            webhook_id=quote(str(webhook_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> IncomingWebhookResult | Problem:
    if response.status_code == 200:
        response_200 = IncomingWebhookResult.from_dict(response.json())

        return response_200

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
) -> Response[IncomingWebhookResult | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    webhook_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: IncomingWebhookPost,
) -> Response[IncomingWebhookResult | Problem]:
    """Post a message through an incoming webhook with the token in Authorization

     The token of the incoming webhook is sent as `Authorization: Bearer <token>`; the path ends with the
    webhook ID. Use this variant where the sender can set a header, so the token never appears in a URL.
    Of the root security only `bearerAuth` applies here, with the token of the incoming webhook; any
    other scheme or credential counts as a wrong token (`404`).

    The format is recognised per request from the body: a body with at least one Alertmanager member
    (`version`, `groupKey`, `truncatedAlerts`, `status`, `receiver`, `groupLabels`, `commonLabels`,
    `commonAnnotations`, `externalURL`, `alerts`) is the unchanged webhook payload of Prometheus
    Alertmanager and gives one message per notification; it must carry no native member (`400
    validation_failed`, `format_mixed`). Every other body is `native`: Markdown in `content_md` with
    optional `title`, `thread_id` and `client_msg_id`. A format outside `webhook_in.formats_enabled`
    gives `400 validation_failed` with `format_disabled`.

    Order of checks (11 8.4): `rl.webhook_in` of the webhook (`429`), the token (`404`), the body up to
    `webhook_in.max_body_bytes` (`413`), the format (`400`), `authorize` of the app (`404`), then the
    message (`201`). An unknown or revoked webhook, a wrong or malformed token, a suspended or
    uninstalled app and every refusal of the app's rights give the same `404`. A failed token check
    counts against `rl.public_auth` of the client IP.

    The message has `origin = app` with the app as author, resolves no mentions and never triggers an
    agent. `native` with `client_msg_id` is idempotent: a repeat answers `200` with the original and
    `Idempotent-Replay: true`, other content with the same `client_msg_id` `422 idempotency_key_reuse`.
    Senders inside the stack reach the same path on the internal listener.

    Args:
        webhook_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (IncomingWebhookPost): Body of an incoming webhook call in one of two shapes,
            recognised per request (11 8.2,
            docs/DECISIONS.md D-M2-27-5). A body with at least one Alertmanager member (`version`,
            `groupKey`, `truncatedAlerts`, `status`, `receiver`, `groupLabels`, `commonLabels`,
            `commonAnnotations`, `externalURL`, `alerts`) is the unchanged webhook payload of
            Prometheus
            Alertmanager (format `alertmanager`) and must contain no native member; every other body
            is
            `native` and needs `content_md`. Unknown members are refused.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[IncomingWebhookResult | Problem]
    """

    kwargs = _get_kwargs(
        webhook_id=webhook_id,
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    webhook_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: IncomingWebhookPost,
) -> IncomingWebhookResult | Problem | None:
    """Post a message through an incoming webhook with the token in Authorization

     The token of the incoming webhook is sent as `Authorization: Bearer <token>`; the path ends with the
    webhook ID. Use this variant where the sender can set a header, so the token never appears in a URL.
    Of the root security only `bearerAuth` applies here, with the token of the incoming webhook; any
    other scheme or credential counts as a wrong token (`404`).

    The format is recognised per request from the body: a body with at least one Alertmanager member
    (`version`, `groupKey`, `truncatedAlerts`, `status`, `receiver`, `groupLabels`, `commonLabels`,
    `commonAnnotations`, `externalURL`, `alerts`) is the unchanged webhook payload of Prometheus
    Alertmanager and gives one message per notification; it must carry no native member (`400
    validation_failed`, `format_mixed`). Every other body is `native`: Markdown in `content_md` with
    optional `title`, `thread_id` and `client_msg_id`. A format outside `webhook_in.formats_enabled`
    gives `400 validation_failed` with `format_disabled`.

    Order of checks (11 8.4): `rl.webhook_in` of the webhook (`429`), the token (`404`), the body up to
    `webhook_in.max_body_bytes` (`413`), the format (`400`), `authorize` of the app (`404`), then the
    message (`201`). An unknown or revoked webhook, a wrong or malformed token, a suspended or
    uninstalled app and every refusal of the app's rights give the same `404`. A failed token check
    counts against `rl.public_auth` of the client IP.

    The message has `origin = app` with the app as author, resolves no mentions and never triggers an
    agent. `native` with `client_msg_id` is idempotent: a repeat answers `200` with the original and
    `Idempotent-Replay: true`, other content with the same `client_msg_id` `422 idempotency_key_reuse`.
    Senders inside the stack reach the same path on the internal listener.

    Args:
        webhook_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (IncomingWebhookPost): Body of an incoming webhook call in one of two shapes,
            recognised per request (11 8.2,
            docs/DECISIONS.md D-M2-27-5). A body with at least one Alertmanager member (`version`,
            `groupKey`, `truncatedAlerts`, `status`, `receiver`, `groupLabels`, `commonLabels`,
            `commonAnnotations`, `externalURL`, `alerts`) is the unchanged webhook payload of
            Prometheus
            Alertmanager (format `alertmanager`) and must contain no native member; every other body
            is
            `native` and needs `content_md`. Unknown members are refused.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        IncomingWebhookResult | Problem
    """

    return sync_detailed(
        webhook_id=webhook_id,
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    webhook_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: IncomingWebhookPost,
) -> Response[IncomingWebhookResult | Problem]:
    """Post a message through an incoming webhook with the token in Authorization

     The token of the incoming webhook is sent as `Authorization: Bearer <token>`; the path ends with the
    webhook ID. Use this variant where the sender can set a header, so the token never appears in a URL.
    Of the root security only `bearerAuth` applies here, with the token of the incoming webhook; any
    other scheme or credential counts as a wrong token (`404`).

    The format is recognised per request from the body: a body with at least one Alertmanager member
    (`version`, `groupKey`, `truncatedAlerts`, `status`, `receiver`, `groupLabels`, `commonLabels`,
    `commonAnnotations`, `externalURL`, `alerts`) is the unchanged webhook payload of Prometheus
    Alertmanager and gives one message per notification; it must carry no native member (`400
    validation_failed`, `format_mixed`). Every other body is `native`: Markdown in `content_md` with
    optional `title`, `thread_id` and `client_msg_id`. A format outside `webhook_in.formats_enabled`
    gives `400 validation_failed` with `format_disabled`.

    Order of checks (11 8.4): `rl.webhook_in` of the webhook (`429`), the token (`404`), the body up to
    `webhook_in.max_body_bytes` (`413`), the format (`400`), `authorize` of the app (`404`), then the
    message (`201`). An unknown or revoked webhook, a wrong or malformed token, a suspended or
    uninstalled app and every refusal of the app's rights give the same `404`. A failed token check
    counts against `rl.public_auth` of the client IP.

    The message has `origin = app` with the app as author, resolves no mentions and never triggers an
    agent. `native` with `client_msg_id` is idempotent: a repeat answers `200` with the original and
    `Idempotent-Replay: true`, other content with the same `client_msg_id` `422 idempotency_key_reuse`.
    Senders inside the stack reach the same path on the internal listener.

    Args:
        webhook_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (IncomingWebhookPost): Body of an incoming webhook call in one of two shapes,
            recognised per request (11 8.2,
            docs/DECISIONS.md D-M2-27-5). A body with at least one Alertmanager member (`version`,
            `groupKey`, `truncatedAlerts`, `status`, `receiver`, `groupLabels`, `commonLabels`,
            `commonAnnotations`, `externalURL`, `alerts`) is the unchanged webhook payload of
            Prometheus
            Alertmanager (format `alertmanager`) and must contain no native member; every other body
            is
            `native` and needs `content_md`. Unknown members are refused.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[IncomingWebhookResult | Problem]
    """

    kwargs = _get_kwargs(
        webhook_id=webhook_id,
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    webhook_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: IncomingWebhookPost,
) -> IncomingWebhookResult | Problem | None:
    """Post a message through an incoming webhook with the token in Authorization

     The token of the incoming webhook is sent as `Authorization: Bearer <token>`; the path ends with the
    webhook ID. Use this variant where the sender can set a header, so the token never appears in a URL.
    Of the root security only `bearerAuth` applies here, with the token of the incoming webhook; any
    other scheme or credential counts as a wrong token (`404`).

    The format is recognised per request from the body: a body with at least one Alertmanager member
    (`version`, `groupKey`, `truncatedAlerts`, `status`, `receiver`, `groupLabels`, `commonLabels`,
    `commonAnnotations`, `externalURL`, `alerts`) is the unchanged webhook payload of Prometheus
    Alertmanager and gives one message per notification; it must carry no native member (`400
    validation_failed`, `format_mixed`). Every other body is `native`: Markdown in `content_md` with
    optional `title`, `thread_id` and `client_msg_id`. A format outside `webhook_in.formats_enabled`
    gives `400 validation_failed` with `format_disabled`.

    Order of checks (11 8.4): `rl.webhook_in` of the webhook (`429`), the token (`404`), the body up to
    `webhook_in.max_body_bytes` (`413`), the format (`400`), `authorize` of the app (`404`), then the
    message (`201`). An unknown or revoked webhook, a wrong or malformed token, a suspended or
    uninstalled app and every refusal of the app's rights give the same `404`. A failed token check
    counts against `rl.public_auth` of the client IP.

    The message has `origin = app` with the app as author, resolves no mentions and never triggers an
    agent. `native` with `client_msg_id` is idempotent: a repeat answers `200` with the original and
    `Idempotent-Replay: true`, other content with the same `client_msg_id` `422 idempotency_key_reuse`.
    Senders inside the stack reach the same path on the internal listener.

    Args:
        webhook_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (IncomingWebhookPost): Body of an incoming webhook call in one of two shapes,
            recognised per request (11 8.2,
            docs/DECISIONS.md D-M2-27-5). A body with at least one Alertmanager member (`version`,
            `groupKey`, `truncatedAlerts`, `status`, `receiver`, `groupLabels`, `commonLabels`,
            `commonAnnotations`, `externalURL`, `alerts`) is the unchanged webhook payload of
            Prometheus
            Alertmanager (format `alertmanager`) and must contain no native member; every other body
            is
            `native` and needs `content_md`. Unknown members are refused.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        IncomingWebhookResult | Problem
    """

    return (
        await asyncio_detailed(
            webhook_id=webhook_id,
            client=client,
            body=body,
        )
    ).parsed
