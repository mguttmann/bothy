# SPDX-License-Identifier: Apache-2.0
"""Mermaid configuration from the design tokens (light theme).

The diagram gate (tools/docs/render_diagrams.py) renders every diagram with this file, so
architecture and flow diagrams use the colours and the font of the design system instead of
the default Mermaid theme. Mermaid's `base` theme is the only one whose variables can be set.
"""

from __future__ import annotations

import json

from . import dtcg, emit_css
from .model import Model

THEME = "light"

# Mermaid theme variable -> semantic token of the light theme.
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
    "tertiaryBorderColor": "color.border.default",
    "tertiaryTextColor": "color.text.primary",
    "textColor": "color.text.primary",
    "lineColor": "color.text.secondary",
    "edgeLabelBackground": "color.surface.canvas",
    "clusterBkg": "color.surface.list",
    "clusterBorder": "color.border.default",
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


def theme_tokens(model: Model) -> dict[str, dtcg.Token]:
    """Return the full token set of the light theme."""
    return model.full["theme"][THEME]


def config(model: Model) -> dict:
    """Return the Mermaid configuration as a dict (theme `base`, variables, font)."""
    tokens = theme_tokens(model)
    variables: dict[str, str] = {}
    for name, path in THEME_VARIABLES.items():
        token = tokens.get(path)
        if token is None or token.type != "color":
            raise dtcg.TokenError(f"mermaid theme variable {name}: {path} is not a colour token of theme {THEME}")
        variables[name] = token.hex
    family = {**model.base, **tokens}["font.family.sans"].value
    variables["fontFamily"] = emit_css.font_family(family)
    variables["fontSize"] = "16px"
    return {"theme": "base", "themeVariables": variables}


def render(model: Model) -> str:
    """Return the configuration as JSON text with a trailing newline."""
    return json.dumps(config(model), indent=2, ensure_ascii=False) + "\n"


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
