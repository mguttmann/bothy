from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.file import File
from ...models.problem import Problem
from ...models.upload_complete import UploadComplete
from ...types import UNSET, Response, Unset


def _get_kwargs(
    file_id: UUID,
    *,
    body: UploadComplete,
    idempotency_key: UUID | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(idempotency_key, Unset):
        headers["Idempotency-Key"] = idempotency_key

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/uploads/{file_id}/complete".format(
            file_id=quote(str(file_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> File | Problem:
    if response.status_code == 200:
        response_200 = File.from_dict(response.json())

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

    if response.status_code == 410:
        response_410 = Problem.from_dict(response.json())

        return response_410

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
) -> Response[File | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    file_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: UploadComplete,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[File | Problem]:
    """Finish an upload

     Closes the session; the verify pass then checks size and SHA-256, detects the MIME type and runs the
    malware scan. The file is
    downloadable after `scan_status` is `clean` or `skipped`. `parts` is checked against the confirmed
    parts: a duplicate or out-of-range number gives 400, another `etag` 409 `conflict` with `reason`
    `part_conflict`. A completed session answers 200 with the file again. Missing parts answer 409
    `conflict` with `reason` `upload_incomplete`; an expired session answers 410 `gone` (12, 3.4 and
    3.6).

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (UploadComplete):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[File | Problem]
    """

    kwargs = _get_kwargs(
        file_id=file_id,
        body=body,
        idempotency_key=idempotency_key,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    file_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: UploadComplete,
    idempotency_key: UUID | Unset = UNSET,
) -> File | Problem | None:
    """Finish an upload

     Closes the session; the verify pass then checks size and SHA-256, detects the MIME type and runs the
    malware scan. The file is
    downloadable after `scan_status` is `clean` or `skipped`. `parts` is checked against the confirmed
    parts: a duplicate or out-of-range number gives 400, another `etag` 409 `conflict` with `reason`
    `part_conflict`. A completed session answers 200 with the file again. Missing parts answer 409
    `conflict` with `reason` `upload_incomplete`; an expired session answers 410 `gone` (12, 3.4 and
    3.6).

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (UploadComplete):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        File | Problem
    """

    return sync_detailed(
        file_id=file_id,
        client=client,
        body=body,
        idempotency_key=idempotency_key,
    ).parsed


async def asyncio_detailed(
    file_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: UploadComplete,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[File | Problem]:
    """Finish an upload

     Closes the session; the verify pass then checks size and SHA-256, detects the MIME type and runs the
    malware scan. The file is
    downloadable after `scan_status` is `clean` or `skipped`. `parts` is checked against the confirmed
    parts: a duplicate or out-of-range number gives 400, another `etag` 409 `conflict` with `reason`
    `part_conflict`. A completed session answers 200 with the file again. Missing parts answer 409
    `conflict` with `reason` `upload_incomplete`; an expired session answers 410 `gone` (12, 3.4 and
    3.6).

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (UploadComplete):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[File | Problem]
    """

    kwargs = _get_kwargs(
        file_id=file_id,
        body=body,
        idempotency_key=idempotency_key,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    file_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: UploadComplete,
    idempotency_key: UUID | Unset = UNSET,
) -> File | Problem | None:
    """Finish an upload

     Closes the session; the verify pass then checks size and SHA-256, detects the MIME type and runs the
    malware scan. The file is
    downloadable after `scan_status` is `clean` or `skipped`. `parts` is checked against the confirmed
    parts: a duplicate or out-of-range number gives 400, another `etag` 409 `conflict` with `reason`
    `part_conflict`. A completed session answers 200 with the file again. Missing parts answer 409
    `conflict` with `reason` `upload_incomplete`; an expired session answers 410 `gone` (12, 3.4 and
    3.6).

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (UploadComplete):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        File | Problem
    """

    return (
        await asyncio_detailed(
            file_id=file_id,
            client=client,
            body=body,
            idempotency_key=idempotency_key,
        )
    ).parsed
