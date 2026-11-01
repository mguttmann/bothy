# SPDX-License-Identifier: Apache-2.0
"""The cost of a running run while its turn works (FIX-cost-display; 09 4.10, 18.7).

The pinned CLI states amounts (``costUSD``) only in a ``result``, at the end of a turn, and a
turn can work for an hour. The run's ``/cost`` therefore grows from two sources:

* At every ``result`` the runner settles on the exact amount of the run so far: the difference
  of the session values the run would book at its finish (`claude_code_runner.usage`).
* Between results every ``assistant`` message adds its ``message.usage`` priced with the start
  prices of the server (``prices.json``, a byte copy of
  ``services/server/internal/billing/domain/startprices/start_prices.json`` that a test holds
  equal). One message counts once with its newest usage (the CLI writes one line per content
  block); a model without a price adds nothing until the next ``result``.

The value is an estimate for display; ``run.finish`` books the reports from ``costUSD`` as
before and the server replaces the run's cost with the booked sum.

Examples:
    >>> base_model("claude-opus-5-5[1m]")
    'claude-opus-5-5'
"""

from __future__ import annotations

import json
import re
from collections.abc import Mapping
from dataclasses import dataclass, field
from importlib import resources
from typing import Any

__all__ = ["LiveCost", "PriceTable", "base_model", "load_prices"]

_MTOK = 1_000_000
_SUFFIX = re.compile(r"(\[[^\]]*\]|-\d{8})$")
# message.usage member, price unit.
_UNITS = (
    ("input_tokens", "input_mtok"),
    ("output_tokens", "output_mtok"),
    ("cache_read_input_tokens", "cache_read_mtok"),
)


def base_model(model: str) -> str:
    """Return the model name the price table uses: without a context or date suffix.

    Args:
        model: The model of an assistant message, for example ``claude-opus-5-5[1m]``.

    Returns:
        The name without ``[...]`` and without a trailing ``-YYYYMMDD``.
    """
    previous = None
    while previous != model:
        previous, model = model, _SUFFIX.sub("", model)
    return model


@dataclass(frozen=True)
class PriceTable:
    """Prices in micros of US dollars per million tokens by model and unit.

    Attributes:
        prices: ``(model, unit)`` to the price of the newest row.
    """

    prices: dict[tuple[str, str], int] = field(default_factory=dict)

    @classmethod
    def from_json(cls, raw: Mapping[str, Any]) -> PriceTable:
        """Read the start file (schema ``start_prices.v1``); rows in other currencies are left out.

        Args:
            raw: The decoded file.

        Returns:
            The table with the row of the newest ``valid_from`` per model and unit.
        """
        newest: dict[tuple[str, str], tuple[str, int]] = {}
        rows = raw.get("rows")
        for row in rows if isinstance(rows, list) else []:
            if not isinstance(row, Mapping) or row.get("currency") != "USD":
                continue
            model, unit, price = row.get("model"), row.get("unit"), row.get("price_micros")
            since = str(row.get("valid_from", ""))
            if not isinstance(model, str) or not isinstance(unit, str):
                continue
            if not isinstance(price, int) or isinstance(price, bool) or price < 0:
                continue
            key = (model, unit)
            if key not in newest or since > newest[key][0]:
                newest[key] = (since, price)
        return cls({k: v[1] for k, v in newest.items()})

    def price(self, model: str, unit: str) -> int | None:
        """Return the price of ``unit`` for ``model``, or ``None`` without one."""
        return self.prices.get((model, unit))


def load_prices() -> PriceTable:
    """Read the packaged price file.

    Returns:
        The table.
    """
    text = resources.files("claude_code_runner").joinpath("prices.json").read_text("utf-8")
    return PriceTable.from_json(json.loads(text))


def _count(value: Any) -> int:
    return value if isinstance(value, int) and not isinstance(value, bool) and value > 0 else 0


class LiveCost:
    """The run's cost so far: the settled amount plus the priced messages since.

    Args:
        prices: The price table.
    """

    def __init__(self, prices: PriceTable) -> None:
        """Start at zero."""
        self._prices = prices
        self._base = 0
        self._open: dict[str, int] = {}
        self._settled: set[str] = set()
        self._lines = 0

    @property
    def amount(self) -> int:
        """The estimate in micros of US dollars."""
        return self._base + sum(self._open.values())

    def assistant(self, message: Mapping[str, Any]) -> bool:
        """Count one ``assistant`` line.

        Args:
            message: The stream message.

        Returns:
            ``True`` when the amount changed.
        """
        inner = message.get("message")
        if not isinstance(inner, Mapping):
            return False
        usage, model = inner.get("usage"), inner.get("model")
        if not isinstance(usage, Mapping) or not isinstance(model, str):
            return False
        micros = self._price(base_model(model), usage)
        msg_id = inner.get("id")
        if isinstance(msg_id, str) and msg_id:
            if msg_id in self._settled:
                return False
            key = msg_id
        else:
            self._lines += 1
            key = f"line-{self._lines}"
        before = self._open.get(key, 0)
        if micros == before:
            return False
        self._open[key] = micros
        return True

    def settle(self, micros: int) -> None:
        """Take the exact amount of the run at a ``result``; the counted messages are in it.

        Args:
            micros: The run's amount so far, in micros of US dollars.
        """
        self._base = max(micros, 0)
        self._settled.update(self._open)
        self._open.clear()

    def _price(self, model: str, usage: Mapping[str, Any]) -> int:
        total = 0
        for member, unit in _UNITS:
            tokens = _count(usage.get(member))
            if tokens:
                price = self._prices.price(model, unit)
                if price is None:
                    return 0
                total += tokens * price
        written = _count(usage.get("cache_creation_input_tokens"))
        split = usage.get("cache_creation")
        hour = _count(split.get("ephemeral_1h_input_tokens")) if isinstance(split, Mapping) else 0
        hour = min(hour, written)
        for tokens, unit in ((written - hour, "cache_write_mtok"), (hour, "cache_write_1h_mtok")):
            if tokens:
                price = self._prices.price(model, unit)
                if price is None:
                    return 0
                total += tokens * price
        return total // _MTOK
