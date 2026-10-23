# SPDX-License-Identifier: Apache-2.0
"""Python SDK of the public API (docs/spec/11-extensions.md section 11).

The generated core lives in `bothy_sdk.generated` (openapi-python-client, regenerated with
``make -C sdk/python generate``, never edited by hand). Importing any part of the SDK runs
`bothy_sdk.tolerance.install` first, which makes the core accept what a newer server may send
(gate G5). The hand layer (M2-29) builds on it: `Adapter` and `AgentHandler` for agent
adapters, `Client` and `SyncClient` for REST, `render_context` for the run context with trust
marks, and `bothy_sdk.testing` with the FakeServer.

Examples:
    >>> from bothy_sdk.generated.client import Client
    >>> from bothy_sdk.generated.api.sync import list_events
    >>> client = Client(base_url="https://example.com/api/v1")
    >>> callable(list_events.sync)
    True
"""

from bothy_sdk import tolerance

tolerance.install()

# The hand layer imports the generated models, so it comes after install() (M2-23).
from bothy_sdk.adapter import (  # noqa: E402
    ActualBilling,
    Adapter,
    AdapterConfig,
    AgentHandler,
    Invocation,
    RuntimeFailure,
    StartOptions,
)
from bothy_sdk.auth import Credentials, FileDpopKey, Secret  # noqa: E402
from bothy_sdk.client import Client  # noqa: E402
from bothy_sdk.context import RenderedContext, render_context  # noqa: E402
from bothy_sdk.errors import ProblemError  # noqa: E402
from bothy_sdk.gateway import CloseReason  # noqa: E402
from bothy_sdk.runs import RunHandle, RunPatcher  # noqa: E402
from bothy_sdk.settings import SdkSettings  # noqa: E402
from bothy_sdk.sync import SyncClient  # noqa: E402

__all__ = [
    "ActualBilling",
    "Adapter",
    "AdapterConfig",
    "AgentHandler",
    "Client",
    "CloseReason",
    "Credentials",
    "FileDpopKey",
    "Invocation",
    "ProblemError",
    "RenderedContext",
    "RunHandle",
    "RunPatcher",
    "RuntimeFailure",
    "SdkSettings",
    "Secret",
    "StartOptions",
    "SyncClient",
    "__version__",
    "render_context",
    "tolerance",
]

__version__ = "0.0.0"
"""Version of this SDK; equal to the ``VERSION`` file of the component."""
