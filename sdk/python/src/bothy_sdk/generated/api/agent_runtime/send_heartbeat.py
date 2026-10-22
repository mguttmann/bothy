from http import HTTPStatus
from typing import Any

import httpx

from ...client import AuthenticatedClient, Client
from ...models.heartbeat import Heartbeat
from ...models.heartbeat_ack import HeartbeatAck
from ...models.problem import Problem
from ...types import Response


def _get_kwargs(
    *,
    body: Heartbeat,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/agents/me/heartbeat",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> HeartbeatAck | Problem:
    if response.status_code == 200:
        response_200 = HeartbeatAck.from_dict(response.json())

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
) -> Response[HeartbeatAck | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: Heartbeat,
) -> Response[HeartbeatAck | Problem]:
    """Report state and actual billing mode

     The adapter reports the whole state of its runtime (`agent_heartbeat.v1`, 09 section 4.8); the
    member `agent` of the gateway op `heartbeat` has the same effect. Only an agent may call it (`agent-
    self`); the body carries no agent id. The last heartbeat wins: an absent optional member means "not
    reported", except `project_readiness`, which is applied only when present and changed since the last
    list this server process applied (`ApplyProjectReadiness` of M2-03, in its own transaction).

    The server validates every member and the closed keys of `details` itself, also the patterns of
    names, versions and codes; a violation answers `400 validation_failed` with every field error,
    `rate_limit` answers `errors[].code` `not_enabled` until it is built. A change of
    `reported_billing_mode` or `reported_tool_class`, also from or to "not reported", is stored at once
    and audited with action `agent.heartbeat`; the other fields are stored at most once per
    `agent.heartbeat_persist_interval`. The presence becomes `offline` once `agent.presence_timeout`
    passes without a heartbeat. The server compares the reported with the configured mode at each
    heartbeat and returns the warnings of `AgentRuntimeStatus.warnings`; the reported mode is evidence
    for display, warning, audit and alarm and never a permission (BR-05). `interval_ms` is
    `sync.heartbeat_interval`.

    Args:
        body (Heartbeat):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[HeartbeatAck | Problem]
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
    body: Heartbeat,
) -> HeartbeatAck | Problem | None:
    """Report state and actual billing mode

     The adapter reports the whole state of its runtime (`agent_heartbeat.v1`, 09 section 4.8); the
    member `agent` of the gateway op `heartbeat` has the same effect. Only an agent may call it (`agent-
    self`); the body carries no agent id. The last heartbeat wins: an absent optional member means "not
    reported", except `project_readiness`, which is applied only when present and changed since the last
    list this server process applied (`ApplyProjectReadiness` of M2-03, in its own transaction).

    The server validates every member and the closed keys of `details` itself, also the patterns of
    names, versions and codes; a violation answers `400 validation_failed` with every field error,
    `rate_limit` answers `errors[].code` `not_enabled` until it is built. A change of
    `reported_billing_mode` or `reported_tool_class`, also from or to "not reported", is stored at once
    and audited with action `agent.heartbeat`; the other fields are stored at most once per
    `agent.heartbeat_persist_interval`. The presence becomes `offline` once `agent.presence_timeout`
    passes without a heartbeat. The server compares the reported with the configured mode at each
    heartbeat and returns the warnings of `AgentRuntimeStatus.warnings`; the reported mode is evidence
    for display, warning, audit and alarm and never a permission (BR-05). `interval_ms` is
    `sync.heartbeat_interval`.

    Args:
        body (Heartbeat):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        HeartbeatAck | Problem
    """

    return sync_detailed(
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: Heartbeat,
) -> Response[HeartbeatAck | Problem]:
    """Report state and actual billing mode

     The adapter reports the whole state of its runtime (`agent_heartbeat.v1`, 09 section 4.8); the
    member `agent` of the gateway op `heartbeat` has the same effect. Only an agent may call it (`agent-
    self`); the body carries no agent id. The last heartbeat wins: an absent optional member means "not
    reported", except `project_readiness`, which is applied only when present and changed since the last
    list this server process applied (`ApplyProjectReadiness` of M2-03, in its own transaction).

    The server validates every member and the closed keys of `details` itself, also the patterns of
    names, versions and codes; a violation answers `400 validation_failed` with every field error,
    `rate_limit` answers `errors[].code` `not_enabled` until it is built. A change of
    `reported_billing_mode` or `reported_tool_class`, also from or to "not reported", is stored at once
    and audited with action `agent.heartbeat`; the other fields are stored at most once per
    `agent.heartbeat_persist_interval`. The presence becomes `offline` once `agent.presence_timeout`
    passes without a heartbeat. The server compares the reported with the configured mode at each
    heartbeat and returns the warnings of `AgentRuntimeStatus.warnings`; the reported mode is evidence
    for display, warning, audit and alarm and never a permission (BR-05). `interval_ms` is
    `sync.heartbeat_interval`.

    Args:
        body (Heartbeat):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[HeartbeatAck | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: Heartbeat,
) -> HeartbeatAck | Problem | None:
    """Report state and actual billing mode

     The adapter reports the whole state of its runtime (`agent_heartbeat.v1`, 09 section 4.8); the
    member `agent` of the gateway op `heartbeat` has the same effect. Only an agent may call it (`agent-
    self`); the body carries no agent id. The last heartbeat wins: an absent optional member means "not
    reported", except `project_readiness`, which is applied only when present and changed since the last
    list this server process applied (`ApplyProjectReadiness` of M2-03, in its own transaction).

    The server validates every member and the closed keys of `details` itself, also the patterns of
    names, versions and codes; a violation answers `400 validation_failed` with every field error,
    `rate_limit` answers `errors[].code` `not_enabled` until it is built. A change of
    `reported_billing_mode` or `reported_tool_class`, also from or to "not reported", is stored at once
    and audited with action `agent.heartbeat`; the other fields are stored at most once per
    `agent.heartbeat_persist_interval`. The presence becomes `offline` once `agent.presence_timeout`
    passes without a heartbeat. The server compares the reported with the configured mode at each
    heartbeat and returns the warnings of `AgentRuntimeStatus.warnings`; the reported mode is evidence
    for display, warning, audit and alarm and never a permission (BR-05). `interval_ms` is
    `sync.heartbeat_interval`.

    Args:
        body (Heartbeat):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        HeartbeatAck | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
        )
    ).parsed
