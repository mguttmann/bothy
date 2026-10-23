from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.autocomplete_query import AutocompleteQuery
from ...models.autocomplete_result import AutocompleteResult
from ...models.problem import Problem
from ...types import Response


def _get_kwargs(
    channel_id: UUID,
    command_id: UUID,
    *,
    body: AutocompleteQuery,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/channels/{channel_id}/commands/{command_id}/autocomplete".format(
            channel_id=quote(str(channel_id), safe=""),
            command_id=quote(str(command_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> AutocompleteResult | Problem:
    if response.status_code == 200:
        response_200 = AutocompleteResult.from_dict(response.json())

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

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[AutocompleteResult | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    channel_id: UUID,
    command_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: AutocompleteQuery,
) -> Response[AutocompleteResult | Problem]:
    """Get suggestions for an option

     Forwards the query to the command owner and waits up to `command.autocomplete_timeout`; returns
    no choices when the owner does not answer in time. Returns an empty list at once when an agent
    owner, or an app owner without `delivery.interactions_url`, has no live gateway connection, when
    the installation of the owning app stops being active after the check, and for an agent command in a
    forum channel without
    `thread_id` after the check (R-AZ-26). An app whose manifest declares `delivery.interactions_url`
    gets the query as a signed `POST` to that address and has `app.interaction_timeout` to answer,
    never beyond `command.autocomplete_timeout`: `200` with valid choices returns them, `202` keeps
    the request open for `answerAutocomplete`, any other answer returns an empty list at once (11, 9).
    A command of an agent is checked like `agent.invoke` without a run: a refusal is
    `403 agent_invocation_denied` with `reason` (for example `level_insufficient`,
    `agent_not_reachable`, `billing_mode_personal_not_holder`, `tool_class_owner_only`,
    `project_not_ready`), a read-only or archived channel `403 channel_read_only`. A command of an app
    is checked by its `required_level` and the reachability of the app (active installation,
    `commands:write`, `write` on the channel), never by the `agent.invoke` guard; a missing level is
    `403 agent_invocation_denied` with `reason`, a read-only or archived channel
    `403 channel_read_only`. An unknown, removed or not covered command, an app command whose app is
    not reachable, an unknown channel or a thread of another channel is `404`.
    The query names an option with `autocomplete` (`400 validation_failed` with `unknown_option`,
    `autocomplete_not_enabled` or `duplicate`; `too_long` when the request does not fit
    `sync.event_payload_max_bytes`).

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        command_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (AutocompleteQuery):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AutocompleteResult | Problem]
    """

    kwargs = _get_kwargs(
        channel_id=channel_id,
        command_id=command_id,
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    channel_id: UUID,
    command_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: AutocompleteQuery,
) -> AutocompleteResult | Problem | None:
    """Get suggestions for an option

     Forwards the query to the command owner and waits up to `command.autocomplete_timeout`; returns
    no choices when the owner does not answer in time. Returns an empty list at once when an agent
    owner, or an app owner without `delivery.interactions_url`, has no live gateway connection, when
    the installation of the owning app stops being active after the check, and for an agent command in a
    forum channel without
    `thread_id` after the check (R-AZ-26). An app whose manifest declares `delivery.interactions_url`
    gets the query as a signed `POST` to that address and has `app.interaction_timeout` to answer,
    never beyond `command.autocomplete_timeout`: `200` with valid choices returns them, `202` keeps
    the request open for `answerAutocomplete`, any other answer returns an empty list at once (11, 9).
    A command of an agent is checked like `agent.invoke` without a run: a refusal is
    `403 agent_invocation_denied` with `reason` (for example `level_insufficient`,
    `agent_not_reachable`, `billing_mode_personal_not_holder`, `tool_class_owner_only`,
    `project_not_ready`), a read-only or archived channel `403 channel_read_only`. A command of an app
    is checked by its `required_level` and the reachability of the app (active installation,
    `commands:write`, `write` on the channel), never by the `agent.invoke` guard; a missing level is
    `403 agent_invocation_denied` with `reason`, a read-only or archived channel
    `403 channel_read_only`. An unknown, removed or not covered command, an app command whose app is
    not reachable, an unknown channel or a thread of another channel is `404`.
    The query names an option with `autocomplete` (`400 validation_failed` with `unknown_option`,
    `autocomplete_not_enabled` or `duplicate`; `too_long` when the request does not fit
    `sync.event_payload_max_bytes`).

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        command_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (AutocompleteQuery):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AutocompleteResult | Problem
    """

    return sync_detailed(
        channel_id=channel_id,
        command_id=command_id,
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    channel_id: UUID,
    command_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: AutocompleteQuery,
) -> Response[AutocompleteResult | Problem]:
    """Get suggestions for an option

     Forwards the query to the command owner and waits up to `command.autocomplete_timeout`; returns
    no choices when the owner does not answer in time. Returns an empty list at once when an agent
    owner, or an app owner without `delivery.interactions_url`, has no live gateway connection, when
    the installation of the owning app stops being active after the check, and for an agent command in a
    forum channel without
    `thread_id` after the check (R-AZ-26). An app whose manifest declares `delivery.interactions_url`
    gets the query as a signed `POST` to that address and has `app.interaction_timeout` to answer,
    never beyond `command.autocomplete_timeout`: `200` with valid choices returns them, `202` keeps
    the request open for `answerAutocomplete`, any other answer returns an empty list at once (11, 9).
    A command of an agent is checked like `agent.invoke` without a run: a refusal is
    `403 agent_invocation_denied` with `reason` (for example `level_insufficient`,
    `agent_not_reachable`, `billing_mode_personal_not_holder`, `tool_class_owner_only`,
    `project_not_ready`), a read-only or archived channel `403 channel_read_only`. A command of an app
    is checked by its `required_level` and the reachability of the app (active installation,
    `commands:write`, `write` on the channel), never by the `agent.invoke` guard; a missing level is
    `403 agent_invocation_denied` with `reason`, a read-only or archived channel
    `403 channel_read_only`. An unknown, removed or not covered command, an app command whose app is
    not reachable, an unknown channel or a thread of another channel is `404`.
    The query names an option with `autocomplete` (`400 validation_failed` with `unknown_option`,
    `autocomplete_not_enabled` or `duplicate`; `too_long` when the request does not fit
    `sync.event_payload_max_bytes`).

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        command_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (AutocompleteQuery):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AutocompleteResult | Problem]
    """

    kwargs = _get_kwargs(
        channel_id=channel_id,
        command_id=command_id,
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    channel_id: UUID,
    command_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: AutocompleteQuery,
) -> AutocompleteResult | Problem | None:
    """Get suggestions for an option

     Forwards the query to the command owner and waits up to `command.autocomplete_timeout`; returns
    no choices when the owner does not answer in time. Returns an empty list at once when an agent
    owner, or an app owner without `delivery.interactions_url`, has no live gateway connection, when
    the installation of the owning app stops being active after the check, and for an agent command in a
    forum channel without
    `thread_id` after the check (R-AZ-26). An app whose manifest declares `delivery.interactions_url`
    gets the query as a signed `POST` to that address and has `app.interaction_timeout` to answer,
    never beyond `command.autocomplete_timeout`: `200` with valid choices returns them, `202` keeps
    the request open for `answerAutocomplete`, any other answer returns an empty list at once (11, 9).
    A command of an agent is checked like `agent.invoke` without a run: a refusal is
    `403 agent_invocation_denied` with `reason` (for example `level_insufficient`,
    `agent_not_reachable`, `billing_mode_personal_not_holder`, `tool_class_owner_only`,
    `project_not_ready`), a read-only or archived channel `403 channel_read_only`. A command of an app
    is checked by its `required_level` and the reachability of the app (active installation,
    `commands:write`, `write` on the channel), never by the `agent.invoke` guard; a missing level is
    `403 agent_invocation_denied` with `reason`, a read-only or archived channel
    `403 channel_read_only`. An unknown, removed or not covered command, an app command whose app is
    not reachable, an unknown channel or a thread of another channel is `404`.
    The query names an option with `autocomplete` (`400 validation_failed` with `unknown_option`,
    `autocomplete_not_enabled` or `duplicate`; `too_long` when the request does not fit
    `sync.event_payload_max_bytes`).

    Args:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        command_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (AutocompleteQuery):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AutocompleteResult | Problem
    """

    return (
        await asyncio_detailed(
            channel_id=channel_id,
            command_id=command_id,
            client=client,
            body=body,
        )
    ).parsed
