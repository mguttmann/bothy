from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...models.instance_setting import InstanceSetting
from ...models.instance_setting_put import InstanceSettingPut
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    key: str,
    *,
    body: InstanceSettingPut,
    if_match: str | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(if_match, Unset):
        headers["If-Match"] = if_match

    _kwargs: dict[str, Any] = {
        "method": "put",
        "url": "/instance/settings/{key}".format(
            key=quote(str(key), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> InstanceSetting | Problem:
    if response.status_code == 200:
        response_200 = InstanceSetting.from_dict(response.json())

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

    if response.status_code == 412:
        response_412 = Problem.from_dict(response.json())

        return response_412

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[InstanceSetting | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    key: str,
    *,
    client: AuthenticatedClient | Client,
    body: InstanceSettingPut,
    if_match: str | Unset = UNSET,
) -> Response[InstanceSetting | Problem]:
    """Set an instance setting

    Args:
        key (str):
        if_match (str | Unset):
        body (InstanceSettingPut):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[InstanceSetting | Problem]
    """

    kwargs = _get_kwargs(
        key=key,
        body=body,
        if_match=if_match,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    key: str,
    *,
    client: AuthenticatedClient | Client,
    body: InstanceSettingPut,
    if_match: str | Unset = UNSET,
) -> InstanceSetting | Problem | None:
    """Set an instance setting

    Args:
        key (str):
        if_match (str | Unset):
        body (InstanceSettingPut):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        InstanceSetting | Problem
    """

    return sync_detailed(
        key=key,
        client=client,
        body=body,
        if_match=if_match,
    ).parsed


async def asyncio_detailed(
    key: str,
    *,
    client: AuthenticatedClient | Client,
    body: InstanceSettingPut,
    if_match: str | Unset = UNSET,
) -> Response[InstanceSetting | Problem]:
    """Set an instance setting

    Args:
        key (str):
        if_match (str | Unset):
        body (InstanceSettingPut):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[InstanceSetting | Problem]
    """

    kwargs = _get_kwargs(
        key=key,
        body=body,
        if_match=if_match,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    key: str,
    *,
    client: AuthenticatedClient | Client,
    body: InstanceSettingPut,
    if_match: str | Unset = UNSET,
) -> InstanceSetting | Problem | None:
    """Set an instance setting

    Args:
        key (str):
        if_match (str | Unset):
        body (InstanceSettingPut):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        InstanceSetting | Problem
    """

    return (
        await asyncio_detailed(
            key=key,
            client=client,
            body=body,
            if_match=if_match,
        )
    ).parsed
