from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.api_token_create import ApiTokenCreate
from ...models.api_token_created import ApiTokenCreated
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    principal_id: UUID,
    *,
    body: ApiTokenCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(idempotency_key, Unset):
        headers["Idempotency-Key"] = idempotency_key

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/principals/{principal_id}/tokens".format(
            principal_id=quote(str(principal_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ApiTokenCreated | Problem:
    if response.status_code == 201:
        response_201 = ApiTokenCreated.from_dict(response.json())

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
) -> Response[ApiTokenCreated | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    principal_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: ApiTokenCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[ApiTokenCreated | Problem]:
    """Create an API token

     Kind follows the principal (`agt` agent, `app` app, `pat` person, 08 section 12). A personal token
    only for oneself and only with the flag `tokens.personal_enabled`. Until app installations exist
    (M2) a token for an app answers `409 conflict` with `reason` `app_not_installed`. The secret is
    shown once; a replay with the same `Idempotency-Key` answers `409 conflict` with `reason`
    `one_time_secret` (06, 7).

    Args:
        principal_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (ApiTokenCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ApiTokenCreated | Problem]
    """

    kwargs = _get_kwargs(
        principal_id=principal_id,
        body=body,
        idempotency_key=idempotency_key,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    principal_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: ApiTokenCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> ApiTokenCreated | Problem | None:
    """Create an API token

     Kind follows the principal (`agt` agent, `app` app, `pat` person, 08 section 12). A personal token
    only for oneself and only with the flag `tokens.personal_enabled`. Until app installations exist
    (M2) a token for an app answers `409 conflict` with `reason` `app_not_installed`. The secret is
    shown once; a replay with the same `Idempotency-Key` answers `409 conflict` with `reason`
    `one_time_secret` (06, 7).

    Args:
        principal_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (ApiTokenCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ApiTokenCreated | Problem
    """

    return sync_detailed(
        principal_id=principal_id,
        client=client,
        body=body,
        idempotency_key=idempotency_key,
    ).parsed


async def asyncio_detailed(
    principal_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: ApiTokenCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[ApiTokenCreated | Problem]:
    """Create an API token

     Kind follows the principal (`agt` agent, `app` app, `pat` person, 08 section 12). A personal token
    only for oneself and only with the flag `tokens.personal_enabled`. Until app installations exist
    (M2) a token for an app answers `409 conflict` with `reason` `app_not_installed`. The secret is
    shown once; a replay with the same `Idempotency-Key` answers `409 conflict` with `reason`
    `one_time_secret` (06, 7).

    Args:
        principal_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (ApiTokenCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ApiTokenCreated | Problem]
    """

    kwargs = _get_kwargs(
        principal_id=principal_id,
        body=body,
        idempotency_key=idempotency_key,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    principal_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: ApiTokenCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> ApiTokenCreated | Problem | None:
    """Create an API token

     Kind follows the principal (`agt` agent, `app` app, `pat` person, 08 section 12). A personal token
    only for oneself and only with the flag `tokens.personal_enabled`. Until app installations exist
    (M2) a token for an app answers `409 conflict` with `reason` `app_not_installed`. The secret is
    shown once; a replay with the same `Idempotency-Key` answers `409 conflict` with `reason`
    `one_time_secret` (06, 7).

    Args:
        principal_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (ApiTokenCreate):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ApiTokenCreated | Problem
    """

    return (
        await asyncio_detailed(
            principal_id=principal_id,
            client=client,
            body=body,
            idempotency_key=idempotency_key,
        )
    ).parsed
