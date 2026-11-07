from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.channel import Channel
from ...models.channel_autonomous_runs_set import ChannelAutonomousRunsSet
from ...models.problem import Problem
from ...types import Response


def _get_kwargs(
    channel_id: UUID,
    *,
    body: ChannelAutonomousRunsSet,
    if_match: str,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    headers["If-Match"] = if_match

    _kwargs: dict[str, Any] = {
        "method": "put",
        "url": "/channels/{channel_id}/autonomous-runs".format(
            channel_id=quote(str(channel_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Channel | Problem:
    if response.status_code == 200:
        response_200 = Channel.from_dict(response.json())

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
) -> Response[Channel | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    channel_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: ChannelAutonomousRunsSet,
    if_match: str,
) -> Response[Channel | Problem]:
    """Switch autonomous runs of a channel

     Switches `allow_autonomous_runs` (09 5.8.6, 07 2.14 item 1): persons other than owners may let
    agents run on alone in this channel (wake-ups, recovery and usage-window wake-ups); owners may
    always. Off by default. Every create and every fire reads the switch at its moment, so switching it
    off refuses the next fire of a pending wake-up of a person who is no owner. An archived channel is
    `409 conflict` with `reason` `channel_archived`, a DM `409 conflict` with `reason` `dm_channel`.
    Audited; the same value answers without a change.

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str):
        body (ChannelAutonomousRunsSet):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Channel | Problem]
    """

    kwargs = _get_kwargs(
        channel_id=channel_id,
        body=body,
        if_match=if_match,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    channel_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: ChannelAutonomousRunsSet,
    if_match: str,
) -> Channel | Problem | None:
    """Switch autonomous runs of a channel

     Switches `allow_autonomous_runs` (09 5.8.6, 07 2.14 item 1): persons other than owners may let
    agents run on alone in this channel (wake-ups, recovery and usage-window wake-ups); owners may
    always. Off by default. Every create and every fire reads the switch at its moment, so switching it
    off refuses the next fire of a pending wake-up of a person who is no owner. An archived channel is
    `409 conflict` with `reason` `channel_archived`, a DM `409 conflict` with `reason` `dm_channel`.
    Audited; the same value answers without a change.

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str):
        body (ChannelAutonomousRunsSet):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Channel | Problem
    """

    return sync_detailed(
        channel_id=channel_id,
        client=client,
        body=body,
        if_match=if_match,
    ).parsed


async def asyncio_detailed(
    channel_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: ChannelAutonomousRunsSet,
    if_match: str,
) -> Response[Channel | Problem]:
    """Switch autonomous runs of a channel

     Switches `allow_autonomous_runs` (09 5.8.6, 07 2.14 item 1): persons other than owners may let
    agents run on alone in this channel (wake-ups, recovery and usage-window wake-ups); owners may
    always. Off by default. Every create and every fire reads the switch at its moment, so switching it
    off refuses the next fire of a pending wake-up of a person who is no owner. An archived channel is
    `409 conflict` with `reason` `channel_archived`, a DM `409 conflict` with `reason` `dm_channel`.
    Audited; the same value answers without a change.

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str):
        body (ChannelAutonomousRunsSet):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Channel | Problem]
    """

    kwargs = _get_kwargs(
        channel_id=channel_id,
        body=body,
        if_match=if_match,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    channel_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: ChannelAutonomousRunsSet,
    if_match: str,
) -> Channel | Problem | None:
    """Switch autonomous runs of a channel

     Switches `allow_autonomous_runs` (09 5.8.6, 07 2.14 item 1): persons other than owners may let
    agents run on alone in this channel (wake-ups, recovery and usage-window wake-ups); owners may
    always. Off by default. Every create and every fire reads the switch at its moment, so switching it
    off refuses the next fire of a pending wake-up of a person who is no owner. An archived channel is
    `409 conflict` with `reason` `channel_archived`, a DM `409 conflict` with `reason` `dm_channel`.
    Audited; the same value answers without a change.

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str):
        body (ChannelAutonomousRunsSet):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Channel | Problem
    """

    return (
        await asyncio_detailed(
            channel_id=channel_id,
            client=client,
            body=body,
            if_match=if_match,
        )
    ).parsed
