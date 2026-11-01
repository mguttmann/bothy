from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.host import Host
from ...models.host_confirm import HostConfirm
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    host_id: UUID,
    *,
    body: HostConfirm,
    idempotency_key: UUID | Unset = UNSET,
    if_match: str,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(idempotency_key, Unset):
        headers["Idempotency-Key"] = idempotency_key

    headers["If-Match"] = if_match

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/hosts/{host_id}/confirm".format(
            host_id=quote(str(host_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Host | Problem:
    if response.status_code == 200:
        response_200 = Host.from_dict(response.json())

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

    if response.status_code == 412:
        response_412 = Problem.from_dict(response.json())

        return response_412

    if response.status_code == 428:
        response_428 = Problem.from_dict(response.json())

        return response_428

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Host | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    host_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: HostConfirm,
    idempotency_key: UUID | Unset = UNSET,
    if_match: str,
) -> Response[Host | Problem]:
    """Confirm the fingerprint and declare the LLM login

     The owner types the first `terminal.fingerprint_confirm_chars` characters shown by `bothy-host
    pair`, without separators, case-insensitive; a prefix of another length or with a character outside
    the Crockford alphabet (also `I`, `L`, `O`, `U`) gives `400 validation_failed` with `errors[].code`
    `invalid_format` and does not count. A mismatch gives `409 fingerprint_mismatch` with
    `attempts_remaining`; the failure counter is not part of `Host`, so `version` and the `If-Match`
    stay. After `terminal.fingerprint_max_attempts` failures the host falls back to `pairing` without
    keys (`attempts_remaining` `0`), and a new pairing code is needed. A failed attempt counts also
    under an `Idempotency-Key`; its replay returns the stored `409`. I6 is checked in the same
    transaction (`409 i6_violation`). `session_users` assigns each person their own session user on this
    host (ruling T12, 24 section 2.2); a name that belongs or belonged to another person on this host,
    or a second valid assignment of one person, gives `409 conflict` with `reason: session_user_taken`;
    a name the host did not report (`reported_session_users`) gives `400 validation_failed` with
    `errors[].code` `session_user_not_on_host`, and nothing is stored. The owner account of the host
    (`owner_account` in `reported_session_users`, ADR 0073) needs `owner_account: true` and only an
    active owner may assign it, only to himself (`403 forbidden` with `reason:
    owner_account_owner_only`). Without an assignment a person gets no terminal there (SR12). Needs
    step-up.

    Args:
        host_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str):
        body (HostConfirm):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Host | Problem]
    """

    kwargs = _get_kwargs(
        host_id=host_id,
        body=body,
        idempotency_key=idempotency_key,
        if_match=if_match,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    host_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: HostConfirm,
    idempotency_key: UUID | Unset = UNSET,
    if_match: str,
) -> Host | Problem | None:
    """Confirm the fingerprint and declare the LLM login

     The owner types the first `terminal.fingerprint_confirm_chars` characters shown by `bothy-host
    pair`, without separators, case-insensitive; a prefix of another length or with a character outside
    the Crockford alphabet (also `I`, `L`, `O`, `U`) gives `400 validation_failed` with `errors[].code`
    `invalid_format` and does not count. A mismatch gives `409 fingerprint_mismatch` with
    `attempts_remaining`; the failure counter is not part of `Host`, so `version` and the `If-Match`
    stay. After `terminal.fingerprint_max_attempts` failures the host falls back to `pairing` without
    keys (`attempts_remaining` `0`), and a new pairing code is needed. A failed attempt counts also
    under an `Idempotency-Key`; its replay returns the stored `409`. I6 is checked in the same
    transaction (`409 i6_violation`). `session_users` assigns each person their own session user on this
    host (ruling T12, 24 section 2.2); a name that belongs or belonged to another person on this host,
    or a second valid assignment of one person, gives `409 conflict` with `reason: session_user_taken`;
    a name the host did not report (`reported_session_users`) gives `400 validation_failed` with
    `errors[].code` `session_user_not_on_host`, and nothing is stored. The owner account of the host
    (`owner_account` in `reported_session_users`, ADR 0073) needs `owner_account: true` and only an
    active owner may assign it, only to himself (`403 forbidden` with `reason:
    owner_account_owner_only`). Without an assignment a person gets no terminal there (SR12). Needs
    step-up.

    Args:
        host_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str):
        body (HostConfirm):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Host | Problem
    """

    return sync_detailed(
        host_id=host_id,
        client=client,
        body=body,
        idempotency_key=idempotency_key,
        if_match=if_match,
    ).parsed


async def asyncio_detailed(
    host_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: HostConfirm,
    idempotency_key: UUID | Unset = UNSET,
    if_match: str,
) -> Response[Host | Problem]:
    """Confirm the fingerprint and declare the LLM login

     The owner types the first `terminal.fingerprint_confirm_chars` characters shown by `bothy-host
    pair`, without separators, case-insensitive; a prefix of another length or with a character outside
    the Crockford alphabet (also `I`, `L`, `O`, `U`) gives `400 validation_failed` with `errors[].code`
    `invalid_format` and does not count. A mismatch gives `409 fingerprint_mismatch` with
    `attempts_remaining`; the failure counter is not part of `Host`, so `version` and the `If-Match`
    stay. After `terminal.fingerprint_max_attempts` failures the host falls back to `pairing` without
    keys (`attempts_remaining` `0`), and a new pairing code is needed. A failed attempt counts also
    under an `Idempotency-Key`; its replay returns the stored `409`. I6 is checked in the same
    transaction (`409 i6_violation`). `session_users` assigns each person their own session user on this
    host (ruling T12, 24 section 2.2); a name that belongs or belonged to another person on this host,
    or a second valid assignment of one person, gives `409 conflict` with `reason: session_user_taken`;
    a name the host did not report (`reported_session_users`) gives `400 validation_failed` with
    `errors[].code` `session_user_not_on_host`, and nothing is stored. The owner account of the host
    (`owner_account` in `reported_session_users`, ADR 0073) needs `owner_account: true` and only an
    active owner may assign it, only to himself (`403 forbidden` with `reason:
    owner_account_owner_only`). Without an assignment a person gets no terminal there (SR12). Needs
    step-up.

    Args:
        host_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str):
        body (HostConfirm):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Host | Problem]
    """

    kwargs = _get_kwargs(
        host_id=host_id,
        body=body,
        idempotency_key=idempotency_key,
        if_match=if_match,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    host_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: HostConfirm,
    idempotency_key: UUID | Unset = UNSET,
    if_match: str,
) -> Host | Problem | None:
    """Confirm the fingerprint and declare the LLM login

     The owner types the first `terminal.fingerprint_confirm_chars` characters shown by `bothy-host
    pair`, without separators, case-insensitive; a prefix of another length or with a character outside
    the Crockford alphabet (also `I`, `L`, `O`, `U`) gives `400 validation_failed` with `errors[].code`
    `invalid_format` and does not count. A mismatch gives `409 fingerprint_mismatch` with
    `attempts_remaining`; the failure counter is not part of `Host`, so `version` and the `If-Match`
    stay. After `terminal.fingerprint_max_attempts` failures the host falls back to `pairing` without
    keys (`attempts_remaining` `0`), and a new pairing code is needed. A failed attempt counts also
    under an `Idempotency-Key`; its replay returns the stored `409`. I6 is checked in the same
    transaction (`409 i6_violation`). `session_users` assigns each person their own session user on this
    host (ruling T12, 24 section 2.2); a name that belongs or belonged to another person on this host,
    or a second valid assignment of one person, gives `409 conflict` with `reason: session_user_taken`;
    a name the host did not report (`reported_session_users`) gives `400 validation_failed` with
    `errors[].code` `session_user_not_on_host`, and nothing is stored. The owner account of the host
    (`owner_account` in `reported_session_users`, ADR 0073) needs `owner_account: true` and only an
    active owner may assign it, only to himself (`403 forbidden` with `reason:
    owner_account_owner_only`). Without an assignment a person gets no terminal there (SR12). Needs
    step-up.

    Args:
        host_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str):
        body (HostConfirm):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Host | Problem
    """

    return (
        await asyncio_detailed(
            host_id=host_id,
            client=client,
            body=body,
            idempotency_key=idempotency_key,
            if_match=if_match,
        )
    ).parsed
