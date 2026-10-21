# SPDX-License-Identifier: Apache-2.0
"""HTTP to the credential broker of the Go test and raw calls to the server under test.

The broker (``services/server/test/contract/broker.go``) answers ``POST /principals`` with fresh
credentials and ``POST /step-up`` with a fresh step-up of a session. Both it and the server listen
on 127.0.0.1 only.
"""

from __future__ import annotations

import http.client
import json
from collections.abc import Mapping
from dataclasses import dataclass
from typing import Any
from urllib.parse import urlsplit

_TIMEOUT = 60.0


@dataclass(frozen=True)
class RawResponse:
    """Status, headers (as sent, last value per name) and body of one call."""

    status: int
    headers: dict[str, str]
    body: bytes


def http_request(
    method: str, url: str, headers: Mapping[str, str] | None = None, body: bytes | None = None
) -> RawResponse:
    """Send one request to a plain-HTTP loopback ``url`` and return the answer.

    Raises:
        ValueError: ``url`` is not ``http://`` (the broker and the test server are plain HTTP on
            loopback).
        OSError: the connection fails.
    """
    parts = urlsplit(url)
    if parts.scheme != "http" or not parts.hostname:
        raise ValueError(f"not a plain HTTP URL: {url}")
    conn = http.client.HTTPConnection(parts.hostname, parts.port or 80, timeout=_TIMEOUT)
    try:
        target = parts.path + ("?" + parts.query if parts.query else "")
        conn.request(method, target or "/", body=body, headers=dict(headers or {}))
        resp = conn.getresponse()
        return RawResponse(resp.status, dict(resp.getheaders()), resp.read())
    finally:
        conn.close()


class BrokerError(RuntimeError):
    """The broker refused a request."""


@dataclass(frozen=True)
class Credentials:
    """A fresh session handed out by the broker."""

    principal_id: str
    device_id: str
    session_id: str
    access_token: str
    refresh_token: str
    dpop_private_jwk: dict[str, str]


class BrokerClient:
    """Client of the credential broker at ``base_url``."""

    def __init__(self, base_url: str) -> None:
        """Remember the origin of the broker."""
        self.base_url = base_url.rstrip("/")

    def _post(self, path: str, payload: Mapping[str, Any]) -> dict[str, Any]:
        resp = http_request(
            "POST",
            self.base_url + path,
            {"Content-Type": "application/json"},
            json.dumps(payload).encode(),
        )
        if resp.status != 200:
            raise BrokerError(
                f"broker {path}: {resp.status} {resp.body.decode('utf-8', 'replace')}"
            )
        data = json.loads(resp.body)
        if not isinstance(data, dict):
            raise BrokerError(f"broker {path}: not a JSON object")
        return data

    def principal(self, kind: str) -> Credentials:
        """Return fresh credentials of ``kind`` (``owner`` or ``user``)."""
        d = self._post("/principals", {"kind": kind})
        return Credentials(
            principal_id=str(d["principal_id"]),
            device_id=str(d["device_id"]),
            session_id=str(d["session_id"]),
            access_token=str(d["access_token"]),
            refresh_token=str(d["refresh_token"]),
            dpop_private_jwk={str(k): str(v) for k, v in d["dpop_private_jwk"].items()},
        )

    def step_up(self, session_id: str) -> None:
        """Perform a fresh step-up for ``session_id``."""
        self._post("/step-up", {"session_id": session_id})
