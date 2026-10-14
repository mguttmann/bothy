from http import HTTPStatus
from typing import Any, cast

import httpx

from ...client import AuthenticatedClient, Client
from ...models.problem import Problem
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    state: str,
    iss: str | Unset = UNSET,
    code: str | Unset = UNSET,
    error: str | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["state"] = state

    params["iss"] = iss

    params["code"] = code

    params["error"] = error

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/auth/oidc/callback",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Any | Problem:
    if response.status_code == 302:
        response_302 = cast(Any, None)
        return response_302

    if response.status_code == 400:
        response_400 = Problem.from_dict(response.json())

        return response_400

    if response.status_code == 401:
        response_401 = Problem.from_dict(response.json())

        return response_401

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
    *,
    client: AuthenticatedClient | Client,
    state: str,
    iss: str | Unset = UNSET,
    code: str | Unset = UNSET,
    error: str | Unset = UNSET,
) -> Response[Any | Problem]:
    """OIDC redirect target of the identity provider

     Exchanges the code at the provider and verifies the ID token. It stores only the verified external
    identity in the login flow; mapping, linking and provisioning happen at `completeOidcLogin` (08,
    2.2). Then it redirects to `app_redirect_uri` with the query parameters `login_code` (one-time,
    valid `auth.server_code_ttl`) and `login_attempt_id`. After the flow was found, every failure
    redirects with `error` and `login_attempt_id`; `error` is one of `access_denied` (the provider sent
    an error), `login_failed` (a check failed) and `flow_expired`. An unknown or expired `state` gets
    `400` without a redirect.

    Args:
        state (str):
        iss (str | Unset):
        code (str | Unset):
        error (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        state=state,
        iss=iss,
        code=code,
        error=error,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    state: str,
    iss: str | Unset = UNSET,
    code: str | Unset = UNSET,
    error: str | Unset = UNSET,
) -> Any | Problem | None:
    """OIDC redirect target of the identity provider

     Exchanges the code at the provider and verifies the ID token. It stores only the verified external
    identity in the login flow; mapping, linking and provisioning happen at `completeOidcLogin` (08,
    2.2). Then it redirects to `app_redirect_uri` with the query parameters `login_code` (one-time,
    valid `auth.server_code_ttl`) and `login_attempt_id`. After the flow was found, every failure
    redirects with `error` and `login_attempt_id`; `error` is one of `access_denied` (the provider sent
    an error), `login_failed` (a check failed) and `flow_expired`. An unknown or expired `state` gets
    `400` without a redirect.

    Args:
        state (str):
        iss (str | Unset):
        code (str | Unset):
        error (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return sync_detailed(
        client=client,
        state=state,
        iss=iss,
        code=code,
        error=error,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    state: str,
    iss: str | Unset = UNSET,
    code: str | Unset = UNSET,
    error: str | Unset = UNSET,
) -> Response[Any | Problem]:
    """OIDC redirect target of the identity provider

     Exchanges the code at the provider and verifies the ID token. It stores only the verified external
    identity in the login flow; mapping, linking and provisioning happen at `completeOidcLogin` (08,
    2.2). Then it redirects to `app_redirect_uri` with the query parameters `login_code` (one-time,
    valid `auth.server_code_ttl`) and `login_attempt_id`. After the flow was found, every failure
    redirects with `error` and `login_attempt_id`; `error` is one of `access_denied` (the provider sent
    an error), `login_failed` (a check failed) and `flow_expired`. An unknown or expired `state` gets
    `400` without a redirect.

    Args:
        state (str):
        iss (str | Unset):
        code (str | Unset):
        error (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        state=state,
        iss=iss,
        code=code,
        error=error,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    state: str,
    iss: str | Unset = UNSET,
    code: str | Unset = UNSET,
    error: str | Unset = UNSET,
) -> Any | Problem | None:
    """OIDC redirect target of the identity provider

     Exchanges the code at the provider and verifies the ID token. It stores only the verified external
    identity in the login flow; mapping, linking and provisioning happen at `completeOidcLogin` (08,
    2.2). Then it redirects to `app_redirect_uri` with the query parameters `login_code` (one-time,
    valid `auth.server_code_ttl`) and `login_attempt_id`. After the flow was found, every failure
    redirects with `error` and `login_attempt_id`; `error` is one of `access_denied` (the provider sent
    an error), `login_failed` (a check failed) and `flow_expired`. An unknown or expired `state` gets
    `400` without a redirect.

    Args:
        state (str):
        iss (str | Unset):
        code (str | Unset):
        error (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            state=state,
            iss=iss,
            code=code,
            error=error,
        )
    ).parsed
