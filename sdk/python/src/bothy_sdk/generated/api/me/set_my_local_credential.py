from http import HTTPStatus
from typing import Any

import httpx

from ...client import AuthenticatedClient, Client
from ...models.local_credential_result import LocalCredentialResult
from ...models.local_credential_set import LocalCredentialSet
from ...models.problem import Problem
from ...types import Response


def _get_kwargs(
    *,
    body: LocalCredentialSet,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "put",
        "url": "/me/local-credential",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> LocalCredentialResult | Problem:
    if response.status_code == 200:
        response_200 = LocalCredentialResult.from_dict(response.json())

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

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[LocalCredentialResult | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: LocalCredentialSet,
) -> Response[LocalCredentialResult | Problem]:
    """Set local password and enroll TOTP

     Needs a local identity (else `409 conflict`) and `auth.local_password_enabled` (else `403
    feature_disabled`). Step-up is required once the password path is established (TOTP confirmed, 07
    4.2 step 8). The TOTP URI in the response is shown once; the call is a PUT and stores no response.

    Args:
        body (LocalCredentialSet): Sets or replaces the local password, starts a TOTP enrollment
            (`totp_enroll`, the response carries the new secret once) or confirms it (`totp_code`). At
            least one member; the first call sets `password`. The password path counts as established
            once TOTP is confirmed (08, 2.5).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[LocalCredentialResult | Problem]
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
    body: LocalCredentialSet,
) -> LocalCredentialResult | Problem | None:
    """Set local password and enroll TOTP

     Needs a local identity (else `409 conflict`) and `auth.local_password_enabled` (else `403
    feature_disabled`). Step-up is required once the password path is established (TOTP confirmed, 07
    4.2 step 8). The TOTP URI in the response is shown once; the call is a PUT and stores no response.

    Args:
        body (LocalCredentialSet): Sets or replaces the local password, starts a TOTP enrollment
            (`totp_enroll`, the response carries the new secret once) or confirms it (`totp_code`). At
            least one member; the first call sets `password`. The password path counts as established
            once TOTP is confirmed (08, 2.5).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        LocalCredentialResult | Problem
    """

    return sync_detailed(
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: LocalCredentialSet,
) -> Response[LocalCredentialResult | Problem]:
    """Set local password and enroll TOTP

     Needs a local identity (else `409 conflict`) and `auth.local_password_enabled` (else `403
    feature_disabled`). Step-up is required once the password path is established (TOTP confirmed, 07
    4.2 step 8). The TOTP URI in the response is shown once; the call is a PUT and stores no response.

    Args:
        body (LocalCredentialSet): Sets or replaces the local password, starts a TOTP enrollment
            (`totp_enroll`, the response carries the new secret once) or confirms it (`totp_code`). At
            least one member; the first call sets `password`. The password path counts as established
            once TOTP is confirmed (08, 2.5).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[LocalCredentialResult | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: LocalCredentialSet,
) -> LocalCredentialResult | Problem | None:
    """Set local password and enroll TOTP

     Needs a local identity (else `409 conflict`) and `auth.local_password_enabled` (else `403
    feature_disabled`). Step-up is required once the password path is established (TOTP confirmed, 07
    4.2 step 8). The TOTP URI in the response is shown once; the call is a PUT and stores no response.

    Args:
        body (LocalCredentialSet): Sets or replaces the local password, starts a TOTP enrollment
            (`totp_enroll`, the response carries the new secret once) or confirms it (`totp_code`). At
            least one member; the first call sets `password`. The password path counts as established
            once TOTP is confirmed (08, 2.5).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        LocalCredentialResult | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
        )
    ).parsed
