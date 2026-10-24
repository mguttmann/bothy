from http import HTTPStatus
from typing import Any
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.price import Price
from ...models.price_create import PriceCreate
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    body: PriceCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(idempotency_key, Unset):
        headers["Idempotency-Key"] = idempotency_key

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/prices",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Price | Problem:
    if response.status_code == 201:
        response_201 = Price.from_dict(response.json())

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
) -> Response[Price | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: PriceCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[Price | Problem]:
    """Add a price with source

     Owner with step-up, audited with row, source and retrieval time (10, 14). `source_url` is an
    absolute https URL without user info and fragment, `retrieved_at` not in the future, `currency` an
    ISO 4217 code. 409 `conflict` with `reason` `price_overlap` when the validity overlaps a row of the
    same provider, model and unit, `price_currency_mismatch` when an overlapping row of the same
    provider and model has another currency (one currency per price set).

    Args:
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (PriceCreate): A price per unit (per million tokens for the `*_mtok` units) with its
            public source. `valid_to`, when present, lies after `valid_from`; `valid_from` may lie in
            the past. `source_url` is an absolute https URL without user info and fragment;
            `retrieved_at` is not in the future.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Price | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
        idempotency_key=idempotency_key,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    body: PriceCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> Price | Problem | None:
    """Add a price with source

     Owner with step-up, audited with row, source and retrieval time (10, 14). `source_url` is an
    absolute https URL without user info and fragment, `retrieved_at` not in the future, `currency` an
    ISO 4217 code. 409 `conflict` with `reason` `price_overlap` when the validity overlaps a row of the
    same provider, model and unit, `price_currency_mismatch` when an overlapping row of the same
    provider and model has another currency (one currency per price set).

    Args:
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (PriceCreate): A price per unit (per million tokens for the `*_mtok` units) with its
            public source. `valid_to`, when present, lies after `valid_from`; `valid_from` may lie in
            the past. `source_url` is an absolute https URL without user info and fragment;
            `retrieved_at` is not in the future.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Price | Problem
    """

    return sync_detailed(
        client=client,
        body=body,
        idempotency_key=idempotency_key,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: PriceCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> Response[Price | Problem]:
    """Add a price with source

     Owner with step-up, audited with row, source and retrieval time (10, 14). `source_url` is an
    absolute https URL without user info and fragment, `retrieved_at` not in the future, `currency` an
    ISO 4217 code. 409 `conflict` with `reason` `price_overlap` when the validity overlaps a row of the
    same provider, model and unit, `price_currency_mismatch` when an overlapping row of the same
    provider and model has another currency (one currency per price set).

    Args:
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (PriceCreate): A price per unit (per million tokens for the `*_mtok` units) with its
            public source. `valid_to`, when present, lies after `valid_from`; `valid_from` may lie in
            the past. `source_url` is an absolute https URL without user info and fragment;
            `retrieved_at` is not in the future.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Price | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
        idempotency_key=idempotency_key,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: PriceCreate,
    idempotency_key: UUID | Unset = UNSET,
) -> Price | Problem | None:
    """Add a price with source

     Owner with step-up, audited with row, source and retrieval time (10, 14). `source_url` is an
    absolute https URL without user info and fragment, `retrieved_at` not in the future, `currency` an
    ISO 4217 code. 409 `conflict` with `reason` `price_overlap` when the validity overlaps a row of the
    same provider, model and unit, `price_currency_mismatch` when an overlapping row of the same
    provider and model has another currency (one currency per price set).

    Args:
        idempotency_key (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        body (PriceCreate): A price per unit (per million tokens for the `*_mtok` units) with its
            public source. `valid_to`, when present, lies after `valid_from`; `valid_from` may lie in
            the past. `source_url` is an absolute https URL without user info and fragment;
            `retrieved_at` is not in the future.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Price | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
            idempotency_key=idempotency_key,
        )
    ).parsed
