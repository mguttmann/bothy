from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.run_wake import RunWake
from ...models.run_wake_create import RunWakeCreate
from ...types import UNSET, Response, Unset


def _get_kwargs(
    run_id: UUID,
    *,
    body: RunWakeCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(idempotency_key, Unset):
        headers["Idempotency-Key"] = idempotency_key

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/runs/{run_id}/wakes".format(
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
    body: RunWakeCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[Problem | RunWake]:
    """Create a wake-up for an active run

     Creates a wake-up of the run's thread for the active run of the calling agent (09 5.8.2, stage AR):
    kind `at`, `pr_checks` or `command`, reason `agent` (from `bothy-wake`) or `usage_limit`. The wake-
    up acts for the run's `triggered_by`; it fires no earlier than `run.wake_min_delay` after its
    creation and no later than its `deadline` (at most `run.wake_max_wait`, for `usage_limit` counted
    from `not_before`). A `usage_limit` request carries the run's `outage`; a second request with the
    outage of a pending wake-up moves its `not_before` and answers it with `200`, a new outage gets its
    own wake-up, at most `run.usage_wake_max` per run. A refusal is `409 conflict` with the reason:
    `run_not_active`, `autonomous_not_allowed` (the person is no owner and the channel has no
    `allow_autonomous_runs`), `pr_checks_owner_only`, `tool_class_mismatch` (a `command` of an agent
    whose tool class is not `host`), `wake_max_pending`, `usage_wake_max`, or the reason of a refused
    `agent.invoke` of the person (for example `tool_class_owner_only`, `agent_not_reachable`,
    `channel_read_only`). A refused request stores nothing, except one with `deferred: true` and a
    refusal with `usage_wake_max`, which are stored in state `refused` with the note in the thread and
    the notification `wake_ended` (09 5.8.4). A wrong field is `400 validation_failed`. Audited.

    Args:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (RunWakeCreate):

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
    body: RunWakeCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> Problem | RunWake | None:
    """Create a wake-up for an active run

     Creates a wake-up of the run's thread for the active run of the calling agent (09 5.8.2, stage AR):
    kind `at`, `pr_checks` or `command`, reason `agent` (from `bothy-wake`) or `usage_limit`. The wake-
    up acts for the run's `triggered_by`; it fires no earlier than `run.wake_min_delay` after its
    creation and no later than its `deadline` (at most `run.wake_max_wait`, for `usage_limit` counted
    from `not_before`). A `usage_limit` request carries the run's `outage`; a second request with the
    outage of a pending wake-up moves its `not_before` and answers it with `200`, a new outage gets its
    own wake-up, at most `run.usage_wake_max` per run. A refusal is `409 conflict` with the reason:
    `run_not_active`, `autonomous_not_allowed` (the person is no owner and the channel has no
    `allow_autonomous_runs`), `pr_checks_owner_only`, `tool_class_mismatch` (a `command` of an agent
    whose tool class is not `host`), `wake_max_pending`, `usage_wake_max`, or the reason of a refused
    `agent.invoke` of the person (for example `tool_class_owner_only`, `agent_not_reachable`,
    `channel_read_only`). A refused request stores nothing, except one with `deferred: true` and a
    refusal with `usage_wake_max`, which are stored in state `refused` with the note in the thread and
    the notification `wake_ended` (09 5.8.4). A wrong field is `400 validation_failed`. Audited.

    Args:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (RunWakeCreate):

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
    body: RunWakeCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[Problem | RunWake]:
    """Create a wake-up for an active run

     Creates a wake-up of the run's thread for the active run of the calling agent (09 5.8.2, stage AR):
    kind `at`, `pr_checks` or `command`, reason `agent` (from `bothy-wake`) or `usage_limit`. The wake-
    up acts for the run's `triggered_by`; it fires no earlier than `run.wake_min_delay` after its
    creation and no later than its `deadline` (at most `run.wake_max_wait`, for `usage_limit` counted
    from `not_before`). A `usage_limit` request carries the run's `outage`; a second request with the
    outage of a pending wake-up moves its `not_before` and answers it with `200`, a new outage gets its
    own wake-up, at most `run.usage_wake_max` per run. A refusal is `409 conflict` with the reason:
    `run_not_active`, `autonomous_not_allowed` (the person is no owner and the channel has no
    `allow_autonomous_runs`), `pr_checks_owner_only`, `tool_class_mismatch` (a `command` of an agent
    whose tool class is not `host`), `wake_max_pending`, `usage_wake_max`, or the reason of a refused
    `agent.invoke` of the person (for example `tool_class_owner_only`, `agent_not_reachable`,
    `channel_read_only`). A refused request stores nothing, except one with `deferred: true` and a
    refusal with `usage_wake_max`, which are stored in state `refused` with the note in the thread and
    the notification `wake_ended` (09 5.8.4). A wrong field is `400 validation_failed`. Audited.

    Args:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (RunWakeCreate):

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
    body: RunWakeCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> Problem | RunWake | None:
    """Create a wake-up for an active run

     Creates a wake-up of the run's thread for the active run of the calling agent (09 5.8.2, stage AR):
    kind `at`, `pr_checks` or `command`, reason `agent` (from `bothy-wake`) or `usage_limit`. The wake-
    up acts for the run's `triggered_by`; it fires no earlier than `run.wake_min_delay` after its
    creation and no later than its `deadline` (at most `run.wake_max_wait`, for `usage_limit` counted
    from `not_before`). A `usage_limit` request carries the run's `outage`; a second request with the
    outage of a pending wake-up moves its `not_before` and answers it with `200`, a new outage gets its
    own wake-up, at most `run.usage_wake_max` per run. A refusal is `409 conflict` with the reason:
    `run_not_active`, `autonomous_not_allowed` (the person is no owner and the channel has no
    `allow_autonomous_runs`), `pr_checks_owner_only`, `tool_class_mismatch` (a `command` of an agent
    whose tool class is not `host`), `wake_max_pending`, `usage_wake_max`, or the reason of a refused
    `agent.invoke` of the person (for example `tool_class_owner_only`, `agent_not_reachable`,
    `channel_read_only`). A refused request stores nothing, except one with `deferred: true` and a
    refusal with `usage_wake_max`, which are stored in state `refused` with the note in the thread and
    the notification `wake_ended` (09 5.8.4). A wrong field is `400 validation_failed`. Audited.

    Args:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (RunWakeCreate):

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
