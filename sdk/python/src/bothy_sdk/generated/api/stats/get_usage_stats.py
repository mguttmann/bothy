import datetime
from http import HTTPStatus
from typing import Any
from uuid import UUID

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.stats_granularity import StatsGranularity
from ...models.stats_series import StatsSeries
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    from_: datetime.datetime,
    to: datetime.datetime,
    granularity: StatsGranularity | Unset = UNSET,
    space_id: UUID | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    json_from_ = from_.isoformat()
    params["from"] = json_from_

    json_to = to.isoformat()
    params["to"] = json_to

    json_granularity: str | Unset = UNSET
    if not isinstance(granularity, Unset):
        json_granularity = granularity.value

    params["granularity"] = json_granularity

    json_space_id: str | Unset = UNSET
    if not isinstance(space_id, Unset):
        json_space_id = str(space_id)
    params["space_id"] = json_space_id

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/stats/usage",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | StatsSeries:
    if response.status_code == 200:
        response_200 = StatsSeries.from_dict(response.json())

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
) -> Response[Problem | StatsSeries]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    from_: datetime.datetime,
    to: datetime.datetime,
    granularity: StatsGranularity | Unset = UNSET,
    space_id: UUID | Unset = UNSET,
) -> Response[Problem | StatsSeries]:
    """Usage per space

     Metrics per space and bucket: `messages` (messages not deleted) and by origin `messages_human`,
    `messages_agent`, `messages_app`, `messages_import`; `threads_created`, `threads_done` (threads
    closed in the bucket); `runs` (runs that ended in the bucket); `active_people` (distinct people with
    at least one message, only for granularity `week` or `month`; below `privacy.stats_min_group_size`
    the value is withheld and named in `below_min_group`); `storage_bytes` (current sum of the files of
    the space, in the last bucket only). Grouped by space (`group_key` is the space ID). Authorized with
    `stats.read` on the instance (owner) or, with `space_id`, on that space (`manage`); without
    `space_id` and without owner status `403`, an unreadable space `404`. Every number of a channel
    counts only the channels the caller may read (`message.read`), intersected with the space; channels
    without a space (DMs) are not counted. No metric is grouped by person and no person is named. `from`
    and `to` are aligned down and up to the granularity (default `day`): `hour` in UTC, `day`, `week`
    (ISO, Monday) and `month` from local midnight of `stats.time_zone`, returned as UTC instants; a
    range longer than `stats.max_range`, with more than `api.stats_max_buckets` buckets or with `from >=
    to` is `400 validation_failed`. Buckets without a value are left out. The result is cached per
    caller scope for `stats.cache_ttl`.

    Args:
        from_ (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        to (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        granularity (StatsGranularity | Unset): Size of a statistics bucket.
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | StatsSeries]
    """

    kwargs = _get_kwargs(
        from_=from_,
        to=to,
        granularity=granularity,
        space_id=space_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    from_: datetime.datetime,
    to: datetime.datetime,
    granularity: StatsGranularity | Unset = UNSET,
    space_id: UUID | Unset = UNSET,
) -> Problem | StatsSeries | None:
    """Usage per space

     Metrics per space and bucket: `messages` (messages not deleted) and by origin `messages_human`,
    `messages_agent`, `messages_app`, `messages_import`; `threads_created`, `threads_done` (threads
    closed in the bucket); `runs` (runs that ended in the bucket); `active_people` (distinct people with
    at least one message, only for granularity `week` or `month`; below `privacy.stats_min_group_size`
    the value is withheld and named in `below_min_group`); `storage_bytes` (current sum of the files of
    the space, in the last bucket only). Grouped by space (`group_key` is the space ID). Authorized with
    `stats.read` on the instance (owner) or, with `space_id`, on that space (`manage`); without
    `space_id` and without owner status `403`, an unreadable space `404`. Every number of a channel
    counts only the channels the caller may read (`message.read`), intersected with the space; channels
    without a space (DMs) are not counted. No metric is grouped by person and no person is named. `from`
    and `to` are aligned down and up to the granularity (default `day`): `hour` in UTC, `day`, `week`
    (ISO, Monday) and `month` from local midnight of `stats.time_zone`, returned as UTC instants; a
    range longer than `stats.max_range`, with more than `api.stats_max_buckets` buckets or with `from >=
    to` is `400 validation_failed`. Buckets without a value are left out. The result is cached per
    caller scope for `stats.cache_ttl`.

    Args:
        from_ (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        to (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        granularity (StatsGranularity | Unset): Size of a statistics bucket.
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | StatsSeries
    """

    return sync_detailed(
        client=client,
        from_=from_,
        to=to,
        granularity=granularity,
        space_id=space_id,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    from_: datetime.datetime,
    to: datetime.datetime,
    granularity: StatsGranularity | Unset = UNSET,
    space_id: UUID | Unset = UNSET,
) -> Response[Problem | StatsSeries]:
    """Usage per space

     Metrics per space and bucket: `messages` (messages not deleted) and by origin `messages_human`,
    `messages_agent`, `messages_app`, `messages_import`; `threads_created`, `threads_done` (threads
    closed in the bucket); `runs` (runs that ended in the bucket); `active_people` (distinct people with
    at least one message, only for granularity `week` or `month`; below `privacy.stats_min_group_size`
    the value is withheld and named in `below_min_group`); `storage_bytes` (current sum of the files of
    the space, in the last bucket only). Grouped by space (`group_key` is the space ID). Authorized with
    `stats.read` on the instance (owner) or, with `space_id`, on that space (`manage`); without
    `space_id` and without owner status `403`, an unreadable space `404`. Every number of a channel
    counts only the channels the caller may read (`message.read`), intersected with the space; channels
    without a space (DMs) are not counted. No metric is grouped by person and no person is named. `from`
    and `to` are aligned down and up to the granularity (default `day`): `hour` in UTC, `day`, `week`
    (ISO, Monday) and `month` from local midnight of `stats.time_zone`, returned as UTC instants; a
    range longer than `stats.max_range`, with more than `api.stats_max_buckets` buckets or with `from >=
    to` is `400 validation_failed`. Buckets without a value are left out. The result is cached per
    caller scope for `stats.cache_ttl`.

    Args:
        from_ (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        to (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        granularity (StatsGranularity | Unset): Size of a statistics bucket.
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | StatsSeries]
    """

    kwargs = _get_kwargs(
        from_=from_,
        to=to,
        granularity=granularity,
        space_id=space_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    from_: datetime.datetime,
    to: datetime.datetime,
    granularity: StatsGranularity | Unset = UNSET,
    space_id: UUID | Unset = UNSET,
) -> Problem | StatsSeries | None:
    """Usage per space

     Metrics per space and bucket: `messages` (messages not deleted) and by origin `messages_human`,
    `messages_agent`, `messages_app`, `messages_import`; `threads_created`, `threads_done` (threads
    closed in the bucket); `runs` (runs that ended in the bucket); `active_people` (distinct people with
    at least one message, only for granularity `week` or `month`; below `privacy.stats_min_group_size`
    the value is withheld and named in `below_min_group`); `storage_bytes` (current sum of the files of
    the space, in the last bucket only). Grouped by space (`group_key` is the space ID). Authorized with
    `stats.read` on the instance (owner) or, with `space_id`, on that space (`manage`); without
    `space_id` and without owner status `403`, an unreadable space `404`. Every number of a channel
    counts only the channels the caller may read (`message.read`), intersected with the space; channels
    without a space (DMs) are not counted. No metric is grouped by person and no person is named. `from`
    and `to` are aligned down and up to the granularity (default `day`): `hour` in UTC, `day`, `week`
    (ISO, Monday) and `month` from local midnight of `stats.time_zone`, returned as UTC instants; a
    range longer than `stats.max_range`, with more than `api.stats_max_buckets` buckets or with `from >=
    to` is `400 validation_failed`. Buckets without a value are left out. The result is cached per
    caller scope for `stats.cache_ttl`.

    Args:
        from_ (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        to (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        granularity (StatsGranularity | Unset): Size of a statistics bucket.
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | StatsSeries
    """

    return (
        await asyncio_detailed(
            client=client,
            from_=from_,
            to=to,
            granularity=granularity,
            space_id=space_id,
        )
    ).parsed
