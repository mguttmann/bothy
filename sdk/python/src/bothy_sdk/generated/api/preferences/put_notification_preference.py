from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.notification_preference import NotificationPreference
from ...models.notification_preference_put import NotificationPreferencePut
from ...models.problem import Problem
from ...models.put_notification_preference_target_kind import (
    PutNotificationPreferenceTargetKind,
)
from ...types import Response


def _get_kwargs(
    target_kind: PutNotificationPreferenceTargetKind,
    target_id: UUID,
    *,
    body: NotificationPreferencePut,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "put",
        "url": "/me/notification-preferences/{target_kind}/{target_id}".format(
            target_kind=quote(str(target_kind), safe=""),
            target_id=quote(str(target_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> NotificationPreference | Problem:
    if response.status_code == 200:
        response_200 = NotificationPreference.from_dict(response.json())

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
) -> Response[NotificationPreference | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    target_kind: PutNotificationPreferenceTargetKind,
    target_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: NotificationPreferencePut,
) -> Response[NotificationPreference | Problem]:
    """Set the notification level of a target

     Sets the caller's level on a space, channel or thread the caller can read, the main thread of a text
    channel included. An unknown target, an id of another kind and a target without read right give the
    same 404. An archived target (or a channel in an archived space) takes no new preference: 409
    conflict with reason target_archived. muted_until is stored as given (whole microseconds), also when
    it has passed; a passed value has no effect. A write that changes nothing answers 200 without event
    and without a new version.

    Args:
        target_kind (PutNotificationPreferenceTargetKind):
        target_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (NotificationPreferencePut):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[NotificationPreference | Problem]
    """

    kwargs = _get_kwargs(
        target_kind=target_kind,
        target_id=target_id,
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    target_kind: PutNotificationPreferenceTargetKind,
    target_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: NotificationPreferencePut,
) -> NotificationPreference | Problem | None:
    """Set the notification level of a target

     Sets the caller's level on a space, channel or thread the caller can read, the main thread of a text
    channel included. An unknown target, an id of another kind and a target without read right give the
    same 404. An archived target (or a channel in an archived space) takes no new preference: 409
    conflict with reason target_archived. muted_until is stored as given (whole microseconds), also when
    it has passed; a passed value has no effect. A write that changes nothing answers 200 without event
    and without a new version.

    Args:
        target_kind (PutNotificationPreferenceTargetKind):
        target_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (NotificationPreferencePut):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        NotificationPreference | Problem
    """

    return sync_detailed(
        target_kind=target_kind,
        target_id=target_id,
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    target_kind: PutNotificationPreferenceTargetKind,
    target_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: NotificationPreferencePut,
) -> Response[NotificationPreference | Problem]:
    """Set the notification level of a target

     Sets the caller's level on a space, channel or thread the caller can read, the main thread of a text
    channel included. An unknown target, an id of another kind and a target without read right give the
    same 404. An archived target (or a channel in an archived space) takes no new preference: 409
    conflict with reason target_archived. muted_until is stored as given (whole microseconds), also when
    it has passed; a passed value has no effect. A write that changes nothing answers 200 without event
    and without a new version.

    Args:
        target_kind (PutNotificationPreferenceTargetKind):
        target_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (NotificationPreferencePut):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[NotificationPreference | Problem]
    """

    kwargs = _get_kwargs(
        target_kind=target_kind,
        target_id=target_id,
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    target_kind: PutNotificationPreferenceTargetKind,
    target_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: NotificationPreferencePut,
) -> NotificationPreference | Problem | None:
    """Set the notification level of a target

     Sets the caller's level on a space, channel or thread the caller can read, the main thread of a text
    channel included. An unknown target, an id of another kind and a target without read right give the
    same 404. An archived target (or a channel in an archived space) takes no new preference: 409
    conflict with reason target_archived. muted_until is stored as given (whole microseconds), also when
    it has passed; a passed value has no effect. A write that changes nothing answers 200 without event
    and without a new version.

    Args:
        target_kind (PutNotificationPreferenceTargetKind):
        target_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (NotificationPreferencePut):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        NotificationPreference | Problem
    """

    return (
        await asyncio_detailed(
            target_kind=target_kind,
            target_id=target_id,
            client=client,
            body=body,
        )
    ).parsed
