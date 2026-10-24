# SPDX-License-Identifier: Apache-2.0
"""Tests of the transport: the opener checks the server certificate and the host name."""

from __future__ import annotations

import ssl
import urllib.request

from cost_reconciler.transport import build_opener


def test_https_handler_verifies_certificate_and_host_name() -> None:
    # typeshed declares neither attribute; vars reads them as the opener stores them.
    opener = vars(build_opener())
    handlers = [h for h in opener["handlers"] if isinstance(h, urllib.request.HTTPSHandler)]
    assert len(handlers) == 1
    context = vars(handlers[0])["_context"]
    assert isinstance(context, ssl.SSLContext)
    assert context.verify_mode == ssl.CERT_REQUIRED
    assert context.check_hostname is True
