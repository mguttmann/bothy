from http import HTTPStatus
from typing import Any

import httpx

from ...client import AuthenticatedClient, Client
from ...models.computer_session_grant import ComputerSessionGrant
from ...models.computer_session_open import ComputerSessionOpen
from ...models.problem import Problem
from ...types import Response


def _get_kwargs(
    *,
    body: ComputerSessionOpen,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/computer/sessions",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ComputerSessionGrant | Problem:
    if response.status_code == 201:
        response_201 = ComputerSessionGrant.from_dict(response.json())

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

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[ComputerSessionGrant | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: ComputerSessionOpen,
) -> Response[ComputerSessionGrant | Problem]:
    """Open a computer session on a desktop for the bound run

     Opens a computer session of the bound run on a desktop of its owner (docs/spec/27-computer-use.md
    5.1). Only an agent bound by the header `Invocation-Token` to an active run (07 SR9) under tool
    class `host` (SR2) that an active owner started with an own message and that no other person fed
    (SR13). The checks in this order: 403 `invocation_token_invalid` (`run_inactive` for an ended run);
    404 `not_found` for a host the owner cannot read; 403 `forbidden` with `reason: computer_tool_class`
    or `computer_not_owner_run`; 409 `host_unavailable` for a host that is not `active` or not
    connected; 403 `forbidden` with `reason: computer_not_enabled` for a host that is no enabled desktop
    of the owner; 409 `conflict` with `reason: computer_busy` (another session on the host) or
    `computer_run_busy` (another session of the run). One transaction writes the session in `opening`,
    the audit entry and `computer_session.changed`; after the commit the server sends `computer.open` to
    the host service. The answer carries the single-use attach token (`typ = computer-session+jwt`,
    valid `computer.attach_token_ttl`), which is never stored; the request is exempt from `Idempotency-
    Key`. Allows and denials are audited.

    Args:
        body (ComputerSessionOpen): Request of `computer.open`: the desktop and the hash of the
            ephemeral key of `bothy-computer-mcp` (27 5.1).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ComputerSessionGrant | Problem]
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
    body: ComputerSessionOpen,
) -> ComputerSessionGrant | Problem | None:
    """Open a computer session on a desktop for the bound run

     Opens a computer session of the bound run on a desktop of its owner (docs/spec/27-computer-use.md
    5.1). Only an agent bound by the header `Invocation-Token` to an active run (07 SR9) under tool
    class `host` (SR2) that an active owner started with an own message and that no other person fed
    (SR13). The checks in this order: 403 `invocation_token_invalid` (`run_inactive` for an ended run);
    404 `not_found` for a host the owner cannot read; 403 `forbidden` with `reason: computer_tool_class`
    or `computer_not_owner_run`; 409 `host_unavailable` for a host that is not `active` or not
    connected; 403 `forbidden` with `reason: computer_not_enabled` for a host that is no enabled desktop
    of the owner; 409 `conflict` with `reason: computer_busy` (another session on the host) or
    `computer_run_busy` (another session of the run). One transaction writes the session in `opening`,
    the audit entry and `computer_session.changed`; after the commit the server sends `computer.open` to
    the host service. The answer carries the single-use attach token (`typ = computer-session+jwt`,
    valid `computer.attach_token_ttl`), which is never stored; the request is exempt from `Idempotency-
    Key`. Allows and denials are audited.

    Args:
        body (ComputerSessionOpen): Request of `computer.open`: the desktop and the hash of the
            ephemeral key of `bothy-computer-mcp` (27 5.1).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ComputerSessionGrant | Problem
    """

    return sync_detailed(
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: ComputerSessionOpen,
) -> Response[ComputerSessionGrant | Problem]:
    """Open a computer session on a desktop for the bound run

     Opens a computer session of the bound run on a desktop of its owner (docs/spec/27-computer-use.md
    5.1). Only an agent bound by the header `Invocation-Token` to an active run (07 SR9) under tool
    class `host` (SR2) that an active owner started with an own message and that no other person fed
    (SR13). The checks in this order: 403 `invocation_token_invalid` (`run_inactive` for an ended run);
    404 `not_found` for a host the owner cannot read; 403 `forbidden` with `reason: computer_tool_class`
    or `computer_not_owner_run`; 409 `host_unavailable` for a host that is not `active` or not
    connected; 403 `forbidden` with `reason: computer_not_enabled` for a host that is no enabled desktop
    of the owner; 409 `conflict` with `reason: computer_busy` (another session on the host) or
    `computer_run_busy` (another session of the run). One transaction writes the session in `opening`,
    the audit entry and `computer_session.changed`; after the commit the server sends `computer.open` to
    the host service. The answer carries the single-use attach token (`typ = computer-session+jwt`,
    valid `computer.attach_token_ttl`), which is never stored; the request is exempt from `Idempotency-
    Key`. Allows and denials are audited.

    Args:
        body (ComputerSessionOpen): Request of `computer.open`: the desktop and the hash of the
            ephemeral key of `bothy-computer-mcp` (27 5.1).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ComputerSessionGrant | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: ComputerSessionOpen,
) -> ComputerSessionGrant | Problem | None:
    """Open a computer session on a desktop for the bound run

     Opens a computer session of the bound run on a desktop of its owner (docs/spec/27-computer-use.md
    5.1). Only an agent bound by the header `Invocation-Token` to an active run (07 SR9) under tool
    class `host` (SR2) that an active owner started with an own message and that no other person fed
    (SR13). The checks in this order: 403 `invocation_token_invalid` (`run_inactive` for an ended run);
    404 `not_found` for a host the owner cannot read; 403 `forbidden` with `reason: computer_tool_class`
    or `computer_not_owner_run`; 409 `host_unavailable` for a host that is not `active` or not
    connected; 403 `forbidden` with `reason: computer_not_enabled` for a host that is no enabled desktop
    of the owner; 409 `conflict` with `reason: computer_busy` (another session on the host) or
    `computer_run_busy` (another session of the run). One transaction writes the session in `opening`,
    the audit entry and `computer_session.changed`; after the commit the server sends `computer.open` to
    the host service. The answer carries the single-use attach token (`typ = computer-session+jwt`,
    valid `computer.attach_token_ttl`), which is never stored; the request is exempt from `Idempotency-
    Key`. Allows and denials are audited.

    Args:
        body (ComputerSessionOpen): Request of `computer.open`: the desktop and the hash of the
            ephemeral key of `bothy-computer-mcp` (27 5.1).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ComputerSessionGrant | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
        )
    ).parsed
