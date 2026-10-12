from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.reason_body import ReasonBody
from ...models.space import Space
from ...types import UNSET, Response, Unset


def _get_kwargs(
    space_id: UUID,
    *,
    body: ReasonBody,
    idempotency_key: UUID | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(idempotency_key, Unset):
        headers["Idempotency-Key"] = idempotency_key

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/spaces/{space_id}/archive".format(
            space_id=quote(str(space_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | Space:
    if response.status_code == 200:
        response_200 = Space.from_dict(response.json())

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
) -> Response[Problem | Space]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    space_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: ReasonBody,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[Problem | Space]:
    """Archive a space

     Final; there is no unarchive. Every channel of the space becomes read-only for everyone, owners
    included; reading stays for those who could read before. Archiving is a permission change with
    `access.changed`; archived spaces leave `listSpaces` (07, 4.5).

    Args:
        space_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (ReasonBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | Space]
    """

    kwargs = _get_kwargs(
        space_id=space_id,
        body=body,
        idempotency_key=idempotency_key,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    space_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: ReasonBody,
    idempotency_key: UUID | Unset = UNSET,
) -> Problem | Space | None:
    """Archive a space

     Final; there is no unarchive. Every channel of the space becomes read-only for everyone, owners
    included; reading stays for those who could read before. Archiving is a permission change with
    `access.changed`; archived spaces leave `listSpaces` (07, 4.5).

    Args:
        space_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (ReasonBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | Space
    """

    return sync_detailed(
        space_id=space_id,
        client=client,
        body=body,
        idempotency_key=idempotency_key,
    ).parsed


async def asyncio_detailed(
    space_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: ReasonBody,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[Problem | Space]:
    """Archive a space

     Final; there is no unarchive. Every channel of the space becomes read-only for everyone, owners
    included; reading stays for those who could read before. Archiving is a permission change with
    `access.changed`; archived spaces leave `listSpaces` (07, 4.5).

    Args:
        space_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (ReasonBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | Space]
    """

    kwargs = _get_kwargs(
        space_id=space_id,
        body=body,
        idempotency_key=idempotency_key,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    space_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: ReasonBody,
    idempotency_key: UUID | Unset = UNSET,
) -> Problem | Space | None:
    """Archive a space

     Final; there is no unarchive. Every channel of the space becomes read-only for everyone, owners
    included; reading stays for those who could read before. Archiving is a permission change with
    `access.changed`; archived spaces leave `listSpaces` (07, 4.5).

    Args:
        space_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (ReasonBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | Space
    """

    return (
        await asyncio_detailed(
            space_id=space_id,
            client=client,
            body=body,
            idempotency_key=idempotency_key,
        )
    ).parsed
