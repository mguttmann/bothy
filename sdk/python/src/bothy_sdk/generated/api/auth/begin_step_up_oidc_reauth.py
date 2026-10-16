from http import HTTPStatus
from typing import Any

import httpx

from ...client import AuthenticatedClient, Client
from ...models.begin_step_up_oidc_reauth_x_console_request import (
    BeginStepUpOidcReauthXConsoleRequest,
)
from ...models.oidc_begin_response import OidcBeginResponse
from ...models.problem import Problem
from ...models.step_up_oidc_begin_request import StepUpOidcBeginRequest
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    body: StepUpOidcBeginRequest,
    x_console_request: BeginStepUpOidcReauthXConsoleRequest | Unset = UNSET,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    if not isinstance(x_console_request, Unset):
        headers["X-Console-Request"] = str(x_console_request)

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/auth/step-up/oidc",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> OidcBeginResponse | Problem:
    if response.status_code == 200:
        response_200 = OidcBeginResponse.from_dict(response.json())

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
) -> Response[OidcBeginResponse | Problem]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: StepUpOidcBeginRequest,
    x_console_request: BeginStepUpOidcReauthXConsoleRequest | Unset = UNSET,
) -> Response[OidcBeginResponse | Problem]:
    """Start a console step-up with OIDC

    Args:
        x_console_request (BeginStepUpOidcReauthXConsoleRequest | Unset):
        body (StepUpOidcBeginRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[OidcBeginResponse | Problem]
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
    body: StepUpOidcBeginRequest,
    x_console_request: BeginStepUpOidcReauthXConsoleRequest | Unset = UNSET,
) -> OidcBeginResponse | Problem | None:
    """Start a console step-up with OIDC

    Args:
        x_console_request (BeginStepUpOidcReauthXConsoleRequest | Unset):
        body (StepUpOidcBeginRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        OidcBeginResponse | Problem
    """

    return sync_detailed(
        client=client,
        body=body,
        x_console_request=x_console_request,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: StepUpOidcBeginRequest,
    x_console_request: BeginStepUpOidcReauthXConsoleRequest | Unset = UNSET,
) -> Response[OidcBeginResponse | Problem]:
    """Start a console step-up with OIDC

    Args:
        x_console_request (BeginStepUpOidcReauthXConsoleRequest | Unset):
        body (StepUpOidcBeginRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[OidcBeginResponse | Problem]
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
    body: StepUpOidcBeginRequest,
    x_console_request: BeginStepUpOidcReauthXConsoleRequest | Unset = UNSET,
) -> OidcBeginResponse | Problem | None:
    """Start a console step-up with OIDC

    Args:
        x_console_request (BeginStepUpOidcReauthXConsoleRequest | Unset):
        body (StepUpOidcBeginRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        OidcBeginResponse | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
            x_console_request=x_console_request,
        )
    ).parsed
