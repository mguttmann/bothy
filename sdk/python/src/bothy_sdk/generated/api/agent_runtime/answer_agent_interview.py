from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.agent_interview_builder_answer import AgentInterviewBuilderAnswer
from ...models.problem import Problem
from ...types import Response


def _get_kwargs(
    request_id: UUID,
    *,
    body: AgentInterviewBuilderAnswer,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/agent-interview-requests/{request_id}/answer".format(
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

    if response.status_code == 409:
        response_409 = Problem.from_dict(response.json())

        return response_409

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
    body: AgentInterviewBuilderAnswer,
) -> Response[Any | Problem]:
    """Answer an interview request

     The builder agent answers an open `agent.interview_requested` once (26 6.1 item 2, 6.2): `kind =
    questions` with one or two questions of at most `agents.interview.answer_max_chars` characters, or
    `kind = draft` with a draft of the shape of 26 4.2 plus `name`, `handle`, `description` and an
    optional `channel_suggestion`; and the usage in `costs`. `404` for an unknown or forgotten request
    and for every principal but the builder agent the request was sent to, `410 gone` after
    `expires_at`, `409 conflict` with `reason: already_answered` for a second answer. The server decodes
    the answer strictly and never trusts it: a member outside the schema, a wrong type or a count
    outside the schema (no question, more than two) is refused by the request validation (`400`, the
    request stays open until `expires_at`); a draft that reaches a surface of 26 5.5 (a frontmatter, an
    inline shell form or an `@` import outside the agent folder) or a text that is no display text ends
    the request with `builder_invalid` and answers `400 validation_failed`. Any other finding of 26 4.3
    or of the catalogue check (`mcp_not_in_catalogue`) goes back to the person as a field error of the
    draft; nothing is stored.

    Args:
        request_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (AgentInterviewBuilderAnswer): Answer of the builder agent to
            `agent.interview_requested` (action `agent.interview.respond`, 26 6.1 item 2). It has no
            member for settings files, hooks, permission rules, commands, subagents or plugin parts
            (26 4.3 item 1).

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
    body: AgentInterviewBuilderAnswer,
) -> Any | Problem | None:
    """Answer an interview request

     The builder agent answers an open `agent.interview_requested` once (26 6.1 item 2, 6.2): `kind =
    questions` with one or two questions of at most `agents.interview.answer_max_chars` characters, or
    `kind = draft` with a draft of the shape of 26 4.2 plus `name`, `handle`, `description` and an
    optional `channel_suggestion`; and the usage in `costs`. `404` for an unknown or forgotten request
    and for every principal but the builder agent the request was sent to, `410 gone` after
    `expires_at`, `409 conflict` with `reason: already_answered` for a second answer. The server decodes
    the answer strictly and never trusts it: a member outside the schema, a wrong type or a count
    outside the schema (no question, more than two) is refused by the request validation (`400`, the
    request stays open until `expires_at`); a draft that reaches a surface of 26 5.5 (a frontmatter, an
    inline shell form or an `@` import outside the agent folder) or a text that is no display text ends
    the request with `builder_invalid` and answers `400 validation_failed`. Any other finding of 26 4.3
    or of the catalogue check (`mcp_not_in_catalogue`) goes back to the person as a field error of the
    draft; nothing is stored.

    Args:
        request_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (AgentInterviewBuilderAnswer): Answer of the builder agent to
            `agent.interview_requested` (action `agent.interview.respond`, 26 6.1 item 2). It has no
            member for settings files, hooks, permission rules, commands, subagents or plugin parts
            (26 4.3 item 1).

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
    body: AgentInterviewBuilderAnswer,
) -> Response[Any | Problem]:
    """Answer an interview request

     The builder agent answers an open `agent.interview_requested` once (26 6.1 item 2, 6.2): `kind =
    questions` with one or two questions of at most `agents.interview.answer_max_chars` characters, or
    `kind = draft` with a draft of the shape of 26 4.2 plus `name`, `handle`, `description` and an
    optional `channel_suggestion`; and the usage in `costs`. `404` for an unknown or forgotten request
    and for every principal but the builder agent the request was sent to, `410 gone` after
    `expires_at`, `409 conflict` with `reason: already_answered` for a second answer. The server decodes
    the answer strictly and never trusts it: a member outside the schema, a wrong type or a count
    outside the schema (no question, more than two) is refused by the request validation (`400`, the
    request stays open until `expires_at`); a draft that reaches a surface of 26 5.5 (a frontmatter, an
    inline shell form or an `@` import outside the agent folder) or a text that is no display text ends
    the request with `builder_invalid` and answers `400 validation_failed`. Any other finding of 26 4.3
    or of the catalogue check (`mcp_not_in_catalogue`) goes back to the person as a field error of the
    draft; nothing is stored.

    Args:
        request_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (AgentInterviewBuilderAnswer): Answer of the builder agent to
            `agent.interview_requested` (action `agent.interview.respond`, 26 6.1 item 2). It has no
            member for settings files, hooks, permission rules, commands, subagents or plugin parts
            (26 4.3 item 1).

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
    body: AgentInterviewBuilderAnswer,
) -> Any | Problem | None:
    """Answer an interview request

     The builder agent answers an open `agent.interview_requested` once (26 6.1 item 2, 6.2): `kind =
    questions` with one or two questions of at most `agents.interview.answer_max_chars` characters, or
    `kind = draft` with a draft of the shape of 26 4.2 plus `name`, `handle`, `description` and an
    optional `channel_suggestion`; and the usage in `costs`. `404` for an unknown or forgotten request
    and for every principal but the builder agent the request was sent to, `410 gone` after
    `expires_at`, `409 conflict` with `reason: already_answered` for a second answer. The server decodes
    the answer strictly and never trusts it: a member outside the schema, a wrong type or a count
    outside the schema (no question, more than two) is refused by the request validation (`400`, the
    request stays open until `expires_at`); a draft that reaches a surface of 26 5.5 (a frontmatter, an
    inline shell form or an `@` import outside the agent folder) or a text that is no display text ends
    the request with `builder_invalid` and answers `400 validation_failed`. Any other finding of 26 4.3
    or of the catalogue check (`mcp_not_in_catalogue`) goes back to the person as a field error of the
    draft; nothing is stored.

    Args:
        request_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (AgentInterviewBuilderAnswer): Answer of the builder agent to
            `agent.interview_requested` (action `agent.interview.respond`, 26 6.1 item 2). It has no
            member for settings files, hooks, permission rules, commands, subagents or plugin parts
            (26 4.3 item 1).

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
