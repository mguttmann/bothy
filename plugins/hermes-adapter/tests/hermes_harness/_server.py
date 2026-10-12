# SPDX-License-Identifier: Apache-2.0
"""A threaded HTTP server on ``127.0.0.1`` port 0, shared by the harness endpoints.

Tests bind only the loopback address and let the system pick the port (P-10); the endpoints
of the harness (egress recorder, fake model, control channel) never bind anything else.
"""

from __future__ import annotations

import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from types import TracebackType
from typing import Self

LOOPBACK = "127.0.0.1"


class _Server(ThreadingHTTPServer):
    daemon_threads = True
    allow_reuse_address = False


class LoopbackServer:
    """Run a request handler class on ``127.0.0.1`` with a system-chosen port.

    Subclasses pass their handler class; the handler reaches the owning object through
    ``self.server.owner``. Use as a context manager or call :meth:`start` and :meth:`close`.
    """

    def __init__(self, handler: type[BaseHTTPRequestHandler]) -> None:
        """Bind the socket; serving starts with :meth:`start`.

        Args:
            handler: The request handler class.
        """
        self._server = _Server((LOOPBACK, 0), handler)
        self._server.owner = self  # type: ignore[attr-defined]
        self._thread: threading.Thread | None = None

    @property
    def port(self) -> int:
        """The bound port."""
        return int(self._server.server_address[1])

    @property
    def url(self) -> str:
        """``http://127.0.0.1:<port>``."""
        return f"http://{LOOPBACK}:{self.port}"

    def start(self) -> Self:
        """Serve in a daemon thread; returns ``self``."""
        if self._thread is None:
            self._thread = threading.Thread(target=self._server.serve_forever, daemon=True)
            self._thread.start()
        return self

    def close(self) -> None:
        """Stop serving and release the socket."""
        if self._thread is not None:
            self._server.shutdown()
            self._thread.join()
            self._thread = None
        self._server.server_close()

    def __enter__(self) -> Self:
        """Start serving."""
        return self.start()

    def __exit__(
        self,
        exc_type: type[BaseException] | None,
        exc: BaseException | None,
        tb: TracebackType | None,
    ) -> None:
        """Stop serving."""
        self.close()


class QuietHandler(BaseHTTPRequestHandler):
    """Request handler without access log lines (nothing of a request reaches a log)."""

    protocol_version = "HTTP/1.1"

    def log_message(self, format: str, *args: object) -> None:
        """Write nothing."""

    def reply(self, status: int, body: bytes = b"", content_type: str = "text/plain") -> None:
        """Send a complete response with a body and close the connection."""
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Connection", "close")
        self.end_headers()
        if body:
            self.wfile.write(body)
        self.close_connection = True

    def read_body(self) -> bytes:
        """Read the request body announced by ``Content-Length``."""
        length = int(self.headers.get("Content-Length") or 0)
        return self.rfile.read(length) if length > 0 else b""
