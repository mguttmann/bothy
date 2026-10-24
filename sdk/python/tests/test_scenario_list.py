# SPDX-License-Identifier: Apache-2.0
"""The scenario functions equal the shared list of both SDKs (docs/decisions.d/M2-30.md D-M2-30-1).

`sdk/testdata/scenarios/README.md` names every scenario under its Python name as a level-3
heading (`### ag08_invalid_tokens`); the TypeScript SDK compares the camelCase form of the same
headings in `scenarios.test.ts`. A scenario added to one SDK only fails one of the two tests.
"""

from __future__ import annotations

import inspect
import re
from pathlib import Path

from bothy_sdk.testing import scenarios

README = Path(__file__).resolve().parents[2] / "testdata" / "scenarios" / "README.md"
_HEADING = re.compile(r"^### (?P<name>\S+)\s*$", re.MULTILINE)


def listed() -> list[str]:
    """The scenario names of the shared README, in their order."""
    return _HEADING.findall(README.read_text(encoding="utf-8"))


def implemented() -> set[str]:
    """The public scenario functions: the coroutine functions exported by the module."""
    return {
        name
        for name in scenarios.__all__
        if inspect.iscoroutinefunction(getattr(scenarios, name, None))
    }


def test_readme_lists_each_scenario_once() -> None:
    names = listed()
    assert names, "no scenario heading found"
    assert len(names) == len(set(names))


def test_scenario_functions_equal_the_shared_list() -> None:
    assert implemented() == set(listed())
