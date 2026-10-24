from http import HTTPStatus
from typing import Any
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.notification_kind import NotificationKind
from ...models.problem import Problem
from ...models.push_preview import PushPreview
from ...types import UNSET, Response


def _get_kwargs(
    *,
    nid: UUID,
    message_id: UUID,
    kind: NotificationKind,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    json_nid = str(nid)
    params["nid"] = json_nid

    json_message_id = str(message_id)
    params["message_id"] = json_message_id

    json_kind = kind.value
    params["kind"] = json_kind

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/push/preview",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | PushPreview:
    if response.status_code == 200:
        response_200 = PushPreview.from_dict(response.json())

        return response_200

    if response.status_code == 400:
        response_400 = Problem.from_dict(response.json())

        return response_400

    if response.status_code == 401:
        response_401 = Problem.from_dict(response.json())

        return response_401

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
) -> Response[Problem | PushPreview]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient,
    nid: UUID,
    message_id: UUID,
    kind: NotificationKind,
) -> Response[Problem | PushPreview]:
    """Load the preview of a notification

     Called by the notification extension with the preview token of the push registration
    (`Authorization: DPoP <token>`) and a DPoP proof of the device key whose `ath` is the hash of the
    preview token; no other operation accepts the token, and no other credential works here (13, 6.4;
    R-PN-07). Checks in this order: the parameter `kind`; exactly one `Authorization` value with the
    scheme `DPoP`; the form, kind `pvw` and checksum of the token without a database access; an active
    registration with the token; the session of the registration, whose device must be the device of the
    registration; the DPoP proof against the key of that session (`DPoP-Nonce` as on every DPoP
    request); then `message.read` on the message and `push.deliver` on its thread with the rights of the
    moment of the fetch. A missing, malformed or unknown token, a deactivated registration, a session
    that is unknown or belongs to another device and a wrong proof give `401`; while the registration is
    still active, a session, device or person that is no longer active (for example an expired session)
    gives `401 session_revoked`. `auth.logout`, `device.revoke` and `user.disable` deactivate the
    registration in their own transaction, so after them the token itself answers `401`. No right, a
    deleted or an unknown message give `404`, identical to each other. The answer follows
    `notify.preview_mode` of the person (13, 6.4 table): `full` with names, excerpt (text without
    Markdown, mentions as names, at most `notify.preview_max_chars`), `attachment_count` and `image_url`
    (a fetch ticket for the `small` thumbnail of the first image); `names` with the four names only;
    `none` only `kind`, `thread_id`, `message_id` and `badge`.

    Args:
        nid (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        message_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        kind (NotificationKind):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | PushPreview]
    """

    kwargs = _get_kwargs(
        nid=nid,
        message_id=message_id,
        kind=kind,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient,
    nid: UUID,
    message_id: UUID,
    kind: NotificationKind,
) -> Problem | PushPreview | None:
    """Load the preview of a notification

     Called by the notification extension with the preview token of the push registration
    (`Authorization: DPoP <token>`) and a DPoP proof of the device key whose `ath` is the hash of the
    preview token; no other operation accepts the token, and no other credential works here (13, 6.4;
    R-PN-07). Checks in this order: the parameter `kind`; exactly one `Authorization` value with the
    scheme `DPoP`; the form, kind `pvw` and checksum of the token without a database access; an active
    registration with the token; the session of the registration, whose device must be the device of the
    registration; the DPoP proof against the key of that session (`DPoP-Nonce` as on every DPoP
    request); then `message.read` on the message and `push.deliver` on its thread with the rights of the
    moment of the fetch. A missing, malformed or unknown token, a deactivated registration, a session
    that is unknown or belongs to another device and a wrong proof give `401`; while the registration is
    still active, a session, device or person that is no longer active (for example an expired session)
    gives `401 session_revoked`. `auth.logout`, `device.revoke` and `user.disable` deactivate the
    registration in their own transaction, so after them the token itself answers `401`. No right, a
    deleted or an unknown message give `404`, identical to each other. The answer follows
    `notify.preview_mode` of the person (13, 6.4 table): `full` with names, excerpt (text without
    Markdown, mentions as names, at most `notify.preview_max_chars`), `attachment_count` and `image_url`
    (a fetch ticket for the `small` thumbnail of the first image); `names` with the four names only;
    `none` only `kind`, `thread_id`, `message_id` and `badge`.

    Args:
        nid (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        message_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        kind (NotificationKind):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | PushPreview
    """

    return sync_detailed(
        client=client,
        nid=nid,
        message_id=message_id,
        kind=kind,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient,
    nid: UUID,
    message_id: UUID,
    kind: NotificationKind,
) -> Response[Problem | PushPreview]:
    """Load the preview of a notification

     Called by the notification extension with the preview token of the push registration
    (`Authorization: DPoP <token>`) and a DPoP proof of the device key whose `ath` is the hash of the
    preview token; no other operation accepts the token, and no other credential works here (13, 6.4;
    R-PN-07). Checks in this order: the parameter `kind`; exactly one `Authorization` value with the
    scheme `DPoP`; the form, kind `pvw` and checksum of the token without a database access; an active
    registration with the token; the session of the registration, whose device must be the device of the
    registration; the DPoP proof against the key of that session (`DPoP-Nonce` as on every DPoP
    request); then `message.read` on the message and `push.deliver` on its thread with the rights of the
    moment of the fetch. A missing, malformed or unknown token, a deactivated registration, a session
    that is unknown or belongs to another device and a wrong proof give `401`; while the registration is
    still active, a session, device or person that is no longer active (for example an expired session)
    gives `401 session_revoked`. `auth.logout`, `device.revoke` and `user.disable` deactivate the
    registration in their own transaction, so after them the token itself answers `401`. No right, a
    deleted or an unknown message give `404`, identical to each other. The answer follows
    `notify.preview_mode` of the person (13, 6.4 table): `full` with names, excerpt (text without
    Markdown, mentions as names, at most `notify.preview_max_chars`), `attachment_count` and `image_url`
    (a fetch ticket for the `small` thumbnail of the first image); `names` with the four names only;
    `none` only `kind`, `thread_id`, `message_id` and `badge`.

    Args:
        nid (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        message_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        kind (NotificationKind):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | PushPreview]
    """

    kwargs = _get_kwargs(
        nid=nid,
        message_id=message_id,
        kind=kind,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient,
    nid: UUID,
    message_id: UUID,
    kind: NotificationKind,
) -> Problem | PushPreview | None:
    """Load the preview of a notification

     Called by the notification extension with the preview token of the push registration
    (`Authorization: DPoP <token>`) and a DPoP proof of the device key whose `ath` is the hash of the
    preview token; no other operation accepts the token, and no other credential works here (13, 6.4;
    R-PN-07). Checks in this order: the parameter `kind`; exactly one `Authorization` value with the
    scheme `DPoP`; the form, kind `pvw` and checksum of the token without a database access; an active
    registration with the token; the session of the registration, whose device must be the device of the
    registration; the DPoP proof against the key of that session (`DPoP-Nonce` as on every DPoP
    request); then `message.read` on the message and `push.deliver` on its thread with the rights of the
    moment of the fetch. A missing, malformed or unknown token, a deactivated registration, a session
    that is unknown or belongs to another device and a wrong proof give `401`; while the registration is
    still active, a session, device or person that is no longer active (for example an expired session)
    gives `401 session_revoked`. `auth.logout`, `device.revoke` and `user.disable` deactivate the
    registration in their own transaction, so after them the token itself answers `401`. No right, a
    deleted or an unknown message give `404`, identical to each other. The answer follows
    `notify.preview_mode` of the person (13, 6.4 table): `full` with names, excerpt (text without
    Markdown, mentions as names, at most `notify.preview_max_chars`), `attachment_count` and `image_url`
    (a fetch ticket for the `small` thumbnail of the first image); `names` with the four names only;
    `none` only `kind`, `thread_id`, `message_id` and `badge`.

    Args:
        nid (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        message_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        kind (NotificationKind):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | PushPreview
    """

    return (
        await asyncio_detailed(
            client=client,
            nid=nid,
            message_id=message_id,
            kind=kind,
        )
    ).parsed
