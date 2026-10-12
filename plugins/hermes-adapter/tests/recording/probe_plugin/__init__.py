# SPDX-License-Identifier: Apache-2.0
"""Package entry of the probe plugin; the pinned Hermes loads a directory plugin from here."""

from .probe import register

__all__ = ["register"]
