# SPDX-License-Identifier: Apache-2.0
"""Tests of the DPoP proofs (RFC 9449) of the G7 auth provider."""

from __future__ import annotations

import base64
import hashlib
import json
from pathlib import Path

import dpop
import pytest
from cryptography.exceptions import InvalidSignature
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives.asymmetric.utils import encode_dss_signature


def _b64(data: str) -> bytes:
    return base64.urlsafe_b64decode(data + "=" * (-len(data) % 4))


def _parts(proof: str) -> tuple[dict[str, object], dict[str, object], bytes, bytes]:
    header, payload, signature = proof.split(".")
    return (
        json.loads(_b64(header)),
        json.loads(_b64(payload)),
        f"{header}.{payload}".encode(),
        _b64(signature),
    )


@pytest.fixture
def key() -> dpop.DPoPKey:
    return dpop.DPoPKey.generate()


def test_header_carries_type_algorithm_and_public_key(key: dpop.DPoPKey) -> None:
    header, _, _, _ = _parts(key.proof("GET", "https://chat.example.com/api/v1/me"))
    assert header["typ"] == "dpop+jwt"
    assert header["alg"] == "ES256"
    jwk = header["jwk"]
    assert isinstance(jwk, dict)
    assert set(jwk) == {"kty", "crv", "x", "y"}
    assert jwk["kty"] == "EC"
    assert jwk["crv"] == "P-256"


def test_claims_without_token_and_nonce(key: dpop.DPoPKey) -> None:
    _, claims, _, _ = _parts(
        key.proof("post", "https://chat.example.com/api/v1/x?a=1#frag", now=1_790_000_000)
    )
    assert claims["htm"] == "POST"
    assert claims["htu"] == "https://chat.example.com/api/v1/x"
    assert claims["iat"] == 1_790_000_000
    assert isinstance(claims["jti"], str)
    assert len(str(claims["jti"])) >= 16
    assert "ath" not in claims
    assert "nonce" not in claims


def test_ath_and_nonce(key: dpop.DPoPKey) -> None:
    _, claims, _, _ = _parts(
        key.proof("GET", "https://chat.example.com/api/v1/me", access_token="tok", nonce="n-1")
    )
    digest = hashlib.sha256(b"tok").digest()
    assert claims["ath"] == base64.urlsafe_b64encode(digest).rstrip(b"=").decode()
    assert claims["nonce"] == "n-1"


def test_jti_is_unique(key: dpop.DPoPKey) -> None:
    jtis = {
        _parts(key.proof("GET", "https://chat.example.com/api/v1/me"))[1]["jti"] for _ in range(20)
    }
    assert len(jtis) == 20


def test_signature_is_raw_r_s_and_verifies(key: dpop.DPoPKey) -> None:
    _, _, signed, signature = _parts(key.proof("GET", "https://chat.example.com/api/v1/me"))
    assert len(signature) == 64
    der = encode_dss_signature(int.from_bytes(signature[:32]), int.from_bytes(signature[32:]))
    public = key.private_key.public_key()
    public.verify(der, signed, ec.ECDSA(hashes.SHA256()))
    with pytest.raises(InvalidSignature):
        public.verify(der, signed + b"x", ec.ECDSA(hashes.SHA256()))


def test_private_jwk_round_trip(key: dpop.DPoPKey) -> None:
    again = dpop.DPoPKey.from_jwk(key.private_jwk())
    assert again.public_jwk() == key.public_jwk()


def test_from_jwk_rejects_other_curves() -> None:
    with pytest.raises(ValueError, match="P-256"):
        dpop.DPoPKey.from_jwk({"kty": "EC", "crv": "P-384", "x": "", "y": "", "d": ""})


def test_command_line_prints_a_proof(
    key: dpop.DPoPKey,
    tmp_path: Path,
    capsys: pytest.CaptureFixture[str],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    file = tmp_path / "key.json"
    file.write_text(json.dumps(key.private_jwk()), encoding="utf-8")
    monkeypatch.setenv("BOTHY_CONTRACT_AUTH", "DPoP tok")
    code = dpop.main(
        [
            "--key",
            str(file),
            "--htm",
            "GET",
            "--htu",
            "https://chat.example.com/api/v1/me",
            "--nonce",
            "n",
        ]
    )
    assert code == 0
    _, claims, _, _ = _parts(capsys.readouterr().out.strip())
    assert claims["htu"] == "https://chat.example.com/api/v1/me"
    assert claims["nonce"] == "n"
    assert (
        claims["ath"]
        == base64.urlsafe_b64encode(hashlib.sha256(b"tok").digest()).rstrip(b"=").decode()
    )
