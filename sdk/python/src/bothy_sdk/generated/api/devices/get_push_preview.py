from http import HTTPStatus
from typing import Any
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.push_preview import PushPreview
from ...types import UNSET, Response


def _get_kwargs(
    *,
    nid: UUID,
    message_id: UUID,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    json_nid = str(nid)
    params["nid"] = json_nid

    json_message_id = str(message_id)
    params["message_id"] = json_message_id

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
) -> Response[Problem | PushPreview]:
    """Load the preview of a notification

     Called by the notification extension with the preview token of the push registration
    (`Authorization: DPoP <token>` and a proof with the device key). Answers `404` if access is gone or
    the message was deleted (13, 6.4).

    Args:
        nid (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        message_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | PushPreview]
    """

    kwargs = _get_kwargs(
        nid=nid,
        message_id=message_id,
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
) -> Problem | PushPreview | None:
    """Load the preview of a notification

     Called by the notification extension with the preview token of the push registration
    (`Authorization: DPoP <token>` and a proof with the device key). Answers `404` if access is gone or
    the message was deleted (13, 6.4).

    Args:
        nid (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        message_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

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
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient,
    nid: UUID,
    message_id: UUID,
) -> Response[Problem | PushPreview]:
    """Load the preview of a notification

     Called by the notification extension with the preview token of the push registration
    (`Authorization: DPoP <token>` and a proof with the device key). Answers `404` if access is gone or
    the message was deleted (13, 6.4).

    Args:
        nid (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        message_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | PushPreview]
    """

    kwargs = _get_kwargs(
        nid=nid,
        message_id=message_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient,
    nid: UUID,
    message_id: UUID,
) -> Problem | PushPreview | None:
    """Load the preview of a notification

     Called by the notification extension with the preview token of the push registration
    (`Authorization: DPoP <token>` and a proof with the device key). Answers `404` if access is gone or
    the message was deleted (13, 6.4).

    Args:
        nid (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        message_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

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
        )
    ).parsed
