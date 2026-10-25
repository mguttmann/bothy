# SPDX-License-Identifier: Apache-2.0
"""Hand-layer helpers for upload sessions in mode ``direct`` (docs/spec/12-files.md 3.2).

The generated ``uploadPart`` sends a fixed ``Content-Type`` and cannot carry the headers the
server names in ``UploadPart.headers``; `upload_part` sends one part with exactly those headers
plus ``Content-Digest`` (RFC 9530) through `bothy_sdk.client.Client.call_as`, so the run
binding (07 SR9), the retry rules and the problem mapping of the client apply as for every
other call. The result is the generated ``UploadPartResult`` (ADR 0026: no own model).

Examples:
    >>> content_digest(b"abc")
    'sha-256=:ungWv48Bz+pBQUDeXa4iI7ADYaOWF3qctBD/YfIAFa0=:'
"""

from __future__ import annotations

import base64
import hashlib
from collections.abc import Mapping
from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

from bothy_sdk.client import IDEMPOTENCY_KEY_HEADER, INVOCATION_TOKEN_HEADER, Client
from bothy_sdk.generated.models import UploadPartResult
from bothy_sdk.generated.types import Response

__all__ = ["RESERVED_HEADERS", "content_digest", "upload_part"]

RESERVED_HEADERS = frozenset(
    {
        "authorization",
        "dpop",
        "host",
        "content-length",
        "content-digest",
        "transfer-encoding",
        IDEMPOTENCY_KEY_HEADER.lower(),
        INVOCATION_TOKEN_HEADER.lower(),
    }
)
"""Headers the client sets itself; a part header of the server never replaces them."""


def content_digest(data: bytes) -> str:
    """The ``Content-Digest`` of a part (RFC 9530, ``sha-256``).

    Args:
        data: The bytes of the part.

    Returns:
        ``sha-256=:<base64 of the SHA-256>:``.
    """
    digest = base64.b64encode(hashlib.sha256(data).digest()).decode("ascii")
    return f"sha-256=:{digest}:"


async def _put_part(
    *, client: Any, file_id: UUID, part_number: int, data: bytes, headers: Mapping[str, str]
) -> Response[UploadPartResult]:
    """The request of ``uploadPart``; the caller (`Client.call_detailed`) maps refusals."""
    url = f"/uploads/{quote(str(file_id), safe='')}/parts/{part_number}"
    response = await client.get_async_httpx_client().request(
        "put", url, content=data, headers=dict(headers)
    )
    parsed = (
        UploadPartResult.from_dict(response.json())
        if response.status_code == HTTPStatus.OK
        else None
    )
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=parsed,
    )


async def upload_part(
    client: Client,
    file_id: UUID,
    part_number: int,
    data: bytes,
    headers: Mapping[str, str] | None = None,
) -> UploadPartResult:
    """Send one part of an upload in mode ``direct`` (``uploadPart``, 12 3.2).

    Args:
        client: The client; inside `Client.bind_run` the part goes out bound to the run.
        file_id: The upload's file.
        part_number: The part number of ``UploadSession.parts``.
        data: The bytes of the part.
        headers: The headers of ``UploadPart.headers`` (``name`` to ``value``); names the client
            sets itself (`RESERVED_HEADERS`) are left out.

    Returns:
        The confirmed part with its ``etag``.

    Raises:
        bothy_sdk.errors.ProblemError: The server refused the part.
        httpx.TransportError: The network failed after the retries.
    """
    out = {k: v for k, v in (headers or {}).items() if k.lower() not in RESERVED_HEADERS}
    if not any(k.lower() == "content-type" for k in out):
        out["Content-Type"] = "application/octet-stream"
    out["Content-Digest"] = content_digest(data)
    return await client.call_as(
        UploadPartResult,
        _put_part,
        file_id=file_id,
        part_number=part_number,
        data=data,
        headers=out,
    )
