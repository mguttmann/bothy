from http import HTTPStatus
from typing import Any, cast

import httpx

from ...client import AuthenticatedClient, Client
from ...models.json_object import JsonObject
from ...models.problem import Problem
from ...types import Response


def _get_kwargs(
    *,
    body: list[JsonObject],
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/csp-reports",
    }

    _kwargs["json"] = []
    for componentsschemas_csp_report_list_item_data in body:
        componentsschemas_csp_report_list_item = (
            componentsschemas_csp_report_list_item_data.to_dict()
        )
        _kwargs["json"].append(componentsschemas_csp_report_list_item)

    headers["Content-Type"] = "application/reports+json"

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

    if response.status_code == 413:
        response_413 = Problem.from_dict(response.json())

        return response_413

    if response.status_code == 415:
        response_415 = Problem.from_dict(response.json())

        return response_415

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
    body: list[JsonObject],
) -> Response[Any | Problem]:
    """Accept a CSP violation report of the web console

     Endpoint of `report-to csp` and of `Reporting-Endpoints: csp="/api/v1/csp-reports"` on every console
    response (19-security 5 rule 6, R-AZ-33). Public, because browsers send reports without cookies in
    many cases; a console cookie is not checked by the CSRF guard here, the action changes nothing.
    Accepts the Reporting API format (`application/reports+json`, an array of reports) and the legacy
    `report-uri` format (`application/csp-report`), both limited by `http.max_json_body_bytes`. The
    server stores no report; it logs only the effective directive, the disposition and the origin of the
    blocked URL and counts `bothy.console.csp_violations`. Members it does not read are ignored.

    Args:
        body (list[JsonObject]): Reports of the Reporting API
            (https://www.w3.org/TR/reporting-1/); each item is a report object whose `body` is a CSP
            violation report (https://www.w3.org/TR/CSP3/#reporting). The server reads only `type`,
            `body.effectiveDirective`, `body.blockedURL` and `body.disposition`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
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
    body: list[JsonObject],
) -> Any | Problem | None:
    """Accept a CSP violation report of the web console

     Endpoint of `report-to csp` and of `Reporting-Endpoints: csp="/api/v1/csp-reports"` on every console
    response (19-security 5 rule 6, R-AZ-33). Public, because browsers send reports without cookies in
    many cases; a console cookie is not checked by the CSRF guard here, the action changes nothing.
    Accepts the Reporting API format (`application/reports+json`, an array of reports) and the legacy
    `report-uri` format (`application/csp-report`), both limited by `http.max_json_body_bytes`. The
    server stores no report; it logs only the effective directive, the disposition and the origin of the
    blocked URL and counts `bothy.console.csp_violations`. Members it does not read are ignored.

    Args:
        body (list[JsonObject]): Reports of the Reporting API
            (https://www.w3.org/TR/reporting-1/); each item is a report object whose `body` is a CSP
            violation report (https://www.w3.org/TR/CSP3/#reporting). The server reads only `type`,
            `body.effectiveDirective`, `body.blockedURL` and `body.disposition`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return sync_detailed(
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: list[JsonObject],
) -> Response[Any | Problem]:
    """Accept a CSP violation report of the web console

     Endpoint of `report-to csp` and of `Reporting-Endpoints: csp="/api/v1/csp-reports"` on every console
    response (19-security 5 rule 6, R-AZ-33). Public, because browsers send reports without cookies in
    many cases; a console cookie is not checked by the CSRF guard here, the action changes nothing.
    Accepts the Reporting API format (`application/reports+json`, an array of reports) and the legacy
    `report-uri` format (`application/csp-report`), both limited by `http.max_json_body_bytes`. The
    server stores no report; it logs only the effective directive, the disposition and the origin of the
    blocked URL and counts `bothy.console.csp_violations`. Members it does not read are ignored.

    Args:
        body (list[JsonObject]): Reports of the Reporting API
            (https://www.w3.org/TR/reporting-1/); each item is a report object whose `body` is a CSP
            violation report (https://www.w3.org/TR/CSP3/#reporting). The server reads only `type`,
            `body.effectiveDirective`, `body.blockedURL` and `body.disposition`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Problem]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: list[JsonObject],
) -> Any | Problem | None:
    """Accept a CSP violation report of the web console

     Endpoint of `report-to csp` and of `Reporting-Endpoints: csp="/api/v1/csp-reports"` on every console
    response (19-security 5 rule 6, R-AZ-33). Public, because browsers send reports without cookies in
    many cases; a console cookie is not checked by the CSRF guard here, the action changes nothing.
    Accepts the Reporting API format (`application/reports+json`, an array of reports) and the legacy
    `report-uri` format (`application/csp-report`), both limited by `http.max_json_body_bytes`. The
    server stores no report; it logs only the effective directive, the disposition and the origin of the
    blocked URL and counts `bothy.console.csp_violations`. Members it does not read are ignored.

    Args:
        body (list[JsonObject]): Reports of the Reporting API
            (https://www.w3.org/TR/reporting-1/); each item is a report object whose `body` is a CSP
            violation report (https://www.w3.org/TR/CSP3/#reporting). The server reads only `type`,
            `body.effectiveDirective`, `body.blockedURL` and `body.disposition`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Problem
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
        )
    ).parsed
