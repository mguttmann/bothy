from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.run_wake_check_result import RunWakeCheckResult
from ...types import Response


def _get_kwargs(
    wake_id: UUID,
) -> dict[str, Any]:

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/wakes/{wake_id}/check".format(
            wake_id=quote(str(wake_id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | RunWakeCheckResult:
    if response.status_code == 200:
        response_200 = RunWakeCheckResult.from_dict(response.json())

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
) -> Response[Problem | RunWakeCheckResult]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    wake_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem | RunWakeCheckResult]:
    """Check the next execution of a pending check

     Before every execution of a pending `command` and every `gh` read of a pending `pr_checks` the
    runner asks the server (09 5.8.3, R-AG-50): the wake-up is still `pending` and its `not_before` has
    passed; its person still has `trigger` on the agent in the channel and the agent is still allowed
    there; the channel is neither archived nor read-only; for `command` the agent's `tool_class` is
    still `host` and SR2 holds for that person; for `pr_checks` the person is still an owner. On allow
    the answer carries the configured tool class and billing mode (`compare_actual`). A failed check
    ends the wake-up `refused` with the reason of the check (for example `agent_not_reachable`,
    `channel_read_only`, `tool_class_owner_only`, `tool_class_mismatch`, `pr_checks_owner_only`), an
    audit entry, the note in the thread and the notification `wake_ended`, and answers `409 conflict`
    with that reason. A wake-up that already ended is `409 conflict` with `reason` `wake_not_pending`,
    one before its `not_before` `409 conflict` with `reason` `wake_not_due` (the wake-up stays pending).

    Args:
        wake_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | RunWakeCheckResult]
    """

    kwargs = _get_kwargs(
        wake_id=wake_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    wake_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Problem | RunWakeCheckResult | None:
    """Check the next execution of a pending check

     Before every execution of a pending `command` and every `gh` read of a pending `pr_checks` the
    runner asks the server (09 5.8.3, R-AG-50): the wake-up is still `pending` and its `not_before` has
    passed; its person still has `trigger` on the agent in the channel and the agent is still allowed
    there; the channel is neither archived nor read-only; for `command` the agent's `tool_class` is
    still `host` and SR2 holds for that person; for `pr_checks` the person is still an owner. On allow
    the answer carries the configured tool class and billing mode (`compare_actual`). A failed check
    ends the wake-up `refused` with the reason of the check (for example `agent_not_reachable`,
    `channel_read_only`, `tool_class_owner_only`, `tool_class_mismatch`, `pr_checks_owner_only`), an
    audit entry, the note in the thread and the notification `wake_ended`, and answers `409 conflict`
    with that reason. A wake-up that already ended is `409 conflict` with `reason` `wake_not_pending`,
    one before its `not_before` `409 conflict` with `reason` `wake_not_due` (the wake-up stays pending).

    Args:
        wake_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | RunWakeCheckResult
    """

    return sync_detailed(
        wake_id=wake_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    wake_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem | RunWakeCheckResult]:
    """Check the next execution of a pending check

     Before every execution of a pending `command` and every `gh` read of a pending `pr_checks` the
    runner asks the server (09 5.8.3, R-AG-50): the wake-up is still `pending` and its `not_before` has
    passed; its person still has `trigger` on the agent in the channel and the agent is still allowed
    there; the channel is neither archived nor read-only; for `command` the agent's `tool_class` is
    still `host` and SR2 holds for that person; for `pr_checks` the person is still an owner. On allow
    the answer carries the configured tool class and billing mode (`compare_actual`). A failed check
    ends the wake-up `refused` with the reason of the check (for example `agent_not_reachable`,
    `channel_read_only`, `tool_class_owner_only`, `tool_class_mismatch`, `pr_checks_owner_only`), an
    audit entry, the note in the thread and the notification `wake_ended`, and answers `409 conflict`
    with that reason. A wake-up that already ended is `409 conflict` with `reason` `wake_not_pending`,
    one before its `not_before` `409 conflict` with `reason` `wake_not_due` (the wake-up stays pending).

    Args:
        wake_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | RunWakeCheckResult]
    """

    kwargs = _get_kwargs(
        wake_id=wake_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    wake_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Problem | RunWakeCheckResult | None:
    """Check the next execution of a pending check

     Before every execution of a pending `command` and every `gh` read of a pending `pr_checks` the
    runner asks the server (09 5.8.3, R-AG-50): the wake-up is still `pending` and its `not_before` has
    passed; its person still has `trigger` on the agent in the channel and the agent is still allowed
    there; the channel is neither archived nor read-only; for `command` the agent's `tool_class` is
    still `host` and SR2 holds for that person; for `pr_checks` the person is still an owner. On allow
    the answer carries the configured tool class and billing mode (`compare_actual`). A failed check
    ends the wake-up `refused` with the reason of the check (for example `agent_not_reachable`,
    `channel_read_only`, `tool_class_owner_only`, `tool_class_mismatch`, `pr_checks_owner_only`), an
    audit entry, the note in the thread and the notification `wake_ended`, and answers `409 conflict`
    with that reason. A wake-up that already ended is `409 conflict` with `reason` `wake_not_pending`,
    one before its `not_before` `409 conflict` with `reason` `wake_not_due` (the wake-up stays pending).

    Args:
        wake_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | RunWakeCheckResult
    """

    return (
        await asyncio_detailed(
            wake_id=wake_id,
            client=client,
        )
    ).parsed
