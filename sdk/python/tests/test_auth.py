# SPDX-License-Identifier: Apache-2.0
"""Credentials: secrets that never render, DPoP proofs, nonces, key files and clear text."""

from __future__ import annotations

import base64
import hashlib
import json
import logging
import pickle
from pathlib import Path
from typing import Any

import httpx
import jwt
import pytest

from bothy_sdk.auth import (
    Credentials,
    FileDpopKey,
    Secret,
    dpop_proof,
    gateway_htu,
    jwk_thumbprint,
    normalize_htu,
    require_safe_url,
)
from bothy_sdk.client import Client
from bothy_sdk.errors import UseDpopNonce
from bothy_sdk.generated.api.discovery import get_discovery
from bothy_sdk.testing.doubles import FakeClock

CANARY = "canary-token-0123456789"
DISCOVERY = json.loads((Path(__file__).parent / "data" / "discovery.json").read_text())


def _decode(proof: str) -> tuple[dict[str, Any], dict[str, Any]]:
    header = jwt.get_unverified_header(proof)
    key = jwt.PyJWK.from_dict(header["jwk"], algorithm="ES256")
    claims = jwt.decode(proof, key=key.key, algorithms=["ES256"])
    return header, claims


@pytest.fixture
def key(tmp_path: Path) -> FileDpopKey:
    return FileDpopKey.generate(tmp_path / "dpop.pem")


def test_dpop_proof_claims(key: FileDpopKey) -> None:
    clock = FakeClock()
    proof = dpop_proof(
        key,
        htm="post",
        htu="HTTPS://Example.COM:443/api/v1/runs?x=1#frag",
        token=Secret(CANARY),
        nonce="n-1",
        clock=clock,
    )
    header, claims = _decode(proof)
    assert header["typ"] == "dpop+jwt"
    assert header["alg"] == "ES256"
    assert set(header["jwk"]) == {"kty", "crv", "x", "y"}
    assert claims["htm"] == "POST"
    assert claims["htu"] == "https://example.com/api/v1/runs"
    assert claims["iat"] == int(clock.now().timestamp())
    assert claims["nonce"] == "n-1"
    expected_ath = base64.urlsafe_b64encode(hashlib.sha256(CANARY.encode()).digest()).rstrip(b"=")
    assert claims["ath"] == expected_ath.decode()
    assert (
        claims["jti"]
        != _decode(
            dpop_proof(
                key, htm="GET", htu="https://example.com/", token=None, nonce=None, clock=clock
            )
        )[1]["jti"]
    )


def _problem(status: int, code: str, **headers: str) -> httpx.Response:
    body = {
        "type": "about:blank",
        "title": code,
        "status": status,
        "code": code,
        "correlation_id": "0192f4a0-7b1c-7000-8a00-000000000009",
    }
    return httpx.Response(status, json=body, headers=headers)


@pytest.mark.anyio
async def test_use_dpop_nonce_retried_once(key: FileDpopKey) -> None:
    seen: list[httpx.Headers] = []

    def handler(request: httpx.Request) -> httpx.Response:
        # The auth flow repeats the same request object, so keep a copy of its headers.
        seen.append(httpx.Headers(request.headers))
        if len(seen) == 1:
            return _problem(401, "use_dpop_nonce", **{"DPoP-Nonce": "n-1"})
        return httpx.Response(200, json=DISCOVERY)

    creds = Credentials(Secret(CANARY), key)
    async with Client(
        "https://example.com/api/v1", creds, transport=httpx.MockTransport(handler)
    ) as client:
        await client.discover()
    assert len(seen) == 2
    assert "nonce" not in _decode(seen[0]["DPoP"])[1]
    assert _decode(seen[1]["DPoP"])[1]["nonce"] == "n-1"
    assert seen[1]["Authorization"].startswith("DPoP ")

    seen.clear()

    def always(request: httpx.Request) -> httpx.Response:
        seen.append(httpx.Headers(request.headers))
        return _problem(401, "use_dpop_nonce", **{"DPoP-Nonce": f"n-{len(seen) + 1}"})

    async with Client(
        "https://example.com/api/v1", creds, transport=httpx.MockTransport(always)
    ) as client:
        with pytest.raises(UseDpopNonce):
            await client.call(get_discovery.asyncio_detailed)
    assert len(seen) == 2


@pytest.mark.anyio
async def test_newest_nonce_used(key: FileDpopKey) -> None:
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json=DISCOVERY, headers={"DPoP-Nonce": f"n-{len(seen)}"})

    async with Client(
        "https://example.com/api/v1",
        Credentials(Secret(CANARY), key),
        transport=httpx.MockTransport(handler),
    ) as client:
        for _ in range(3):
            await client.discover()
    nonces = [_decode(r.headers["DPoP"])[1].get("nonce") for r in seen]
    assert nonces == [None, "n-1", "n-2"]


def test_gateway_proof_htu_and_nonce(key: FileDpopKey) -> None:
    assert (
        gateway_htu("wss://Example.com:443/api/v1/gateway?v=1")
        == "https://example.com/api/v1/gateway"
    )
    # The HTTPS form of GatewayIdentify.dpop, also for a clear-text loopback gateway.
    assert gateway_htu("ws://127.0.0.1:8099/api/v1/gateway") == (
        "https://127.0.0.1:8099/api/v1/gateway"
    )
    assert gateway_htu("ws://example.com:80/api/v1/gateway") == "https://example.com/api/v1/gateway"
    proof = dpop_proof(
        key,
        htm="GET",
        htu=gateway_htu("wss://example.com/api/v1/gateway"),
        token=Secret(CANARY),
        nonce="hello-nonce",
        clock=FakeClock(),
    )
    claims = _decode(proof)[1]
    assert (claims["htm"], claims["htu"], claims["nonce"]) == (
        "GET",
        "https://example.com/api/v1/gateway",
        "hello-nonce",
    )
    assert normalize_htu("http://[::1]:80/x") == "http://[::1]/x"


def test_key_file_mode_and_thumbprint(tmp_path: Path, key: FileDpopKey) -> None:
    # RFC 7638 section 3.1 example.
    rfc = {  # RFC 7638 section 3.1
        "kty": "RSA",
        "e": "AQAB",
        "n": (
            "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1"
            "L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4"
            "QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbO"
            "pbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csF"
            "Cur-kEgU8awapJzKnqDKgw"
        ),
        "alg": "RS256",
        "kid": "2011-04-29",
    }
    assert jwk_thumbprint(rfc) == "NzbLsXh8uDCcd-6MNwXF4W_7noWXFZAfHkxZsRGC9Xs"
    path = tmp_path / "dpop.pem"
    assert path.stat().st_mode & 0o777 == 0o600
    with pytest.raises(FileExistsError):
        FileDpopKey.generate(path)
    assert FileDpopKey.load(path).jkt == key.jkt == jwk_thumbprint(key.public_jwk())
    path.chmod(0o644)
    with pytest.raises(PermissionError):
        FileDpopKey.load(path)
    assert "PRIVATE" not in repr(key)


def test_secret_never_rendered(tmp_path: Path, caplog: pytest.LogCaptureFixture) -> None:
    secret = Secret(CANARY)
    rendered = [
        repr(secret),
        str(secret),
        f"{secret}",
        f"{secret!r}",
        f"{secret:>40}",
    ]
    assert all(CANARY not in r for r in rendered)
    with caplog.at_level(logging.DEBUG):
        logging.getLogger("bothy_sdk").info("token %s %r", secret, secret)
    assert CANARY not in caplog.text
    with pytest.raises(ValueError, match=r"failed with \[redacted\]"):
        raise ValueError(f"failed with {secret}")
    with pytest.raises(TypeError):
        pickle.dumps(secret)
    assert secret.reveal() == CANARY
    f = tmp_path / "token"
    f.write_text(CANARY + "\n")
    f.chmod(0o640)
    with pytest.raises(PermissionError):
        Secret.from_file(f)
    f.chmod(0o600)
    assert Secret.from_file(f) == secret


def test_cleartext_only_on_loopback() -> None:
    for ok in (
        "https://example.com/api/v1",
        "http://127.0.0.1:8080/api/v1",
        "http://[::1]/api/v1",
        "http://localhost:1/api/v1",
        "wss://example.com/api/v1/gateway",
        "ws://127.0.0.1:9/g",
    ):
        require_safe_url(ok)
    for bad in ("http://example.com/api/v1", "ws://192.0.2.1/g", "ftp://example.com/"):
        with pytest.raises(ValueError, match="only https and wss"):
            require_safe_url(bad)
    with pytest.raises(ValueError, match="only https and wss"):
        Client("http://example.com/api/v1", Credentials(Secret(CANARY)))
