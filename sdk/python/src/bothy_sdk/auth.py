# SPDX-License-Identifier: Apache-2.0
"""Credentials of an agent: the API token, optional DPoP proofs and the nonce handling.

The API token travels as ``Authorization: Bearer <token>``; a token bound to a key at
``createToken`` (``dpop_jkt``) travels as ``Authorization: DPoP <token>`` with a fresh proof per
request (docs/spec/06-api.md 3.1, docs/spec/08-identity.md 7.3 and 12). The proof is an ES256
JWT with ``typ = dpop+jwt``, the public ``jwk``, ``jti``, ``htm``, ``htu``, ``iat``, ``ath`` and the
newest ``nonce`` the server sent. A ``401 use_dpop_nonce`` rebuilds the proof once with the
nonce of that answer (docs/DECISIONS.md D-M2-29-3).

Secrets never show: `Secret` renders as ``[redacted]`` in ``repr``, ``str``, format strings and
logs, and refuses pickling (SEC-LOG-2).

Examples:
    >>> token = Secret("example-token")
    >>> (str(token), f"{token}", token.reveal())
    ('[redacted]', '[redacted]', 'example-token')
"""

from __future__ import annotations

import base64
import hashlib
import ipaddress
import json
import os
import stat
from collections.abc import Callable, Generator, Mapping
from dataclasses import dataclass
from pathlib import Path
from typing import Any, NoReturn, Protocol, runtime_checkable
from urllib.parse import urlsplit, urlunsplit

import httpx
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives.asymmetric.utils import decode_dss_signature

from bothy_sdk.clock import Clock, SystemClock
from bothy_sdk.ids import uuid7

__all__ = [
    "Credentials",
    "DpopAuth",
    "DpopKeySource",
    "FileDpopKey",
    "Secret",
    "dpop_proof",
    "gateway_htu",
    "jwk_thumbprint",
    "normalize_htu",
    "require_safe_url",
]

_REDACTED = "[redacted]"
_LOOPBACK_NAMES = frozenset({"localhost"})


class Secret:
    """A secret string (API token, invocation token) that never renders its value.

    ``repr``, ``str`` and every format spec give ``[redacted]``; ``reveal`` returns the value
    for the one place that must send it. Equality compares values in constant time.

    Args:
        value: The secret text; must not be empty.

    Raises:
        ValueError: The value is empty.
    """

    __slots__ = ("_value",)

    def __init__(self, value: str) -> None:
        """Wrap ``value``; see the class docstring."""
        if not value:
            raise ValueError("a secret must not be empty")
        self._value = value

    def reveal(self) -> str:
        """Return the secret value; call it only where the value is sent.

        Returns:
            The secret text.
        """
        return self._value

    @classmethod
    def from_env(cls, name: str) -> Secret:
        """Read a secret from the environment variable ``name``.

        Args:
            name: Name of the variable.

        Returns:
            The secret.

        Raises:
            KeyError: The variable is not set or empty; the message names the variable only.
        """
        value = os.environ.get(name, "")
        if not value:
            raise KeyError(f"environment variable {name} is not set")
        return cls(value)

    @classmethod
    def from_file(cls, path: str | os.PathLike[str]) -> Secret:
        """Read a secret from a file that only its owner can read (mode ``0600`` or tighter).

        Leading and trailing white space is removed.

        Args:
            path: The file.

        Returns:
            The secret.

        Raises:
            PermissionError: On POSIX the file is readable or writable by group or others.
            ValueError: The file is empty.
        """
        p = Path(path)
        _require_private(p)
        return cls(p.read_text(encoding="utf-8").strip())

    def __repr__(self) -> str:
        """Return ``Secret('[redacted]')``."""
        return f"Secret({_REDACTED!r})"

    def __str__(self) -> str:
        """Return ``[redacted]``."""
        return _REDACTED

    def __format__(self, spec: str) -> str:
        """Return ``[redacted]`` for every format spec."""
        return _REDACTED

    def __eq__(self, other: object) -> bool:
        """Compare two secrets in constant time."""
        if not isinstance(other, Secret):
            return NotImplemented
        import hmac  # noqa: PLC0415 - only needed here

        return hmac.compare_digest(self._value.encode(), other._value.encode())

    def __hash__(self) -> int:
        """Hash by a digest of the value, so equal secrets hash alike."""
        return hash(hashlib.sha256(self._value.encode()).digest())

    def __reduce__(self) -> NoReturn:
        """Refuse pickling, so a secret never lands in a pickle file or a queue."""
        raise TypeError("a Secret cannot be pickled")


def _require_private(path: Path) -> None:
    if os.name != "posix":
        return
    mode = path.stat().st_mode
    if mode & (stat.S_IRWXG | stat.S_IRWXO):
        raise PermissionError(f"{path} must not be accessible by group or others (mode 0600)")


def _b64url(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode("ascii")


def jwk_thumbprint(jwk: Mapping[str, str]) -> str:
    """Return the RFC 7638 thumbprint (SHA-256, base64url) of a public JWK.

    Only the required members of the key type enter the hash, in lexicographic order.

    Args:
        jwk: A public JWK of type ``EC``, ``OKP`` or ``RSA``.

    Returns:
        The thumbprint, the value of ``dpop_jkt`` at ``createToken``.

    Raises:
        ValueError: The key type is not supported or a required member is missing.

    Examples:
        >>> jwk_thumbprint({"kty": "OKP", "crv": "Ed25519", "x": "AA"})
        '3I7TDbW-lKbNSt8372-ez4nbmhZyQrtliLvMotQEsgE'
    """
    required = {
        "EC": ("crv", "kty", "x", "y"),
        "OKP": ("crv", "kty", "x"),
        "RSA": ("e", "kty", "n"),
    }
    names = required.get(jwk.get("kty", ""))
    if names is None or any(n not in jwk for n in names):
        raise ValueError("unsupported or incomplete JWK")
    canonical = json.dumps({n: jwk[n] for n in names}, separators=(",", ":"), sort_keys=True)
    return _b64url(hashlib.sha256(canonical.encode("utf-8")).digest())


@runtime_checkable
class DpopKeySource(Protocol):
    """A DPoP key: its JWS algorithm, its public JWK and a signer.

    An adapter can plug a key store (for example an HSM) without a new SDK dependency.
    """

    @property
    def algorithm(self) -> str:
        """The JWS ``alg`` of the proofs, for example ``ES256``."""
        ...

    def public_jwk(self) -> dict[str, str]:
        """Return the public JWK without private members."""
        ...

    def sign(self, data: bytes) -> bytes:
        """Return the JWS signature of ``data`` in the form of ``algorithm`` (raw r||s for ES256).

        Args:
            data: The JWS signing input.
        """
        ...


class FileDpopKey:
    """An ES256 (P-256) DPoP key in a PKCS#8 PEM file of mode ``0600``.

    Use `generate` once and `load` afterwards; `jkt` is the thumbprint the owner enters at
    ``createToken`` (``dpop_jkt``).

    Args:
        key: The private key.
    """

    algorithm = "ES256"

    def __init__(self, key: ec.EllipticCurvePrivateKey) -> None:
        """Wrap ``key``; prefer `generate` or `load`."""
        if not isinstance(key.curve, ec.SECP256R1):
            raise ValueError("a DPoP key must be P-256")
        self._key = key

    @classmethod
    def generate(cls, path: str | os.PathLike[str]) -> FileDpopKey:
        """Create a new key and write it to ``path`` (must not exist) with mode ``0600``.

        Args:
            path: The new PEM file.

        Returns:
            The key.

        Raises:
            FileExistsError: The file exists already.
        """
        key = ec.generate_private_key(ec.SECP256R1())
        pem = key.private_bytes(
            serialization.Encoding.PEM,
            serialization.PrivateFormat.PKCS8,
            serialization.NoEncryption(),
        )
        # S8707: the caller picks the file by design; O_EXCL never replaces one (SONAR-MAIN-GATE).
        fd = os.open(os.fspath(path), os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)  # NOSONAR
        with os.fdopen(fd, "wb") as fh:
            fh.write(pem)
        return cls(key)

    @classmethod
    def load(cls, path: str | os.PathLike[str]) -> FileDpopKey:
        """Load a key that `generate` wrote.

        Args:
            path: The PEM file.

        Returns:
            The key.

        Raises:
            PermissionError: On POSIX the file is accessible by group or others.
            ValueError: The file is not an unencrypted P-256 PKCS#8 key.
        """
        p = Path(path)
        _require_private(p)
        key = serialization.load_pem_private_key(p.read_bytes(), password=None)
        if not isinstance(key, ec.EllipticCurvePrivateKey):
            raise ValueError("a DPoP key must be an EC key")
        return cls(key)

    def public_jwk(self) -> dict[str, str]:
        """Return the public JWK (``kty``, ``crv``, ``x``, ``y``)."""
        numbers = self._key.public_key().public_numbers()
        return {
            "kty": "EC",
            "crv": "P-256",
            "x": _b64url(numbers.x.to_bytes(32, "big")),
            "y": _b64url(numbers.y.to_bytes(32, "big")),
        }

    @property
    def jkt(self) -> str:
        """The RFC 7638 thumbprint of the public key."""
        return jwk_thumbprint(self.public_jwk())

    def sign(self, data: bytes) -> bytes:
        """Return the raw ES256 signature (r || s, 64 bytes) of ``data``.

        Args:
            data: The JWS signing input.
        """
        der = self._key.sign(data, ec.ECDSA(hashes.SHA256()))
        r, s = decode_dss_signature(der)
        return r.to_bytes(32, "big") + s.to_bytes(32, "big")

    def __repr__(self) -> str:
        """Show the thumbprint only, never the key."""
        return f"FileDpopKey(jkt={self.jkt!r})"

    def __reduce__(self) -> NoReturn:
        """Refuse pickling of the private key."""
        raise TypeError("a DPoP key cannot be pickled")


@dataclass(frozen=True)
class Credentials:
    """The credentials of an agent adapter.

    Attributes:
        api_token: The agent's API token.
        dpop: The key of a DPoP-bound token; ``None`` sends the token as a bearer token.
    """

    api_token: Secret
    dpop: DpopKeySource | None = None


def _is_loopback(host: str) -> bool:
    if host.lower() in _LOOPBACK_NAMES:
        return True
    try:
        return ipaddress.ip_address(host.strip("[]")).is_loopback
    except ValueError:
        return False


def require_safe_url(url: str) -> None:
    """Refuse a clear-text URL that leaves the host.

    ``http`` and ``ws`` are accepted only for a loopback host (``127.0.0.1``, ``::1``,
    ``localhost``), so a token never travels in clear text off the machine (D-M2-29-3).

    Args:
        url: A base URL or gateway URL.

    Raises:
        ValueError: The scheme is not ``http``, ``https``, ``ws`` or ``wss``, or a clear-text
            scheme names another host.
    """
    parts = urlsplit(url)
    scheme = parts.scheme.lower()
    if scheme in ("https", "wss"):
        return
    if scheme in ("http", "ws") and _is_loopback(parts.hostname or ""):
        return
    raise ValueError("only https and wss are allowed off the loopback host")


def normalize_htu(url: str) -> str:
    """Return the DPoP ``htu`` of a request URL.

    Scheme and host lower case, the default port removed, no query and no fragment
    (RFC 9449 4.2 and 4.3).

    Args:
        url: The request URL.

    Returns:
        The normalized URL.

    Examples:
        >>> normalize_htu("HTTPS://Example.COM:443/api/v1/runs?x=1#f")
        'https://example.com/api/v1/runs'
    """
    parts = urlsplit(url)
    scheme = parts.scheme.lower()
    host = (parts.hostname or "").lower()
    if ":" in host:
        host = f"[{host}]"
    port = parts.port
    default = {"http": 80, "https": 443}.get(scheme)
    netloc = host if port is None or port == default else f"{host}:{port}"
    return urlunsplit((scheme, netloc, parts.path or "/", "", ""))


def gateway_htu(gateway_url: str) -> str:
    """Return the ``htu`` of a gateway proof: the HTTPS form of the gateway URL.

    ``GatewayIdentify.dpop`` names "the HTTPS form of the gateway URL" (04 6.3 step 3), also when
    the server speaks HTTP behind a TLS terminator: the scheme is always ``https``, the host keeps
    its port, a default port (``80`` of ``ws``/``http``, ``443`` of ``wss``/``https``) is dropped,
    and query and fragment are removed (docs/DECISIONS.md D-M2-29-3).

    Args:
        gateway_url: The WebSocket URL of the gateway.

    Returns:
        The ``htu``.

    Examples:
        >>> gateway_htu("ws://127.0.0.1:8099/api/v1/gateway")
        'https://127.0.0.1:8099/api/v1/gateway'
    """
    parts = urlsplit(gateway_url)
    scheme = parts.scheme.lower()
    host = (parts.hostname or "").lower()
    if ":" in host:
        host = f"[{host}]"
    port = parts.port
    default = {"ws": 80, "http": 80, "wss": 443, "https": 443}.get(scheme)
    netloc = host if port is None or port == default else f"{host}:{port}"
    return urlunsplit(("https", netloc, parts.path or "/", "", ""))


def dpop_proof(  # noqa: PLR0913 - every input of RFC 9449 4.2 is explicit
    key: DpopKeySource,
    *,
    htm: str,
    htu: str,
    token: Secret | None,
    nonce: str | None,
    clock: Clock,
) -> str:
    """Build one DPoP proof (RFC 9449 4.2).

    Args:
        key: The DPoP key.
        htm: The HTTP method (``GET`` for the gateway).
        htu: The target URL; it is normalized here.
        token: The access token the proof binds through ``ath``; ``None`` leaves ``ath`` out.
        nonce: The newest server nonce, if any.
        clock: The time source of ``iat``.

    Returns:
        The compact JWS.
    """
    header = {"typ": "dpop+jwt", "alg": key.algorithm, "jwk": key.public_jwk()}
    claims: dict[str, Any] = {
        "jti": str(uuid7()),
        "htm": htm.upper(),
        "htu": normalize_htu(htu),
        "iat": int(clock.now().timestamp()),
    }
    if token is not None:
        claims["ath"] = _b64url(hashlib.sha256(token.reveal().encode("ascii")).digest())
    if nonce:
        claims["nonce"] = nonce
    signing_input = (
        _b64url(json.dumps(header, separators=(",", ":")).encode())
        + "."
        + _b64url(json.dumps(claims, separators=(",", ":")).encode())
    )
    return signing_input + "." + _b64url(key.sign(signing_input.encode("ascii")))


class DpopAuth(httpx.Auth):
    """``httpx`` authentication flow for the API token, with DPoP when a key is set.

    Every response's ``DPoP-Nonce`` replaces the stored nonce; a ``401`` with problem code
    ``use_dpop_nonce`` repeats the request once with a fresh proof carrying the new nonce. A
    second ``use_dpop_nonce`` is returned to the caller, who raises it.

    Args:
        credentials: Token and optional key.
        clock: Time source of the proofs.
        on_nonce: Called with every new nonce (the gateway shares it).
    """

    requires_response_body = True

    def __init__(
        self,
        credentials: Credentials,
        *,
        clock: Clock | None = None,
        on_nonce: Callable[[str], None] | None = None,
    ) -> None:
        """Create the flow; see the class docstring."""
        self._credentials = credentials
        self._clock = clock or SystemClock()
        self._nonce: str | None = None
        self._on_nonce = on_nonce

    @property
    def nonce(self) -> str | None:
        """The newest nonce the server sent, if any."""
        return self._nonce

    def remember_nonce(self, nonce: str | None) -> None:
        """Store a nonce received elsewhere (for example in the gateway ``hello``).

        Args:
            nonce: The nonce; ``None`` or empty changes nothing.
        """
        if nonce:
            self._nonce = nonce
            if self._on_nonce is not None:
                self._on_nonce(nonce)

    def apply(self, request: httpx.Request) -> None:
        """Set ``Authorization`` and, with a key, a fresh ``DPoP`` proof on ``request``.

        The retry transport calls it before every repeated attempt, because a proof's ``jti``
        is single use.

        Args:
            request: The outgoing request.
        """
        token = self._credentials.api_token
        key = self._credentials.dpop
        if key is None:
            request.headers["Authorization"] = f"Bearer {token.reveal()}"
            return
        request.headers["Authorization"] = f"DPoP {token.reveal()}"
        request.headers["DPoP"] = dpop_proof(
            key,
            htm=request.method,
            htu=str(request.url),
            token=token,
            nonce=self._nonce,
            clock=self._clock,
        )

    def auth_flow(self, request: httpx.Request) -> Generator[httpx.Request, httpx.Response, None]:
        """Send the request with credentials and handle one nonce challenge.

        Args:
            request: The outgoing request.

        Yields:
            The request, and once more after a ``use_dpop_nonce`` challenge.
        """
        self.apply(request)
        response = yield request
        self.remember_nonce(response.headers.get("DPoP-Nonce"))
        if (
            self._credentials.dpop is not None
            and response.status_code == httpx.codes.UNAUTHORIZED
            and _problem_code(response) == "use_dpop_nonce"
        ):
            self.apply(request)
            response = yield request
            self.remember_nonce(response.headers.get("DPoP-Nonce"))


def _problem_code(response: httpx.Response) -> str | None:
    try:
        body = response.json()
    except ValueError:
        return None
    code = body.get("code") if isinstance(body, dict) else None
    return code if isinstance(code, str) else None
