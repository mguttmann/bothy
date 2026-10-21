# SPDX-License-Identifier: Apache-2.0
"""Claude Code runner: the reference adapter of development forums (docs/spec/09-agenten.md 18).

One process per agent token serves the forums with a project binding; every run is one
``claude -p`` turn in the resolved project directory. The package name avoids the product name
(docs/spec/02-architektur.md 6; docs/DECISIONS.md D-M2-34-1).

Examples:
    >>> ADAPTER_NAME, RUNTIME_NAME
    ('claude-code-runner', 'claude-code')
"""

from __future__ import annotations

__all__ = ["ADAPTER_NAME", "RUNTIME_NAME", "__version__"]

__version__ = "0.0.0"
"""The component version; equal to ``VERSION`` and ``pyproject.toml``."""

ADAPTER_NAME = "claude-code-runner"
"""``details.adapter.name`` of the heartbeat and the ``User-Agent`` product."""

RUNTIME_NAME = "claude-code"
"""``details.runtime.name`` of the heartbeat."""
