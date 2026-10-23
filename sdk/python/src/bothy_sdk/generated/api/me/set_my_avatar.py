from http import HTTPStatus
from typing import Any

import httpx

from ...client import AuthenticatedClient, Client
from ...models.image_upload import ImageUpload
from ...models.problem import Problem
from ...models.user import User
from ...types import Response


def _get_kwargs(
    *,
    body: ImageUpload,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "put",
        "url": "/me/avatar",
    }

    _kwargs["files"] = body.to_multipart()

    headers["Content-Type"] = "multipart/form-data; boundary=+++"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | User:
    if response.status_code == 200:
        response_200 = User.from_dict(response.json())

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

    if response.status_code == 413:
        response_413 = Problem.from_dict(response.json())

        return response_413

    if response.status_code == 415:
        response_415 = Problem.from_dict(response.json())

        return response_415

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Problem | User]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: ImageUpload,
) -> Response[Problem | User]:
    """Upload own avatar

     Only for people; nothing of the body is read before `authorize` allowed the action. Checks in this
    order (12 3.7): the first part is `file` (400 `validation_failed`); the declared type is JPEG, PNG
    or WebP (415 `unsupported_media_type`, with `reason` `file_type_denied` for a type in
    `file.denied_mime_types`); the part holds at most the smaller of `file.max_upload_bytes` and
    `file.upload_part_bytes` (413 `payload_too_large`) and at least one byte, and no further part
    follows (400 `validation_failed`); open sessions (429) and free space of the file system driver (503
    `unavailable` with `reason` `storage_full`). The new file is linked at once (`avatar_file_id`) and
    replaces the previous avatar; it becomes deliverable later, after scan and metadata removal, and
    until then file downloads answer 409 `file_not_ready`.

    Args:
        body (ImageUpload): Image as the one multipart part `file` (a further part is rejected
            with 400 `validation_failed`). The server detects the type from the bytes; it accepts only
            JPEG and PNG, whose metadata it removes losslessly, and still WebP, which it converts to
            PNG or JPEG without metadata. Thumbnails are re-encoded. Any other detected type ends the
            file `failed` with `file_type_denied`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | User]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    body: ImageUpload,
) -> Problem | User | None:
    """Upload own avatar

     Only for people; nothing of the body is read before `authorize` allowed the action. Checks in this
    order (12 3.7): the first part is `file` (400 `validation_failed`); the declared type is JPEG, PNG
    or WebP (415 `unsupported_media_type`, with `reason` `file_type_denied` for a type in
    `file.denied_mime_types`); the part holds at most the smaller of `file.max_upload_bytes` and
    `file.upload_part_bytes` (413 `payload_too_large`) and at least one byte, and no further part
    follows (400 `validation_failed`); open sessions (429) and free space of the file system driver (503
    `unavailable` with `reason` `storage_full`). The new file is linked at once (`avatar_file_id`) and
    replaces the previous avatar; it becomes deliverable later, after scan and metadata removal, and
    until then file downloads answer 409 `file_not_ready`.

    Args:
        body (ImageUpload): Image as the one multipart part `file` (a further part is rejected
            with 400 `validation_failed`). The server detects the type from the bytes; it accepts only
            JPEG and PNG, whose metadata it removes losslessly, and still WebP, which it converts to
            PNG or JPEG without metadata. Thumbnails are re-encoded. Any other detected type ends the
            file `failed` with `file_type_denied`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | User
    """

    return sync_detailed(
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: ImageUpload,
) -> Response[Problem | User]:
    """Upload own avatar

     Only for people; nothing of the body is read before `authorize` allowed the action. Checks in this
    order (12 3.7): the first part is `file` (400 `validation_failed`); the declared type is JPEG, PNG
    or WebP (415 `unsupported_media_type`, with `reason` `file_type_denied` for a type in
    `file.denied_mime_types`); the part holds at most the smaller of `file.max_upload_bytes` and
    `file.upload_part_bytes` (413 `payload_too_large`) and at least one byte, and no further part
    follows (400 `validation_failed`); open sessions (429) and free space of the file system driver (503
    `unavailable` with `reason` `storage_full`). The new file is linked at once (`avatar_file_id`) and
    replaces the previous avatar; it becomes deliverable later, after scan and metadata removal, and
    until then file downloads answer 409 `file_not_ready`.

    Args:
        body (ImageUpload): Image as the one multipart part `file` (a further part is rejected
            with 400 `validation_failed`). The server detects the type from the bytes; it accepts only
            JPEG and PNG, whose metadata it removes losslessly, and still WebP, which it converts to
            PNG or JPEG without metadata. Thumbnails are re-encoded. Any other detected type ends the
            file `failed` with `file_type_denied`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | User]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: ImageUpload,
) -> Problem | User | None:
    """Upload own avatar

     Only for people; nothing of the body is read before `authorize` allowed the action. Checks in this
    order (12 3.7): the first part is `file` (400 `validation_failed`); the declared type is JPEG, PNG
    or WebP (415 `unsupported_media_type`, with `reason` `file_type_denied` for a type in
    `file.denied_mime_types`); the part holds at most the smaller of `file.max_upload_bytes` and
    `file.upload_part_bytes` (413 `payload_too_large`) and at least one byte, and no further part
    follows (400 `validation_failed`); open sessions (429) and free space of the file system driver (503
    `unavailable` with `reason` `storage_full`). The new file is linked at once (`avatar_file_id`) and
    replaces the previous avatar; it becomes deliverable later, after scan and metadata removal, and
    until then file downloads answer 409 `file_not_ready`.

    Args:
        body (ImageUpload): Image as the one multipart part `file` (a further part is rejected
            with 400 `validation_failed`). The server detects the type from the bytes; it accepts only
            JPEG and PNG, whose metadata it removes losslessly, and still WebP, which it converts to
            PNG or JPEG without metadata. Thumbnails are re-encoded. Any other detected type ends the
            file `failed` with `file_type_denied`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | User
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
        )
    ).parsed
