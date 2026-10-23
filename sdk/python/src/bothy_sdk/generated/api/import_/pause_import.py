from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.import_source import ImportSource
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    import_id: UUID,
    *,
    idempotency_key: UUID | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(idempotency_key, Unset):
        headers["Idempotency-Key"] = idempotency_key

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/imports/{import_id}/pause".format(
            import_id=quote(str(import_id), safe=""),
        ),
    }

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ImportSource | Problem:
    if response.status_code == 200:
        response_200 = ImportSource.from_dict(response.json())

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
) -> Response[ImportSource | Problem]:
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
    idempotency_key: UUID | Unset = UNSET,
) -> Response[ImportSource | Problem]:
    """Pause an import

     Moves a `running` source to `paused`; batches and cursor writes then answer `409 conflict` with
    `reason` `import_not_running`. Pausing a `paused` source answers it unchanged. Any other state is
    `409 conflict` with `reason` `import_state`.

    Args:
        import_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ImportSource | Problem]
    """

    kwargs = _get_kwargs(
        import_id=import_id,
        idempotency_key=idempotency_key,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    import_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    idempotency_key: UUID | Unset = UNSET,
) -> ImportSource | Problem | None:
    """Pause an import

     Moves a `running` source to `paused`; batches and cursor writes then answer `409 conflict` with
    `reason` `import_not_running`. Pausing a `paused` source answers it unchanged. Any other state is
    `409 conflict` with `reason` `import_state`.

    Args:
        import_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ImportSource | Problem
    """

    return sync_detailed(
        import_id=import_id,
        client=client,
        idempotency_key=idempotency_key,
    ).parsed


async def asyncio_detailed(
    import_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[ImportSource | Problem]:
    """Pause an import

     Moves a `running` source to `paused`; batches and cursor writes then answer `409 conflict` with
    `reason` `import_not_running`. Pausing a `paused` source answers it unchanged. Any other state is
    `409 conflict` with `reason` `import_state`.

    Args:
        import_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ImportSource | Problem]
    """

    kwargs = _get_kwargs(
        import_id=import_id,
        idempotency_key=idempotency_key,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    import_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    idempotency_key: UUID | Unset = UNSET,
) -> ImportSource | Problem | None:
    """Pause an import

     Moves a `running` source to `paused`; batches and cursor writes then answer `409 conflict` with
    `reason` `import_not_running`. Pausing a `paused` source answers it unchanged. Any other state is
    `409 conflict` with `reason` `import_state`.

    Args:
        import_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ImportSource | Problem
    """

    return (
        await asyncio_detailed(
            import_id=import_id,
            client=client,
            idempotency_key=idempotency_key,
        )
    ).parsed
