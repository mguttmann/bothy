from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.run_wake import RunWake
from ...models.run_wake_fire import RunWakeFire
from ...types import Response


def _get_kwargs(
    wake_id: UUID,
    *,
    body: RunWakeFire,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/wakes/{wake_id}/fire".format(
            wake_id=quote(str(wake_id), safe=""),
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
) -> Response[Problem | RunWake]:
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
    body: RunWakeFire,
) -> Response[Problem | RunWake]:
    """Fire a checked wake-up, end it failed or return a skipped wake input

     The runner reports the outcome of a `pr_checks` or `command` wake-up (09 5.8.2): `fired` with the
    result text fires it with the checks of a new run, as a wake run when the thread has no active run,
    as a wake input `agent.wake_input` into the running run of the same person, and keeps it `pending`
    while another person's run is active; a refused fire ends it `refused` with note and notification.
    `failed` ends it `failed` with the reason after `runner.wake_check_failures` failures in a row.
    `input_skipped` reports a delivered wake input the runner did not write (`wake_input_check_failed`):
    the wake-up returns to `pending` and fires after that run as a wake run. A wake-up that ended is
    `409 conflict` with `reason` `wake_not_pending`; a fire before `not_before` is `409 conflict` with
    `reason` `wake_not_due`; an `at` wake-up is fired by the server only (`409 conflict`, `reason`
    `wake_fired_by_server`). Audited.

    Args:
        wake_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (RunWakeFire):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | RunWake]
    """

    kwargs = _get_kwargs(
        wake_id=wake_id,
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    wake_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: RunWakeFire,
) -> Problem | RunWake | None:
    """Fire a checked wake-up, end it failed or return a skipped wake input

     The runner reports the outcome of a `pr_checks` or `command` wake-up (09 5.8.2): `fired` with the
    result text fires it with the checks of a new run, as a wake run when the thread has no active run,
    as a wake input `agent.wake_input` into the running run of the same person, and keeps it `pending`
    while another person's run is active; a refused fire ends it `refused` with note and notification.
    `failed` ends it `failed` with the reason after `runner.wake_check_failures` failures in a row.
    `input_skipped` reports a delivered wake input the runner did not write (`wake_input_check_failed`):
    the wake-up returns to `pending` and fires after that run as a wake run. A wake-up that ended is
    `409 conflict` with `reason` `wake_not_pending`; a fire before `not_before` is `409 conflict` with
    `reason` `wake_not_due`; an `at` wake-up is fired by the server only (`409 conflict`, `reason`
    `wake_fired_by_server`). Audited.

    Args:
        wake_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (RunWakeFire):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | RunWake
    """

    return sync_detailed(
        wake_id=wake_id,
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    wake_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: RunWakeFire,
) -> Response[Problem | RunWake]:
    """Fire a checked wake-up, end it failed or return a skipped wake input

     The runner reports the outcome of a `pr_checks` or `command` wake-up (09 5.8.2): `fired` with the
    result text fires it with the checks of a new run, as a wake run when the thread has no active run,
    as a wake input `agent.wake_input` into the running run of the same person, and keeps it `pending`
    while another person's run is active; a refused fire ends it `refused` with note and notification.
    `failed` ends it `failed` with the reason after `runner.wake_check_failures` failures in a row.
    `input_skipped` reports a delivered wake input the runner did not write (`wake_input_check_failed`):
    the wake-up returns to `pending` and fires after that run as a wake run. A wake-up that ended is
    `409 conflict` with `reason` `wake_not_pending`; a fire before `not_before` is `409 conflict` with
    `reason` `wake_not_due`; an `at` wake-up is fired by the server only (`409 conflict`, `reason`
    `wake_fired_by_server`). Audited.

    Args:
        wake_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (RunWakeFire):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | RunWake]
    """

    kwargs = _get_kwargs(
        wake_id=wake_id,
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    wake_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: RunWakeFire,
) -> Problem | RunWake | None:
    """Fire a checked wake-up, end it failed or return a skipped wake input

     The runner reports the outcome of a `pr_checks` or `command` wake-up (09 5.8.2): `fired` with the
    result text fires it with the checks of a new run, as a wake run when the thread has no active run,
    as a wake input `agent.wake_input` into the running run of the same person, and keeps it `pending`
    while another person's run is active; a refused fire ends it `refused` with note and notification.
    `failed` ends it `failed` with the reason after `runner.wake_check_failures` failures in a row.
    `input_skipped` reports a delivered wake input the runner did not write (`wake_input_check_failed`):
    the wake-up returns to `pending` and fires after that run as a wake run. A wake-up that ended is
    `409 conflict` with `reason` `wake_not_pending`; a fire before `not_before` is `409 conflict` with
    `reason` `wake_not_due`; an `at` wake-up is fired by the server only (`409 conflict`, `reason`
    `wake_fired_by_server`). Audited.

    Args:
        wake_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (RunWakeFire):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | RunWake
    """

    return (
        await asyncio_detailed(
            wake_id=wake_id,
            client=client,
            body=body,
        )
    ).parsed
