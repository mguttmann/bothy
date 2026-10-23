from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.import_file_slot import ImportFileSlot
from ...models.problem import Problem
from ...models.upload_complete import UploadComplete
from ...types import Response


def _get_kwargs(
    import_id: UUID,
    file_id: UUID,
    *,
    body: UploadComplete,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/imports/{import_id}/files/{file_id}/complete".format(
            import_id=quote(str(import_id), safe=""),
            file_id=quote(str(file_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ImportFileSlot | Problem:
    if response.status_code == 200:
        response_200 = ImportFileSlot.from_dict(response.json())

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

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[ImportFileSlot | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    import_id: UUID,
    file_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: UploadComplete,
) -> Response[ImportFileSlot | Problem]:
    """Finish the upload of an imported file

     Completes the upload of the file of a file item of this source with the rules of `completeUpload`
    (12, 3.4): `parts` is checked against the confirmed parts, missing parts are `409 conflict` with
    `reason` `upload_incomplete`, another `etag` `409 conflict` with `reason` `part_conflict`, an
    expired session `410 gone`. The job `files.scan` checks size and SHA-256 against the declaration of
    `createImportFile`, detects the type and scans; it runs with the lowest job priority, so live
    uploads are scanned first (12, 11). The item becomes `posted` in the same transaction; a repeated
    call answers the posted item. It authorizes `import.apply` on the running source instead of
    `file.upload` (05 rule 1.8, 4.12). A file that is not the target of a file item of this source is
    `404`.

    Args:
        import_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (UploadComplete):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ImportFileSlot | Problem]
    """

    kwargs = _get_kwargs(
        import_id=import_id,
        file_id=file_id,
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    import_id: UUID,
    file_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: UploadComplete,
) -> ImportFileSlot | Problem | None:
    """Finish the upload of an imported file

     Completes the upload of the file of a file item of this source with the rules of `completeUpload`
    (12, 3.4): `parts` is checked against the confirmed parts, missing parts are `409 conflict` with
    `reason` `upload_incomplete`, another `etag` `409 conflict` with `reason` `part_conflict`, an
    expired session `410 gone`. The job `files.scan` checks size and SHA-256 against the declaration of
    `createImportFile`, detects the type and scans; it runs with the lowest job priority, so live
    uploads are scanned first (12, 11). The item becomes `posted` in the same transaction; a repeated
    call answers the posted item. It authorizes `import.apply` on the running source instead of
    `file.upload` (05 rule 1.8, 4.12). A file that is not the target of a file item of this source is
    `404`.

    Args:
        import_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (UploadComplete):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ImportFileSlot | Problem
    """

    return sync_detailed(
        import_id=import_id,
        file_id=file_id,
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    import_id: UUID,
    file_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: UploadComplete,
) -> Response[ImportFileSlot | Problem]:
    """Finish the upload of an imported file

     Completes the upload of the file of a file item of this source with the rules of `completeUpload`
    (12, 3.4): `parts` is checked against the confirmed parts, missing parts are `409 conflict` with
    `reason` `upload_incomplete`, another `etag` `409 conflict` with `reason` `part_conflict`, an
    expired session `410 gone`. The job `files.scan` checks size and SHA-256 against the declaration of
    `createImportFile`, detects the type and scans; it runs with the lowest job priority, so live
    uploads are scanned first (12, 11). The item becomes `posted` in the same transaction; a repeated
    call answers the posted item. It authorizes `import.apply` on the running source instead of
    `file.upload` (05 rule 1.8, 4.12). A file that is not the target of a file item of this source is
    `404`.

    Args:
        import_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (UploadComplete):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ImportFileSlot | Problem]
    """

    kwargs = _get_kwargs(
        import_id=import_id,
        file_id=file_id,
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    import_id: UUID,
    file_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: UploadComplete,
) -> ImportFileSlot | Problem | None:
    """Finish the upload of an imported file

     Completes the upload of the file of a file item of this source with the rules of `completeUpload`
    (12, 3.4): `parts` is checked against the confirmed parts, missing parts are `409 conflict` with
    `reason` `upload_incomplete`, another `etag` `409 conflict` with `reason` `part_conflict`, an
    expired session `410 gone`. The job `files.scan` checks size and SHA-256 against the declaration of
    `createImportFile`, detects the type and scans; it runs with the lowest job priority, so live
    uploads are scanned first (12, 11). The item becomes `posted` in the same transaction; a repeated
    call answers the posted item. It authorizes `import.apply` on the running source instead of
    `file.upload` (05 rule 1.8, 4.12). A file that is not the target of a file item of this source is
    `404`.

    Args:
        import_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (UploadComplete):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ImportFileSlot | Problem
    """

    return (
        await asyncio_detailed(
            import_id=import_id,
            file_id=file_id,
            client=client,
            body=body,
        )
    ).parsed
