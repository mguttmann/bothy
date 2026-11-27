from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.host import Host
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    host_id: UUID,
    *,
    idempotency_key: UUID | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(idempotency_key, Unset):
        headers["Idempotency-Key"] = str(idempotency_key)

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/hosts/{host_id}/computer/disable".format(
            host_id=quote(str(host_id), safe=""),
        ),
    }

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Host | Problem:
    if response.status_code == 200:
        response_200 = Host.from_dict(response.json())

        return response_200

    if response.status_code == 401:
        response_401 = Problem.from_dict(response.json())

        return response_401

    if response.status_code == 403:
        response_403 = Problem.from_dict(response.json())

        return response_403

    if response.status_code == 404:
        response_404 = Problem.from_dict(response.json())

        return response_404

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
    idempotency_key: UUID | Unset = UNSET,
) -> Response[Host | Problem]:
    """Switch the computer enablement of a host off

     Any active owner in a session switches the enablement of a host off, from any device, without step-
    up (docs/spec/27-computer-use.md 4 item 4). The server stores the reported generation as the
    disabled generation, ends the host's computer session with `device_disabled` in the same transaction
    and sends `computer.disable` over the control connection; the host stays unlisted until a new local
    switch-on raises the generation. A host without a reported helper answers unchanged. Switching on is
    no operation of the API. Audited.

    Args:
        host_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Host | Problem]
    """

    kwargs = _get_kwargs(
        host_id=host_id,
        idempotency_key=idempotency_key,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    host_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    idempotency_key: UUID | Unset = UNSET,
) -> Host | Problem | None:
    """Switch the computer enablement of a host off

     Any active owner in a session switches the enablement of a host off, from any device, without step-
    up (docs/spec/27-computer-use.md 4 item 4). The server stores the reported generation as the
    disabled generation, ends the host's computer session with `device_disabled` in the same transaction
    and sends `computer.disable` over the control connection; the host stays unlisted until a new local
    switch-on raises the generation. A host without a reported helper answers unchanged. Switching on is
    no operation of the API. Audited.

    Args:
        host_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Host | Problem
    """

    return sync_detailed(
        host_id=host_id,
        client=client,
        idempotency_key=idempotency_key,
    ).parsed


async def asyncio_detailed(
    host_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[Host | Problem]:
    """Switch the computer enablement of a host off

     Any active owner in a session switches the enablement of a host off, from any device, without step-
    up (docs/spec/27-computer-use.md 4 item 4). The server stores the reported generation as the
    disabled generation, ends the host's computer session with `device_disabled` in the same transaction
    and sends `computer.disable` over the control connection; the host stays unlisted until a new local
    switch-on raises the generation. A host without a reported helper answers unchanged. Switching on is
    no operation of the API. Audited.

    Args:
        host_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Host | Problem]
    """

    kwargs = _get_kwargs(
        host_id=host_id,
        idempotency_key=idempotency_key,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    host_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    idempotency_key: UUID | Unset = UNSET,
) -> Host | Problem | None:
    """Switch the computer enablement of a host off

     Any active owner in a session switches the enablement of a host off, from any device, without step-
    up (docs/spec/27-computer-use.md 4 item 4). The server stores the reported generation as the
    disabled generation, ends the host's computer session with `device_disabled` in the same transaction
    and sends `computer.disable` over the control connection; the host stays unlisted until a new local
    switch-on raises the generation. A host without a reported helper answers unchanged. Switching on is
    no operation of the API. Audited.

    Args:
        host_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

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
            idempotency_key=idempotency_key,
        )
    ).parsed
