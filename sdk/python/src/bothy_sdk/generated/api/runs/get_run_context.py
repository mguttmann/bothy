from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.run_context import RunContext
from ...types import Response


def _get_kwargs(
    run_id: UUID,
) -> dict[str, Any]:

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/runs/{run_id}/context".format(
            run_id=quote(str(run_id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | RunContext:
    if response.status_code == 200:
        response_200 = RunContext.from_dict(response.json())

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
) -> Response[Problem | RunContext]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    run_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem | RunContext]:
    """Load the context of an own active run

     Agent-self, only for own active runs in `queued`, `running` or `waiting_input` (09, 4.6): the
    adapter reads it before `startRun`. An unknown run, a run the agent cannot see and an own run
    that is no longer active answer the same 404; another agent gets 403 `not_self`, a person or
    an app 403 `subject_kind_not_allowed`. The server computes `role` and `trust` of every
    element at the time of the request from the current rights; `trigger_message` is never cut
    and never counted, the other parts are cut per element at `agent.context_message_max_bytes`
    and in sum at `agent.context_max_bytes` from old to new.

    Args:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | RunContext]
    """

    kwargs = _get_kwargs(
        run_id=run_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    run_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Problem | RunContext | None:
    """Load the context of an own active run

     Agent-self, only for own active runs in `queued`, `running` or `waiting_input` (09, 4.6): the
    adapter reads it before `startRun`. An unknown run, a run the agent cannot see and an own run
    that is no longer active answer the same 404; another agent gets 403 `not_self`, a person or
    an app 403 `subject_kind_not_allowed`. The server computes `role` and `trust` of every
    element at the time of the request from the current rights; `trigger_message` is never cut
    and never counted, the other parts are cut per element at `agent.context_message_max_bytes`
    and in sum at `agent.context_max_bytes` from old to new.

    Args:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | RunContext
    """

    return sync_detailed(
        run_id=run_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    run_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem | RunContext]:
    """Load the context of an own active run

     Agent-self, only for own active runs in `queued`, `running` or `waiting_input` (09, 4.6): the
    adapter reads it before `startRun`. An unknown run, a run the agent cannot see and an own run
    that is no longer active answer the same 404; another agent gets 403 `not_self`, a person or
    an app 403 `subject_kind_not_allowed`. The server computes `role` and `trust` of every
    element at the time of the request from the current rights; `trigger_message` is never cut
    and never counted, the other parts are cut per element at `agent.context_message_max_bytes`
    and in sum at `agent.context_max_bytes` from old to new.

    Args:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | RunContext]
    """

    kwargs = _get_kwargs(
        run_id=run_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    run_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Problem | RunContext | None:
    """Load the context of an own active run

     Agent-self, only for own active runs in `queued`, `running` or `waiting_input` (09, 4.6): the
    adapter reads it before `startRun`. An unknown run, a run the agent cannot see and an own run
    that is no longer active answer the same 404; another agent gets 403 `not_self`, a person or
    an app 403 `subject_kind_not_allowed`. The server computes `role` and `trust` of every
    element at the time of the request from the current rights; `trigger_message` is never cut
    and never counted, the other parts are cut per element at `agent.context_message_max_bytes`
    and in sum at `agent.context_max_bytes` from old to new.

    Args:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | RunContext
    """

    return (
        await asyncio_detailed(
            run_id=run_id,
            client=client,
        )
    ).parsed
