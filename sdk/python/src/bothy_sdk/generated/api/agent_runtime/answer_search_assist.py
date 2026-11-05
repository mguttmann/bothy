from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.search_assist_answer import SearchAssistAnswer
from ...types import Response


def _get_kwargs(
    request_id: UUID,
    *,
    body: SearchAssistAnswer,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/search-assist-requests/{request_id}/answer".format(
            request_id=quote(str(request_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Any | Problem:
    if response.status_code == 204:
        response_204 = cast(Any, None)
        return response_204

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

    if response.status_code == 410:
        response_410 = Problem.from_dict(response.json())

        return response_410

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Any | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    request_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: SearchAssistAnswer,
) -> Response[Any | Problem]:
    """Answer a search assistant request

     The assistant agent answers an open `search.assist_requested` (14, 4.10; 09, 10.6); the
    first valid answer goes back to the person. `404` for an unknown or forgotten request and for
    every principal but the addressed agent, `410 gone` after the wait ended, `204` without effect for
    an
    answer after the first one. `400 validation_failed` for a reference that was not sent or is
    named twice (`unknown_ref`, `duplicate`), more picks than `max_picks` or more terms than
    `search.assist.max_terms` (`too_many_items`), an empty reason or term (`too_short`), a reason
    longer than `search.assist.reason_max_chars` or a term longer than 64 characters (`too_long`)
    or a text that is no display text (`invalid_utf8`, `control_character`, `bidi_control`); the
    request stays open.

    Args:
        request_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (SearchAssistAnswer): Answer of the assistant agent to `search.assist_requested`
            (action `search.assist.respond`).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        request_id=request_id,
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    request_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: SearchAssistAnswer,
) -> Any | Problem | None:
    """Answer a search assistant request

     The assistant agent answers an open `search.assist_requested` (14, 4.10; 09, 10.6); the
    first valid answer goes back to the person. `404` for an unknown or forgotten request and for
    every principal but the addressed agent, `410 gone` after the wait ended, `204` without effect for
    an
    answer after the first one. `400 validation_failed` for a reference that was not sent or is
    named twice (`unknown_ref`, `duplicate`), more picks than `max_picks` or more terms than
    `search.assist.max_terms` (`too_many_items`), an empty reason or term (`too_short`), a reason
    longer than `search.assist.reason_max_chars` or a term longer than 64 characters (`too_long`)
    or a text that is no display text (`invalid_utf8`, `control_character`, `bidi_control`); the
    request stays open.

    Args:
        request_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (SearchAssistAnswer): Answer of the assistant agent to `search.assist_requested`
            (action `search.assist.respond`).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return sync_detailed(
        request_id=request_id,
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    request_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: SearchAssistAnswer,
) -> Response[Any | Problem]:
    """Answer a search assistant request

     The assistant agent answers an open `search.assist_requested` (14, 4.10; 09, 10.6); the
    first valid answer goes back to the person. `404` for an unknown or forgotten request and for
    every principal but the addressed agent, `410 gone` after the wait ended, `204` without effect for
    an
    answer after the first one. `400 validation_failed` for a reference that was not sent or is
    named twice (`unknown_ref`, `duplicate`), more picks than `max_picks` or more terms than
    `search.assist.max_terms` (`too_many_items`), an empty reason or term (`too_short`), a reason
    longer than `search.assist.reason_max_chars` or a term longer than 64 characters (`too_long`)
    or a text that is no display text (`invalid_utf8`, `control_character`, `bidi_control`); the
    request stays open.

    Args:
        request_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (SearchAssistAnswer): Answer of the assistant agent to `search.assist_requested`
            (action `search.assist.respond`).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        request_id=request_id,
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    request_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: SearchAssistAnswer,
) -> Any | Problem | None:
    """Answer a search assistant request

     The assistant agent answers an open `search.assist_requested` (14, 4.10; 09, 10.6); the
    first valid answer goes back to the person. `404` for an unknown or forgotten request and for
    every principal but the addressed agent, `410 gone` after the wait ended, `204` without effect for
    an
    answer after the first one. `400 validation_failed` for a reference that was not sent or is
    named twice (`unknown_ref`, `duplicate`), more picks than `max_picks` or more terms than
    `search.assist.max_terms` (`too_many_items`), an empty reason or term (`too_short`), a reason
    longer than `search.assist.reason_max_chars` or a term longer than 64 characters (`too_long`)
    or a text that is no display text (`invalid_utf8`, `control_character`, `bidi_control`); the
    request stays open.

    Args:
        request_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (SearchAssistAnswer): Answer of the assistant agent to `search.assist_requested`
            (action `search.assist.respond`).

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return (
        await asyncio_detailed(
            request_id=request_id,
            client=client,
            body=body,
        )
    ).parsed
