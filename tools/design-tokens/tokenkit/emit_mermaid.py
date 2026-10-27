# SPDX-License-Identifier: Apache-2.0
"""Mermaid configurations from the design tokens, one per theme (light and dark).

The diagram gate (tools/docs/render_diagrams.py) renders every diagram with the light file and
every `.mmd` file a second time with the dark file, so architecture and flow diagrams use the
colours and the font of the design system instead of the default Mermaid theme and the docs
site can show each graphic in its colour scheme. Mermaid's `base` theme is the only one whose
variables can be set.
"""

from __future__ import annotations

import json

from . import dtcg, emit_css
from .model import Model

THEME = "light"
DARK_THEME = "dark"

# Mermaid theme variable -> semantic token, the same map for both themes. Borders that outline a
# box on a nearly canvas-coloured fill act as lines and need non-text contrast against the canvas
# (tools/docs/diagram_contrast.py), hence `color.border.control` and not `color.border.default`.
THEME_VARIABLES: dict[str, str] = {
    "background": "color.surface.canvas",
    "mainBkg": "color.accent.subtle",
    "primaryColor": "color.accent.subtle",
    "primaryBorderColor": "color.accent.bg",
    "primaryTextColor": "color.text.primary",
    "secondaryColor": "color.info.subtle",
    "secondaryBorderColor": "color.info.border",
    "secondaryTextColor": "color.text.primary",
    "tertiaryColor": "color.surface.list",
    "tertiaryBorderColor": "color.border.control",
    "tertiaryTextColor": "color.text.primary",
    "textColor": "color.text.primary",
    "lineColor": "color.text.secondary",
    "edgeLabelBackground": "color.surface.canvas",
    "clusterBkg": "color.surface.list",
    "clusterBorder": "color.border.control",
    "titleColor": "color.text.primary",
    "noteBkgColor": "color.attention.subtle",
    "noteBorderColor": "color.attention.border",
    "noteTextColor": "color.text.primary",
    "actorBkg": "color.accent.subtle",
    "actorBorder": "color.accent.bg",
    "actorTextColor": "color.text.primary",
    "actorLineColor": "color.border.control",
    "signalColor": "color.text.primary",
    "signalTextColor": "color.text.primary",
    "labelBoxBkgColor": "color.surface.raised",
    "labelBoxBorderColor": "color.border.control",
    "labelTextColor": "color.text.primary",
    "loopTextColor": "color.text.primary",
    "activationBkgColor": "color.surface.sunken",
    "activationBorderColor": "color.border.control",
    "sequenceNumberColor": "color.text.on-accent",
    "attributeBackgroundColorOdd": "color.surface.canvas",
    "attributeBackgroundColorEven": "color.surface.raised",
}


def theme_tokens(model: Model, theme: str = THEME) -> dict[str, dtcg.Token]:
    """Return the full token set of `theme` (a context of the `theme` modifier)."""
    return model.full["theme"][theme]


def config(model: Model, theme: str = THEME) -> dict:
    """Return the Mermaid configuration of `theme` as a dict (theme `base`, variables, font).

    Raises:
        dtcg.TokenError: a variable maps to a token that is no colour token of `theme`.
    """
    tokens = theme_tokens(model, theme)
    variables: dict[str, str] = {}
    for name, path in THEME_VARIABLES.items():
        token = tokens.get(path)
        if token is None or token.type != "color":
            raise dtcg.TokenError(f"mermaid theme variable {name}: {path} is not a colour token of theme {theme}")
        variables[name] = token.hex
    family = {**model.base, **tokens}["font.family.sans"].value
    variables["fontFamily"] = emit_css.font_family(family)
    variables["fontSize"] = "16px"
    return {"theme": "base", "themeVariables": variables}


def render(model: Model, theme: str = THEME) -> str:
    """Return the configuration of `theme` as JSON text with a trailing newline."""
    return json.dumps(config(model, theme), indent=2, ensure_ascii=False) + "\n"


# Material for MkDocs variable -> semantic token. The doc site renders the ```mermaid blocks of
# Markdown pages itself; these variables give them, and links and headers, the design colours.
MATERIAL_VARIABLES: dict[str, str] = {
    "--md-primary-fg-color": "color.accent.bg",
    "--md-primary-fg-color--light": "color.accent.bg-hover",
    "--md-primary-fg-color--dark": "color.accent.bg-pressed",
    "--md-primary-bg-color": "color.text.on-accent",
    "--md-accent-fg-color": "color.accent.fg",
    "--md-accent-fg-color--transparent": "color.accent.subtle",
    "--md-typeset-a-color": "color.text.link",
    "--md-mermaid-node-bg-color": "color.accent.subtle",
    "--md-mermaid-node-fg-color": "color.accent.fg",
    "--md-mermaid-edge-color": "color.text.secondary",
    "--md-mermaid-label-bg-color": "color.surface.canvas",
    "--md-mermaid-label-fg-color": "color.text.primary",
    "--md-mermaid-sequence-note-bg-color": "color.attention.subtle",
    "--md-mermaid-sequence-note-border-color": "color.attention.border",
    "--md-mermaid-sequence-note-fg-color": "color.text.primary",
}
# Material colour scheme name -> theme context.
MATERIAL_SCHEMES = {"default": "light", "slate": "dark"}


def material_css(model: Model, header: str) -> str:
    """Return CSS that maps the Material for MkDocs colour variables to the design tokens."""
    lines = [f"/* {header} */", ""]
    for scheme, theme in MATERIAL_SCHEMES.items():
        tokens = model.full["theme"][theme]
        lines.append(f'[data-md-color-scheme="{scheme}"] {{')
        for name, path in MATERIAL_VARIABLES.items():
            token = tokens.get(path)
            if token is None or token.type != "color":
                raise dtcg.TokenError(f"{name}: {path} is not a colour token of theme {theme}")
            lines.append(f"  {name}: {token.hex};")
        lines.append("}")
        lines.append("")
    return "\n".join(lines)
