from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.identity_provider import IdentityProvider
from ...models.identity_provider_update import IdentityProviderUpdate
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    provider_id: UUID,
    *,
    body: IdentityProviderUpdate,
    idempotency_key: UUID | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(idempotency_key, Unset):
        headers["Idempotency-Key"] = idempotency_key

    _kwargs: dict[str, Any] = {
        "method": "patch",
        "url": "/identity-providers/{provider_id}".format(
            provider_id=quote(str(provider_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> IdentityProvider | Problem:
    if response.status_code == 200:
        response_200 = IdentityProvider.from_dict(response.json())

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
) -> Response[IdentityProvider | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    provider_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: IdentityProviderUpdate,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[IdentityProvider | Problem]:
    """Change an identity provider

     Changes an identity provider; absent members stay. Setting `enabled` to true on a `trusted_header`
    provider while the public listener is not bound to a loopback address is refused with `409
    conflict`, `reason: trusted_header_requires_loopback` and a `hint` naming `server.listen_addr` (08,
    2.4 rule 1). Disabling a provider ends the sessions of its identities (08, 3.3).

    Args:
        provider_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (IdentityProviderUpdate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[IdentityProvider | Problem]
    """

    kwargs = _get_kwargs(
        provider_id=provider_id,
        body=body,
        idempotency_key=idempotency_key,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    provider_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: IdentityProviderUpdate,
    idempotency_key: UUID | Unset = UNSET,
) -> IdentityProvider | Problem | None:
    """Change an identity provider

     Changes an identity provider; absent members stay. Setting `enabled` to true on a `trusted_header`
    provider while the public listener is not bound to a loopback address is refused with `409
    conflict`, `reason: trusted_header_requires_loopback` and a `hint` naming `server.listen_addr` (08,
    2.4 rule 1). Disabling a provider ends the sessions of its identities (08, 3.3).

    Args:
        provider_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (IdentityProviderUpdate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        IdentityProvider | Problem
    """

    return sync_detailed(
        provider_id=provider_id,
        client=client,
        body=body,
        idempotency_key=idempotency_key,
    ).parsed


async def asyncio_detailed(
    provider_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: IdentityProviderUpdate,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[IdentityProvider | Problem]:
    """Change an identity provider

     Changes an identity provider; absent members stay. Setting `enabled` to true on a `trusted_header`
    provider while the public listener is not bound to a loopback address is refused with `409
    conflict`, `reason: trusted_header_requires_loopback` and a `hint` naming `server.listen_addr` (08,
    2.4 rule 1). Disabling a provider ends the sessions of its identities (08, 3.3).

    Args:
        provider_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (IdentityProviderUpdate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[IdentityProvider | Problem]
    """

    kwargs = _get_kwargs(
        provider_id=provider_id,
        body=body,
        idempotency_key=idempotency_key,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    provider_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: IdentityProviderUpdate,
    idempotency_key: UUID | Unset = UNSET,
) -> IdentityProvider | Problem | None:
    """Change an identity provider

     Changes an identity provider; absent members stay. Setting `enabled` to true on a `trusted_header`
    provider while the public listener is not bound to a loopback address is refused with `409
    conflict`, `reason: trusted_header_requires_loopback` and a `hint` naming `server.listen_addr` (08,
    2.4 rule 1). Disabling a provider ends the sessions of its identities (08, 3.3).

    Args:
        provider_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (IdentityProviderUpdate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        IdentityProvider | Problem
    """

    return (
        await asyncio_detailed(
            provider_id=provider_id,
            client=client,
            body=body,
            idempotency_key=idempotency_key,
        )
    ).parsed
