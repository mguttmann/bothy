# SPDX-License-Identifier: Apache-2.0
"""Python SDK of the public API (docs/spec/11-extensions.md section 11).

The generated core lives in `bothy_sdk.generated` (openapi-python-client, regenerated with
``make -C sdk/python generate``, never edited by hand). Importing any part of the SDK runs
`bothy_sdk.tolerance.install`, which makes the core accept what a newer server may send
(gate G5). The rest of the hand layer (auth, gateway, invocation, runs, retries, webhooks,
testing) comes with M2-29.

Example:
    >>> from bothy_sdk.generated.client import Client
    >>> from bothy_sdk.generated.api.sync import list_events
    >>> client = Client(base_url="https://example.com/api/v1")
    >>> callable(list_events.sync)
    True
"""

from bothy_sdk import tolerance

__all__ = ["__version__", "tolerance"]

__version__ = "0.0.0"
"""Version of this SDK; equal to the ``VERSION`` file of the component."""

tolerance.install()
