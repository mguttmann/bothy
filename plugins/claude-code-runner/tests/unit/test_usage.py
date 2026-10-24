# SPDX-License-Identifier: Apache-2.0
"""Cost of a run as the difference of cumulative session values (09 18.7, 4.10, note 14)."""

from __future__ import annotations

from decimal import Decimal
from pathlib import Path
from typing import Any

from bothy_sdk.generated.models import CostReport

from claude_code_runner.usage import (
    ModelUsage,
    UsageStore,
    context_windows,
    cost_reports,
    counters_reset,
    difference,
    metrics_totals,
    parse_model_usage,
    subtract_baseline,
    to_micros,
)


def result(models: dict[str, tuple[int, int, int, int, float]]) -> dict[str, Any]:
    return {
        "type": "result",
        "modelUsage": {
            name: {
                "inputTokens": v[0],
                "outputTokens": v[1],
                "cacheReadInputTokens": v[2],
                "cacheCreationInputTokens": v[3],
                "costUSD": v[4],
                "contextWindow": 200000,
            }
            for name, v in models.items()
        },
    }


def test_first_run_reports_full_values() -> None:
    current = parse_model_usage(result({"model-large": (100, 20, 5, 7, 0.0125)}))
    diff = difference({}, current)
    reports = [r.to_dict() for r in cost_reports(diff)]
    assert reports == [
        {
            "provider": "anthropic",
            "model": "model-large",
            "tokens_in": 100,
            "tokens_out": 20,
            "tokens_cache_read": 5,
            "tokens_cache_write": 7,
            "amount_micros": 12500,
            "currency": "USD",
        }
    ]
    assert metrics_totals(diff) == {
        "tokens_in": 100,
        "tokens_out": 20,
        "tokens_cache_read": 5,
        "tokens_cache_write": 7,
    }


def test_resumed_run_reports_difference_and_new_model() -> None:
    before = parse_model_usage(result({"model-large": (100, 20, 5, 7, 0.0125)}))
    now = parse_model_usage(
        result({"model-large": (250, 50, 15, 7, 0.0300), "model-small": (10, 2, 0, 0, 0.0001)})
    )
    diff = difference(before, now)
    assert diff["model-large"] == ModelUsage(150, 30, 10, 0, Decimal("0.0175"))
    assert diff["model-small"] == ModelUsage(10, 2, 0, 0, Decimal("0.0001"))


def test_negative_difference_resets() -> None:
    before = parse_model_usage(result({"model-large": (500, 20, 5, 7, 0.5)}))
    now = parse_model_usage(result({"model-large": (40, 30, 5, 7, 0.6)}))
    assert difference(before, now)["model-large"] == now["model-large"]


def test_micros_half_even() -> None:
    assert to_micros(Decimal("0.0000005")) == 0
    assert to_micros(Decimal("0.0000015")) == 2
    assert to_micros(Decimal("0.0000025")) == 2
    assert to_micros(Decimal("1.25")) == 1250000
    # The float of the stream is read as its decimal text, never as a binary fraction.
    assert parse_model_usage(result({"m": (0, 0, 0, 0, 0.1)}))["m"].cost_usd == Decimal("0.1")


def test_context_windows() -> None:
    assert context_windows(result({"m": (0, 0, 0, 0, 0.0)})) == {"m": 200000}
    assert parse_model_usage({"type": "result"}) == {}


def test_store_pending_reused_and_replaced(tmp_path: Path) -> None:
    store = UsageStore(tmp_path / "sessions" / "s")
    assert store.load().values == {}
    first = parse_model_usage(result({"m": (10, 1, 0, 0, 0.001)}))
    reports = cost_reports(difference(store.load().values, first))
    store.record("run-1", first, reports, {"m": 200000})
    assert (tmp_path / "sessions" / "s" / "usage.json").stat().st_mode & 0o777 == 0o600
    again = store.pending_for("run-1")
    assert again is not None
    assert [r.to_dict() for r in again] == [r.to_dict() for r in reports]
    assert store.pending_for("run-2") is None
    assert store.load().values == first
    assert store.load().context_windows == {"m": 200000}
    second = parse_model_usage(result({"m": (30, 3, 0, 0, 0.003)}))
    reports2 = cost_reports(difference(store.load().values, second))
    store.record("run-2", second, reports2, {"m": 200000})
    assert store.pending_for("run-1") is None
    pending = store.pending_for("run-2")
    assert pending is not None
    assert pending[0].tokens_in == 20


def _reports(amounts: dict[str, int]) -> list[Any]:
    return [
        CostReport(provider="anthropic", model=m, currency="USD", tokens_in=7, amount_micros=a)
        for m, a in sorted(amounts.items())
    ]


def test_subtract_baseline_proportional_and_never_negative() -> None:
    out, rest = subtract_baseline(_reports({"model-a": 9000, "model-b": 3000}), 10000)
    assert {r.model: r.amount_micros for r in out} == {"model-a": 1500, "model-b": 500}
    assert rest == 0
    assert [r.tokens_in for r in out] == [7, 7]
    out, rest = subtract_baseline(_reports({"model-a": 1, "model-b": 1, "model-c": 1}), 2)
    assert sum(r.amount_micros for r in out) == 1
    assert all(r.amount_micros >= 0 for r in out)
    assert rest == 0
    out, rest = subtract_baseline(_reports({"model-a": 30}), 50)
    assert (out[0].amount_micros, rest) == (0, 20)
    out, rest = subtract_baseline(_reports({"model-a": 30}), 0)
    assert (out[0].amount_micros, rest) == (30, 0)


def test_counters_reset() -> None:
    before = parse_model_usage(result({"model-large": (500, 20, 5, 7, 0.5)}))
    assert counters_reset(before, parse_model_usage(result({"model-large": (40, 30, 5, 7, 0.6)})))
    assert not counters_reset(
        before, parse_model_usage(result({"model-large": (600, 30, 5, 7, 0.6)}))
    )
    assert not counters_reset({}, before)


def test_store_keeps_the_remaining_baseline(tmp_path: Path) -> None:
    store = UsageStore(tmp_path / "sessions" / "s")
    assert store.load().baseline_remaining is None
    current = parse_model_usage(result({"model-large": (1, 1, 0, 0, 0.001)}))
    store.record("run-1", current, [], {}, 1234)
    assert store.load().baseline_remaining == 1234
    store.record("run-2", current, [], {})
    assert store.load().baseline_remaining is None
