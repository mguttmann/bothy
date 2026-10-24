# SPDX-License-Identifier: Apache-2.0
"""Defaults of the adapter's configuration keys, in a module without imports.

The plugin entry (`bothy_hermes_adapter.entry`) needs the message limit when Hermes loads the
plugin and must stay free of the SDK and of Hermes at that moment; `bothy_hermes_adapter.config`
re-exports these values.
"""

from typing import Final

__all__ = ["DEFAULT_ATTACHMENT_TEXT_MAX_BYTES", "DEFAULT_MAX_MESSAGE_BYTES"]

DEFAULT_MAX_MESSAGE_BYTES: Final = 262144
"""Default of `hermes_adapter.max_message_bytes`: the default of ``message.max_content_bytes``."""

DEFAULT_ATTACHMENT_TEXT_MAX_BYTES: Final = 32768
"""Default of `hermes_adapter.attachment_text_max_bytes`: the default of
``agent.context_message_max_bytes``."""
