# SPDX-License-Identifier: Apache-2.0
"""The cost of a running run grows during its turn (FIX-cost-display, 09 4.10 and 18.7)."""

from __future__ import annotations

from pathlib import Path
from typing import Any

from claude_code_runner.livecost import LiveCost, PriceTable, base_model, load_prices

REPO = Path(__file__).resolve().parents[4]
SERVER_START_FILE = REPO / "services/server/internal/billing/domain/startprices/start_prices.json"


def assistant(
    msg_id: str | None,
    model: str = "claude-opus-5-5[1m]",
    **usage: Any,
) -> dict[str, Any]:
    inner: dict[str, Any] = {"model": model, "role": "assistant", "content": [], "usage": usage}
    if msg_id is not None:
        inner["id"] = msg_id
    return {"type": "assistant", "parent_tool_use_id": None, "message": inner}


def test_cost_grows_with_every_assistant_message() -> None:
    live = LiveCost(load_prices())
    assert live.amount == 0
    # Opus 5.5: input 4, cache write 5, cache read 0.2, output 20 USD per million tokens.
    assert live.assistant(
        assistant("msg_1", input_tokens=8, cache_read_input_tokens=100_000, output_tokens=500)
    )
    first = live.amount
    assert first == (8 * 4_000_000 + 100_000 * 200_000 + 500 * 20_000_000) // 1_000_000
    assert live.assistant(
        assistant("msg_2", cache_creation_input_tokens=2_000, output_tokens=1_000)
    )
    assert live.amount > first
    assert live.amount == first + (2_000 * 5_000_000 + 1_000 * 20_000_000) // 1_000_000


def test_one_message_counts_once_with_its_newest_usage() -> None:
    # The CLI writes one line per content block of a message, all with the message's usage.
    live = LiveCost(load_prices())
    live.assistant(assistant("msg_1", output_tokens=100))
    once = live.amount
    assert not live.assistant(assistant("msg_1", output_tokens=100))
    assert live.amount == once
    assert live.assistant(assistant("msg_1", output_tokens=300))
    assert live.amount == 300 * 20


def test_a_result_settles_on_the_exact_amount_and_counts_on_from_it() -> None:
    live = LiveCost(load_prices())
    live.assistant(assistant("msg_1", output_tokens=10_000))
    live.settle(250_000)
    assert live.amount == 250_000
    live.assistant(assistant("msg_2", output_tokens=1_000))
    assert live.amount == 250_000 + 20_000
    # The message of the settled turn read again (a recovery) adds nothing.
    live.assistant(assistant("msg_1", output_tokens=10_000))
    assert live.amount == 270_000


def test_unknown_model_and_bad_usage_add_nothing() -> None:
    live = LiveCost(load_prices())
    assert not live.assistant(assistant("m1", model="<synthetic>", output_tokens=1_000))
    assert not live.assistant(assistant("m2", model="model-large", output_tokens=1_000))
    assert not live.assistant(assistant("m3", output_tokens=-5, input_tokens=True))
    assert not live.assistant({"type": "assistant", "message": "text"})
    assert live.amount == 0


def test_one_hour_cache_writes_take_their_price() -> None:
    live = LiveCost(load_prices())
    live.assistant(
        assistant(
            "m1",
            cache_creation_input_tokens=3_000,
            cache_creation={"ephemeral_5m_input_tokens": 1_000, "ephemeral_1h_input_tokens": 2_000},
        )
    )
    assert live.amount == (1_000 * 5_000_000 + 2_000 * 8_000_000) // 1_000_000


def test_message_without_id_counts_each_line() -> None:
    live = LiveCost(load_prices())
    live.assistant(assistant(None, output_tokens=100))
    live.assistant(assistant(None, output_tokens=100))
    assert live.amount == 2 * 100 * 20


def test_base_model_drops_context_and_date_suffixes() -> None:
    assert base_model("claude-opus-5-5[1m]") == "claude-opus-5-5"
    assert base_model("claude-haiku-5-5-20261001") == "claude-haiku-5-5"
    assert base_model("claude-sonnet-5-5") == "claude-sonnet-5-5"


def test_price_file_is_the_server_start_file() -> None:
    # One home for prices: the runner's copy is the server's start file byte for byte.
    packaged = Path(__file__).resolve().parents[2] / "src/claude_code_runner/prices.json"
    assert packaged.read_bytes() == SERVER_START_FILE.read_bytes()


def test_price_table_reads_usd_rows_only() -> None:
    table = PriceTable.from_json(
        {
            "schema": "start_prices.v1",
            "rows": [
                {
                    "model": "m",
                    "unit": "output_mtok",
                    "price_micros": 1_000_000,
                    "currency": "USD",
                    "valid_from": "2026-01-01T00:00:00Z",
                },
                {
                    "model": "m",
                    "unit": "output_mtok",
                    "price_micros": 2_000_000,
                    "currency": "USD",
                    "valid_from": "2026-06-01T00:00:00Z",
                },
                {
                    "model": "e",
                    "unit": "output_mtok",
                    "price_micros": 9,
                    "currency": "EUR",
                    "valid_from": "2026-01-01T00:00:00Z",
                },
            ],
        }
    )
    assert table.price("m", "output_mtok") == 2_000_000
    assert table.price("e", "output_mtok") is None
