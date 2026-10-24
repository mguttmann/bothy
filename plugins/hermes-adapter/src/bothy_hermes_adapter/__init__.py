# SPDX-License-Identifier: Apache-2.0
"""Reference adapter for Hermes Agent (docs/spec/09-agents.md section 17).

A Hermes platform plugin that connects one Hermes profile to the server as one agent principal.
Only ``platform`` (the runtime) and ``hermes_config`` (the configuration bridge of the installer
and the credential resolution) import Hermes; every other module depends on the protocol
`bothy_hermes_adapter.runtime.Runtime` and runs without Hermes (09 17.2, docs/DECISIONS.md
D-M2-33-1).

Examples:
    >>> __version__
    '0.0.0'
"""

__all__ = ["__version__"]

__version__ = "0.0.0"
"""Version of this component; equal to its ``VERSION`` file (a test holds both equal)."""
