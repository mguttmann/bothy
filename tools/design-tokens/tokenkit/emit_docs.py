# SPDX-License-Identifier: Apache-2.0
"""Generated documentation pages: docs/design/palette.md, contrast.md and tokens.md."""

from __future__ import annotations

from . import color, dtcg, emit_css
from .model import Model
from .rules import ContrastRow


def _step_key(step: str):
    return int(step) if step.isdigit() else 10**6


def _plain_value(token: dtcg.Token) -> str:
    decls = emit_css.declarations(token)
    if token.type == "typography":
        parts = {name.rsplit("-", 2)[-2] + "-" + name.rsplit("-", 2)[-1]: val for name, val in decls}
        return ", ".join(f"{k} {v}" for k, v in parts.items())
    return "; ".join(val for _, val in decls).replace("|", "\\|")


def tokens_md(model: Model, header: str) -> str:
    """Reference of every non-colour token with its value, per modifier context."""
    lines = [
        "# Token reference (without colors)",
        "",
        f"> {header}",
        "",
        "All values except colors, as the apps and `tokens.css` receive them. Colors: [palette.md](palette.md).",
        "Hand-written pages name only token names; the numbers are here and are bound to the source",
        "by `tokens.py check`.",
        "",
        "## Base (the same in all contexts)",
        "",
        "| Token | Type | Value |",
        "|---|---|---|",
    ]
    for path, token in sorted(model.base.items()):
        if token.type == "color":
            continue
        lines.append(f"| `{path}` | {token.type} | {_plain_value(token)} |")
    lines.append("")
    for name, contexts in model.contexts.items():
        paths = sorted(p for p, t in next(iter(contexts.values())).items() if t.type != "color")
        if not paths:
            continue
        ctx_names = list(contexts)
        lines.append(f"## Modifier `{name}` (default: `{model.defaults[name]}`)")
        lines.append("")
        lines.append("| Token | Type | " + " | ".join(f"`{c}`" for c in ctx_names) + " |")
        lines.append("|---|---|" + "---|" * len(ctx_names))
        for path in paths:
            kind = contexts[ctx_names[0]][path].type
            cells = " | ".join(_plain_value(contexts[c][path]) for c in ctx_names)
            lines.append(f"| `{path}` | {kind} | {cells} |")
        lines.append("")
    return "\n".join(lines)


def _alias_chain(token: dtcg.Token) -> str:
    return token.alias_of.removeprefix("palette.") if token.alias_of else "own value"


def palette_md(model: Model, header: str) -> str:
    """Render the palette page (tonal scales in OKLCH and sRGB) as Markdown."""
    lines = [
        "# Color palette",
        "",
        f"> {header}",
        "",
        "Primitive tonal scales in OKLCH and their sRGB values. Components use only the",
        "semantic roles further down. Images: [Palette light](previews/palette-light.png),",
        "[Palette dark](previews/palette-dark.png). Rationale for the color choice: [README.md](README.md).",
        "",
        'Columns "to white" and "to black": WCAG contrast ratio against #ffffff and #000000 respectively.',
        "",
    ]
    scales: dict[str, list[tuple[str, dtcg.Token]]] = {}
    for path, token in model.base.items():
        if path.startswith("palette."):
            _, scale, step = path.split(".")
            scales.setdefault(scale, []).append((step, token))
    for scale in scales:
        lines.append(f"## {scale}")
        lines.append("")
        lines.append("| Step | Hex | OKLCH (L, C, H) | to white | to black |")
        lines.append("|---|---|---|---|---|")
        for step, token in sorted(scales[scale], key=lambda s: _step_key(s[0])):
            comps = token.value["components"]
            oklch = ", ".join(f"{c:g}" for c in comps)
            lines.append(
                f"| {step} | `{token.hex}` | {oklch} | {color.contrast(token.hex, '#ffffff'):.2f} | "
                f"{color.contrast(token.hex, '#000000'):.2f} |"
            )
        lines.append("")
    theme = model.contexts.get("theme", {})
    if theme:
        names = list(theme)
        lines.append("## Semantic roles")
        lines.append("")
        lines.append("Each role points to a step of the palette per theme. Translucent roles carry their alpha value.")
        lines.append("")
        lines.append("| Role | " + " | ".join(names) + " |")
        lines.append("|---|" + "---|" * len(names))
        for path in sorted(theme[names[0]]):
            if theme[names[0]][path].type != "color":
                continue
            cells = []
            for ctx in names:
                token = theme[ctx][path]
                alpha = dtcg.color_alpha(token.value)
                suffix = f" (alpha {alpha:g})" if alpha < 1 else ""
                cells.append(f"`{token.hex}` {_alias_chain(token)}{suffix}")
            lines.append(f"| `{path}` | " + " | ".join(cells) + " |")
        lines.append("")
    return "\n".join(lines)


def contrast_md(rows: list[ContrastRow], guards: list[str], header: str) -> str:
    """Render the contrast table page with failed pairs and guard problems as Markdown."""
    failed = [r for r in rows if not r.passed]
    lines = [
        "# Contrast table (WCAG 2.2)",
        "",
        f"> {header}",
        "",
        "Every text and surface pairing that a component uses (list: `design/design-rules.json`),",
        "for every theme. Thresholds: `text` 4.5:1 (SC 1.4.3), `large-text` 3:1 (SC 1.4.3),",
        "`ui` 3:1 (SC 1.4.11), source https://www.w3.org/TR/WCAG22/ (retrieved 2026-10-10).",
        "The tool stops if a pairing falls below its threshold; this page is written only on success.",
        "",
        f"Result: {len(rows)} pairings checked, {len(failed)} below the threshold.",
        "",
        "Palette guards (ban list as a test): " + ("all met." if not guards else f"{len(guards)} violations."),
        "",
    ]
    for theme in dict.fromkeys(r.theme for r in rows):
        lines.append(f"## Theme {theme}")
        lines.append("")
        lines.append("| Foreground | Background | Kind | Values | Ratio | Threshold | Result | Usage |")
        lines.append("|---|---|---|---|---|---|---|---|")
        for r in rows:
            if r.theme != theme:
                continue
            verdict = "passed" if r.passed else "FAILED"
            lines.append(
                f"| `{r.fg}` | `{r.bg}` | {r.kind} | `{r.fg_hex}` on `{r.bg_hex}` | {r.ratio:.2f} | "
                f"{r.required:g} | {verdict} | {r.usage} |"
            )
        lines.append("")
    return "\n".join(lines)
