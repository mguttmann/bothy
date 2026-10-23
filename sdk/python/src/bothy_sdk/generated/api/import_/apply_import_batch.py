from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.import_batch import ImportBatch
from ...models.import_batch_result import ImportBatchResult
from ...models.problem import Problem
from ...types import Response


def _get_kwargs(
    import_id: UUID,
    *,
    body: ImportBatch,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/imports/{import_id}/batches".format(
            import_id=quote(str(import_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ImportBatchResult | Problem:
    if response.status_code == 200:
        response_200 = ImportBatchResult.from_dict(response.json())

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

    if response.status_code == 413:
        response_413 = Problem.from_dict(response.json())

        return response_413

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[ImportBatchResult | Problem]:
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
    body: ImportBatch,
) -> Response[ImportBatchResult | Problem]:
    """Write a batch of imported objects

     One transaction, at most `import.batch_max_items` objects. Every target must lie in a channel of
    this running source. Existing `posted` keys return their target without a duplicate or event (22,
    7.2). Writes `thread.updated` and `import.progress` instead of one event per message.
    A source that is not `running` is `409 conflict` with `reason` `import_not_running`; a target
    outside the channels of the source is `409 conflict` with `reason` `target_outside_source`; a new
    channel whose name equals a live channel of its space is `409 conflict` with `reason`
    `name_collision`. The `data` of a structure item follows `ImportSpaceData`, `ImportChannelData`,
    `ImportForumTagData` or `ImportUserData`.
    A batch with `thread_origin_key` concerns exactly one thread (`thread:<id>`, or `channel:<id>` for
    the main thread of a text channel; a forum has none, `400 validation_failed`). Its optional first
    item is the `thread` item of that key (`ImportThreadData`), every other item a `message`
    (`ImportMessageData`) or a `pin` (`ImportPinData`); a structure item there, or a content item in a
    batch without `thread_origin_key`, is `400 validation_failed` at its `item_kind`. A thread that is
    neither posted nor created by the batch is `409 conflict` with `reason` `thread_not_imported`; a
    message whose author item is not posted `409 conflict` with `reason` `author_not_imported`. Imported
    messages have `origin = import`, are silent, write no mentions and never trigger an agent; a posted
    thread only follows the mirror rules of 22 12.2 step 3. At its end a batch that changed something
    sets `threads.unread_floor_seq` to the end of the thread and appends one `thread.updated` and one
    `import.progress`, never `message.created` (04 15.8).

    Args:
        import_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (ImportBatch): One transaction. Objects of one thread, in source order; structure
            objects (space, channel, forum tag, person) come in batches without `thread_origin_key`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ImportBatchResult | Problem]
    """

    kwargs = _get_kwargs(
        import_id=import_id,
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    import_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: ImportBatch,
) -> ImportBatchResult | Problem | None:
    """Write a batch of imported objects

     One transaction, at most `import.batch_max_items` objects. Every target must lie in a channel of
    this running source. Existing `posted` keys return their target without a duplicate or event (22,
    7.2). Writes `thread.updated` and `import.progress` instead of one event per message.
    A source that is not `running` is `409 conflict` with `reason` `import_not_running`; a target
    outside the channels of the source is `409 conflict` with `reason` `target_outside_source`; a new
    channel whose name equals a live channel of its space is `409 conflict` with `reason`
    `name_collision`. The `data` of a structure item follows `ImportSpaceData`, `ImportChannelData`,
    `ImportForumTagData` or `ImportUserData`.
    A batch with `thread_origin_key` concerns exactly one thread (`thread:<id>`, or `channel:<id>` for
    the main thread of a text channel; a forum has none, `400 validation_failed`). Its optional first
    item is the `thread` item of that key (`ImportThreadData`), every other item a `message`
    (`ImportMessageData`) or a `pin` (`ImportPinData`); a structure item there, or a content item in a
    batch without `thread_origin_key`, is `400 validation_failed` at its `item_kind`. A thread that is
    neither posted nor created by the batch is `409 conflict` with `reason` `thread_not_imported`; a
    message whose author item is not posted `409 conflict` with `reason` `author_not_imported`. Imported
    messages have `origin = import`, are silent, write no mentions and never trigger an agent; a posted
    thread only follows the mirror rules of 22 12.2 step 3. At its end a batch that changed something
    sets `threads.unread_floor_seq` to the end of the thread and appends one `thread.updated` and one
    `import.progress`, never `message.created` (04 15.8).

    Args:
        import_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (ImportBatch): One transaction. Objects of one thread, in source order; structure
            objects (space, channel, forum tag, person) come in batches without `thread_origin_key`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ImportBatchResult | Problem
    """

    return sync_detailed(
        import_id=import_id,
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    import_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: ImportBatch,
) -> Response[ImportBatchResult | Problem]:
    """Write a batch of imported objects

     One transaction, at most `import.batch_max_items` objects. Every target must lie in a channel of
    this running source. Existing `posted` keys return their target without a duplicate or event (22,
    7.2). Writes `thread.updated` and `import.progress` instead of one event per message.
    A source that is not `running` is `409 conflict` with `reason` `import_not_running`; a target
    outside the channels of the source is `409 conflict` with `reason` `target_outside_source`; a new
    channel whose name equals a live channel of its space is `409 conflict` with `reason`
    `name_collision`. The `data` of a structure item follows `ImportSpaceData`, `ImportChannelData`,
    `ImportForumTagData` or `ImportUserData`.
    A batch with `thread_origin_key` concerns exactly one thread (`thread:<id>`, or `channel:<id>` for
    the main thread of a text channel; a forum has none, `400 validation_failed`). Its optional first
    item is the `thread` item of that key (`ImportThreadData`), every other item a `message`
    (`ImportMessageData`) or a `pin` (`ImportPinData`); a structure item there, or a content item in a
    batch without `thread_origin_key`, is `400 validation_failed` at its `item_kind`. A thread that is
    neither posted nor created by the batch is `409 conflict` with `reason` `thread_not_imported`; a
    message whose author item is not posted `409 conflict` with `reason` `author_not_imported`. Imported
    messages have `origin = import`, are silent, write no mentions and never trigger an agent; a posted
    thread only follows the mirror rules of 22 12.2 step 3. At its end a batch that changed something
    sets `threads.unread_floor_seq` to the end of the thread and appends one `thread.updated` and one
    `import.progress`, never `message.created` (04 15.8).

    Args:
        import_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (ImportBatch): One transaction. Objects of one thread, in source order; structure
            objects (space, channel, forum tag, person) come in batches without `thread_origin_key`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ImportBatchResult | Problem]
    """

    kwargs = _get_kwargs(
        import_id=import_id,
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    import_id: UUID,
    *,
    client: AuthenticatedClient | Client,
    body: ImportBatch,
) -> ImportBatchResult | Problem | None:
    """Write a batch of imported objects

     One transaction, at most `import.batch_max_items` objects. Every target must lie in a channel of
    this running source. Existing `posted` keys return their target without a duplicate or event (22,
    7.2). Writes `thread.updated` and `import.progress` instead of one event per message.
    A source that is not `running` is `409 conflict` with `reason` `import_not_running`; a target
    outside the channels of the source is `409 conflict` with `reason` `target_outside_source`; a new
    channel whose name equals a live channel of its space is `409 conflict` with `reason`
    `name_collision`. The `data` of a structure item follows `ImportSpaceData`, `ImportChannelData`,
    `ImportForumTagData` or `ImportUserData`.
    A batch with `thread_origin_key` concerns exactly one thread (`thread:<id>`, or `channel:<id>` for
    the main thread of a text channel; a forum has none, `400 validation_failed`). Its optional first
    item is the `thread` item of that key (`ImportThreadData`), every other item a `message`
    (`ImportMessageData`) or a `pin` (`ImportPinData`); a structure item there, or a content item in a
    batch without `thread_origin_key`, is `400 validation_failed` at its `item_kind`. A thread that is
    neither posted nor created by the batch is `409 conflict` with `reason` `thread_not_imported`; a
    message whose author item is not posted `409 conflict` with `reason` `author_not_imported`. Imported
    messages have `origin = import`, are silent, write no mentions and never trigger an agent; a posted
    thread only follows the mirror rules of 22 12.2 step 3. At its end a batch that changed something
    sets `threads.unread_floor_seq` to the end of the thread and appends one `thread.updated` and one
    `import.progress`, never `message.created` (04 15.8).

    Args:
        import_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        body (ImportBatch): One transaction. Objects of one thread, in source order; structure
            objects (space, channel, forum tag, person) come in batches without `thread_origin_key`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ImportBatchResult | Problem
    """

    return (
        await asyncio_detailed(
            import_id=import_id,
            client=client,
            body=body,
        )
    ).parsed
