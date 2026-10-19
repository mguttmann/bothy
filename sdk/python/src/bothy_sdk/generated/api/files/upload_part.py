from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.upload_part_result import UploadPartResult
from ...types import File, Response


def _get_kwargs(
    file_id: UUID,
    part_number: int,
    *,
    body: File,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "put",
        "url": "/uploads/{file_id}/parts/{part_number}".format(
            file_id=quote(str(file_id), safe=""),
            part_number=quote(str(part_number), safe=""),
        ),
    }

    _kwargs["content"] = body.payload
    headers["Content-Type"] = "application/octet-stream"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | UploadPartResult:
    if response.status_code == 200:
        response_200 = UploadPartResult.from_dict(response.json())

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
) -> Response[Problem | UploadPartResult]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    file_id: UUID,
    part_number: int,
    *,
    client: AuthenticatedClient | Client,
    body: File,
) -> Response[Problem | UploadPartResult]:
    """Upload one part in direct mode

     Parts can be sent in parallel and in any order (12, 3.2). The same part number with the same
    `Content-Digest` answers 200 without new work; with another digest 409 `conflict` with `reason`
    `part_conflict`. Wrong length or digest: 400 `validation_failed` with `errors[].code`
    `part_digest_mismatch`, nothing is stored. An expired session answers 410 `gone`.

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        part_number (int):
        body (File):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | UploadPartResult]
    """

    kwargs = _get_kwargs(
        file_id=file_id,
        part_number=part_number,
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    file_id: UUID,
    part_number: int,
    *,
    client: AuthenticatedClient | Client,
    body: File,
) -> Problem | UploadPartResult | None:
    """Upload one part in direct mode

     Parts can be sent in parallel and in any order (12, 3.2). The same part number with the same
    `Content-Digest` answers 200 without new work; with another digest 409 `conflict` with `reason`
    `part_conflict`. Wrong length or digest: 400 `validation_failed` with `errors[].code`
    `part_digest_mismatch`, nothing is stored. An expired session answers 410 `gone`.

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        part_number (int):
        body (File):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | UploadPartResult
    """

    return sync_detailed(
        file_id=file_id,
        part_number=part_number,
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    file_id: UUID,
    part_number: int,
    *,
    client: AuthenticatedClient | Client,
    body: File,
) -> Response[Problem | UploadPartResult]:
    """Upload one part in direct mode

     Parts can be sent in parallel and in any order (12, 3.2). The same part number with the same
    `Content-Digest` answers 200 without new work; with another digest 409 `conflict` with `reason`
    `part_conflict`. Wrong length or digest: 400 `validation_failed` with `errors[].code`
    `part_digest_mismatch`, nothing is stored. An expired session answers 410 `gone`.

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        part_number (int):
        body (File):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | UploadPartResult]
    """

    kwargs = _get_kwargs(
        file_id=file_id,
        part_number=part_number,
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    file_id: UUID,
    part_number: int,
    *,
    client: AuthenticatedClient | Client,
    body: File,
) -> Problem | UploadPartResult | None:
    """Upload one part in direct mode

     Parts can be sent in parallel and in any order (12, 3.2). The same part number with the same
    `Content-Digest` answers 200 without new work; with another digest 409 `conflict` with `reason`
    `part_conflict`. Wrong length or digest: 400 `validation_failed` with `errors[].code`
    `part_digest_mismatch`, nothing is stored. An expired session answers 410 `gone`.

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        part_number (int):
        body (File):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | UploadPartResult
    """

    return (
        await asyncio_detailed(
            file_id=file_id,
            part_number=part_number,
            client=client,
            body=body,
        )
    ).parsed
