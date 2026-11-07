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
    run_id: UUID,
) -> dict[str, Any]:

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/runs/{run_id}/launch-check".format(
            run_id=quote(str(run_id), safe=""),
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
    run_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem | RunWakeCheckResult]:
    """Check a launch by a runner that did not accept the run

     Before a runner launches any process of a run it did not accept itself (a repeated try, the next
    attempt of an adopted run, the retry of a compaction, a start over), it asks the server (09 18.19.6,
    R-AG-48): the run is still `running`, its person still has `trigger` on the agent in the channel and
    the agent is still allowed there, the billing rules of 10 hold, and for tool class `host` the
    agent's `tool_class` is still `host` and SR2 holds for that person. On allow the answer carries the
    configured tool class and billing mode, which the runner compares with its own (`compare_actual`). A
    refusal is `409 conflict` with the reason of the failed check (`run_not_running` or the reason of a
    refused `agent.invoke`), audited as a denial; the server changes nothing, the runner finishes the
    run.

    Args:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | RunWakeCheckResult]
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
) -> Problem | RunWakeCheckResult | None:
    """Check a launch by a runner that did not accept the run

     Before a runner launches any process of a run it did not accept itself (a repeated try, the next
    attempt of an adopted run, the retry of a compaction, a start over), it asks the server (09 18.19.6,
    R-AG-48): the run is still `running`, its person still has `trigger` on the agent in the channel and
    the agent is still allowed there, the billing rules of 10 hold, and for tool class `host` the
    agent's `tool_class` is still `host` and SR2 holds for that person. On allow the answer carries the
    configured tool class and billing mode, which the runner compares with its own (`compare_actual`). A
    refusal is `409 conflict` with the reason of the failed check (`run_not_running` or the reason of a
    refused `agent.invoke`), audited as a denial; the server changes nothing, the runner finishes the
    run.

    Args:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | RunWakeCheckResult
    """

    return sync_detailed(
        run_id=run_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    run_id: UUID,
    *,
    client: AuthenticatedClient | Client,
) -> Response[Problem | RunWakeCheckResult]:
    """Check a launch by a runner that did not accept the run

     Before a runner launches any process of a run it did not accept itself (a repeated try, the next
    attempt of an adopted run, the retry of a compaction, a start over), it asks the server (09 18.19.6,
    R-AG-48): the run is still `running`, its person still has `trigger` on the agent in the channel and
    the agent is still allowed there, the billing rules of 10 hold, and for tool class `host` the
    agent's `tool_class` is still `host` and SR2 holds for that person. On allow the answer carries the
    configured tool class and billing mode, which the runner compares with its own (`compare_actual`). A
    refusal is `409 conflict` with the reason of the failed check (`run_not_running` or the reason of a
    refused `agent.invoke`), audited as a denial; the server changes nothing, the runner finishes the
    run.

    Args:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | RunWakeCheckResult]
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
) -> Problem | RunWakeCheckResult | None:
    """Check a launch by a runner that did not accept the run

     Before a runner launches any process of a run it did not accept itself (a repeated try, the next
    attempt of an adopted run, the retry of a compaction, a start over), it asks the server (09 18.19.6,
    R-AG-48): the run is still `running`, its person still has `trigger` on the agent in the channel and
    the agent is still allowed there, the billing rules of 10 hold, and for tool class `host` the
    agent's `tool_class` is still `host` and SR2 holds for that person. On allow the answer carries the
    configured tool class and billing mode, which the runner compares with its own (`compare_actual`). A
    refusal is `409 conflict` with the reason of the failed check (`run_not_running` or the reason of a
    refused `agent.invoke`), audited as a denial; the server changes nothing, the runner finishes the
    run.

    Args:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | RunWakeCheckResult
    """

    return (
        await asyncio_detailed(
            run_id=run_id,
            client=client,
        )
    ).parsed
