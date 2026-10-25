from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.transcript import Transcript
from ...models.transcript_submit import TranscriptSubmit
from ...types import Response


def _get_kwargs(
    file_id: UUID,
    *,
    body: TranscriptSubmit,
    if_match: str,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    headers["If-Match"] = if_match

    _kwargs: dict[str, Any] = {
        "method": "put",
        "url": "/files/{file_id}/transcript".format(
            file_id=quote(str(file_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | Transcript:
    if response.status_code == 200:
        response_200 = Transcript.from_dict(response.json())

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

    if response.status_code == 412:
        response_412 = Problem.from_dict(response.json())

        return response_412

    if response.status_code == 428:
        response_428 = Problem.from_dict(response.json())

        return response_428

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Problem | Transcript]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    file_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: TranscriptSubmit,
    if_match: str,
) -> Response[Problem | Transcript]:
    """Store a transcript created by the agent

     Mode `agent` of `voice.transcription_mode` only (12, 8.2). Only the operator agent of the thread
    whose message the voice file is attached to, with read access, may store the transcript; a file the
    caller cannot read, an unattached file of another uploader and an unknown id answer the same 404,
    and a visible caller who is not that operator gets 403 `forbidden`. Another mode, or a file that is
    not a voice format, gives 409 `conflict` with `reason` `transcription_unavailable`; a file that is
    not ready gives 409 `file_not_ready` or `file_blocked`. `If-Match` carries the transcript `ETag` of
    `getFile` or `getTranscript` (derived from status, source, language and text, 03 R-DM-48); without
    it or with `*` 428 `precondition_required`, with another value 412 `version_conflict` with the
    current `Transcript` in `current` and its `ETag`. `text` may be empty, must not contain U+0000
    (`invalid`) and is at most `message.max_content_bytes` UTF-8 bytes (`too_long`). The answer has
    `status` `done` and `source` `agent` and the new `ETag`; a later submit with that value replaces the
    transcript. No audit entry.

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str):
        body (TranscriptSubmit):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | Transcript]
    """

    kwargs = _get_kwargs(
        file_id=file_id,
        body=body,
        if_match=if_match,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    file_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: TranscriptSubmit,
    if_match: str,
) -> Problem | Transcript | None:
    """Store a transcript created by the agent

     Mode `agent` of `voice.transcription_mode` only (12, 8.2). Only the operator agent of the thread
    whose message the voice file is attached to, with read access, may store the transcript; a file the
    caller cannot read, an unattached file of another uploader and an unknown id answer the same 404,
    and a visible caller who is not that operator gets 403 `forbidden`. Another mode, or a file that is
    not a voice format, gives 409 `conflict` with `reason` `transcription_unavailable`; a file that is
    not ready gives 409 `file_not_ready` or `file_blocked`. `If-Match` carries the transcript `ETag` of
    `getFile` or `getTranscript` (derived from status, source, language and text, 03 R-DM-48); without
    it or with `*` 428 `precondition_required`, with another value 412 `version_conflict` with the
    current `Transcript` in `current` and its `ETag`. `text` may be empty, must not contain U+0000
    (`invalid`) and is at most `message.max_content_bytes` UTF-8 bytes (`too_long`). The answer has
    `status` `done` and `source` `agent` and the new `ETag`; a later submit with that value replaces the
    transcript. No audit entry.

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str):
        body (TranscriptSubmit):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | Transcript
    """

    return sync_detailed(
        file_id=file_id,
        client=client,
        body=body,
        if_match=if_match,
    ).parsed


async def asyncio_detailed(
    file_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: TranscriptSubmit,
    if_match: str,
) -> Response[Problem | Transcript]:
    """Store a transcript created by the agent

     Mode `agent` of `voice.transcription_mode` only (12, 8.2). Only the operator agent of the thread
    whose message the voice file is attached to, with read access, may store the transcript; a file the
    caller cannot read, an unattached file of another uploader and an unknown id answer the same 404,
    and a visible caller who is not that operator gets 403 `forbidden`. Another mode, or a file that is
    not a voice format, gives 409 `conflict` with `reason` `transcription_unavailable`; a file that is
    not ready gives 409 `file_not_ready` or `file_blocked`. `If-Match` carries the transcript `ETag` of
    `getFile` or `getTranscript` (derived from status, source, language and text, 03 R-DM-48); without
    it or with `*` 428 `precondition_required`, with another value 412 `version_conflict` with the
    current `Transcript` in `current` and its `ETag`. `text` may be empty, must not contain U+0000
    (`invalid`) and is at most `message.max_content_bytes` UTF-8 bytes (`too_long`). The answer has
    `status` `done` and `source` `agent` and the new `ETag`; a later submit with that value replaces the
    transcript. No audit entry.

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str):
        body (TranscriptSubmit):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | Transcript]
    """

    kwargs = _get_kwargs(
        file_id=file_id,
        body=body,
        if_match=if_match,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    file_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: TranscriptSubmit,
    if_match: str,
) -> Problem | Transcript | None:
    """Store a transcript created by the agent

     Mode `agent` of `voice.transcription_mode` only (12, 8.2). Only the operator agent of the thread
    whose message the voice file is attached to, with read access, may store the transcript; a file the
    caller cannot read, an unattached file of another uploader and an unknown id answer the same 404,
    and a visible caller who is not that operator gets 403 `forbidden`. Another mode, or a file that is
    not a voice format, gives 409 `conflict` with `reason` `transcription_unavailable`; a file that is
    not ready gives 409 `file_not_ready` or `file_blocked`. `If-Match` carries the transcript `ETag` of
    `getFile` or `getTranscript` (derived from status, source, language and text, 03 R-DM-48); without
    it or with `*` 428 `precondition_required`, with another value 412 `version_conflict` with the
    current `Transcript` in `current` and its `ETag`. `text` may be empty, must not contain U+0000
    (`invalid`) and is at most `message.max_content_bytes` UTF-8 bytes (`too_long`). The answer has
    `status` `done` and `source` `agent` and the new `ETag`; a later submit with that value replaces the
    transcript. No audit entry.

    Args:
        file_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        if_match (str):
        body (TranscriptSubmit):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | Transcript
    """

    return (
        await asyncio_detailed(
            file_id=file_id,
            client=client,
            body=body,
            if_match=if_match,
        )
    ).parsed
