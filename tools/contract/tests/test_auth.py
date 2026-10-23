# SPDX-License-Identifier: Apache-2.0
"""Tests of the G7 auth provider against a stub broker and a stub server on loopback."""

from __future__ import annotations

import base64
import json
import threading
import uuid
from collections.abc import Iterator
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any, ClassVar

import auth
import pytest
from broker_client import BrokerClient, BrokerError, http_request
from dpop import DPoPKey, ath

KEY = DPoPKey.generate()


def _claims(proof: str) -> dict[str, Any]:
    payload = proof.split(".")[1]
    data = json.loads(base64.urlsafe_b64decode(payload + "=" * (-len(payload) % 4)))
    assert isinstance(data, dict)
    return data


class Stub(BaseHTTPRequestHandler):
    """Broker and server in one: /principals, /step-up and the API under /api/v1."""

    tokens: ClassVar[list[str]] = []
    log: ClassVar[list[str]] = []
    revoked: ClassVar[set[str]] = set()
    registrations: ClassVar[list[dict[str, Any]]] = []
    register_status: ClassVar[int] = 201

    def log_message(self, format: str, *args: Any) -> None:
        """Silence the default request log."""

    def _send(
        self, status: int, body: dict[str, Any], headers: dict[str, str] | None = None
    ) -> None:
        raw = json.dumps(body).encode()
        self.send_response(status)
        for k, v in (headers or {}).items():
            self.send_header(k, v)
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def do_POST(self) -> None:
        """Broker endpoints."""
        length = int(self.headers.get("Content-Length", "0"))
        payload = json.loads(self.rfile.read(length) or b"{}")
        self.log.append(f"POST {self.path}")
        if self.path == "/api/v1" + auth.PUSH_REGISTRATIONS_PATH:
            self._register(payload)
        elif self.path == "/principals":
            token = self.tokens.pop(0)
            self._send(
                200,
                {
                    "principal_id": "p",
                    "device_id": "d",
                    "session_id": "s-" + token,
                    "access_token": token,
                    "refresh_token": "r",
                    "dpop_private_jwk": KEY.private_jwk(),
                },
            )
        elif self.path == "/step-up" and payload.get("session_id", "").startswith("s-"):
            self._send(200, {})
        else:
            self._send(404, {})

    def _register(self, payload: dict[str, Any]) -> None:
        """registerPush: needs nonce n-9 and a live token; answers a fresh preview token."""
        proof = _claims(self.headers["DPoP"])
        token = (self.headers.get("Authorization") or "").removeprefix("DPoP ")
        if proof.get("nonce") != "n-9":
            self._send(401, {"code": "use_dpop_nonce"}, {"DPoP-Nonce": "n-9"})
        elif token in self.revoked:
            self._send(401, {"code": "session_revoked"})
        elif self.register_status != 201:
            self._send(self.register_status, {"code": "feature_disabled"})
        else:
            self.registrations.append({**payload, "session_token": token})
            n = len(self.registrations)
            self._send(
                201, {"id": f"r-{n}", "preview_token": f"bothy_pvw_{n}"}, {"DPoP-Nonce": "n-9"}
            )

    def do_GET(self) -> None:
        """API: needs nonce n-9; answers session_revoked for revoked tokens."""
        proof = _claims(self.headers["DPoP"])
        token = (self.headers.get("Authorization") or "").removeprefix("DPoP ")
        self.log.append(f"GET {self.path} nonce={proof.get('nonce')} token={token}")
        if proof.get("nonce") != "n-9":
            self._send(401, {"code": "use_dpop_nonce"}, {"DPoP-Nonce": "n-9"})
        elif token in self.revoked:
            self._send(401, {"code": "session_revoked"})
        else:
            self._send(200, {"ok": True}, {"DPoP-Nonce": "n-9"})


@pytest.fixture
def stub() -> Iterator[str]:
    Stub.tokens, Stub.log, Stub.revoked = ["tok-1", "tok-2"], [], {"tok-1"}
    Stub.registrations, Stub.register_status = [], 201
    srv = ThreadingHTTPServer(("127.0.0.1", 0), Stub)
    thread = threading.Thread(target=srv.serve_forever, daemon=True)
    thread.start()
    yield f"http://127.0.0.1:{srv.server_address[1]}"
    srv.shutdown()
    srv.server_close()


def _state(base: str, tmp_path: Path) -> auth.PassAuth:
    return auth.PassAuth(
        broker=BrokerClient(base),
        kind="owner",
        api_base=base + "/api/v1",
        key_dir=tmp_path,
        step_up_max_age=300,
        access_token_ttl=600,
        nonce_ttl=300,
    )


def test_nonce_probe_when_none_is_known(stub: str, tmp_path: Path) -> None:
    state = _state(stub, tmp_path)
    headers = state.headers("GET", stub + "/api/v1/spaces?x=1", authenticated=True, proof=True)
    assert headers["Authorization"] == "DPoP tok-1"
    claims = _claims(headers["DPoP"])
    assert claims["nonce"] == "n-9"
    assert claims["htu"] == stub + "/api/v1/spaces"
    assert any(line.startswith("GET /api/v1/me") for line in Stub.log)
    assert [line for line in Stub.log if line.startswith("POST")] == [
        "POST /principals",
        "POST /step-up",
    ]
    assert (tmp_path / "owner-s-tok-1.jwk.json").exists()


def test_refresh_on_session_revoked(stub: str, tmp_path: Path) -> None:
    state = _state(stub, tmp_path)
    state.nonce, state.nonce_at = "n-9", state.clock()
    resp = state.send("GET", stub + "/api/v1/me", {}, None, authenticated=True)
    assert resp.status == 200
    assert state.renewals == 2
    assert state.current().credentials.access_token == "tok-2"


def test_renewal_after_the_shortest_lifetime(stub: str, tmp_path: Path) -> None:
    now = [0.0]
    state = _state(stub, tmp_path)
    state.clock = lambda: now[0]
    first = state.current()
    now[0] = 149.0
    assert state.current() is first
    now[0] = 150.0
    assert state.current() is not first


def test_observe() -> None:
    state = auth.PassAuth(BrokerClient("http://127.0.0.1:1"), "user", "x", Path(), 1, 1, 1)
    assert state.observe(200, {"dpop-nonce": "a"}, b"") is None
    assert state.nonce == "a"
    assert state.observe(401, {}, b'{"code":"use_dpop_nonce"}') == "nonce"
    assert state.observe(401, {}, b'{"code":"step_up_required"}') == "step_up"
    assert state.observe(401, {}, b'{"code":"invalid_dpop_proof"}') is None
    assert state.observe(401, {}, b"not json") is None


def test_uuid7() -> None:
    value = uuid.UUID(auth.uuid7())
    assert value.version == 7
    assert value.variant == uuid.RFC_4122
    assert auth.uuid7() != auth.uuid7()


def test_provider_class(stub: str, tmp_path: Path) -> None:
    state = _state(stub, tmp_path)
    cls = auth.provider_class(state, lambda case: {"X-Auth": "1"})
    provider = cls()

    class Case:
        headers: dict[str, str] | None = None

    case = Case()
    assert provider.get(case, None).credentials.access_token == "tok-1"
    provider.set(case, provider.get(case, None), None)
    assert case.headers == {"X-Auth": "1"}


def test_broker_errors(stub: str) -> None:
    with pytest.raises(BrokerError, match="404"):
        BrokerClient(stub).step_up("unknown")
    with pytest.raises(ValueError, match="plain HTTP"):
        http_request("GET", "https://127.0.0.1/")


def test_without_host_drops_generated_host_headers() -> None:
    """A generated Host header in any spelling is dropped; the transport sets the real one."""
    generated = {"host": "&uJ", "HOST": "hIWc&n", "Idempotency-Key": "B@1q", "gl": "a.R&"}
    assert auth.without_host(generated) == {"Idempotency-Key": "B@1q", "gl": "a.R&"}
    assert auth.without_host({}) == {}


def test_preview_headers_carry_a_fresh_registration(stub: str, tmp_path: Path) -> None:
    """Each request of pushPreviewToken registers anew and binds the proof to its token."""
    state = _state(stub, tmp_path)
    state.push_app_id = "com.example.chat"
    url = stub + "/api/v1/push/preview?nid=1&message_id=2&kind=message"
    first = state.preview_headers("GET", url)
    second = state.preview_headers("GET", url)
    assert first["Authorization"] == "DPoP bothy_pvw_1"
    assert second["Authorization"] == "DPoP bothy_pvw_2"
    claims = _claims(first["DPoP"])
    assert claims["ath"] == ath("bothy_pvw_1")
    assert claims["nonce"] == "n-9"
    assert claims["htm"] == "GET"
    assert claims["htu"] == stub + "/api/v1/push/preview"
    header = json.loads(base64.urlsafe_b64decode(first["DPoP"].split(".")[0] + "=="))
    assert header["jwk"] == KEY.public_jwk()
    # The first session was revoked, so the renewed session (tok-2) registered both targets.
    assert [r["session_token"] for r in Stub.registrations] == ["tok-2", "tok-2"]
    for reg in Stub.registrations:
        assert reg["provider"] == "apns"
        assert reg["app_id"] == "com.example.chat"
        assert len(reg["token"]) == 64
        assert reg["token"] == reg["token"].lower()
        int(reg["token"], 16)
    assert Stub.registrations[0]["token"] != Stub.registrations[1]["token"]


def test_register_push_returns_the_registering_session(stub: str, tmp_path: Path) -> None:
    state = _state(stub, tmp_path)
    state.push_app_id = "com.example.chat"
    reg = state.register_push()
    assert reg.registration_id == "r-1"
    assert reg.preview_token == "bothy_pvw_1"
    assert reg.session is state.session
    assert reg.session.credentials.access_token == "tok-2"


def test_register_push_errors(stub: str, tmp_path: Path) -> None:
    state = _state(stub, tmp_path)
    with pytest.raises(auth.RegistrationError, match="BOTHY_CONTRACT_PUSH_APP_ID"):
        state.register_push()
    state.push_app_id = "com.example.chat"
    Stub.register_status = 403
    with pytest.raises(auth.RegistrationError, match="403 feature_disabled"):
        state.preview_headers("GET", stub + "/api/v1/push/preview")
