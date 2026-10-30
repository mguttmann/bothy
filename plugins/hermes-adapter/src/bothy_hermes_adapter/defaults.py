# SPDX-License-Identifier: Apache-2.0
"""Defaults of the adapter's configuration keys, in a module without imports.

The plugin entry (`bothy_hermes_adapter.entry`) needs the message length when Hermes loads the
plugin and must stay free of the SDK and of Hermes at that moment; `bothy_hermes_adapter.config`
re-exports these values.
"""

from typing import Final

__all__ = ["DEFAULT_ATTACHMENT_TEXT_MAX_BYTES", "DEFAULT_MAX_MESSAGE_BYTES", "NO_SPLIT_LENGTH"]

DEFAULT_MAX_MESSAGE_BYTES: Final = 262144
"""Default of `hermes_adapter.max_message_bytes`: the default of ``message.max_content_bytes``."""

NO_SPLIT_LENGTH: Final = 2**31 - 1
"""``MAX_MESSAGE_LENGTH`` and ``max_message_length`` the adapter gives Hermes (M4-47): so large
that Hermes never splits or truncates an answer; the adapter applies
`hermes_adapter.max_message_bytes` itself and sends the rest of a longer answer as one text
attachment of the same message (D-M4-47-5)."""

DEFAULT_ATTACHMENT_TEXT_MAX_BYTES: Final = 32768
"""Default of `hermes_adapter.attachment_text_max_bytes`: the default of
``agent.context_message_max_bytes``."""
