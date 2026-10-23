# SPDX-License-Identifier: Apache-2.0
"""API token and DPoP checks of the FakeServer (private).

The rules follow the server's verifier (docs/spec/06-api.md 3.1, docs/spec/08-identity.md 7.3):
one proof, ``typ = dpop+jwt``, ES256 with a public ``jwk`` whose thumbprint is the token's bound
``jkt``, ``htm``, normalized ``htu``, ``iat`` within ``auth.dpop_proof_max_age`` and
``auth.clock_leeway``, ``ath``, a current nonce (a missing or old one answers ``401
use_dpop_nonce``) and no reused ``jti``. Nonces are windows of ``auth.dpop_nonce_ttl``; the current
and the previous window pass, and `NonceSource.rotate` starts a new window at once.
"""

from __future__ import annotations

import hashlib
import hmac
import secrets
from collections.abc import Callable, Mapping
from typing import Any

from cryptography.exceptions import InvalidSignature
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives.asymmetric.utils import encode_dss_signature

from bothy_sdk.auth import jwk_thumbprint, normalize_htu
from bothy_sdk.testing._fake_base import FakeProblem, b64url, b64url_decode, strict_object
from bothy_sdk.testing.doubles import FakeClock

WWW_AUTHENTICATE = 'DPoP algs="ES256 EdDSA"'


class NonceSource:
    """Stateless windowed nonces: ``b64url(window || HMAC16)``.

    Args:
        clock: The FakeServer's clock.
        ttl: A function returning ``auth.dpop_nonce_ttl`` in seconds.
    """

    def __init__(self, clock: FakeClock, ttl: Callable[[], float]) -> None:
        self._clock = clock
        self._ttl = ttl
        self._key = secrets.token_bytes(32)
        self._bump = 0

    def _window(self) -> int:
        return int(self._clock.now().timestamp() // self._ttl()) + self._bump

    def _encode(self, window: int) -> str:
        raw = window.to_bytes(8, "big")
        return b64url(raw + hmac.new(self._key, raw, hashlib.sha256).digest()[:16])

    def current(self) -> str:
        """The nonce of the current window."""
        return self._encode(self._window())

    def rotate(self) -> None:
        """Start a new window now, so a client must follow the newest nonce."""
        self._bump += 1

    def valid(self, nonce: str) -> bool:
        """Whether ``nonce`` belongs to the current or the previous window."""
        try:
            raw = b64url_decode(nonce)
        except ValueError:
            return False
        if len(raw) != 24:
            return False
        window = int.from_bytes(raw[:8], "big")
        mac = hmac.new(self._key, raw[:8], hashlib.sha256).digest()[:16]
        return hmac.compare_digest(mac, raw[8:]) and self._window() - window in (0, 1)


class Authenticator:
    """Checks ``Authorization`` and ``DPoP`` of one request or gateway credential.

    Args:
        api_token: The agent's API token value.
        jkt: The bound key thumbprint, or ``None`` for a bearer token.
        clock: The FakeServer's clock.
        timings: The FakeServer's timing values.
    """

    def __init__(
        self, api_token: str, jkt: str | None, clock: FakeClock, timings: Mapping[str, float]
    ) -> None:
        self.api_token = api_token
        self.jkt = jkt
        self.clock = clock
        self.timings = timings
        self.nonces = NonceSource(clock, lambda: timings["auth.dpop_nonce_ttl"])
        self._seen: dict[bytes, float] = {}

    def _dpop_error(self, code: str) -> FakeProblem:
        return FakeProblem(
            code,
            headers={
                "WWW-Authenticate": f'DPoP error="{code}", algs="ES256 EdDSA"',
                "DPoP-Nonce": self.nonces.current(),
            },
        )

    def _unauthenticated(self) -> FakeProblem:
        return FakeProblem("unauthenticated", headers={"WWW-Authenticate": WWW_AUTHENTICATE})

    def authenticate(
        self, *, method: str, htu: str, authorization: list[str], proofs: list[str]
    ) -> None:
        """Accept the credentials or raise the problem the server answers.

        Raises:
            FakeProblem: ``401 unauthenticated``, ``invalid_dpop_proof`` or ``use_dpop_nonce``.
        """
        if len(authorization) != 1:
            raise self._unauthenticated()
        scheme, _, token = authorization[0].partition(" ")
        scheme = scheme.lower()
        if scheme not in ("bearer", "dpop") or not token or " " in token or "," in token:
            raise self._unauthenticated()
        if not hmac.compare_digest(token.encode(), self.api_token.encode()):
            raise self._unauthenticated()
        if self.jkt is None:
            if scheme == "dpop" or proofs:
                raise self._unauthenticated()
            return
        if scheme == "bearer":
            raise self._dpop_error("invalid_dpop_proof")
        self._check_proof(proofs, method=method, htu=htu, token=token)

    def _check_proof(self, proofs: list[str], *, method: str, htu: str, token: str) -> None:
        if len(proofs) != 1:
            raise self._dpop_error("invalid_dpop_proof")
        header, claims = self._parse(proofs[0])
        if claims.get("htm") != method:
            raise self._dpop_error("invalid_dpop_proof")
        try:
            same_htu = normalize_htu(str(claims["htu"])) == normalize_htu(htu)
        except ValueError:
            same_htu = False
        if not same_htu:
            raise self._dpop_error("invalid_dpop_proof")
        now = self.clock.now().timestamp()
        iat = claims["iat"]
        if iat < now - self.timings["auth.dpop_proof_max_age"]:
            raise self._dpop_error("invalid_dpop_proof")
        if iat > now + self.timings["auth.clock_leeway"]:
            raise self._dpop_error("invalid_dpop_proof")
        if not self.nonces.valid(claims["nonce"]):
            raise self._dpop_error("use_dpop_nonce")
        ath = b64url(hashlib.sha256(token.encode("ascii")).digest())
        if claims.get("ath") != ath or jwk_thumbprint(header["jwk"]) != self.jkt:
            raise self._dpop_error("invalid_dpop_proof")
        replay = hashlib.sha256(normalize_htu(htu).encode() + b"\x00" + claims["jti"].encode())
        key = replay.digest()
        self._seen = {k: v for k, v in self._seen.items() if v > now}
        if key in self._seen:
            raise self._dpop_error("invalid_dpop_proof")
        self._seen[key] = (
            now + self.timings["auth.dpop_proof_max_age"] + self.timings["auth.clock_leeway"]
        )

    def _parse(self, proof: str) -> tuple[dict[str, Any], dict[str, Any]]:
        parts = proof.split(".")
        try:
            if len(parts) != 3:
                raise ValueError("parts")
            header = strict_object(b64url_decode(parts[0]))
            claims = strict_object(b64url_decode(parts[1]))
            signature = b64url_decode(parts[2])
            if str(header.get("typ", "")).lower() != "dpop+jwt" or header.get("alg") != "ES256":
                raise ValueError("header")
            jwk = header.get("jwk")
            if not isinstance(jwk, dict) or jwk.get("kty") != "EC" or jwk.get("crv") != "P-256":
                raise ValueError("jwk")
            if set(jwk) & {"d", "p", "q", "dp", "dq", "qi", "k", "oth"}:
                raise ValueError("private jwk")
            public = ec.EllipticCurvePublicNumbers(
                int.from_bytes(b64url_decode(jwk["x"]), "big"),
                int.from_bytes(b64url_decode(jwk["y"]), "big"),
                ec.SECP256R1(),
            ).public_key()
            if len(signature) != 64:
                raise ValueError("signature size")
            der = encode_dss_signature(
                int.from_bytes(signature[:32], "big"), int.from_bytes(signature[32:], "big")
            )
            public.verify(der, f"{parts[0]}.{parts[1]}".encode("ascii"), ec.ECDSA(hashes.SHA256()))
        except (ValueError, KeyError, TypeError, InvalidSignature) as exc:
            raise self._dpop_error("invalid_dpop_proof") from exc
        jti, iat = claims.get("jti"), claims.get("iat")
        if not isinstance(jti, str) or not jti or not isinstance(claims.get("htu"), str):
            raise self._dpop_error("invalid_dpop_proof")
        if not isinstance(iat, int) or isinstance(iat, bool):
            raise self._dpop_error("invalid_dpop_proof")
        if "nonce" not in claims:
            raise self._dpop_error("use_dpop_nonce")
        if not isinstance(claims["nonce"], str):
            raise self._dpop_error("invalid_dpop_proof")
        return header, claims
