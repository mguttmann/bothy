from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.enrollment_code import EnrollmentCode
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    user_id: UUID,
    *,
    idempotency_key: UUID | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(idempotency_key, Unset):
        headers["Idempotency-Key"] = idempotency_key

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/users/{user_id}/enrollment-codes".format(
            user_id=quote(str(user_id), safe=""),
        ),
    }

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> EnrollmentCode | Problem:
    if response.status_code == 201:
        response_201 = EnrollmentCode.from_dict(response.json())

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
) -> Response[EnrollmentCode | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    user_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[EnrollmentCode | Problem]:
    """Issue an enrollment code

     Issues a code for an active person: `first_login` for a person with a local identity and no device
    yet, `recovery` for a person with a local identity and a device, `identity_link` for a person
    without a local identity (08, 3.2). Issuing ends every other unused code of the person. The code is
    a one-time secret: a replay with the same `Idempotency-Key` answers `409 conflict` with `reason`
    `one_time_secret` instead of repeating it (06, 7).

    Args:
        user_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[EnrollmentCode | Problem]
    """

    kwargs = _get_kwargs(
        user_id=user_id,
        idempotency_key=idempotency_key,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    user_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    idempotency_key: UUID | Unset = UNSET,
) -> EnrollmentCode | Problem | None:
    """Issue an enrollment code

     Issues a code for an active person: `first_login` for a person with a local identity and no device
    yet, `recovery` for a person with a local identity and a device, `identity_link` for a person
    without a local identity (08, 3.2). Issuing ends every other unused code of the person. The code is
    a one-time secret: a replay with the same `Idempotency-Key` answers `409 conflict` with `reason`
    `one_time_secret` instead of repeating it (06, 7).

    Args:
        user_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        EnrollmentCode | Problem
    """

    return sync_detailed(
        user_id=user_id,
        client=client,
        idempotency_key=idempotency_key,
    ).parsed


async def asyncio_detailed(
    user_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[EnrollmentCode | Problem]:
    """Issue an enrollment code

     Issues a code for an active person: `first_login` for a person with a local identity and no device
    yet, `recovery` for a person with a local identity and a device, `identity_link` for a person
    without a local identity (08, 3.2). Issuing ends every other unused code of the person. The code is
    a one-time secret: a replay with the same `Idempotency-Key` answers `409 conflict` with `reason`
    `one_time_secret` instead of repeating it (06, 7).

    Args:
        user_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[EnrollmentCode | Problem]
    """

    kwargs = _get_kwargs(
        user_id=user_id,
        idempotency_key=idempotency_key,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    user_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    idempotency_key: UUID | Unset = UNSET,
) -> EnrollmentCode | Problem | None:
    """Issue an enrollment code

     Issues a code for an active person: `first_login` for a person with a local identity and no device
    yet, `recovery` for a person with a local identity and a device, `identity_link` for a person
    without a local identity (08, 3.2). Issuing ends every other unused code of the person. The code is
    a one-time secret: a replay with the same `Idempotency-Key` answers `409 conflict` with `reason`
    `one_time_secret` instead of repeating it (06, 7).

    Args:
        user_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        EnrollmentCode | Problem
    """

    return (
        await asyncio_detailed(
            user_id=user_id,
            client=client,
            idempotency_key=idempotency_key,
        )
    ).parsed
