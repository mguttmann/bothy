from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.download_thumbnail_variant import DownloadThumbnailVariant
from ...models.problem import Problem
from ...types import Response


def _get_kwargs(
    file_id: UUID,
    variant: DownloadThumbnailVariant,
) -> dict[str, Any]:

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/files/{file_id}/thumbnails/{variant}".format(
            file_id=quote(str(file_id), safe=""),
            variant=quote(str(variant), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem:
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
) -> Response[Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    file_id: UUID,
    variant: DownloadThumbnailVariant,
    *,
    client: AuthenticatedClient,
) -> Response[Problem]:
    """Download a thumbnail

     Accepts a fetch ticket in the query parameter `ticket` like `downloadFile` (push preview
    `image_url`, ADR 0048). 409 `file_not_ready` or `file_blocked` as for `downloadFile`.

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        variant (DownloadThumbnailVariant):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem]
    """

    kwargs = _get_kwargs(
        file_id=file_id,
        variant=variant,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    file_id: UUID,
    variant: DownloadThumbnailVariant,
    *,
    client: AuthenticatedClient,
) -> Problem | None:
    """Download a thumbnail

     Accepts a fetch ticket in the query parameter `ticket` like `downloadFile` (push preview
    `image_url`, ADR 0048). 409 `file_not_ready` or `file_blocked` as for `downloadFile`.

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        variant (DownloadThumbnailVariant):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem
    """

    return sync_detailed(
        file_id=file_id,
        variant=variant,
        client=client,
    ).parsed


async def asyncio_detailed(
    file_id: UUID,
    variant: DownloadThumbnailVariant,
    *,
    client: AuthenticatedClient,
) -> Response[Problem]:
    """Download a thumbnail

     Accepts a fetch ticket in the query parameter `ticket` like `downloadFile` (push preview
    `image_url`, ADR 0048). 409 `file_not_ready` or `file_blocked` as for `downloadFile`.

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        variant (DownloadThumbnailVariant):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem]
    """

    kwargs = _get_kwargs(
        file_id=file_id,
        variant=variant,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    file_id: UUID,
    variant: DownloadThumbnailVariant,
    *,
    client: AuthenticatedClient,
) -> Problem | None:
    """Download a thumbnail

     Accepts a fetch ticket in the query parameter `ticket` like `downloadFile` (push preview
    `image_url`, ADR 0048). 409 `file_not_ready` or `file_blocked` as for `downloadFile`.

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        variant (DownloadThumbnailVariant):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem
    """

    return (
        await asyncio_detailed(
            file_id=file_id,
            variant=variant,
            client=client,
        )
    ).parsed
