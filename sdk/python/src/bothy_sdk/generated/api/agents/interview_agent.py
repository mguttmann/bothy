from http import HTTPStatus
from typing import Any

import httpx

from ...client import AuthenticatedClient, Client
from ...models.agent_interview_answer import AgentInterviewAnswer
from ...models.agent_interview_request import AgentInterviewRequest
from ...models.problem import Problem
from ...types import Response


def _get_kwargs(
    *,
    body: AgentInterviewRequest,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/agents/interview",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> AgentInterviewAnswer | Problem:
    if response.status_code == 200:
        response_200 = AgentInterviewAnswer.from_dict(response.json())

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

    if response.status_code == 503:
        response_503 = Problem.from_dict(response.json())

        return response_503

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[AgentInterviewAnswer | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: AgentInterviewRequest,
) -> Response[AgentInterviewAnswer | Problem]:
    """Let the builder agent interview the owner for a new agent

     The interview of "New agent" (26 6.1, R-API-29): the client sends the interview so far, at most
    `agents.interview.max_turns` turns of at most `agents.interview.answer_max_chars` characters each,
    and the server sends it as the ephemeral event `agent.interview_requested` to one live gateway
    connection of the builder agent named in `agents.builder_agent_id`, with the template catalogue (26
    7.4), the definition limits and `expires_at`, and waits at most `agents.interview.timeout`, the
    deadline of this request instead of `http.request_timeout` (02 14.2). The answer is `outcome =
    questions` with one or two questions, `outcome = draft` with a draft that the server checked with
    the rules of 26 4.3 and the catalogue check of 26 7.4 (its findings in `errors`, empty when it
    passes), or `outcome = fallback` with `fallback_reason` (`builder_not_configured`,
    `builder_offline`, `builder_timeout`, `builder_invalid`); the apps then offer the templates and the
    manual form. `cost_estimate` is the builder's estimate, never booked. Nothing of the interview is
    stored; every request writes one audit entry `agent.interview` with outcome, fallback reason and
    counts, never a text. Owners only (`403` for everyone else, also in "view as"), `503 maintenance` in
    maintenance mode. More turns than `agents.interview.max_turns` is `400 validation_failed` with
    `too_many_items` on `/turns`, a longer turn `too_long` on `/turns/<index>/text`, an interview that
    no longer fits `sync.event_payload_max_bytes` `too_large` on `/turns`.

    Args:
        body (AgentInterviewRequest): The interview so far, kept by the client (26 6.1). Nothing
            of it is stored.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AgentInterviewAnswer | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    body: AgentInterviewRequest,
) -> AgentInterviewAnswer | Problem | None:
    """Let the builder agent interview the owner for a new agent

     The interview of "New agent" (26 6.1, R-API-29): the client sends the interview so far, at most
    `agents.interview.max_turns` turns of at most `agents.interview.answer_max_chars` characters each,
    and the server sends it as the ephemeral event `agent.interview_requested` to one live gateway
    connection of the builder agent named in `agents.builder_agent_id`, with the template catalogue (26
    7.4), the definition limits and `expires_at`, and waits at most `agents.interview.timeout`, the
    deadline of this request instead of `http.request_timeout` (02 14.2). The answer is `outcome =
    questions` with one or two questions, `outcome = draft` with a draft that the server checked with
    the rules of 26 4.3 and the catalogue check of 26 7.4 (its findings in `errors`, empty when it
    passes), or `outcome = fallback` with `fallback_reason` (`builder_not_configured`,
    `builder_offline`, `builder_timeout`, `builder_invalid`); the apps then offer the templates and the
    manual form. `cost_estimate` is the builder's estimate, never booked. Nothing of the interview is
    stored; every request writes one audit entry `agent.interview` with outcome, fallback reason and
    counts, never a text. Owners only (`403` for everyone else, also in "view as"), `503 maintenance` in
    maintenance mode. More turns than `agents.interview.max_turns` is `400 validation_failed` with
    `too_many_items` on `/turns`, a longer turn `too_long` on `/turns/<index>/text`, an interview that
    no longer fits `sync.event_payload_max_bytes` `too_large` on `/turns`.

    Args:
        body (AgentInterviewRequest): The interview so far, kept by the client (26 6.1). Nothing
            of it is stored.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AgentInterviewAnswer | Problem
    """

    return sync_detailed(
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: AgentInterviewRequest,
) -> Response[AgentInterviewAnswer | Problem]:
    """Let the builder agent interview the owner for a new agent

     The interview of "New agent" (26 6.1, R-API-29): the client sends the interview so far, at most
    `agents.interview.max_turns` turns of at most `agents.interview.answer_max_chars` characters each,
    and the server sends it as the ephemeral event `agent.interview_requested` to one live gateway
    connection of the builder agent named in `agents.builder_agent_id`, with the template catalogue (26
    7.4), the definition limits and `expires_at`, and waits at most `agents.interview.timeout`, the
    deadline of this request instead of `http.request_timeout` (02 14.2). The answer is `outcome =
    questions` with one or two questions, `outcome = draft` with a draft that the server checked with
    the rules of 26 4.3 and the catalogue check of 26 7.4 (its findings in `errors`, empty when it
    passes), or `outcome = fallback` with `fallback_reason` (`builder_not_configured`,
    `builder_offline`, `builder_timeout`, `builder_invalid`); the apps then offer the templates and the
    manual form. `cost_estimate` is the builder's estimate, never booked. Nothing of the interview is
    stored; every request writes one audit entry `agent.interview` with outcome, fallback reason and
    counts, never a text. Owners only (`403` for everyone else, also in "view as"), `503 maintenance` in
    maintenance mode. More turns than `agents.interview.max_turns` is `400 validation_failed` with
    `too_many_items` on `/turns`, a longer turn `too_long` on `/turns/<index>/text`, an interview that
    no longer fits `sync.event_payload_max_bytes` `too_large` on `/turns`.

    Args:
        body (AgentInterviewRequest): The interview so far, kept by the client (26 6.1). Nothing
            of it is stored.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AgentInterviewAnswer | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: AgentInterviewRequest,
) -> AgentInterviewAnswer | Problem | None:
    """Let the builder agent interview the owner for a new agent

     The interview of "New agent" (26 6.1, R-API-29): the client sends the interview so far, at most
    `agents.interview.max_turns` turns of at most `agents.interview.answer_max_chars` characters each,
    and the server sends it as the ephemeral event `agent.interview_requested` to one live gateway
    connection of the builder agent named in `agents.builder_agent_id`, with the template catalogue (26
    7.4), the definition limits and `expires_at`, and waits at most `agents.interview.timeout`, the
    deadline of this request instead of `http.request_timeout` (02 14.2). The answer is `outcome =
    questions` with one or two questions, `outcome = draft` with a draft that the server checked with
    the rules of 26 4.3 and the catalogue check of 26 7.4 (its findings in `errors`, empty when it
    passes), or `outcome = fallback` with `fallback_reason` (`builder_not_configured`,
    `builder_offline`, `builder_timeout`, `builder_invalid`); the apps then offer the templates and the
    manual form. `cost_estimate` is the builder's estimate, never booked. Nothing of the interview is
    stored; every request writes one audit entry `agent.interview` with outcome, fallback reason and
    counts, never a text. Owners only (`403` for everyone else, also in "view as"), `503 maintenance` in
    maintenance mode. More turns than `agents.interview.max_turns` is `400 validation_failed` with
    `too_many_items` on `/turns`, a longer turn `too_long` on `/turns/<index>/text`, an interview that
    no longer fits `sync.event_payload_max_bytes` `too_large` on `/turns`.

    Args:
        body (AgentInterviewRequest): The interview so far, kept by the client (26 6.1). Nothing
            of it is stored.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AgentInterviewAnswer | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
        )
    ).parsed
