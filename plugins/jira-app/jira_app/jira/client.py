# SPDX-License-Identifier: Apache-2.0
"""The Jira REST client: two calls of REST version 3 (brief M2-50 Design note 15).

``POST <jira_base_url>/rest/api/3/issue`` creates an issue, ``POST
<jira_base_url>/rest/api/3/issue/<issue id>/comment`` a comment, both with ADF documents
(`jira_app.jira.adf`). One ``httpx.AsyncClient`` with ``follow_redirects=False`` (a ``3xx`` is a
failure) and the time limit ``sdk.http_timeout`` over the whole call. Headers ``Accept`` and
``Content-Type`` ``application/json``, ``Authorization: Basic`` with the standard base64 of the
credentials line (read at each use with the file check of `jira_app.config`) and
``User-Agent: jira-app/<VERSION>``. The response body is read only on ``201``; errors carry the
status only. Only Jira Cloud as cited in docs/spec/11-extensions.md 10.2.

A write to Jira is never repeated by this client: the caller marks the step ``attempted``
before it calls (Design note 9), because Jira's create calls are not idempotent.

Examples:
    >>> result = JiraResult("refused", status=400)
    >>> (result.kind, result.status)
    ('refused', 400)
"""

from __future__ import annotations

import asyncio
import base64
import json
import re
from collections.abc import Callable
from dataclasses import dataclass
from typing import Any, Literal

import httpx

from jira_app import APP_VERSION
from jira_app.config import ConfigError, JiraCredentials
from jira_app.jira.events import ISSUE_KEY

__all__ = ["JiraClient", "JiraResult"]

_CREATED = 201
_REDIRECT = 300
_SERVER_ERROR = 500
_DIGITS = re.compile(r"^[0-9]+$")


@dataclass(frozen=True)
class JiraResult:
    """The outcome of one Jira call.

    Attributes:
        kind: ``created`` (``201`` with a readable answer), ``refused`` (``3xx`` or ``4xx``:
            nothing was created), ``unknown`` (``5xx``, time-out, transport failure or an
            unreadable ``201``: the outcome is unknown) or ``not_sent`` (the credentials file
            failed its check, nothing was sent).
        status: The HTTP status when there was one, else 0.
        object_id: The ``id`` of the created issue or comment.
        key: The ``key`` of a created issue.
        reason: ``timeout``, ``transport``, ``unreadable`` or ``credentials_file``.
    """

    kind: Literal["created", "refused", "unknown", "not_sent"]
    status: int = 0
    object_id: str = ""
    key: str = ""
    reason: str = ""

    def to_json(self) -> dict[str, Any]:
        """Return the result as a JSON object for the job store.

        Returns:
            The members.
        """
        return {
            "kind": self.kind,
            "status": self.status,
            "object_id": self.object_id,
            "key": self.key,
            "reason": self.reason,
        }

    @classmethod
    def from_json(cls, data: dict[str, Any]) -> JiraResult:
        """Rebuild a stored result.

        Args:
            data: The stored object.

        Returns:
            The result.
        """
        return cls(**data)


class JiraClient:
    """Creates issues and comments at the configured Jira site.

    Args:
        base_url: ``jira_app.jira_base_url`` without a trailing slash.
        credentials: Reads the credentials at each use; raises `ConfigError` when the file
            fails its check.
        timeout: ``sdk.http_timeout`` in seconds, the limit of each call.
        transport: An ``httpx`` transport (tests); default the network transport.
    """

    def __init__(
        self,
        base_url: str,
        credentials: Callable[[], JiraCredentials],
        timeout: float,
        transport: httpx.AsyncBaseTransport | None = None,
    ) -> None:
        """Create the client; see the class docstring."""
        self._base = base_url.rstrip("/")
        self._credentials = credentials
        self._timeout = timeout
        self._http = httpx.AsyncClient(
            follow_redirects=False,
            timeout=timeout,
            transport=transport,
            headers={"User-Agent": f"jira-app/{APP_VERSION}"},
        )

    async def aclose(self) -> None:
        """Close the connections."""
        await self._http.aclose()

    async def create_issue(
        self, project: str, issue_type: str, summary: str, description: dict[str, Any]
    ) -> JiraResult:
        """Create an issue.

        Args:
            project: The project key.
            issue_type: The name of the issue type.
            summary: The summary.
            description: The description as an ADF document.

        Returns:
            The result; ``created`` carries ``id`` and ``key``.
        """
        body = {
            "fields": {
                "project": {"key": project},
                "issuetype": {"name": issue_type},
                "summary": summary,
                "description": description,
            }
        }
        return await self._post("/rest/api/3/issue", body, issue=True)

    async def create_comment(self, issue_id: str, body: dict[str, Any]) -> JiraResult:
        """Add a comment to an issue.

        Args:
            issue_id: The issue ID (decimal digits).
            body: The comment as an ADF document.

        Returns:
            The result; ``created`` carries the comment ``id``.
        """
        return await self._post(f"/rest/api/3/issue/{issue_id}/comment", {"body": body})

    async def _post(self, path: str, body: dict[str, Any], *, issue: bool = False) -> JiraResult:
        try:
            line = self._credentials().line
        except ConfigError:
            return JiraResult("not_sent", reason="credentials_file")
        token = base64.b64encode(line.encode("utf-8")).decode("ascii")
        request = self._http.build_request(
            "POST",
            self._base + path,
            content=json.dumps(body, ensure_ascii=False).encode("utf-8"),
            headers={
                "Accept": "application/json",
                "Content-Type": "application/json",
                "Authorization": f"Basic {token}",
            },
        )
        try:
            async with asyncio.timeout(self._timeout):
                response = await self._http.send(request, stream=True)
                try:
                    if response.status_code != _CREATED:
                        return _failure(response.status_code)
                    raw = await response.aread()
                finally:
                    await response.aclose()
        except TimeoutError:
            return JiraResult("unknown", reason="timeout")
        except httpx.TimeoutException:
            return JiraResult("unknown", reason="timeout")
        except httpx.HTTPError:
            return JiraResult("unknown", reason="transport")
        return _created(raw, issue=issue)


def _failure(status: int) -> JiraResult:
    if status < _SERVER_ERROR and status >= _REDIRECT:
        return JiraResult("refused", status=status)
    return JiraResult("unknown", status=status)


def _created(raw: bytes, *, issue: bool) -> JiraResult:
    try:
        data = json.loads(raw)
    except ValueError:
        return JiraResult("unknown", status=_CREATED, reason="unreadable")
    if not isinstance(data, dict):
        return JiraResult("unknown", status=_CREATED, reason="unreadable")
    object_id = data.get("id")
    if isinstance(object_id, int) and not isinstance(object_id, bool):
        object_id = str(object_id)
    if not isinstance(object_id, str) or _DIGITS.fullmatch(object_id) is None:
        return JiraResult("unknown", status=_CREATED, reason="unreadable")
    if not issue:
        return JiraResult("created", status=_CREATED, object_id=object_id)
    key = data.get("key")
    if not isinstance(key, str) or ISSUE_KEY.fullmatch(key) is None:
        return JiraResult("unknown", status=_CREATED, reason="unreadable")
    return JiraResult("created", status=_CREATED, object_id=object_id, key=key)
