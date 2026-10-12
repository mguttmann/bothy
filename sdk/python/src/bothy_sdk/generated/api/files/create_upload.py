from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.upload_create import UploadCreate
from ...models.upload_session import UploadSession
from ...types import UNSET, Response, Unset


def _get_kwargs(
    channel_id: UUID,
    *,
    body: UploadCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(idempotency_key, Unset):
        headers["Idempotency-Key"] = idempotency_key

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/channels/{channel_id}/uploads".format(
            channel_id=quote(str(channel_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | UploadSession:
    if response.status_code == 201:
        response_201 = UploadSession.from_dict(response.json())

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

    if response.status_code == 413:
        response_413 = Problem.from_dict(response.json())

        return response_413

    if response.status_code == 415:
        response_415 = Problem.from_dict(response.json())

        return response_415

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
) -> Response[Problem | UploadSession]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    channel_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: UploadCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[Problem | UploadSession]:
    """Start an upload

     Checks in this order (12, 3.1): size against `file.max_upload_bytes` (413 `payload_too_large`),
    quota `file.quota_bytes_per_principal` (409 `conflict` with `reason` `quota_exceeded`), open
    sessions (429), free space of the file system driver (503 `unavailable` with `reason`
    `storage_full`). A denied type answers 415 `unsupported_media_type` with `reason`
    `file_type_denied`.

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (UploadCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | UploadSession]
    """

    kwargs = _get_kwargs(
        channel_id=channel_id,
        body=body,
        idempotency_key=idempotency_key,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    channel_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: UploadCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> Problem | UploadSession | None:
    """Start an upload

     Checks in this order (12, 3.1): size against `file.max_upload_bytes` (413 `payload_too_large`),
    quota `file.quota_bytes_per_principal` (409 `conflict` with `reason` `quota_exceeded`), open
    sessions (429), free space of the file system driver (503 `unavailable` with `reason`
    `storage_full`). A denied type answers 415 `unsupported_media_type` with `reason`
    `file_type_denied`.

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (UploadCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | UploadSession
    """

    return sync_detailed(
        channel_id=channel_id,
        client=client,
        body=body,
        idempotency_key=idempotency_key,
    ).parsed


async def asyncio_detailed(
    channel_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: UploadCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[Problem | UploadSession]:
    """Start an upload

     Checks in this order (12, 3.1): size against `file.max_upload_bytes` (413 `payload_too_large`),
    quota `file.quota_bytes_per_principal` (409 `conflict` with `reason` `quota_exceeded`), open
    sessions (429), free space of the file system driver (503 `unavailable` with `reason`
    `storage_full`). A denied type answers 415 `unsupported_media_type` with `reason`
    `file_type_denied`.

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (UploadCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | UploadSession]
    """

    kwargs = _get_kwargs(
        channel_id=channel_id,
        body=body,
        idempotency_key=idempotency_key,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    channel_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: UploadCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> Problem | UploadSession | None:
    """Start an upload

     Checks in this order (12, 3.1): size against `file.max_upload_bytes` (413 `payload_too_large`),
    quota `file.quota_bytes_per_principal` (409 `conflict` with `reason` `quota_exceeded`), open
    sessions (429), free space of the file system driver (503 `unavailable` with `reason`
    `storage_full`). A denied type answers 415 `unsupported_media_type` with `reason`
    `file_type_denied`.

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (UploadCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | UploadSession
    """

    return (
        await asyncio_detailed(
            channel_id=channel_id,
            client=client,
            body=body,
            idempotency_key=idempotency_key,
        )
    ).parsed
