from http import HTTPStatus
from typing import Any

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...models.step_up_password_request import StepUpPasswordRequest
from ...models.step_up_result import StepUpResult
from ...models.step_up_with_password_x_console_request import (
    StepUpWithPasswordXConsoleRequest,
)
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    body: StepUpPasswordRequest,
    x_console_request: StepUpWithPasswordXConsoleRequest | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_console_request, Unset):
        headers["X-Console-Request"] = str(x_console_request)

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/auth/step-up/password",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Problem | StepUpResult:
    if response.status_code == 200:
        response_200 = StepUpResult.from_dict(response.json())

        return response_200

    if response.status_code == 401:
        response_401 = Problem.from_dict(response.json())

        return response_401

    if response.status_code == 403:
        response_403 = Problem.from_dict(response.json())

        return response_403

    response_default = Problem.from_dict(response.json())

    return response_default


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Problem | StepUpResult]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: StepUpPasswordRequest,
    x_console_request: StepUpWithPasswordXConsoleRequest | Unset = UNSET,
) -> Response[Problem | StepUpResult]:
    """Console step-up with password and TOTP

    Args:
        x_console_request (StepUpWithPasswordXConsoleRequest | Unset):
        body (StepUpPasswordRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | StepUpResult]
    """

    kwargs = _get_kwargs(
        body=body,
        x_console_request=x_console_request,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    body: StepUpPasswordRequest,
    x_console_request: StepUpWithPasswordXConsoleRequest | Unset = UNSET,
) -> Problem | StepUpResult | None:
    """Console step-up with password and TOTP

    Args:
        x_console_request (StepUpWithPasswordXConsoleRequest | Unset):
        body (StepUpPasswordRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | StepUpResult
    """

    return sync_detailed(
        client=client,
        body=body,
        x_console_request=x_console_request,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: StepUpPasswordRequest,
    x_console_request: StepUpWithPasswordXConsoleRequest | Unset = UNSET,
) -> Response[Problem | StepUpResult]:
    """Console step-up with password and TOTP

    Args:
        x_console_request (StepUpWithPasswordXConsoleRequest | Unset):
        body (StepUpPasswordRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Problem | StepUpResult]
    """

    kwargs = _get_kwargs(
        body=body,
        x_console_request=x_console_request,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: StepUpPasswordRequest,
    x_console_request: StepUpWithPasswordXConsoleRequest | Unset = UNSET,
) -> Problem | StepUpResult | None:
    """Console step-up with password and TOTP

    Args:
        x_console_request (StepUpWithPasswordXConsoleRequest | Unset):
        body (StepUpPasswordRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Problem | StepUpResult
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
            x_console_request=x_console_request,
        )
    ).parsed
