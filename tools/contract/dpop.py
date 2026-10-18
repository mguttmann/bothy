# SPDX-License-Identifier: Apache-2.0
r"""DPoP proofs (RFC 9449) with ES256 for the contract tests G7.

Builds the proof the server expects (06 "DPoP"): header ``typ=dpop+jwt``, ``alg=ES256`` and the
public ``jwk``; claims ``jti``, ``htm``, ``htu`` (without query and fragment), ``iat``, ``ath``
when a token is sent and ``nonce`` when known; the signature is raw ``r||s`` (64 bytes).

Command line (reproduction of a failed G7 case)::

    PYTHONPATH=tools/contract uv run --project tools python -m dpop \\
        --key key.json --htm GET --htu https://chat.example.com/api/v1/me [--nonce N]

With ``BOTHY_CONTRACT_AUTH="DPoP <token>"`` in the environment the proof carries ``ath``.
"""

from __future__ import annotations

import argparse
import base64
import hashlib
import json
import os
import sys
import time
import uuid
from collections.abc import Mapping, Sequence
from dataclasses import dataclass
from pathlib import Path
from urllib.parse import urlsplit, urlunsplit

from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives.asymmetric.utils import decode_dss_signature

_COORDINATE = 32
_AUTH_ENV = "BOTHY_CONTRACT_AUTH"


def b64url(data: bytes) -> str:
    """Return ``data`` as base64url without padding."""
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode("ascii")


def _unb64(text: str) -> bytes:
    return base64.urlsafe_b64decode(text + "=" * (-len(text) % 4))


def ath(access_token: str) -> str:
    """Return the ``ath`` claim of ``access_token``: base64url of its SHA-256."""
    return b64url(hashlib.sha256(access_token.encode("ascii")).digest())


def canonical_htu(url: str) -> str:
    """Return ``url`` without query and fragment, as ``htu`` requires."""
    parts = urlsplit(url)
    return urlunsplit((parts.scheme, parts.netloc, parts.path, "", ""))


@dataclass(frozen=True)
class DPoPKey:
    """An ES256 (P-256) device key that signs DPoP proofs."""

    private_key: ec.EllipticCurvePrivateKey

    @classmethod
    def generate(cls) -> DPoPKey:
        """Return a fresh key."""
        return cls(ec.generate_private_key(ec.SECP256R1()))

    @classmethod
    def from_jwk(cls, jwk: Mapping[str, str]) -> DPoPKey:
        """Load a private EC JWK on P-256 (``kty``, ``crv``, ``x``, ``y``, ``d``).

        Raises:
            ValueError: the JWK is not a private P-256 key.
        """
        if jwk.get("kty") != "EC" or jwk.get("crv") != "P-256" or not jwk.get("d"):
            raise ValueError("the DPoP key must be a private EC JWK on P-256")
        numbers = ec.EllipticCurvePrivateNumbers(
            int.from_bytes(_unb64(jwk["d"])),
            ec.EllipticCurvePublicNumbers(
                int.from_bytes(_unb64(jwk["x"])), int.from_bytes(_unb64(jwk["y"])), ec.SECP256R1()
            ),
        )
        return cls(numbers.private_key())

    def public_jwk(self) -> dict[str, str]:
        """Return the public JWK (no private member)."""
        numbers = self.private_key.public_key().public_numbers()
        return {
            "kty": "EC",
            "crv": "P-256",
            "x": b64url(numbers.x.to_bytes(_COORDINATE)),
            "y": b64url(numbers.y.to_bytes(_COORDINATE)),
        }

    def private_jwk(self) -> dict[str, str]:
        """Return the private JWK."""
        d = self.private_key.private_numbers().private_value
        return {**self.public_jwk(), "d": b64url(d.to_bytes(_COORDINATE))}

    def proof(
        self,
        htm: str,
        htu: str,
        *,
        access_token: str | None = None,
        nonce: str | None = None,
        now: int | None = None,
    ) -> str:
        """Return a compact DPoP proof for method ``htm`` and URL ``htu``.

        Args:
            htm: HTTP method; upper-cased.
            htu: target URL; query and fragment are removed.
            access_token: the token sent with the request; adds ``ath``.
            nonce: the newest server nonce; adds ``nonce``.
            now: ``iat`` in seconds; default the current time.
        """
        header = {"typ": "dpop+jwt", "alg": "ES256", "jwk": self.public_jwk()}
        claims: dict[str, object] = {
            "jti": str(uuid.uuid4()),
            "htm": htm.upper(),
            "htu": canonical_htu(htu),
            "iat": int(time.time()) if now is None else now,
        }
        if access_token:
            claims["ath"] = ath(access_token)
        if nonce:
            claims["nonce"] = nonce
        return self.sign(header, claims)

    def sign(self, header: Mapping[str, object], claims: Mapping[str, object]) -> str:
        """Return the compact JWS of ``claims`` with ``header``, signed as raw ``r||s``."""
        signing_input = (
            b64url(json.dumps(header, separators=(",", ":")).encode())
            + "."
            + b64url(json.dumps(claims, separators=(",", ":")).encode())
        )
        der = self.private_key.sign(signing_input.encode("ascii"), ec.ECDSA(hashes.SHA256()))
        r, s = decode_dss_signature(der)
        return signing_input + "." + b64url(r.to_bytes(_COORDINATE) + s.to_bytes(_COORDINATE))


def main(argv: Sequence[str] | None = None) -> int:
    """Print one proof for ``--htm`` and ``--htu`` with the key in ``--key``; exit code 0."""
    parser = argparse.ArgumentParser(prog="dpop", description="Print a DPoP proof (RFC 9449).")
    parser.add_argument("--key", required=True, type=Path, help="file with the private JWK")
    parser.add_argument("--htm", required=True, help="HTTP method")
    parser.add_argument("--htu", required=True, help="target URL")
    parser.add_argument("--nonce", default=None, help="server nonce (DPoP-Nonce)")
    args = parser.parse_args(argv)
    key = DPoPKey.from_jwk(json.loads(args.key.read_text(encoding="utf-8")))
    auth = os.environ.get(_AUTH_ENV, "")
    token = auth.removeprefix("DPoP ") if auth.startswith("DPoP ") else None
    sys.stdout.write(key.proof(args.htm, args.htu, access_token=token, nonce=args.nonce) + "\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
