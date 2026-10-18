# SPDX-License-Identifier: Apache-2.0
"""Reproduction of a failed G7 case: a ``curl`` command without credentials, and the seed.

``Authorization`` becomes ``"$BOTHY_CONTRACT_AUTH"`` and ``DPoP`` a fresh proof from the command
line of :mod:`dpop`, so the printed command never contains a token or a proof.
"""

from __future__ import annotations

import shlex
from collections.abc import Mapping
from pathlib import Path

AUTH_PLACEHOLDER = '"$BOTHY_CONTRACT_AUTH"'


def _proof_command(key_file: Path | str, method: str, url: str) -> str:
    return (
        '"$(PYTHONPATH=tools/contract uv run --project tools python -m dpop '
        f"--key {shlex.quote(str(key_file))} "
        f'--htm {shlex.quote(method.upper())} --htu {shlex.quote(url)})"'
    )


def curl_command(
    method: str,
    url: str,
    headers: Mapping[str, str],
    body: bytes | str | None,
    *,
    key_file: Path | str,
) -> str:
    """Return a ``curl`` command for the request, credentials masked.

    Args:
        method: HTTP method.
        url: full request URL.
        headers: request headers; ``Authorization`` and ``DPoP`` are replaced.
        body: request body, or None.
        key_file: file with the private DPoP JWK of the pass.
    """
    parts = ["curl", "-sS", "-X", shlex.quote(method.upper()), shlex.quote(url)]
    for name, value in headers.items():
        lower = name.lower()
        if lower == "authorization":
            parts += ["-H", f'"Authorization: "{AUTH_PLACEHOLDER}']
        elif lower == "dpop":
            parts += ["-H", f'"DPoP: "{_proof_command(key_file, method, url)}']
        elif lower in {"content-length", "host"}:
            continue
        else:
            parts += ["-H", shlex.quote(f"{name}: {value}")]
    if body:
        text = body.decode("utf-8", "replace") if isinstance(body, bytes) else body
        parts += ["--data-binary", shlex.quote(text)]
    return " ".join(parts)


def failure_report(reason: str, curl: str, seed: int | str) -> str:
    """Return the message of a failure: reason, reproduction command and seed."""
    return (
        f"{reason}\n\nReproduce (run from the repository root):\n  {curl}\n"
        f"Seed: CONTRACT_SEED={seed}"
    )
