from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.quiet_hours_rule import QuietHoursRule
from ...models.quiet_hours_rule_input import QuietHoursRuleInput
from ...types import Response


def _get_kwargs(
    rule_id: UUID,
    *,
    body: QuietHoursRuleInput,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "put",
        "url": "/me/quiet-hours/{rule_id}".format(
            rule_id=quote(str(rule_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | QuietHoursRule:
    if response.status_code == 200:
        response_200 = QuietHoursRule.from_dict(response.json())

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
) -> Response[Problem | QuietHoursRule]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    rule_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: QuietHoursRuleInput,
) -> Response[Problem | QuietHoursRule]:
    """Change a quiet hours rule

     Replaces the whole rule; an absent time_zone clears it (the rule then follows the person's time
    zone). Equal values answer 200 without event and without a new version. time_zone and the list size
    are checked as for createQuietHoursRule (unknown_time_zone, limit_reached).

    Args:
        rule_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (QuietHoursRuleInput):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | QuietHoursRule]
    """

    kwargs = _get_kwargs(
        rule_id=rule_id,
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    rule_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: QuietHoursRuleInput,
) -> Problem | QuietHoursRule | None:
    """Change a quiet hours rule

     Replaces the whole rule; an absent time_zone clears it (the rule then follows the person's time
    zone). Equal values answer 200 without event and without a new version. time_zone and the list size
    are checked as for createQuietHoursRule (unknown_time_zone, limit_reached).

    Args:
        rule_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (QuietHoursRuleInput):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | QuietHoursRule
    """

    return sync_detailed(
        rule_id=rule_id,
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    rule_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: QuietHoursRuleInput,
) -> Response[Problem | QuietHoursRule]:
    """Change a quiet hours rule

     Replaces the whole rule; an absent time_zone clears it (the rule then follows the person's time
    zone). Equal values answer 200 without event and without a new version. time_zone and the list size
    are checked as for createQuietHoursRule (unknown_time_zone, limit_reached).

    Args:
        rule_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (QuietHoursRuleInput):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | QuietHoursRule]
    """

    kwargs = _get_kwargs(
        rule_id=rule_id,
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    rule_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: QuietHoursRuleInput,
) -> Problem | QuietHoursRule | None:
    """Change a quiet hours rule

     Replaces the whole rule; an absent time_zone clears it (the rule then follows the person's time
    zone). Equal values answer 200 without event and without a new version. time_zone and the list size
    are checked as for createQuietHoursRule (unknown_time_zone, limit_reached).

    Args:
        rule_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (QuietHoursRuleInput):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | QuietHoursRule
    """

    return (
        await asyncio_detailed(
            rule_id=rule_id,
            client=client,
            body=body,
        )
    ).parsed
