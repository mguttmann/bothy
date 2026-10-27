# SPDX-License-Identifier: Apache-2.0
"""Tests for the contrast check of the architecture graphics (light and dark rendering)."""

from __future__ import annotations

import json
from pathlib import Path

import diagram_contrast as dc
import render_diagrams as rd

LIGHT = {
    **{bg: "#fbfeff" for _, bg in dc.TEXT_PAIRS},
    **{fg: "#08191c" for fg, _ in dc.TEXT_PAIRS},
    **dict.fromkeys(dc.LINE_COLOURS, "#08191c"),
    "sequenceNumberColor": "#fbfeff",
}
SVG = '<svg style="background-color: rgb(251, 254, 255);"><style>#a{fill:#08191c;}</style></svg>'


def test_passing_theme_variables_give_no_problem() -> None:
    assert dc.check("a.mmd", "light", LIGHT, "flowchart LR\n", SVG) == []


def test_text_pair_below_the_gate_is_reported() -> None:
    variables = {**LIGHT, "noteTextColor": "#748486"}
    assert dc.check("a.mmd", "light", variables, "flowchart LR\n", SVG) == [
        "a.mmd [light] noteTextColor on noteBkgColor: 3.85 < 4.5 (#748486 on #fbfeff, text)"
    ]


def test_line_colour_below_the_gate_is_reported() -> None:
    variables = {**LIGHT, "clusterBorder": "#ccd9db"}
    assert dc.check("a.mmd", "light", variables, "flowchart LR\n", SVG) == [
        "a.mmd [light] clusterBorder on background: 1.43 < 3.0 (#ccd9db on #fbfeff, ui)"
    ]


def test_thresholds_come_from_the_token_gate() -> None:
    thresholds = dc.rules.THRESHOLDS
    expected = {dc.TEXT: thresholds["text"], dc.GRAPHICS: thresholds["ui"]}
    assert expected == dc.REQUIRED


def test_missing_theme_variable_is_reported() -> None:
    variables = dict(LIGHT)
    del variables["actorLineColor"]
    assert dc.check("a.mmd", "dark", variables, "flowchart LR\n", SVG) == [
        "a.mmd [dark] theme variable actorLineColor is missing"
    ]


def test_class_colour_below_the_gate_is_reported() -> None:
    source = (
        "flowchart LR\n"
        "  classDef ok fill:#136f69,stroke:#063b38,color:#fbfeff\n"
        "  classDef weak fill:#59696c,color:#748486\n"
        "  style N fill:#eddc59,color:#8d7716\n"
    )
    assert dc.check("a.mmd", "dark", LIGHT, source, SVG) == [
        "a.mmd [dark] classDef weak color on fill: 1.47 < 4.5 (#748486 on #59696c, text)",
        "a.mmd [dark] style N color on fill: 3.13 < 4.5 (#8d7716 on #eddc59, text)",
        "a.mmd [dark] style N outline (fill) on background: 1.38 < 3.0 (#eddc59 on #fbfeff, ui)",
    ]


def test_class_without_colour_uses_the_node_text_of_the_theme() -> None:
    source = "flowchart LR\n  classDef dim fill:#24363a\n"
    problems = dc.check("a.mmd", "light", LIGHT, source, SVG)
    assert problems == [
        "a.mmd [light] classDef dim primaryTextColor on fill: 1.43 < 4.5 (#08191c on #24363a, text)"
    ]


def test_node_outline_is_the_stronger_of_fill_and_stroke_against_the_canvas() -> None:
    # Dark fills with dark strokes vanish on the dark canvas; a mid stroke outlines them.
    dark = json.loads(
        (Path(__file__).resolve().parents[3] / "design/build/mermaid.dark.config.json").read_text(
            encoding="utf-8"
        )
    )["themeVariables"]
    source = (
        "flowchart LR\n"
        "  classDef lost fill:#063b38,stroke:#08191c,color:#fbfeff\n"
        "  classDef seen fill:#063b38,stroke:#1b8a83,color:#fbfeff\n"
    )
    assert dc.check("a.mmd", "dark", dark, source, None) == [
        "a.mmd [dark] classDef lost outline (fill) on background: 1.23 < 3.0 "
        "(#063b38 on #15282b, ui)"
    ]


def test_colours_without_fill_and_link_styles_are_checked_against_the_canvas() -> None:
    source = (
        "flowchart LR\n"
        "  classDef ghost color:#fbfeff\n"
        "  style A stroke:#fbfeff\n"
        "  linkStyle 0 stroke:#fbfeff,color:#fbfeff\n"
    )
    assert dc.check("a.mmd", "light", LIGHT, source, None) == [
        "a.mmd [light] classDef ghost color on background: 1.00 < 4.5 (#fbfeff on #fbfeff, text)",
        "a.mmd [light] style A stroke on background: 1.00 < 3.0 (#fbfeff on #fbfeff, ui)",
        "a.mmd [light] linkStyle 0 stroke on background: 1.00 < 3.0 (#fbfeff on #fbfeff, ui)",
        "a.mmd [light] linkStyle 0 color on edgeLabelBackground: 1.00 < 4.5 "
        "(#fbfeff on #fbfeff, text)",
    ]


def test_named_class_colour_is_reported_not_raised() -> None:
    source = "flowchart LR\n  classDef x fill:white,color:#08191c\n"
    assert dc.check("a.mmd", "light", LIGHT, source, SVG) == [
        "a.mmd [light] classDef x: colours must be #rrggbb colours"
    ]


def test_svg_without_the_theme_colours_is_reported() -> None:
    other = '<svg style="background-color: #15282b;"><style>#a{fill:#f5fbfc;}</style></svg>'
    assert dc.check("a.mmd", "light", LIGHT, "flowchart LR\n", other) == [
        "a.mmd [light] the rendering has not the background #fbfeff of the theme",
        "a.mmd [light] the rendering does not use the text colour #08191c of the theme",
    ]


def test_missing_svg_is_left_to_the_svg_check() -> None:
    assert dc.check("a.mmd", "light", LIGHT, "flowchart LR\n", None) == []


def test_every_committed_graphic_passes_in_light_and_dark() -> None:
    """The gate on the real artefacts: both configs, the class pairs and both renderings."""
    root = Path(__file__).resolve().parents[3]
    names = rd.tracked(root)
    assert any(n.startswith("docs/architecture/") and n.endswith(".mmd") for n in names)
    assert rd.contrast_problems(root, names) == []
