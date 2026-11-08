from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.run_wake import RunWake
from ...models.run_wake_recover import RunWakeRecover
from ...types import UNSET, Response, Unset


def _get_kwargs(
    run_id: UUID,
    *,
    body: RunWakeRecover,
    idempotency_key: UUID | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(idempotency_key, Unset):
        headers["Idempotency-Key"] = idempotency_key

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/runs/{run_id}/wakes/recover".format(
            run_id=quote(str(run_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | RunWake:
    if response.status_code == 200:
        response_200 = RunWake.from_dict(response.json())

        return response_200

    if response.status_code == 201:
        response_201 = RunWake.from_dict(response.json())

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
) -> Response[Problem | RunWake]:
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
    body: RunWakeRecover,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[Problem | RunWake]:
    """Create the recovery or usage-limit wake-up of an ended run

     Creates a wake-up of kind `at` with reason `recovery` (lost background work, 09 18.19.9) or
    `usage_limit` (the usage window ended the run, 18.19.10) for an ended run of the calling agent in
    state `failed` (which `interrupted` and `stale` runs are), at most one per run and reason: a
    repetition answers the first one with `200`; a `usage_limit` request while a `usage_limit` wake-up
    of an outage of the run is pending answers that wake-up with `200` and changes nothing. A run that
    has not ended is `409 conflict` with `reason` `run_not_ended` and stores nothing; a run that ended
    `cancelled` or `completed` stores the wake-up in state `refused` with the note in the thread and the
    notification `wake_ended` and answers `409 conflict` with `reason` `run_not_failed`. A refusal of
    the checks of 09 5.8.6 (`autonomous_not_allowed`, `wake_max_pending`, `usage_wake_max`, a refused
    `agent.invoke`) stores the wake-up in state `refused` with the note in the thread and the
    notification `wake_ended`, and answers `409 conflict` with the reason. Audited.

    Args:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (RunWakeRecover):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | RunWake]
    """

    kwargs = _get_kwargs(
        run_id=run_id,
        body=body,
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
    body: RunWakeRecover,
    idempotency_key: UUID | Unset = UNSET,
) -> Problem | RunWake | None:
    """Create the recovery or usage-limit wake-up of an ended run

     Creates a wake-up of kind `at` with reason `recovery` (lost background work, 09 18.19.9) or
    `usage_limit` (the usage window ended the run, 18.19.10) for an ended run of the calling agent in
    state `failed` (which `interrupted` and `stale` runs are), at most one per run and reason: a
    repetition answers the first one with `200`; a `usage_limit` request while a `usage_limit` wake-up
    of an outage of the run is pending answers that wake-up with `200` and changes nothing. A run that
    has not ended is `409 conflict` with `reason` `run_not_ended` and stores nothing; a run that ended
    `cancelled` or `completed` stores the wake-up in state `refused` with the note in the thread and the
    notification `wake_ended` and answers `409 conflict` with `reason` `run_not_failed`. A refusal of
    the checks of 09 5.8.6 (`autonomous_not_allowed`, `wake_max_pending`, `usage_wake_max`, a refused
    `agent.invoke`) stores the wake-up in state `refused` with the note in the thread and the
    notification `wake_ended`, and answers `409 conflict` with the reason. Audited.

    Args:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (RunWakeRecover):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | RunWake
    """

    return sync_detailed(
        run_id=run_id,
        client=client,
        body=body,
        idempotency_key=idempotency_key,
    ).parsed


async def asyncio_detailed(
    run_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: RunWakeRecover,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[Problem | RunWake]:
    """Create the recovery or usage-limit wake-up of an ended run

     Creates a wake-up of kind `at` with reason `recovery` (lost background work, 09 18.19.9) or
    `usage_limit` (the usage window ended the run, 18.19.10) for an ended run of the calling agent in
    state `failed` (which `interrupted` and `stale` runs are), at most one per run and reason: a
    repetition answers the first one with `200`; a `usage_limit` request while a `usage_limit` wake-up
    of an outage of the run is pending answers that wake-up with `200` and changes nothing. A run that
    has not ended is `409 conflict` with `reason` `run_not_ended` and stores nothing; a run that ended
    `cancelled` or `completed` stores the wake-up in state `refused` with the note in the thread and the
    notification `wake_ended` and answers `409 conflict` with `reason` `run_not_failed`. A refusal of
    the checks of 09 5.8.6 (`autonomous_not_allowed`, `wake_max_pending`, `usage_wake_max`, a refused
    `agent.invoke`) stores the wake-up in state `refused` with the note in the thread and the
    notification `wake_ended`, and answers `409 conflict` with the reason. Audited.

    Args:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (RunWakeRecover):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | RunWake]
    """

    kwargs = _get_kwargs(
        run_id=run_id,
        body=body,
        idempotency_key=idempotency_key,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    run_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: RunWakeRecover,
    idempotency_key: UUID | Unset = UNSET,
) -> Problem | RunWake | None:
    """Create the recovery or usage-limit wake-up of an ended run

     Creates a wake-up of kind `at` with reason `recovery` (lost background work, 09 18.19.9) or
    `usage_limit` (the usage window ended the run, 18.19.10) for an ended run of the calling agent in
    state `failed` (which `interrupted` and `stale` runs are), at most one per run and reason: a
    repetition answers the first one with `200`; a `usage_limit` request while a `usage_limit` wake-up
    of an outage of the run is pending answers that wake-up with `200` and changes nothing. A run that
    has not ended is `409 conflict` with `reason` `run_not_ended` and stores nothing; a run that ended
    `cancelled` or `completed` stores the wake-up in state `refused` with the note in the thread and the
    notification `wake_ended` and answers `409 conflict` with `reason` `run_not_failed`. A refusal of
    the checks of 09 5.8.6 (`autonomous_not_allowed`, `wake_max_pending`, `usage_wake_max`, a refused
    `agent.invoke`) stores the wake-up in state `refused` with the note in the thread and the
    notification `wake_ended`, and answers `409 conflict` with the reason. Audited.

    Args:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (RunWakeRecover):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | RunWake
    """

    return (
        await asyncio_detailed(
            run_id=run_id,
            client=client,
            body=body,
            idempotency_key=idempotency_key,
        )
    ).parsed
