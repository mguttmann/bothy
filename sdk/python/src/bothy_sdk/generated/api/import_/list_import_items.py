from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.import_item_page import ImportItemPage
from ...models.import_item_state import ImportItemState
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    import_id: UUID,
    *,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    state: ImportItemState | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["cursor"] = cursor

    params["limit"] = limit

    json_state: str | Unset = UNSET
    if not isinstance(state, Unset):
        json_state = state.value

    params["state"] = json_state

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/imports/{import_id}/items".format(
            import_id=quote(str(import_id), safe=""),
        ),
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ImportItemPage | Problem:
    if response.status_code == 200:
        response_200 = ImportItemPage.from_dict(response.json())

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
) -> Response[ImportItemPage | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    import_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    state: ImportItemState | Unset = UNSET,
) -> Response[ImportItemPage | Problem]:
    """List imported items

    Args:
        import_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        state (ImportItemState | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ImportItemPage | Problem]
    """

    kwargs = _get_kwargs(
        import_id=import_id,
        cursor=cursor,
        limit=limit,
        state=state,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    import_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    state: ImportItemState | Unset = UNSET,
) -> ImportItemPage | Problem | None:
    """List imported items

    Args:
        import_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        state (ImportItemState | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ImportItemPage | Problem
    """

    return sync_detailed(
        import_id=import_id,
        client=client,
        cursor=cursor,
        limit=limit,
        state=state,
    ).parsed


async def asyncio_detailed(
    import_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    state: ImportItemState | Unset = UNSET,
) -> Response[ImportItemPage | Problem]:
    """List imported items

    Args:
        import_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        state (ImportItemState | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ImportItemPage | Problem]
    """

    kwargs = _get_kwargs(
        import_id=import_id,
        cursor=cursor,
        limit=limit,
        state=state,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    import_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    state: ImportItemState | Unset = UNSET,
) -> ImportItemPage | Problem | None:
    """List imported items

    Args:
        import_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        cursor (str | Unset): Opaque pagination cursor. Clients must not parse or build cursors.
        limit (int | Unset): Page size. Default `api.page_size_default`; larger values than
            `api.page_size_max` are capped.
        state (ImportItemState | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ImportItemPage | Problem
    """

    return (
        await asyncio_detailed(
            import_id=import_id,
            client=client,
            cursor=cursor,
            limit=limit,
            state=state,
        )
    ).parsed
