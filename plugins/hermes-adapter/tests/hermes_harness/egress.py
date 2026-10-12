# SPDX-License-Identifier: Apache-2.0
"""The egress recorder: an HTTP proxy that refuses everything and records the destination.

Every proxy variable of the Hermes process tree points here. A ``CONNECT`` or an
absolute-URI request gets ``403`` and is never forwarded; only ``host:port`` and the phase
(``start`` before the first scenario step, ``turn`` after it) are kept, never a path, header
or body.

Example:
    >>> with EgressRecorder() as egress:
    ...     env = build_environment(homes, "probe", egress.url, {}, venv_bin=bin_dir)
"""

from __future__ import annotations

import threading
from urllib.parse import urlsplit

from hermes_harness._server import LoopbackServer, QuietHandler

#: Destination recorded for a request that names no host (origin form).
ORIGIN_FORM = "<origin-form>"
_DEFAULT_PORTS = {"http": 80, "https": 443}


def _destination(method: str, target: str) -> str:
    if method == "CONNECT":
        host, _, port = target.rpartition(":")
        return f"{host}:{port}" if host and port.isdigit() else ORIGIN_FORM
    parts = urlsplit(target)
    if not parts.hostname:
        return ORIGIN_FORM
    number = parts.port or _DEFAULT_PORTS.get(parts.scheme, 80)
    return f"{parts.hostname}:{number}"


class _Handler(QuietHandler):
    def _refuse(self) -> None:
        recorder: EgressRecorder = self.server.owner  # type: ignore[attr-defined]
        recorder.record(_destination(self.command, self.path))
        self.reply(403, b"refused by the recording harness\n")

    do_CONNECT = _refuse  # noqa: N815 - http.server dispatch names
    do_GET = _refuse  # noqa: N815
    do_POST = _refuse  # noqa: N815
    do_PUT = _refuse  # noqa: N815
    do_PATCH = _refuse  # noqa: N815
    do_DELETE = _refuse  # noqa: N815
    do_HEAD = _refuse  # noqa: N815
    do_OPTIONS = _refuse  # noqa: N815


class EgressRecorder(LoopbackServer):
    """Refusing HTTP proxy on ``127.0.0.1`` port 0.

    Attributes:
        phase: ``"start"`` until the harness sets ``"turn"`` before the first scenario step and
            ``"shutdown"`` before it stops the gateway.
    """

    def __init__(self) -> None:
        """Bind the proxy socket."""
        super().__init__(_Handler)
        self.phase = "start"
        self._lock = threading.Lock()
        self._seen: list[tuple[str, str]] = []

    def record(self, destination: str) -> None:
        """Keep one attempted destination with the current phase."""
        with self._lock:
            self._seen.append((destination, self.phase))

    def seen(self) -> list[tuple[str, str]]:
        """All attempts in arrival order as ``(host:port, phase)``."""
        with self._lock:
            return list(self._seen)

    def __repr__(self) -> str:
        """Name the port and the number of attempts, never a destination."""
        return f"EgressRecorder(port={self.port}, attempts={len(self._seen)})"
