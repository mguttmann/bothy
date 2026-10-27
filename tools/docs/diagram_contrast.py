# SPDX-License-Identifier: Apache-2.0
"""Contrast check of the architecture graphics, per graphic and per theme (light and dark).

The docs site shows every `docs/architecture/*.mmd` graphic in the light and in the dark colour
scheme (render_diagrams.py renders both). Texts and lines must stay readable on the backgrounds
they sit on, measured with the contrast gate of the design tokens (tokenkit `color.contrast`
and the thresholds `rules.THRESHOLDS`, M4-03):

- the Mermaid theme variables of the theme's configuration: every text colour against the
  backgrounds it is drawn on (kind `text`), every line colour against the canvas (kind `ui`,
  non-text contrast);
- every `classDef` and `style` of the source: its `color` against its `fill` (kind `text`);
  without `color` the node text of the theme (`primaryTextColor`) is drawn on the fill;
- the rendered SVG carries the theme's `background` and `textColor`, proof that it was rendered
  with that configuration.

All functions are pure; render_diagrams.py reads the files and calls `check`.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

# The token gate lives in tools/design-tokens, which is no package of the tool environment;
# tokens.py itself puts the folder on the path the same way.
_DESIGN_TOKENS = Path(__file__).resolve().parents[1] / "design-tokens"
if str(_DESIGN_TOKENS) not in sys.path:
    sys.path.insert(0, str(_DESIGN_TOKENS))

from tokenkit import color, rules  # noqa: E402

TEXT = "text"
GRAPHICS = "ui"
# Required ratio per kind, taken from the gate and never copied.
REQUIRED = {TEXT: rules.THRESHOLDS[TEXT], GRAPHICS: rules.THRESHOLDS[GRAPHICS]}

# (text colour variable, background variable it is drawn on).
TEXT_PAIRS: tuple[tuple[str, str], ...] = (
    ("textColor", "background"),
    ("titleColor", "background"),
    ("loopTextColor", "background"),
    ("signalTextColor", "background"),
    ("textColor", "edgeLabelBackground"),
    ("primaryTextColor", "primaryColor"),
    ("primaryTextColor", "mainBkg"),
    ("secondaryTextColor", "secondaryColor"),
    ("tertiaryTextColor", "tertiaryColor"),
    ("titleColor", "clusterBkg"),
    ("textColor", "clusterBkg"),
    ("noteTextColor", "noteBkgColor"),
    ("actorTextColor", "actorBkg"),
    ("labelTextColor", "labelBoxBkgColor"),
    ("sequenceNumberColor", "signalColor"),
    ("textColor", "attributeBackgroundColorOdd"),
    ("textColor", "attributeBackgroundColorEven"),
)
# Line colours, checked against the canvas `background`. The borders count as lines: the box
# fills are nearly canvas-coloured, so the border is what outlines the box.
LINE_COLOURS: tuple[str, ...] = (
    "lineColor",
    "signalColor",
    "actorLineColor",
    "primaryBorderColor",
    "secondaryBorderColor",
    "tertiaryBorderColor",
    "clusterBorder",
    "noteBorderColor",
    "actorBorder",
    "labelBoxBorderColor",
    "activationBorderColor",
)
NODE_TEXT = "primaryTextColor"

_CLASS = re.compile(r"^\s*(?P<what>classDef|style)\s+(?P<name>\S+)\s+(?P<props>.+?)\s*;?\s*$")
_SVG_BACKGROUND = re.compile(
    r"background-color:\s*(?:(?P<hex>#[0-9a-fA-F]{6})|rgb\(\s*(?P<r>\d+)\s*,\s*(?P<g>\d+)\s*,\s*(?P<b>\d+)\s*\))"
)


def _problem(
    graphic: str, theme: str, fg: str, bg: str, fg_hex: str, bg_hex: str, kind: str
) -> str | None:
    """Return the problem line of one pair, or None if it meets the gate.

    The rounding is that of the token gate (`rules.ContrastRow.passed`).
    """
    row = rules.ContrastRow(
        theme=theme,
        fg=fg,
        bg=bg,
        kind=kind,
        fg_hex=fg_hex.lower(),
        bg_hex=bg_hex.lower(),
        ratio=color.contrast(fg_hex, bg_hex),
        required=REQUIRED[kind],
        usage=graphic,
    )
    if row.passed:
        return None
    return (
        f"{graphic} [{theme}] {fg} on {bg}: {row.ratio:.2f} < {row.required} "
        f"({row.fg_hex} on {row.bg_hex}, {kind})"
    )


def class_colours(source: str) -> list[tuple[str, str | None, str]]:
    """Return (label, color or None, fill) of every `classDef` and `style` with a fill.

    The label is e.g. `classDef person` or `style N`. Entries without `fill` are skipped: their
    text sits on a theme background that the variable pairs already cover.
    """
    out = []
    for line in source.splitlines():
        m = _CLASS.match(line)
        if m is None:
            continue
        props = {
            key.strip(): value.strip()
            for key, _, value in (p.partition(":") for p in m.group("props").split(","))
        }
        if "fill" in props:
            out.append((f"{m.group('what')} {m.group('name')}", props.get("color"), props["fill"]))
    return out


def svg_background(svg: str) -> str | None:
    """Return the background colour of the `<svg>` element as `#rrggbb`, or None.

    mermaid-cli writes it as `background-color: rgb(r, g, b)` into the style of the root tag.
    """
    start = svg.find("<svg")
    end = svg.find(">", start)
    if start < 0 or end < 0:
        return None
    m = _SVG_BACKGROUND.search(svg[start:end])
    if m is None:
        return None
    if m.group("hex"):
        return m.group("hex").lower()
    return "#" + "".join(f"{int(m.group(c)):02x}" for c in "rgb")


def check(
    graphic: str, theme: str, variables: dict[str, str], source: str, svg: str | None
) -> list[str]:
    """Return one problem line per pair of `graphic` in `theme` that misses the gate.

    Args:
        graphic: repository path of the `.mmd` file, used in the messages.
        theme: `light` or `dark`, used in the messages.
        variables: `themeVariables` of the theme's Mermaid configuration.
        source: the `.mmd` text (its `classDef` and `style` colours).
        svg: the rendering of `source` with that configuration, or None if it is missing (the
            SVG check of render_diagrams.py reports a missing file).

    Returns:
        Problem lines naming graphic, theme, pair, ratio and required ratio; a missing theme
        variable or a rendering without the theme's colours is a problem too.
    """
    prefix = f"{graphic} [{theme}]"
    needed = sorted(
        {name for pair in TEXT_PAIRS for name in pair} | set(LINE_COLOURS) | {NODE_TEXT}
    )
    missing = [name for name in needed if name not in variables]
    if missing:
        return [f"{prefix} theme variable {name} is missing" for name in missing]
    found = [
        _problem(graphic, theme, fg, bg, variables[fg], variables[bg], TEXT)
        for fg, bg in TEXT_PAIRS
    ]
    found += [
        _problem(
            graphic, theme, line, "background", variables[line], variables["background"], GRAPHICS
        )
        for line in LINE_COLOURS
    ]
    for label, text, fill in class_colours(source):
        fg = f"{label} color" if text else f"{label} {NODE_TEXT}"
        try:
            found.append(
                _problem(graphic, theme, fg, "fill", text or variables[NODE_TEXT], fill, TEXT)
            )
        except ValueError:
            # A named or short colour; the token colour check of render_diagrams.py names it too.
            found.append(f"{prefix} {label}: colour and fill must be #rrggbb colours")
    problems = [p for p in found if p is not None]
    if svg is not None:
        background, text = variables["background"].lower(), variables["textColor"].lower()
        if svg_background(svg) != background:
            problems.append(
                f"{prefix} the rendering has not the background {background} of the theme"
            )
        if text not in svg.lower():
            problems.append(
                f"{prefix} the rendering does not use the text colour {text} of the theme"
            )
    return problems
