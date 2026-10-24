from http import HTTPStatus
from typing import Any
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.push_registration_create import PushRegistrationCreate
from ...models.push_registration_created import PushRegistrationCreated
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    body: PushRegistrationCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(idempotency_key, Unset):
        headers["Idempotency-Key"] = idempotency_key

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/me/devices/current/push-registrations",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | PushRegistrationCreated:
    if response.status_code == 201:
        response_201 = PushRegistrationCreated.from_dict(response.json())

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
) -> Response[Problem | PushRegistrationCreated]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: PushRegistrationCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[Problem | PushRegistrationCreated]:
    """Register a push target for the current device

     Registers the push target of the caller's own active device in the caller's session and returns the
    preview token of the registration exactly once (13, 6.4 and 13). The row binds the person, the
    device and the session. A known `(provider, token)` is taken over: its row is deleted, whoever it
    belonged to, so the token stops reaching its former owner; the other active registrations of this
    device with the same provider and `app_id` end with `disabled_reason` `token_invalid` (one current
    token per app installation). Every new registration rotates the preview token. `unifiedpush` and
    `wns` give `400 validation_failed` with `errors[].code` `not_enabled`; `apns` or `fcm` without a
    configured adapter on this instance give `403 feature_disabled` with `reason`
    `push_provider_disabled`; a principal without a session (API tokens, agents, apps) gives `403
    forbidden` with `reason` `step_up_not_possible`; a pending device gives `403 device_pending`. The
    preview token is a one-time secret: a replay with the same `Idempotency-Key` answers `409 conflict`
    with `reason` `one_time_secret` (06, 7), and the app registers again with a new key. A second
    concurrent registration of the same token can answer `409 conflict`.

    Args:
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (PushRegistrationCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | PushRegistrationCreated]
    """

    kwargs = _get_kwargs(
        body=body,
        idempotency_key=idempotency_key,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    body: PushRegistrationCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> Problem | PushRegistrationCreated | None:
    """Register a push target for the current device

     Registers the push target of the caller's own active device in the caller's session and returns the
    preview token of the registration exactly once (13, 6.4 and 13). The row binds the person, the
    device and the session. A known `(provider, token)` is taken over: its row is deleted, whoever it
    belonged to, so the token stops reaching its former owner; the other active registrations of this
    device with the same provider and `app_id` end with `disabled_reason` `token_invalid` (one current
    token per app installation). Every new registration rotates the preview token. `unifiedpush` and
    `wns` give `400 validation_failed` with `errors[].code` `not_enabled`; `apns` or `fcm` without a
    configured adapter on this instance give `403 feature_disabled` with `reason`
    `push_provider_disabled`; a principal without a session (API tokens, agents, apps) gives `403
    forbidden` with `reason` `step_up_not_possible`; a pending device gives `403 device_pending`. The
    preview token is a one-time secret: a replay with the same `Idempotency-Key` answers `409 conflict`
    with `reason` `one_time_secret` (06, 7), and the app registers again with a new key. A second
    concurrent registration of the same token can answer `409 conflict`.

    Args:
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (PushRegistrationCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | PushRegistrationCreated
    """

    return sync_detailed(
        client=client,
        body=body,
        idempotency_key=idempotency_key,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: PushRegistrationCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[Problem | PushRegistrationCreated]:
    """Register a push target for the current device

     Registers the push target of the caller's own active device in the caller's session and returns the
    preview token of the registration exactly once (13, 6.4 and 13). The row binds the person, the
    device and the session. A known `(provider, token)` is taken over: its row is deleted, whoever it
    belonged to, so the token stops reaching its former owner; the other active registrations of this
    device with the same provider and `app_id` end with `disabled_reason` `token_invalid` (one current
    token per app installation). Every new registration rotates the preview token. `unifiedpush` and
    `wns` give `400 validation_failed` with `errors[].code` `not_enabled`; `apns` or `fcm` without a
    configured adapter on this instance give `403 feature_disabled` with `reason`
    `push_provider_disabled`; a principal without a session (API tokens, agents, apps) gives `403
    forbidden` with `reason` `step_up_not_possible`; a pending device gives `403 device_pending`. The
    preview token is a one-time secret: a replay with the same `Idempotency-Key` answers `409 conflict`
    with `reason` `one_time_secret` (06, 7), and the app registers again with a new key. A second
    concurrent registration of the same token can answer `409 conflict`.

    Args:
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (PushRegistrationCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | PushRegistrationCreated]
    """

    kwargs = _get_kwargs(
        body=body,
        idempotency_key=idempotency_key,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: PushRegistrationCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> Problem | PushRegistrationCreated | None:
    """Register a push target for the current device

     Registers the push target of the caller's own active device in the caller's session and returns the
    preview token of the registration exactly once (13, 6.4 and 13). The row binds the person, the
    device and the session. A known `(provider, token)` is taken over: its row is deleted, whoever it
    belonged to, so the token stops reaching its former owner; the other active registrations of this
    device with the same provider and `app_id` end with `disabled_reason` `token_invalid` (one current
    token per app installation). Every new registration rotates the preview token. `unifiedpush` and
    `wns` give `400 validation_failed` with `errors[].code` `not_enabled`; `apns` or `fcm` without a
    configured adapter on this instance give `403 feature_disabled` with `reason`
    `push_provider_disabled`; a principal without a session (API tokens, agents, apps) gives `403
    forbidden` with `reason` `step_up_not_possible`; a pending device gives `403 device_pending`. The
    preview token is a one-time secret: a replay with the same `Idempotency-Key` answers `409 conflict`
    with `reason` `one_time_secret` (06, 7), and the app registers again with a new key. A second
    concurrent registration of the same token can answer `409 conflict`.

    Args:
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (PushRegistrationCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | PushRegistrationCreated
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
            idempotency_key=idempotency_key,
        )
    ).parsed
