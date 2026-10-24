# SPDX-License-Identifier: Apache-2.0
"""HTTP plumbing shared by both clients: an opener without redirects and a request pacer.

The opener follows no redirect: a `3xx` answer is an error, so a header with the admin key or the
app token never goes to another host (SEC-NET-4 applied to the app). It also ignores proxy
settings of the environment and verifies TLS certificates with the system's trust store.
"""

from __future__ import annotations

import http.client
import ssl
import urllib.request
from collections.abc import Callable
from dataclasses import dataclass
from email.message import Message
from typing import IO, NoReturn
from urllib.error import HTTPError, URLError


class StopRequestedError(Exception):
    """A stop was requested (SIGTERM or SIGINT); the pass ends before its next request."""


class TransportError(Exception):
    """The request did not produce an HTTP answer (connection, TLS or time limit).

    The message is fixed; it never holds the address, a header or a body.
    """


class _RefuseRedirects(urllib.request.HTTPRedirectHandler):
    """Turns every redirect into an error instead of following it."""

    def redirect_request(
        self,
        req: urllib.request.Request,
        fp: IO[bytes],
        code: int,
        msg: str,
        headers: Message,
        newurl: str,
    ) -> NoReturn:
        """Refuse the redirect.

        Raises:
            HTTPError: Always, with the redirect status, so the caller sees a `3xx` error.
        """
        del newurl
        raise HTTPError(req.full_url, code, msg, headers, fp)


def build_opener() -> urllib.request.OpenerDirector:
    """Return an opener for `http` and `https` that follows no redirect and uses no proxy.

    Returns:
        The opener; `open` raises `HTTPError` for every status outside `2xx`.
    """
    opener = urllib.request.OpenerDirector()
    for handler in (
        urllib.request.UnknownHandler(),
        urllib.request.HTTPHandler(),
        # The default context checks the certificate and the host name (test_transport.py);
        # SonarCloud S4830 does not see that through OpenerDirector (D-M2-28-14).
        urllib.request.HTTPSHandler(context=ssl.create_default_context()),  # NOSONAR
        urllib.request.HTTPDefaultErrorHandler(),
        _RefuseRedirects(),
        urllib.request.HTTPErrorProcessor(),
    ):
        opener.add_handler(handler)
    return opener


@dataclass(frozen=True)
class Answer:
    """An HTTP answer.

    Attributes:
        status: The status code.
        body: The body; empty when the caller asked not to read it.
    """

    status: int
    body: bytes


def send(
    opener: urllib.request.OpenerDirector,
    request: urllib.request.Request,
    timeout_s: float,
    *,
    read_error_body: bool,
) -> Answer:
    """Send one request.

    Args:
        opener: The opener of `build_opener`.
        request: The request.
        timeout_s: Time limit in seconds (`reconciler.http_timeout`).
        read_error_body: Whether to read the body of an answer outside `2xx`; the cost API's
            error bodies are never read, the server's problem documents are.

    Returns:
        The answer, for every status.

    Raises:
        TransportError: No HTTP answer arrived.
    """
    try:
        with opener.open(request, timeout=timeout_s) as response:
            return Answer(response.status, response.read())
    except HTTPError as err:
        try:
            body = err.read() if read_error_body else b""
        except OSError:
            body = b""
        finally:
            err.close()
        return Answer(err.code, body)
    except (URLError, OSError, ValueError, http.client.HTTPException):
        raise TransportError("the request got no answer") from None


class Pacer:
    """Keeps at least the interval between the starts of two requests.

    The clock is monotonic and, like the sleep function, injectable, so tests never wait. One
    pacer serves the whole process, so the interval also holds across passes.
    """

    def __init__(
        self,
        interval_s: float,
        monotonic: Callable[[], float],
        sleep: Callable[[float], object],
        stopped: Callable[[], bool] = lambda: False,
    ) -> None:
        """Create the pacer.

        Args:
            interval_s: `reconciler.min_request_interval` in seconds.
            monotonic: A monotonic clock in seconds.
            sleep: Waits the given seconds; may return early when a stop is requested.
            stopped: Reports whether a stop was requested.
        """
        self._interval = interval_s
        self._monotonic = monotonic
        self._sleep = sleep
        self._stopped = stopped
        self._last: float | None = None

    def wait(self) -> None:
        """Wait until the next request may start and note its start.

        Raises:
            StopRequestedError: A stop was requested before or during the wait.
        """
        if self._last is not None:
            remaining = self._last + self._interval - self._monotonic()
            if remaining > 0:
                self._sleep(remaining)
        if self._stopped():
            raise StopRequestedError
        self._last = self._monotonic()
