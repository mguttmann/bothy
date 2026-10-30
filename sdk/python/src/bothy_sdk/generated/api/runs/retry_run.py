from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.run_summary import RunSummary
from ...types import UNSET, Response, Unset


def _get_kwargs(
    run_id: UUID,
    *,
    idempotency_key: UUID | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(idempotency_key, Unset):
        headers["Idempotency-Key"] = idempotency_key

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/runs/{run_id}/retry".format(
            run_id=quote(str(run_id), safe=""),
        ),
    }

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | RunSummary:
    if response.status_code == 202:
        response_202 = RunSummary.from_dict(response.json())

        return response_202

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
) -> Response[Problem | RunSummary]:
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
    idempotency_key: UUID | Unset = UNSET,
) -> Response[Problem | RunSummary]:
    """Retry a failed or cancelled run

     Starts a new run of the same agent for the triggering message of a `failed` or `cancelled`
    run, so the person does not have to write the message again (09 5.7, addendum of
    2026-10-30). Only the newest run of its thread can be retried, and only a run with
    `trigger_kind = message`; otherwise `409 invalid_state_transition`. The rights are those of
    triggering: `agent.invoke` for the caller on the run's thread with the run's agent, the
    mention of the triggering message and the rules of 05 section 5; a denial answers
    `403 agent_invocation_denied` with `reason`, also `operator_not_in_channel` when the agent
    is the thread's operator and no longer allowed in the channel (09 3.4 step 4a). The new run
    has `triggered_by` = the caller and the triggering message's `trigger_state` follows it.
    Idempotent: when the newest run of the thread is already a retry of the same message, the
    answer is that run and nothing is written.

    Args:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | RunSummary]
    """

    kwargs = _get_kwargs(
        run_id=run_id,
        idempotency_key=idempotency_key,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    run_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    idempotency_key: UUID | Unset = UNSET,
) -> Problem | RunSummary | None:
    """Retry a failed or cancelled run

     Starts a new run of the same agent for the triggering message of a `failed` or `cancelled`
    run, so the person does not have to write the message again (09 5.7, addendum of
    2026-10-30). Only the newest run of its thread can be retried, and only a run with
    `trigger_kind = message`; otherwise `409 invalid_state_transition`. The rights are those of
    triggering: `agent.invoke` for the caller on the run's thread with the run's agent, the
    mention of the triggering message and the rules of 05 section 5; a denial answers
    `403 agent_invocation_denied` with `reason`, also `operator_not_in_channel` when the agent
    is the thread's operator and no longer allowed in the channel (09 3.4 step 4a). The new run
    has `triggered_by` = the caller and the triggering message's `trigger_state` follows it.
    Idempotent: when the newest run of the thread is already a retry of the same message, the
    answer is that run and nothing is written.

    Args:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | RunSummary
    """

    return sync_detailed(
        run_id=run_id,
        client=client,
        idempotency_key=idempotency_key,
    ).parsed


async def asyncio_detailed(
    run_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[Problem | RunSummary]:
    """Retry a failed or cancelled run

     Starts a new run of the same agent for the triggering message of a `failed` or `cancelled`
    run, so the person does not have to write the message again (09 5.7, addendum of
    2026-10-30). Only the newest run of its thread can be retried, and only a run with
    `trigger_kind = message`; otherwise `409 invalid_state_transition`. The rights are those of
    triggering: `agent.invoke` for the caller on the run's thread with the run's agent, the
    mention of the triggering message and the rules of 05 section 5; a denial answers
    `403 agent_invocation_denied` with `reason`, also `operator_not_in_channel` when the agent
    is the thread's operator and no longer allowed in the channel (09 3.4 step 4a). The new run
    has `triggered_by` = the caller and the triggering message's `trigger_state` follows it.
    Idempotent: when the newest run of the thread is already a retry of the same message, the
    answer is that run and nothing is written.

    Args:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | RunSummary]
    """

    kwargs = _get_kwargs(
        run_id=run_id,
        idempotency_key=idempotency_key,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    run_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    idempotency_key: UUID | Unset = UNSET,
) -> Problem | RunSummary | None:
    """Retry a failed or cancelled run

     Starts a new run of the same agent for the triggering message of a `failed` or `cancelled`
    run, so the person does not have to write the message again (09 5.7, addendum of
    2026-10-30). Only the newest run of its thread can be retried, and only a run with
    `trigger_kind = message`; otherwise `409 invalid_state_transition`. The rights are those of
    triggering: `agent.invoke` for the caller on the run's thread with the run's agent, the
    mention of the triggering message and the rules of 05 section 5; a denial answers
    `403 agent_invocation_denied` with `reason`, also `operator_not_in_channel` when the agent
    is the thread's operator and no longer allowed in the channel (09 3.4 step 4a). The new run
    has `triggered_by` = the caller and the triggering message's `trigger_state` follows it.
    Idempotent: when the newest run of the thread is already a retry of the same message, the
    answer is that run and nothing is written.

    Args:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | RunSummary
    """

    return (
        await asyncio_detailed(
            run_id=run_id,
            client=client,
            idempotency_key=idempotency_key,
        )
    ).parsed
