# SPDX-License-Identifier: Apache-2.0
"""Testing support of the SDK: FakeServer, faults, scenarios and doubles (extra ``testing``).

`FakeServer` and `Fault` need the optional extra ``testing`` (``aiohttp``) and are imported on
first use; the doubles and the scenario functions need only the SDK itself.

Examples:
    >>> from bothy_sdk.testing import FakeClock, fingerprint
    >>> len(fingerprint("example-token"))
    16
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from bothy_sdk.testing.doubles import (
    KEEP,
    REMOVE,
    FakeClock,
    RecordingHandler,
    RecordingSource,
    TamperingSource,
    fingerprint,
)
from bothy_sdk.testing.webhooks import sign_webhook, webhook_public_key

if TYPE_CHECKING:
    from bothy_sdk.testing.fakeserver import FakeServer, RecordedRequest
    from bothy_sdk.testing.faults import Fault

__all__ = [
    "KEEP",
    "REMOVE",
    "FakeClock",
    "FakeServer",
    "Fault",
    "RecordedRequest",
    "RecordingHandler",
    "RecordingSource",
    "TamperingSource",
    "fingerprint",
    "sign_webhook",
    "webhook_public_key",
]

_LAZY = {
    "FakeServer": "bothy_sdk.testing.fakeserver",
    "RecordedRequest": "bothy_sdk.testing.fakeserver",
    "Fault": "bothy_sdk.testing.faults",
}


def __getattr__(name: str) -> Any:
    """Import the aiohttp-based parts on first use."""
    module = _LAZY.get(name)
    if module is None:
        raise AttributeError(name)
    import importlib  # noqa: PLC0415 - lazy by design

    return getattr(importlib.import_module(module), name)
