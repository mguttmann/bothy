from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.api_token_created import ApiTokenCreated
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    agent_id: UUID,
    *,
    idempotency_key: UUID | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(idempotency_key, Unset):
        headers["Idempotency-Key"] = idempotency_key

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/agents/me/hosted/{agent_id}/token".format(
            agent_id=quote(str(agent_id), safe=""),
        ),
    }

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
    agent_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[ApiTokenCreated | Problem]:
    """Issue a token for a hosted agent

     Issues an API token for the built-in agent `agent_id` to its hosting runner (26 3.3 item 1). Only
    the runner named in the agent's `hosted_by` may call it, with its own DPoP-bound agent token, while
    both are active and the runner's last stored heartbeat reports `hosts_agents`; every other caller
    and every other case get `404` like an unknown agent, audited as a denial. The token has kind `agt`,
    is bound to the `dpop_jkt` of the calling token (a calling token without one is `409 conflict` with
    `reason` `dpop_required`) and expires after `agents.hosted_token_ttl`; every older active token of
    the agent from the same runner is revoked in the same transaction. One audit entry names the agent,
    the runner and the token prefix, never the value. The secret is shown once; a replay with the same
    `Idempotency-Key` answers `409 conflict` with `reason` `one_time_secret` (06, 7). A person never
    receives such a token: `createToken` for a built-in agent is `409 conflict` with `reason`
    `hosted_agent_token`.

    Args:
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ApiTokenCreated | Problem]
    """

    kwargs = _get_kwargs(
        agent_id=agent_id,
        idempotency_key=idempotency_key,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    agent_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    idempotency_key: UUID | Unset = UNSET,
) -> ApiTokenCreated | Problem | None:
    """Issue a token for a hosted agent

     Issues an API token for the built-in agent `agent_id` to its hosting runner (26 3.3 item 1). Only
    the runner named in the agent's `hosted_by` may call it, with its own DPoP-bound agent token, while
    both are active and the runner's last stored heartbeat reports `hosts_agents`; every other caller
    and every other case get `404` like an unknown agent, audited as a denial. The token has kind `agt`,
    is bound to the `dpop_jkt` of the calling token (a calling token without one is `409 conflict` with
    `reason` `dpop_required`) and expires after `agents.hosted_token_ttl`; every older active token of
    the agent from the same runner is revoked in the same transaction. One audit entry names the agent,
    the runner and the token prefix, never the value. The secret is shown once; a replay with the same
    `Idempotency-Key` answers `409 conflict` with `reason` `one_time_secret` (06, 7). A person never
    receives such a token: `createToken` for a built-in agent is `409 conflict` with `reason`
    `hosted_agent_token`.

    Args:
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ApiTokenCreated | Problem
    """

    return sync_detailed(
        agent_id=agent_id,
        client=client,
        idempotency_key=idempotency_key,
    ).parsed


async def asyncio_detailed(
    agent_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[ApiTokenCreated | Problem]:
    """Issue a token for a hosted agent

     Issues an API token for the built-in agent `agent_id` to its hosting runner (26 3.3 item 1). Only
    the runner named in the agent's `hosted_by` may call it, with its own DPoP-bound agent token, while
    both are active and the runner's last stored heartbeat reports `hosts_agents`; every other caller
    and every other case get `404` like an unknown agent, audited as a denial. The token has kind `agt`,
    is bound to the `dpop_jkt` of the calling token (a calling token without one is `409 conflict` with
    `reason` `dpop_required`) and expires after `agents.hosted_token_ttl`; every older active token of
    the agent from the same runner is revoked in the same transaction. One audit entry names the agent,
    the runner and the token prefix, never the value. The secret is shown once; a replay with the same
    `Idempotency-Key` answers `409 conflict` with `reason` `one_time_secret` (06, 7). A person never
    receives such a token: `createToken` for a built-in agent is `409 conflict` with `reason`
    `hosted_agent_token`.

    Args:
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ApiTokenCreated | Problem]
    """

    kwargs = _get_kwargs(
        agent_id=agent_id,
        idempotency_key=idempotency_key,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    agent_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    idempotency_key: UUID | Unset = UNSET,
) -> ApiTokenCreated | Problem | None:
    """Issue a token for a hosted agent

     Issues an API token for the built-in agent `agent_id` to its hosting runner (26 3.3 item 1). Only
    the runner named in the agent's `hosted_by` may call it, with its own DPoP-bound agent token, while
    both are active and the runner's last stored heartbeat reports `hosts_agents`; every other caller
    and every other case get `404` like an unknown agent, audited as a denial. The token has kind `agt`,
    is bound to the `dpop_jkt` of the calling token (a calling token without one is `409 conflict` with
    `reason` `dpop_required`) and expires after `agents.hosted_token_ttl`; every older active token of
    the agent from the same runner is revoked in the same transaction. One audit entry names the agent,
    the runner and the token prefix, never the value. The secret is shown once; a replay with the same
    `Idempotency-Key` answers `409 conflict` with `reason` `one_time_secret` (06, 7). A person never
    receives such a token: `createToken` for a built-in agent is `409 conflict` with `reason`
    `hosted_agent_token`.

    Args:
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ApiTokenCreated | Problem
    """

    return (
        await asyncio_detailed(
            agent_id=agent_id,
            client=client,
            idempotency_key=idempotency_key,
        )
    ).parsed
