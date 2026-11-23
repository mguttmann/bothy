# SPDX-License-Identifier: Apache-2.0
"""The tiers of docs/spec/02-architecture.md 4.4 follow the module register.

02 4.4 names the modules of the server in tiers from the bottom and says that every edge points
downward. The edges themselves live in `services/server/architecture/modules.yaml` (the key
`depends_on` of each module, 02 7.1), and `make arch` enforces them in the code; nothing held the
tier line to them, so a module could depend on one of a higher tier while the chapter still said
otherwise (audit finding architecture-9: `status` uses `stats` and `search` and stood below both).

This test reads both files: every module of the register stands in exactly one tier, the line
names no other module, and every module of the register uses only modules of lower tiers.

The test lives with the tools tests because they run on every change, also on one that touches
only a document.
"""

from __future__ import annotations

import re
from pathlib import Path

import yaml

REPO = Path(__file__).resolve().parents[3]
CHAPTER = REPO / "docs" / "spec" / "02-architecture.md"
REGISTER = REPO / "services" / "server" / "architecture" / "modules.yaml"

_TIERS = re.compile(r"Tiers \(from the bottom\):(?P<body>.*?)Every edge points downward", re.DOTALL)


def tiers_of(text: str) -> list[list[str]]:
    """Return the tiers of the tier line, from the bottom, each as its module names.

    Example: `tiers_of("Tiers (from the bottom): `a`, `b` · `c`. Every edge points downward.")`
    returns `[["a", "b"], ["c"]]`.
    """
    match = _TIERS.search(text)
    assert match is not None, "02 4.4 has no line 'Tiers (from the bottom): ...'"
    body = " ".join(match.group("body").split()).rstrip(". ")
    return [re.findall(r"`([a-z]+)`", tier) for tier in body.split("·")]


def problems(tiers: list[list[str]], register: dict[str, list[str]]) -> list[str]:
    """Return every difference between the tier line and the edges of the register.

    `register` maps a module to the modules it may use. Example: with the tiers `[["a"], ["b"]]`
    and the register `{"a": ["b"], "b": []}` the result names the edge from `a` to `b`.
    """
    out: list[str] = []
    level: dict[str, int] = {}
    for index, tier in enumerate(tiers):
        for name in tier:
            if name in level:
                out.append(f"{name} stands in more than one tier")
            level[name] = index
    for name in sorted(set(register) - set(level)):
        out.append(f"{name} is a module of the register and stands in no tier")
    for name in sorted(set(level) - set(register)):
        out.append(f"{name} stands in a tier and is no module of the register")
    for name, uses in register.items():
        for used in uses:
            if name in level and used in level and level[used] >= level[name]:
                out.append(f"{name} uses {used}, which does not stand in a lower tier")
    return out


def register_edges() -> dict[str, list[str]]:
    """Return the modules of the register with the modules each may use."""
    data = yaml.safe_load(REGISTER.read_text(encoding="utf-8"))
    return {name: list(entry.get("depends_on") or []) for name, entry in data["modules"].items()}


def test_the_tier_line_follows_the_register() -> None:
    tiers = tiers_of(CHAPTER.read_text(encoding="utf-8"))
    assert problems(tiers, register_edges()) == []


def test_the_tier_line_is_read() -> None:
    text = "Tiers (from the bottom): `a`, `b` ·\n`c` · `d`. Every edge points downward;"
    assert tiers_of(text) == [["a", "b"], ["c"], ["d"]]


def test_an_edge_to_the_same_or_a_higher_tier_is_named() -> None:
    tiers = [["a"], ["b", "c"]]
    register = {"a": ["b"], "b": ["c"], "c": ["a"]}
    assert problems(tiers, register) == [
        "a uses b, which does not stand in a lower tier",
        "b uses c, which does not stand in a lower tier",
    ]


def test_a_missing_or_unknown_module_is_named() -> None:
    assert problems([["a"], ["x"]], {"a": [], "b": ["a"]}) == [
        "b is a module of the register and stands in no tier",
        "x stands in a tier and is no module of the register",
    ]
