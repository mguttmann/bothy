# SPDX-License-Identifier: Apache-2.0
"""No test of the SDK waits a fixed time or a fixed number of loop turns.

A wait bounded by a few seconds or a polling loop bounded by turns fails a correct program on a
loaded machine or a slow CI runner (the flaky `test_rate_limited_close_waits_retry_after` gave up
after 100 turns). Tests wait for a condition with `bothy_sdk.testing._patience`, whose only bound
is `quality.test.patience`.
"""

from __future__ import annotations

import ast
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SOURCES = sorted(
    [*ROOT.joinpath("tests").rglob("*.py"), *ROOT.joinpath("src/bothy_sdk/testing").glob("*.py")]
)


def _literal_number(node: ast.AST | None) -> bool:
    return isinstance(node, ast.Constant) and isinstance(node.value, int | float)


def _fixed_waits(tree: ast.AST) -> list[tuple[int, str]]:
    found: list[tuple[int, str]] = []
    for node in ast.walk(tree):
        if isinstance(node, ast.Call):
            func = node.func
            name = func.attr if isinstance(func, ast.Attribute) else getattr(func, "id", "")
            timeout = node.args[1] if len(node.args) > 1 else None
            for kw in node.keywords:
                if kw.arg == "timeout":
                    timeout = kw.value
            if name == "wait_for" and _literal_number(timeout):
                found.append((node.lineno, "wait_for with a fixed timeout"))
        if (
            isinstance(node, ast.For | ast.AsyncFor)
            and isinstance(node.iter, ast.Call)
            and getattr(node.iter.func, "id", "") == "range"
            and any(isinstance(n, ast.Await) for b in node.body for n in ast.walk(b))
            and any(isinstance(n, ast.Break | ast.Return) for b in node.body for n in ast.walk(b))
        ):
            found.append((node.lineno, "a polling loop of fixed turns"))
    return found


def test_no_fixed_waits() -> None:
    offenders = [
        f"{path.relative_to(ROOT)}:{line}: {what}"
        for path in SOURCES
        for line, what in _fixed_waits(ast.parse(path.read_text(encoding="utf-8")))
    ]
    assert offenders == []


def test_the_check_finds_both_forms() -> None:
    sample = ast.parse(
        "async def f(c):\n"
        "    await asyncio.wait_for(c(), 5)\n"
        "    for _ in range(100):\n"
        "        if c.done():\n"
        "            break\n"
        "        await asyncio.sleep(0.01)\n"
        "    for _ in range(3):\n"
        "        await c()\n"
    )
    assert [what for _, what in sorted(_fixed_waits(sample))] == [
        "wait_for with a fixed timeout",
        "a polling loop of fixed turns",
    ]
