from http import HTTPStatus
from typing import Any

import httpx

from ...client import AuthenticatedClient, Client
from ...models.app_manifest import AppManifest
from ...models.manifest_validation import ManifestValidation
from ...models.problem import Problem
from ...types import Response


def _get_kwargs(
    *,
    body: AppManifest,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/app-manifests/validate",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ManifestValidation | Problem:
    if response.status_code == 200:
        response_200 = ManifestValidation.from_dict(response.json())

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

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[ManifestValidation | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: AppManifest,
) -> Response[ManifestValidation | Problem]:
    """Validate a manifest without installing

     Owner only. Schema violations give `400 validation_failed`; the check rules of 11, 3.2 give `200`
    with `valid` false and every rule error, pointers relative to the manifest root. Nothing is stored
    and no audit entry is written.

    Args:
        body (AppManifest): Schema `app_manifest.v1` (11, 3.1). Unknown members are an error.
            Beyond the schema the server applies the check rules of 11, 3.2: scopes from the
            catalogue, only subscribable events with the scope of their receive action and
            `events:subscribe`, `https` delivery addresses whose host is in `webhook.allowed_hosts`,
            `requested_grants` at most `write` and at most the capability `upload` and never on a
            host, commands without reserved names and at most `command.max_per_principal`, and at most
            `app.manifest_max_bytes`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ManifestValidation | Problem]
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
    body: AppManifest,
) -> ManifestValidation | Problem | None:
    """Validate a manifest without installing

     Owner only. Schema violations give `400 validation_failed`; the check rules of 11, 3.2 give `200`
    with `valid` false and every rule error, pointers relative to the manifest root. Nothing is stored
    and no audit entry is written.

    Args:
        body (AppManifest): Schema `app_manifest.v1` (11, 3.1). Unknown members are an error.
            Beyond the schema the server applies the check rules of 11, 3.2: scopes from the
            catalogue, only subscribable events with the scope of their receive action and
            `events:subscribe`, `https` delivery addresses whose host is in `webhook.allowed_hosts`,
            `requested_grants` at most `write` and at most the capability `upload` and never on a
            host, commands without reserved names and at most `command.max_per_principal`, and at most
            `app.manifest_max_bytes`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ManifestValidation | Problem
    """

    return sync_detailed(
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: AppManifest,
) -> Response[ManifestValidation | Problem]:
    """Validate a manifest without installing

     Owner only. Schema violations give `400 validation_failed`; the check rules of 11, 3.2 give `200`
    with `valid` false and every rule error, pointers relative to the manifest root. Nothing is stored
    and no audit entry is written.

    Args:
        body (AppManifest): Schema `app_manifest.v1` (11, 3.1). Unknown members are an error.
            Beyond the schema the server applies the check rules of 11, 3.2: scopes from the
            catalogue, only subscribable events with the scope of their receive action and
            `events:subscribe`, `https` delivery addresses whose host is in `webhook.allowed_hosts`,
            `requested_grants` at most `write` and at most the capability `upload` and never on a
            host, commands without reserved names and at most `command.max_per_principal`, and at most
            `app.manifest_max_bytes`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ManifestValidation | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: AppManifest,
) -> ManifestValidation | Problem | None:
    """Validate a manifest without installing

     Owner only. Schema violations give `400 validation_failed`; the check rules of 11, 3.2 give `200`
    with `valid` false and every rule error, pointers relative to the manifest root. Nothing is stored
    and no audit entry is written.

    Args:
        body (AppManifest): Schema `app_manifest.v1` (11, 3.1). Unknown members are an error.
            Beyond the schema the server applies the check rules of 11, 3.2: scopes from the
            catalogue, only subscribable events with the scope of their receive action and
            `events:subscribe`, `https` delivery addresses whose host is in `webhook.allowed_hosts`,
            `requested_grants` at most `write` and at most the capability `upload` and never on a
            host, commands without reserved names and at most `command.max_per_principal`, and at most
            `app.manifest_max_bytes`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ManifestValidation | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
        )
    ).parsed
