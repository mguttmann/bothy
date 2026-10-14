import datetime
from http import HTTPStatus
from typing import Any
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.audit_decision import AuditDecision
from ...models.audit_event_page import AuditEventPage
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    actor_id: UUID | Unset = UNSET,
    action: str | Unset = UNSET,
    decision: AuditDecision | Unset = UNSET,
    resource_kind: str | Unset = UNSET,
    resource_id: UUID | Unset = UNSET,
    from_: datetime.datetime | Unset = UNSET,
    to: datetime.datetime | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["cursor"] = cursor

    params["limit"] = limit

    json_actor_id: str | Unset = UNSET
    if not isinstance(actor_id, Unset):
        json_actor_id = str(actor_id)
    params["actor_id"] = json_actor_id

    params["action"] = action

    json_decision: str | Unset = UNSET
    if not isinstance(decision, Unset):
        json_decision = decision.value

    params["decision"] = json_decision

    params["resource_kind"] = resource_kind

    json_resource_id: str | Unset = UNSET
    if not isinstance(resource_id, Unset):
        json_resource_id = str(resource_id)
    params["resource_id"] = json_resource_id

    json_from_: str | Unset = UNSET
    if not isinstance(from_, Unset):
        json_from_ = from_.isoformat()
    params["from"] = json_from_

    json_to: str | Unset = UNSET
    if not isinstance(to, Unset):
        json_to = to.isoformat()
    params["to"] = json_to

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/audit/events",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> AuditEventPage | Problem:
    if response.status_code == 200:
        response_200 = AuditEventPage.from_dict(response.json())

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

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[AuditEventPage | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    actor_id: UUID | Unset = UNSET,
    action: str | Unset = UNSET,
    decision: AuditDecision | Unset = UNSET,
    resource_kind: str | Unset = UNSET,
    resource_id: UUID | Unset = UNSET,
    from_: datetime.datetime | Unset = UNSET,
    to: datetime.datetime | Unset = UNSET,
) -> Response[AuditEventPage | Problem]:
    """Query the audit log

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        actor_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        action (str | Unset):
        decision (AuditDecision | Unset):
        resource_kind (str | Unset):
        resource_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        from_ (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision
            and suffix `Z`.
        to (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and
            suffix `Z`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AuditEventPage | Problem]
    """

    kwargs = _get_kwargs(
        cursor=cursor,
        limit=limit,
        actor_id=actor_id,
        action=action,
        decision=decision,
        resource_kind=resource_kind,
        resource_id=resource_id,
        from_=from_,
        to=to,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    actor_id: UUID | Unset = UNSET,
    action: str | Unset = UNSET,
    decision: AuditDecision | Unset = UNSET,
    resource_kind: str | Unset = UNSET,
    resource_id: UUID | Unset = UNSET,
    from_: datetime.datetime | Unset = UNSET,
    to: datetime.datetime | Unset = UNSET,
) -> AuditEventPage | Problem | None:
    """Query the audit log

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        actor_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        action (str | Unset):
        decision (AuditDecision | Unset):
        resource_kind (str | Unset):
        resource_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        from_ (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision
            and suffix `Z`.
        to (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and
            suffix `Z`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AuditEventPage | Problem
    """

    return sync_detailed(
        client=client,
        cursor=cursor,
        limit=limit,
        actor_id=actor_id,
        action=action,
        decision=decision,
        resource_kind=resource_kind,
        resource_id=resource_id,
        from_=from_,
        to=to,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    actor_id: UUID | Unset = UNSET,
    action: str | Unset = UNSET,
    decision: AuditDecision | Unset = UNSET,
    resource_kind: str | Unset = UNSET,
    resource_id: UUID | Unset = UNSET,
    from_: datetime.datetime | Unset = UNSET,
    to: datetime.datetime | Unset = UNSET,
) -> Response[AuditEventPage | Problem]:
    """Query the audit log

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        actor_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        action (str | Unset):
        decision (AuditDecision | Unset):
        resource_kind (str | Unset):
        resource_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        from_ (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision
            and suffix `Z`.
        to (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and
            suffix `Z`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AuditEventPage | Problem]
    """

    kwargs = _get_kwargs(
        cursor=cursor,
        limit=limit,
        actor_id=actor_id,
        action=action,
        decision=decision,
        resource_kind=resource_kind,
        resource_id=resource_id,
        from_=from_,
        to=to,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    actor_id: UUID | Unset = UNSET,
    action: str | Unset = UNSET,
    decision: AuditDecision | Unset = UNSET,
    resource_kind: str | Unset = UNSET,
    resource_id: UUID | Unset = UNSET,
    from_: datetime.datetime | Unset = UNSET,
    to: datetime.datetime | Unset = UNSET,
) -> AuditEventPage | Problem | None:
    """Query the audit log

    Args:
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        actor_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        action (str | Unset):
        decision (AuditDecision | Unset):
        resource_kind (str | Unset):
        resource_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        from_ (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision
            and suffix `Z`.
        to (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and
            suffix `Z`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AuditEventPage | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            cursor=cursor,
            limit=limit,
            actor_id=actor_id,
            action=action,
            decision=decision,
            resource_kind=resource_kind,
            resource_id=resource_id,
            from_=from_,
            to=to,
        )
    ).parsed
