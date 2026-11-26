from http import HTTPStatus
from typing import Any

import httpx

from ...client import AuthenticatedClient, Client
from ...models.computer_desktop_list import ComputerDesktopList
from ...models.problem import Problem
from ...types import Response


def _get_kwargs() -> dict[str, Any]:

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/computer/desktops",
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ComputerDesktopList | Problem:
    if response.status_code == 200:
        response_200 = ComputerDesktopList.from_dict(response.json())

        return response_200

    if response.status_code == 401:
        response_401 = Problem.from_dict(response.json())

        return response_401

    if response.status_code == 403:
        response_403 = Problem.from_dict(response.json())

        return response_403

    if response.status_code == 429:
        response_429 = Problem.from_dict(response.json())

        return response_429

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[ComputerDesktopList | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
) -> Response[ComputerDesktopList | Problem]:
    """List the desktops of the run's owner

     The desktops of the triggering owner of the bound run (docs/spec/27-computer-use.md 4 item 3 and 7):
    every host that is `active` and connected, whose last `report` has a connected helper that is
    enabled with every permission granted, whose reported generation is above the disabled generation,
    and whose assignee is absent or the owner. Only an agent bound by the header `Invocation-Token` to
    an active run under tool class `host` that its owner started with an own message (07 SR9, SR2,
    SR13); otherwise 403 `forbidden` with `reason: computer_not_owner_run` or `computer_tool_class`, or
    403 `invocation_token_invalid` (`run_inactive` for an ended run). Denials are audited.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ComputerDesktopList | Problem]
    """

    kwargs = _get_kwargs()

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
) -> ComputerDesktopList | Problem | None:
    """List the desktops of the run's owner

     The desktops of the triggering owner of the bound run (docs/spec/27-computer-use.md 4 item 3 and 7):
    every host that is `active` and connected, whose last `report` has a connected helper that is
    enabled with every permission granted, whose reported generation is above the disabled generation,
    and whose assignee is absent or the owner. Only an agent bound by the header `Invocation-Token` to
    an active run under tool class `host` that its owner started with an own message (07 SR9, SR2,
    SR13); otherwise 403 `forbidden` with `reason: computer_not_owner_run` or `computer_tool_class`, or
    403 `invocation_token_invalid` (`run_inactive` for an ended run). Denials are audited.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ComputerDesktopList | Problem
    """

    return sync_detailed(
        client=client,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
) -> Response[ComputerDesktopList | Problem]:
    """List the desktops of the run's owner

     The desktops of the triggering owner of the bound run (docs/spec/27-computer-use.md 4 item 3 and 7):
    every host that is `active` and connected, whose last `report` has a connected helper that is
    enabled with every permission granted, whose reported generation is above the disabled generation,
    and whose assignee is absent or the owner. Only an agent bound by the header `Invocation-Token` to
    an active run under tool class `host` that its owner started with an own message (07 SR9, SR2,
    SR13); otherwise 403 `forbidden` with `reason: computer_not_owner_run` or `computer_tool_class`, or
    403 `invocation_token_invalid` (`run_inactive` for an ended run). Denials are audited.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ComputerDesktopList | Problem]
    """

    kwargs = _get_kwargs()

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
) -> ComputerDesktopList | Problem | None:
    """List the desktops of the run's owner

     The desktops of the triggering owner of the bound run (docs/spec/27-computer-use.md 4 item 3 and 7):
    every host that is `active` and connected, whose last `report` has a connected helper that is
    enabled with every permission granted, whose reported generation is above the disabled generation,
    and whose assignee is absent or the owner. Only an agent bound by the header `Invocation-Token` to
    an active run under tool class `host` that its owner started with an own message (07 SR9, SR2,
    SR13); otherwise 403 `forbidden` with `reason: computer_not_owner_run` or `computer_tool_class`, or
    403 `invocation_token_invalid` (`run_inactive` for an ended run). Denials are audited.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ComputerDesktopList | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
        )
    ).parsed
