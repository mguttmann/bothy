# SPDX-License-Identifier: Apache-2.0
"""Self-contained preview page: palette, desktop layout, component sheet, mobile, terminal and charts.

Fonts and icons are embedded (base64 and an SVG sprite) so the file renders the same everywhere,
offline, and in screenshots. `?view=palette|layout|components|mobile|terminal|viz` shows one section only,
`?theme=light|dark` forces a theme, `?density=compact|comfortable|touch` forces a density.
All example data is fictitious (people Alex, Sam, Robin; spaces Support, Marketing, Engineering,
Research).
"""

from __future__ import annotations

import base64
import html
import re
from pathlib import Path

from . import color, emit_css, emit_dart
from .model import Model

_FONT_FACES = [
    (
        "Atkinson Hyperlegible Next",
        "atkinson-hyperlegible-next-latin-wght-normal.woff2",
        "U+0000-00FF,U+0131,U+0152-0153,U+02BB-02BC,U+02C6,U+02DA,U+02DC,U+0304,U+0308,U+0329,U+2000-206F,U+20AC,U+2122,U+2191,U+2193,U+2212,U+2215,U+FEFF,U+FFFD",
    ),
    (
        "Atkinson Hyperlegible Next",
        "atkinson-hyperlegible-next-latin-ext-wght-normal.woff2",
        "U+0100-02BA,U+02BD-02C5,U+02C7-02CC,U+02CE-02D7,U+02DD-02FF,U+0304,U+0308,U+0329,U+1D00-1DBF,U+1E00-1E9F,U+1EF2-1EFF,U+2020,U+20A0-20AB,U+20AD-20C0,U+2113,U+2C60-2C7F,U+A720-A7FF",
    ),
    (
        "Atkinson Hyperlegible Mono",
        "atkinson-hyperlegible-mono-latin-wght-normal.woff2",
        "U+0000-00FF,U+0131,U+0152-0153,U+02BB-02BC,U+02C6,U+02DA,U+02DC,U+0304,U+0308,U+0329,U+2000-206F,U+20AC,U+2122,U+2191,U+2193,U+2212,U+2215,U+FEFF,U+FFFD",
    ),
    (
        "Atkinson Hyperlegible Mono",
        "atkinson-hyperlegible-mono-latin-ext-wght-normal.woff2",
        "U+0100-02BA,U+02BD-02C5,U+02C7-02CC,U+02CE-02D7,U+02DD-02FF,U+0304,U+0308,U+0329,U+1D00-1DBF,U+1E00-1E9F,U+1EF2-1EFF,U+2020,U+20A0-20AB,U+20AD-20C0,U+2113,U+2C60-2C7F,U+A720-A7FF",
    ),
]


def _font_faces(vendor: Path) -> str:
    out = []
    for family, file, ranges in _FONT_FACES:
        data = base64.b64encode((vendor / "fonts" / file).read_bytes()).decode("ascii")
        out.append(
            f"@font-face{{font-family:'{family}';font-style:normal;font-display:block;font-weight:200 800;"
            f"src:url(data:font/woff2;base64,{data}) format('woff2');unicode-range:{ranges};}}"
        )
    return "\n".join(out)


def _sprite(model: Model, vendor: Path) -> str:
    icons = emit_dart.icon_extension(model)
    needed = {(g["name"], g["weight"]) for g in icons["glyphs"].values()} | {
        (n, "regular") for n in icons["space-icons"]
    }
    symbols = []
    for name, weight in sorted(needed):
        suffix = "" if weight == "regular" else f"-{weight}"
        svg = (vendor / "phosphor" / f"{name}{suffix}.svg").read_text(encoding="utf-8")
        inner = re.sub(r"^<svg[^>]*>|</svg>\s*$", "", svg.strip())
        symbols.append(f'<symbol id="i-{name}{suffix}" viewBox="0 0 256 256">{inner}</symbol>')
    return '<svg xmlns="http://www.w3.org/2000/svg" style="display:none">' + "".join(symbols) + "</svg>"


def _glyphs(model: Model) -> dict[str, str]:
    icons = emit_dart.icon_extension(model)
    out = {}
    for key, g in icons["glyphs"].items():
        suffix = "" if g["weight"] == "regular" else f"-{g['weight']}"
        out[key] = f"i-{g['name']}{suffix}"
    return out


class _Ctx:
    def __init__(self, model: Model):
        self.g = _glyphs(model)

    def ic(self, key: str, cls: str = "", label: str | None = None) -> str:
        sym = self.g[key] if key in self.g else f"i-{key}"
        spin = " spin" if key in ("trigger.working", "run.running", "state.reconnecting") else ""
        aria = f' role="img" aria-label="{html.escape(label)}"' if label else ' aria-hidden="true"'
        return f'<svg class="ic {cls}{spin}"{aria}><use href="#{sym}"/></svg>'


CSS = r"""
*{box-sizing:border-box}
html,body{margin:0}
body{font-family:var(--ds-font-family-sans);font-size:var(--ds-type-body-font-size);line-height:var(--ds-type-body-line-height);
  color:var(--ds-color-text-primary);background:var(--ds-color-surface-canvas);-webkit-font-smoothing:antialiased;font-optical-sizing:auto}
.ic{width:var(--ds-icon-size-md);height:var(--ds-icon-size-md);fill:currentColor;flex:none;vertical-align:middle}
.ic.sm{width:var(--ds-icon-size-sm);height:var(--ds-icon-size-sm)}
.ic.lg{width:var(--ds-icon-size-lg);height:var(--ds-icon-size-lg)}
.spin{animation:spin var(--ds-motion-duration-pulse) linear infinite}
@keyframes spin{to{transform:rotate(360deg)}}
.mono{font-family:var(--ds-font-family-mono)}
.t-title{font-size:var(--ds-type-title-font-size);font-weight:var(--ds-type-title-font-weight);line-height:var(--ds-type-title-line-height);letter-spacing:var(--ds-type-title-letter-spacing)}
.t-heading{font-size:var(--ds-type-heading-font-size);font-weight:var(--ds-type-heading-font-weight);line-height:var(--ds-type-heading-line-height)}
.t-body-sm{font-size:var(--ds-type-body-sm-font-size);line-height:var(--ds-type-body-sm-line-height)}
.t-label{font-size:var(--ds-type-label-font-size);font-weight:var(--ds-type-label-font-weight);line-height:var(--ds-type-label-line-height)}
.t-caption{font-size:var(--ds-type-caption-font-size);line-height:var(--ds-type-caption-line-height)}
.t-section{font-size:var(--ds-type-section-font-size);font-weight:var(--ds-type-section-font-weight);letter-spacing:var(--ds-type-section-letter-spacing);line-height:var(--ds-type-section-line-height)}
.t-code-sm{font-family:var(--ds-font-family-mono);font-size:var(--ds-type-code-sm-font-size);font-weight:var(--ds-type-code-sm-font-weight);line-height:var(--ds-type-code-sm-line-height)}
.muted{color:var(--ds-color-text-muted)} .secondary{color:var(--ds-color-text-secondary)}
.pv-bar{display:flex;gap:var(--ds-space-16);align-items:center;padding:var(--ds-space-12) var(--ds-space-24);border-bottom:1px solid var(--ds-color-border-default);background:var(--ds-color-surface-list)}
.pv-bar .grow{flex:1}
.pv-bar select{font:inherit;background:var(--ds-color-surface-input);color:var(--ds-color-text-primary);border:1px solid var(--ds-color-border-control);border-radius:var(--ds-radius-sm);padding:2px 6px}
section.pv{padding:var(--ds-space-24)}
section.pv>h2{margin:0 0 var(--ds-space-16)}
body.single .pv-bar{display:none}
body.single section.pv{padding:var(--ds-space-24)}
body.single section.pv#layout{padding:0}
.pv-note{color:var(--ds-color-text-secondary);margin:0 0 var(--ds-space-16)}

/* palette */
.scale{display:grid;grid-template-columns:96px repeat(14,1fr);gap:4px;margin-bottom:6px;align-items:stretch}
.scale .name{display:flex;align-items:center;font-weight:600}
.sw{border-radius:var(--ds-radius-sm);padding:6px 6px 5px;min-height:54px;display:flex;flex-direction:column;justify-content:flex-end;font-size:11px;line-height:1.25}
.sw b{font-weight:700;font-size:12px}
.roles{display:grid;grid-template-columns:repeat(4,1fr);gap:var(--ds-space-12);margin-top:var(--ds-space-24)}
.role{border:1px solid var(--ds-color-border-default);border-radius:var(--ds-radius-md);padding:var(--ds-space-12);min-height:96px}
.role h4{margin:0 0 4px;font-size:var(--ds-type-label-font-size)}
.chips{display:flex;flex-wrap:wrap;gap:6px}
.statusrow{display:grid;grid-template-columns:repeat(6,1fr);gap:var(--ds-space-12);margin-top:var(--ds-space-16)}
.status{border-radius:var(--ds-radius-md);padding:10px 12px;display:flex;align-items:center;gap:8px;font-weight:600;font-size:var(--ds-type-label-font-size)}

/* app shell */
.app{display:grid;grid-template-columns:var(--ds-layout-sidebar-width) var(--ds-layout-list-width) minmax(0,1fr) var(--ds-layout-session-width);grid-template-rows:minmax(0,1fr);height:1000px;overflow:hidden}
.side{background:var(--ds-color-surface-sidebar);color:var(--ds-color-text-on-sidebar-secondary);display:flex;flex-direction:column;min-width:0}
.brand{display:flex;align-items:center;gap:10px;padding:14px 14px 10px;color:var(--ds-color-text-on-sidebar)}
.brand .mark{width:28px;height:28px;border-radius:var(--ds-radius-md);background:var(--ds-color-category-gorse-on-sidebar);display:grid;place-items:center}
.brand .mark svg{width:18px;height:18px}
.brand .name{font-weight:800;font-size:17px;letter-spacing:-0.2px}
.jump{margin:4px 10px 8px;display:flex;align-items:center;gap:8px;padding:6px 10px;border-radius:var(--ds-radius-sm);background:var(--ds-color-surface-sidebar-hover);color:var(--ds-color-text-on-sidebar-secondary);font-size:var(--ds-type-body-sm-font-size)}
.jump kbd{margin-left:auto;font:inherit;font-family:var(--ds-font-family-mono);font-size:11px;padding:0 5px;border-radius:var(--ds-radius-xs);border:1px solid var(--ds-color-border-on-sidebar);color:var(--ds-color-text-on-sidebar-muted)}
.running{margin:0 10px 6px;display:flex;align-items:center;gap:8px;padding:6px 10px;border-radius:var(--ds-radius-sm);color:var(--ds-color-text-on-sidebar);font-weight:600;font-size:var(--ds-type-body-sm-font-size)}
.running .count{margin-left:auto}
.tree{flex:1;overflow:hidden;padding:4px 6px}
.space{display:flex;align-items:center;gap:8px;padding:12px 8px 4px;color:var(--ds-color-text-on-sidebar)}
.space .sico{width:20px;height:20px;border-radius:var(--ds-radius-sm);display:grid;place-items:center}
.space .sico svg{width:15px;height:15px}
.space .caret{width:12px;height:12px;color:var(--ds-color-text-on-sidebar-muted)}
.space .dot{margin-left:auto;width:6px;height:6px;border-radius:50%;background:var(--ds-color-text-on-sidebar)}
.ch{position:relative;display:flex;align-items:center;gap:8px;height:var(--ds-density-row-channel);padding:0 8px 0 28px;border-radius:var(--ds-radius-sm);font-size:var(--ds-type-body-sm-font-size)}
.ch .ic{width:16px;height:16px;color:var(--ds-color-text-on-sidebar-muted)}
.ch.unread{color:var(--ds-color-text-on-sidebar);font-weight:700}
.ch.unread::before{content:"";position:absolute;left:-6px;top:50%;width:4px;height:8px;margin-top:-4px;border-radius:0 4px 4px 0;background:var(--ds-color-text-on-sidebar)}
.ch.sel{background:var(--ds-color-surface-sidebar-selected);color:var(--ds-color-text-on-sidebar)}
.ch.hover{background:var(--ds-color-surface-sidebar-hover)}
.ch.mute{color:var(--ds-color-text-on-sidebar-muted)}
.ch .end{margin-left:auto;display:flex;gap:6px;align-items:center}
.livedot{width:8px;height:8px;border-radius:50%;background:var(--ds-color-category-spruce-on-sidebar);box-shadow:0 0 0 3px color-mix(in oklab,var(--ds-color-category-spruce-on-sidebar) 30%,transparent);animation:pulse var(--ds-motion-duration-pulse) ease-in-out infinite}
@keyframes pulse{50%{opacity:.35}}
.cnt{min-width:18px;height:18px;padding:0 5px;border-radius:var(--ds-radius-pill);background:var(--ds-color-attention-bg);color:var(--ds-color-attention-on-bg);font-size:11px;font-weight:800;display:grid;place-items:center}
.me{display:flex;align-items:center;gap:10px;padding:10px 12px;border-top:1px solid var(--ds-color-border-on-sidebar);color:var(--ds-color-text-on-sidebar)}
.me .ic{color:var(--ds-color-text-on-sidebar-muted);margin-left:auto}
.av{width:var(--ds-size-avatar-md);height:var(--ds-size-avatar-md);border-radius:50%;display:grid;place-items:center;font-weight:700;font-size:13px;flex:none}
.av.sm{width:var(--ds-size-avatar-sm);height:var(--ds-size-avatar-sm);font-size:10px}
.av.agent{border-radius:var(--ds-radius-md)}
.av.agent svg{width:20px;height:20px}
.av.c-heather{background:var(--ds-color-category-heather-chip-bg);color:var(--ds-color-category-heather-chip-fg)}
.av.c-loch{background:var(--ds-color-category-loch-chip-bg);color:var(--ds-color-category-loch-chip-fg)}
.av.c-moss{background:var(--ds-color-category-moss-chip-bg);color:var(--ds-color-category-moss-chip-fg)}
.av.c-spruce{background:var(--ds-color-accent-bg);color:var(--ds-color-text-on-accent)}
.av.c-amber{background:var(--ds-color-category-amber-chip-bg);color:var(--ds-color-category-amber-chip-fg)}

.list{background:var(--ds-color-surface-list);border-right:1px solid var(--ds-color-border-default);display:flex;flex-direction:column;min-width:0}
.colhead{display:flex;align-items:center;gap:8px;height:52px;padding:0 14px;border-bottom:1px solid var(--ds-color-border-default);flex:none}
.colhead .t-heading{white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.colhead .grow{flex:1}
.filters{display:flex;gap:6px;padding:10px 14px;border-bottom:1px solid var(--ds-color-border-subtle);flex-wrap:wrap}
.chip{display:inline-flex;align-items:center;gap:4px;height:22px;padding:0 8px;border-radius:var(--ds-radius-sm);font-size:12px;font-weight:600;white-space:nowrap}
.chip.f{border:1px solid var(--ds-color-border-default);color:var(--ds-color-text-secondary);background:transparent}
.chip.f.on{border-color:var(--ds-color-accent-fg);color:var(--ds-color-accent-fg);background:var(--ds-color-accent-subtle)}
.tag-spruce{background:var(--ds-color-category-spruce-chip-bg);color:var(--ds-color-category-spruce-chip-fg)}
.tag-loch{background:var(--ds-color-category-loch-chip-bg);color:var(--ds-color-category-loch-chip-fg)}
.tag-heather{background:var(--ds-color-category-heather-chip-bg);color:var(--ds-color-category-heather-chip-fg)}
.tag-rowan{background:var(--ds-color-category-rowan-chip-bg);color:var(--ds-color-category-rowan-chip-fg)}
.tag-amber{background:var(--ds-color-category-amber-chip-bg);color:var(--ds-color-category-amber-chip-fg)}
.tag-gorse{background:var(--ds-color-category-gorse-chip-bg);color:var(--ds-color-category-gorse-chip-fg)}
.tag-moss{background:var(--ds-color-category-moss-chip-bg);color:var(--ds-color-category-moss-chip-fg)}
.tag-granite{background:var(--ds-color-category-granite-chip-bg);color:var(--ds-color-category-granite-chip-fg)}
.posts{flex:1;overflow:hidden}
.post{display:grid;grid-template-columns:20px 1fr auto;gap:2px 10px;padding:8px 14px;min-height:var(--ds-density-row-post);border-bottom:1px solid var(--ds-color-border-subtle);align-items:start}
.post .ic{margin-top:2px}
.post .ti{font-size:var(--ds-type-body-sm-font-size);line-height:1.35;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.post.unread .ti{font-weight:700}
.post .meta{grid-column:2/4;display:flex;gap:8px;align-items:center;color:var(--ds-color-text-secondary);font-size:12px;white-space:nowrap;overflow:hidden}
.post .when{font-size:12px;color:var(--ds-color-text-muted)}
.post.sel{background:var(--ds-color-surface-selected)}
.post.hover{background:var(--ds-color-surface-hover)}
.done-sec{display:flex;align-items:center;gap:6px;padding:10px 14px;color:var(--ds-color-text-muted);font-weight:600;font-size:13px;border-top:1px solid var(--ds-color-border-default)}
.c-queued{color:var(--ds-color-run-queued)} .c-running{color:var(--ds-color-run-running)} .c-waiting{color:var(--ds-color-run-waiting)}
.c-completed{color:var(--ds-color-run-completed)} .c-failed{color:var(--ds-color-run-failed)} .c-cancelled{color:var(--ds-color-run-cancelled)}
.c-attn{color:var(--ds-color-attention-fg)} .c-accent{color:var(--ds-color-accent-fg)}
.tr-received{color:var(--ds-color-trigger-received)} .tr-queued{color:var(--ds-color-trigger-queued)} .tr-working{color:var(--ds-color-trigger-working)}
.tr-done{color:var(--ds-color-trigger-done)} .tr-failed{color:var(--ds-color-trigger-failed)} .tr-followup{color:var(--ds-color-trigger-followup)}

.conv{display:flex;flex-direction:column;min-width:0;background:var(--ds-color-surface-canvas)}
.thead{display:flex;align-items:center;gap:10px;height:52px;padding:0 16px;border-bottom:1px solid var(--ds-color-border-default);flex:none}
.thead .t-heading{white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.badge{display:inline-flex;align-items:center;gap:4px;height:var(--ds-size-badge-height);padding:0 7px;border-radius:var(--ds-radius-pill);font-size:12px;font-weight:700;white-space:nowrap}
.badge .ic{width:13px;height:13px}
.b-org{background:var(--ds-color-badge-organization-bg);color:var(--ds-color-badge-organization-fg)}
.b-pers{background:var(--ds-color-badge-personal-bg);color:var(--ds-color-badge-personal-fg)}
.b-attn{background:var(--ds-color-attention-bg);color:var(--ds-color-attention-on-bg)}
.b-agent{background:var(--ds-color-accent-subtle);color:var(--ds-color-accent-fg);border-radius:var(--ds-radius-xs);height:16px;font-size:10.5px;padding:0 5px;letter-spacing:.3px}
.iconbtn{width:32px;height:32px;border-radius:var(--ds-radius-sm);display:grid;place-items:center;color:var(--ds-color-text-secondary)}
.iconbtn.on{color:var(--ds-color-accent-fg);background:var(--ds-color-accent-subtle)}
.scroll{flex:1;min-height:0;overflow:hidden;position:relative;display:flex;flex-direction:column}
.pinned{padding:12px 16px 4px;background:var(--ds-color-surface-canvas)}
.msgs{flex:1;min-height:0;overflow:hidden;padding:4px 0 8px;display:flex;flex-direction:column;justify-content:flex-end}
.msg{display:grid;grid-template-columns:var(--ds-size-avatar-md) 1fr;gap:0 12px;padding:var(--ds-density-message-pad-y) 16px;margin-top:calc(var(--ds-density-message-gap) - 8px);position:relative}
.msg.cont{margin-top:0;padding-top:0}
.msg.cont .av{visibility:hidden;height:0}
.msg .who{display:flex;align-items:center;gap:8px;line-height:1.3}
.msg .who b{font-weight:700}
.msg .who .ts{font-size:12px;color:var(--ds-color-text-muted)}
.msg .who .ic{width:16px;height:16px}
.msg .body{font-size:var(--ds-density-message-font-size);line-height:var(--ds-type-body-line-height)}
.msg .body p{margin:2px 0}
.msg.agent .who b{color:var(--ds-color-accent-fg)}
.msg.mention{background:var(--ds-color-attention-subtle)}
.msg.mention::before{content:"";position:absolute;left:0;top:0;bottom:0;width:var(--ds-border-width-spine);background:var(--ds-color-attention-border)}
.pill{color:var(--ds-color-text-link);font-weight:700}
code.inl{font-family:var(--ds-font-family-mono);font-size:.88em;background:var(--ds-color-code-bg);color:var(--ds-color-code-fg);padding:1px 5px;border-radius:var(--ds-radius-xs)}
pre.code{font-family:var(--ds-font-family-mono);font-size:var(--ds-type-code-font-size);line-height:var(--ds-type-code-line-height);background:var(--ds-color-code-bg);color:var(--ds-color-code-fg);padding:10px 12px;border-radius:var(--ds-radius-md);margin:6px 0;white-space:pre;overflow:hidden}
.reacts{display:flex;gap:6px;margin-top:6px}
.react{display:inline-flex;gap:5px;align-items:center;height:24px;padding:0 8px;border-radius:var(--ds-radius-md);border:1px solid var(--ds-color-border-default);font-size:12px;font-weight:700;color:var(--ds-color-text-secondary);background:var(--ds-color-surface-raised)}
.react.mine{border-color:var(--ds-color-accent-fg);background:var(--ds-color-accent-subtle);color:var(--ds-color-accent-fg)}
.more{color:var(--ds-color-text-link);font-weight:700;font-size:13px}

/* live box */
.lb{position:relative;background:var(--ds-color-surface-raised);border:1px solid var(--ds-color-border-default);border-radius:var(--ds-radius-md);padding:var(--ds-density-livebox-pad) var(--ds-density-livebox-pad) var(--ds-density-livebox-pad) calc(var(--ds-density-livebox-pad) + 6px);overflow:hidden}
.lb::before{content:"";position:absolute;left:0;top:0;bottom:0;width:var(--ds-border-width-spine)}
.lb.s-running::before{background:var(--ds-color-run-running)} .lb.s-waiting::before{background:var(--ds-color-run-waiting)}
.lb.s-failed::before{background:var(--ds-color-run-failed)} .lb.s-completed::before{background:var(--ds-color-run-completed)}
.lb.s-cancelled::before{background:var(--ds-color-run-cancelled)} .lb.s-queued::before{background:var(--ds-color-run-queued)}
.lb-head{display:flex;align-items:center;gap:10px;white-space:nowrap;min-width:0}
.lb-state{display:inline-flex;align-items:center;gap:6px;font-weight:700;font-size:var(--ds-type-label-font-size)}
.metrics{display:flex;gap:12px;color:var(--ds-color-text-secondary)}
.metrics span{display:inline-flex;align-items:center;gap:4px}
.lb-head .grow{flex:1}
.statusline{margin:8px 0 6px;font-size:var(--ds-type-body-sm-font-size);color:var(--ds-color-text-primary)}
.steps{display:flex;flex-direction:column;gap:4px;margin:4px 0 2px}
.step{display:flex;align-items:center;gap:8px;font-size:13px;min-height:24px}
.step.child{padding-left:22px}
.tool{display:inline-flex;align-items:center;gap:6px;height:22px;padding:0 8px;border-radius:var(--ds-radius-sm);background:var(--ds-color-surface-sunken);color:var(--ds-color-text-primary)}
.tool .ic{width:14px;height:14px}
.step .muted{font-size:12px}
.excerpt{margin-top:8px;padding:6px 10px;border-left:2px solid var(--ds-color-border-default);color:var(--ds-color-text-secondary);font-size:13px}
.lb-foot{display:flex;gap:8px;align-items:center;margin-top:10px}
.lb-line{display:flex;align-items:center;gap:10px;white-space:nowrap;padding:6px 12px;border-radius:var(--ds-radius-md);background:var(--ds-color-surface-sunken);font-size:13px;color:var(--ds-color-text-secondary)}
.lb-line b{color:var(--ds-color-text-primary)}
.lb-line .grow{flex:1}

/* buttons */
.btn{display:inline-flex;align-items:center;gap:6px;height:var(--ds-density-control-height);padding:0 12px;border-radius:var(--ds-radius-sm);font:inherit;font-size:var(--ds-type-label-font-size);font-weight:var(--ds-type-label-font-weight);border:1px solid transparent;cursor:pointer;white-space:nowrap}
.btn .ic{width:16px;height:16px}
.btn.primary{background:var(--ds-color-accent-bg);color:var(--ds-color-text-on-accent)}
.btn.primary.hover{background:var(--ds-color-accent-bg-hover)}
.btn.secondary{background:transparent;border-color:var(--ds-color-border-control);color:var(--ds-color-text-primary)}
.btn.ghost{background:transparent;color:var(--ds-color-text-secondary)}
.btn.danger{background:var(--ds-color-danger-bg);color:var(--ds-color-danger-on-bg)}
.btn.danger-out{background:transparent;border-color:var(--ds-color-danger-border);color:var(--ds-color-danger-fg)}
.btn.success{background:var(--ds-color-success-bg);color:var(--ds-color-success-on-bg)}
.btn.disabled{opacity:.45}
.btn.focus{outline:var(--ds-border-width-focus) solid var(--ds-color-focus-ring);outline-offset:2px}
.card{background:var(--ds-color-surface-raised);border:1px solid var(--ds-color-border-default);border-radius:var(--ds-radius-md);padding:12px;margin-top:6px;max-width:620px}
.card .hd{display:flex;align-items:center;gap:8px;font-weight:700;margin-bottom:6px}
.card .row{display:flex;gap:8px;flex-wrap:wrap;margin-top:10px}
.card .note{font-size:12px;color:var(--ds-color-text-muted);margin-top:8px;display:flex;gap:6px;align-items:center}
.card.used .row{opacity:1}
.used-line{font-size:13px;color:var(--ds-color-text-secondary);display:flex;gap:6px;align-items:center;margin-top:8px}

/* composer */
.composer-wrap{padding:0 16px 14px;position:relative}
.typing{font-size:12px;color:var(--ds-color-text-secondary);height:20px;display:flex;align-items:center;gap:6px;padding-left:2px}
.composer{display:flex;align-items:center;gap:8px;min-height:var(--ds-density-composer-min-height);padding:6px 8px 6px 12px;border-radius:var(--ds-radius-lg);background:var(--ds-color-surface-input);border:1px solid var(--ds-color-border-control)}
.composer .txt{flex:1;color:var(--ds-color-text-primary)}
.composer .ph{color:var(--ds-color-text-muted)}
.composer .ic{color:var(--ds-color-text-secondary)}
.textcaret{display:inline-block;width:1.5px;height:1.1em;background:var(--ds-color-text-primary);vertical-align:-2px;margin-left:1px}
.suggest{position:absolute;left:16px;right:16px;bottom:calc(100% - 14px);background:var(--ds-color-surface-overlay);border:1px solid var(--ds-color-border-default);border-radius:var(--ds-radius-lg);box-shadow:var(--ds-elevation-menu);padding:6px}
.suggest .hd{font-size:12px;color:var(--ds-color-text-muted);padding:4px 8px 6px;display:flex;gap:6px;align-items:center}
.sug{display:grid;grid-template-columns:auto 1fr auto;gap:10px;align-items:center;min-height:var(--ds-density-row-suggestion);padding:0 8px;border-radius:var(--ds-radius-sm)}
.sug.sel{background:var(--ds-color-surface-selected)}
.sug .cmd{font-family:var(--ds-font-family-mono);font-weight:700;font-size:13px}
.sug .desc{color:var(--ds-color-text-secondary);font-size:13px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.sug .opt{font-family:var(--ds-font-family-mono);font-size:12px;color:var(--ds-color-text-muted)}
.sug .src{font-size:12px;color:var(--ds-color-text-muted)}

/* session column */
.sess{background:var(--ds-color-surface-list);border-left:1px solid var(--ds-color-border-default);display:flex;flex-direction:column;min-width:0}
.sess .blk{padding:14px;border-bottom:1px solid var(--ds-color-border-subtle)}
.agentcard{display:flex;gap:10px;align-items:center}
.agentcard .name{font-weight:700}
.presence{display:inline-flex;align-items:center;gap:5px;font-size:12px;color:var(--ds-color-success-fg)}
.presence i{width:8px;height:8px;border-radius:50%;background:var(--ds-color-run-completed)}
.kv{display:grid;grid-template-columns:auto 1fr;gap:6px 12px;font-size:13px;margin-top:10px}
.kv dt{color:var(--ds-color-text-secondary)} .kv dd{margin:0;text-align:right;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.gauge{height:6px;border-radius:3px;background:var(--ds-color-surface-sunken);overflow:hidden;margin-top:4px}
.gauge i{display:block;height:100%;background:var(--ds-color-run-running)}
.sess h5{margin:0 0 8px;font-size:12px;color:var(--ds-color-text-secondary);font-weight:700;letter-spacing:.3px}
.prev{display:flex;gap:8px;align-items:center;font-size:13px;padding:4px 0;color:var(--ds-color-text-secondary)}

/* components sheet */
.sheet{display:grid;grid-template-columns:1fr 1fr;gap:var(--ds-space-24)}
.grp{border:1px solid var(--ds-color-border-default);border-radius:var(--ds-radius-lg);padding:var(--ds-space-16);background:var(--ds-color-surface-canvas)}
.grp h3{margin:0 0 12px;font-size:var(--ds-type-label-font-size);color:var(--ds-color-text-secondary);font-weight:700;letter-spacing:.3px}
.grp .row{display:flex;gap:10px;flex-wrap:wrap;align-items:center;margin-bottom:10px}
.grp .stack{display:flex;flex-direction:column;gap:10px}
.trig{display:flex;align-items:center;gap:6px;font-size:13px;min-width:150px}
.banner{display:flex;align-items:center;gap:10px;padding:8px 12px;border-radius:var(--ds-radius-md);font-size:13px;border:1px solid}
.banner.warn{background:var(--ds-color-warning-subtle);border-color:var(--ds-color-warning-border);color:var(--ds-color-text-primary)}
.banner.warn .ic{color:var(--ds-color-warning-fg)}
.banner.info{background:var(--ds-color-info-subtle);border-color:var(--ds-color-info-border)}
.banner.info .ic{color:var(--ds-color-info-fg)}
.banner.err{background:var(--ds-color-danger-subtle);border-color:var(--ds-color-danger-border)}
.banner.err .ic{color:var(--ds-color-danger-fg)}
.banner.attn{background:var(--ds-color-attention-subtle);border-color:var(--ds-color-attention-border)}
.banner.attn .ic{color:var(--ds-color-attention-fg)}
.banner .grow{flex:1}
.field{display:flex;flex-direction:column;gap:4px;font-size:13px}
.field label{font-weight:600}
.input{height:var(--ds-density-control-height);border-radius:var(--ds-radius-sm);border:1px solid var(--ds-color-border-control);background:var(--ds-color-surface-input);padding:0 10px;display:flex;align-items:center;gap:8px}
.input .ph{color:var(--ds-color-text-muted)}
.select .ic{margin-left:auto;width:14px;height:14px;color:var(--ds-color-text-secondary)}
.menu{background:var(--ds-color-surface-overlay);border:1px solid var(--ds-color-border-default);border-radius:var(--ds-radius-md);box-shadow:var(--ds-elevation-menu);padding:4px;max-width:260px}
.menu .it{display:flex;gap:8px;align-items:center;height:30px;padding:0 8px;border-radius:var(--ds-radius-sm);font-size:13px}
.menu .it.sel{background:var(--ds-color-surface-selected)}
.qs{background:var(--ds-color-surface-overlay);border:1px solid var(--ds-color-border-default);border-radius:var(--ds-radius-lg);box-shadow:var(--ds-elevation-dialog);overflow:hidden}
.qs .q{display:flex;align-items:center;gap:10px;padding:12px 14px;border-bottom:1px solid var(--ds-color-border-subtle);font-size:16px}
.qs .res{padding:6px}
.qs .r{display:flex;align-items:center;gap:10px;height:var(--ds-density-row-suggestion);padding:0 10px;border-radius:var(--ds-radius-sm);font-size:14px}
.qs .r.sel{background:var(--ds-color-surface-selected)}
.qs .r .path{margin-left:auto;font-size:12px;color:var(--ds-color-text-muted)}
.qs kbd{font-family:var(--ds-font-family-mono);font-size:11px;border:1px solid var(--ds-color-border-default);border-radius:var(--ds-radius-xs);padding:0 4px;color:var(--ds-color-text-secondary)}
.sidedemo{background:var(--ds-color-surface-sidebar);border-radius:var(--ds-radius-md);padding:6px 6px 10px;color:var(--ds-color-text-on-sidebar-secondary)}

/* mobile */
.phones{display:flex;gap:28px;justify-content:center}
.phone{width:375px;height:812px;border-radius:44px;border:10px solid var(--ds-color-surface-sidebar);overflow:hidden;display:flex;flex-direction:column;background:var(--ds-color-surface-canvas);position:relative}
.phone .sb{height:44px;flex:none}
.phone .nav{display:flex;align-items:center;gap:8px;height:48px;padding:0 12px;border-bottom:1px solid var(--ds-color-border-default);flex:none}
.phone .nav .t-heading{flex:1;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.phone .back{color:var(--ds-color-accent-fg);display:flex;align-items:center;font-weight:600}
.m-side{background:var(--ds-color-surface-sidebar);flex:1;color:var(--ds-color-text-on-sidebar-secondary);padding:4px 10px}
.m-side .sb{background:var(--ds-color-surface-sidebar)}
.m-side .ch{height:var(--ds-density-row-channel);font-size:16px;padding-left:34px}
.m-side .ch .ic{width:20px;height:20px}
.m-side .space{padding:14px 6px 6px;font-size:13px}
.m-lb{display:flex;align-items:center;gap:10px;padding:0 6px 0 14px;height:56px;border-bottom:1px solid var(--ds-color-border-default);background:var(--ds-color-surface-raised);position:relative;flex:none}
.m-lb::before{content:"";position:absolute;left:0;top:0;bottom:0;width:var(--ds-border-width-spine);background:var(--ds-color-run-running)}
.m-lb .grow{flex:1;min-width:0}
.m-lb .stl{font-size:13px;color:var(--ds-color-text-secondary);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.m-stop{width:48px;height:48px;border-radius:var(--ds-radius-md);display:grid;place-items:center;color:var(--ds-color-danger-fg)}
.m-stop .ic{width:28px;height:28px}
.phone .msgs{padding:4px 0}
.phone .msg{padding-left:12px;padding-right:12px}
.phone .card{margin-right:0}
.phone .btn{height:var(--ds-density-control-height);font-size:15px;padding:0 14px}
.phone .composer{border-radius:22px;margin:0 10px 10px}
.m-tabbar{display:flex;justify-content:space-around;align-items:center;height:56px;border-top:1px solid var(--ds-color-border-on-sidebar);background:var(--ds-color-surface-sidebar);color:var(--ds-color-text-on-sidebar-muted);flex:none}
.m-tabbar .on{color:var(--ds-color-text-on-sidebar)}
.m-tabbar div{display:flex;flex-direction:column;align-items:center;gap:2px;font-size:11px;min-width:64px}
.m-tabbar .ic{width:24px;height:24px}
.offline{display:flex;align-items:center;gap:8px;padding:8px 12px;background:var(--ds-color-warning-subtle);border-bottom:1px solid var(--ds-color-warning-border);font-size:13px}
.offline .ic{color:var(--ds-color-warning-fg)}
.pending{opacity:.72}
.pending .ts{display:inline-flex;gap:4px;align-items:center}

/* terminal */
.tw{display:grid;grid-template-columns:minmax(0,1fr) 375px;gap:28px;align-items:start}
.term{border:1px solid var(--ds-color-border-default);border-radius:var(--ds-radius-lg);overflow:hidden;background:var(--ds-color-surface-canvas)}
.term-head{display:flex;align-items:center;gap:10px;height:52px;padding:0 14px;border-bottom:1px solid var(--ds-color-border-default);white-space:nowrap}
.term-head .grow{flex:1}
.term-head .fp{font-family:var(--ds-font-family-mono);font-size:12px;color:var(--ds-color-text-secondary)}
.term-tabs{display:flex;align-items:center;gap:6px;padding:8px 14px;border-bottom:1px solid var(--ds-color-border-subtle);background:var(--ds-color-surface-list)}
.term-tab{display:inline-flex;align-items:center;gap:6px;height:28px;padding:0 10px;border-radius:var(--ds-radius-sm);font-size:13px;color:var(--ds-color-text-secondary)}
.term-tab.on{background:var(--ds-color-surface-selected);color:var(--ds-color-text-primary);font-weight:600}
.term-tabs .grow{flex:1}
.screen{background:var(--ds-color-terminal-bg);color:var(--ds-color-terminal-fg);font-family:var(--ds-font-family-mono);font-size:var(--ds-type-code-font-size);line-height:1.5;padding:12px 14px;white-space:pre;overflow:hidden}
.screen .cur{background:var(--ds-color-terminal-cursor);color:var(--ds-color-terminal-cursor-text)}
.screen .sel{background:var(--ds-color-terminal-selection-bg)}
.a0{color:var(--ds-color-terminal-ansi-black)} .a1{color:var(--ds-color-terminal-ansi-red)} .a2{color:var(--ds-color-terminal-ansi-green)} .a3{color:var(--ds-color-terminal-ansi-yellow)}
.a4{color:var(--ds-color-terminal-ansi-blue)} .a5{color:var(--ds-color-terminal-ansi-magenta)} .a6{color:var(--ds-color-terminal-ansi-cyan)} .a7{color:var(--ds-color-terminal-ansi-white)}
.a8{color:var(--ds-color-terminal-ansi-bright-black)} .a9{color:var(--ds-color-terminal-ansi-bright-red)} .a10{color:var(--ds-color-terminal-ansi-bright-green)} .a11{color:var(--ds-color-terminal-ansi-bright-yellow)}
.a12{color:var(--ds-color-terminal-ansi-bright-blue)} .a13{color:var(--ds-color-terminal-ansi-bright-magenta)} .a14{color:var(--ds-color-terminal-ansi-bright-cyan)} .a15{color:var(--ds-color-terminal-ansi-bright-white)}
.term-foot{display:flex;gap:16px;align-items:center;padding:8px 14px;border-top:1px solid var(--ds-color-border-subtle);font-size:12px;color:var(--ds-color-text-secondary)}
.term-foot span{display:inline-flex;gap:6px;align-items:center}
.ansi{display:grid;grid-template-columns:repeat(8,1fr);gap:6px;margin-top:16px;padding:10px;border-radius:var(--ds-radius-md);background:var(--ds-color-terminal-bg);font-family:var(--ds-font-family-mono);font-size:12px}
.ansi div{display:flex;flex-direction:column;gap:2px}
.ansi i{display:block;height:18px;border-radius:var(--ds-radius-xs);background:currentColor}
.dlgs{display:grid;grid-template-columns:1fr 1fr;gap:16px;margin-top:16px}
.dlg{background:var(--ds-color-surface-overlay);border:1px solid var(--ds-color-border-default);border-radius:var(--ds-radius-lg);box-shadow:var(--ds-elevation-dialog);padding:14px;font-size:13px}
.dlg .hd{display:flex;align-items:center;gap:8px;font-weight:700;margin-bottom:10px;font-size:14px}
.dlg .kv{margin-top:0}
.dlg .row{display:flex;gap:8px;justify-content:flex-end;margin-top:12px}
.dlg.warn{border-color:var(--ds-color-danger-border)}
.phone .screen{flex:1;padding:10px 12px;font-size:13px}
.keys{display:flex;flex-wrap:wrap;gap:6px;padding:8px;background:var(--ds-color-surface-list);border-top:1px solid var(--ds-color-border-default);flex:none}
.key{min-width:44px;height:var(--ds-density-control-height);padding:0 8px;border-radius:var(--ds-radius-sm);border:1px solid var(--ds-color-border-control);display:grid;place-items:center;font-family:var(--ds-font-family-mono);font-size:14px;color:var(--ds-color-text-primary);background:var(--ds-color-surface-raised)}
.key.on{background:var(--ds-color-accent-bg);color:var(--ds-color-text-on-accent);border-color:transparent}

/* charts */
.vz{display:grid;grid-template-columns:repeat(4,1fr);gap:16px}
.tile{border:1px solid var(--ds-color-border-default);border-radius:var(--ds-radius-lg);padding:12px 14px;background:var(--ds-color-surface-raised)}
.tile .lab{font-size:var(--ds-type-label-font-size);color:var(--ds-color-text-secondary);font-weight:600}
.tile .num{font-family:var(--ds-font-family-mono);font-size:var(--ds-type-display-font-size);font-weight:700;line-height:1.15;margin:4px 0}
.tile .cmp{display:flex;gap:6px;align-items:center;font-size:12px;color:var(--ds-color-text-secondary)}
.lanes{grid-column:1/5;border:1px solid var(--ds-color-border-default);border-radius:var(--ds-radius-lg);overflow:hidden;background:var(--ds-color-surface-canvas)}
.lane{display:grid;grid-template-columns:150px 1fr;min-height:104px;border-bottom:1px solid var(--ds-color-viz-grid)}
.lane:last-child{border-bottom:0}
.lane .lh{display:flex;align-items:center;gap:8px;padding:0 14px;font-weight:700;background:var(--ds-color-surface-list)}
.lane .cards{display:flex;gap:12px;align-items:center;padding:12px}
.rc{position:relative;width:260px;padding:8px 10px 8px 16px;border-radius:var(--ds-radius-md);border:1px solid var(--ds-color-border-default);background:var(--ds-color-surface-raised);font-size:13px;display:flex;flex-direction:column;gap:4px}
.rc::before{content:"";position:absolute;left:0;top:0;bottom:0;width:var(--ds-border-width-spine);border-radius:var(--ds-radius-md) 0 0 var(--ds-radius-md)}
.rc.s-running::before{background:var(--ds-color-run-running)} .rc.s-waiting::before{background:var(--ds-color-run-waiting)} .rc.s-completed::before{background:var(--ds-color-run-completed)}
.rc .top{display:flex;align-items:center;gap:6px;font-weight:700}
.rc .bot{display:flex;align-items:center;gap:8px}
.chart{grid-column:span 2;border:1px solid var(--ds-color-border-default);border-radius:var(--ds-radius-lg);padding:12px 14px;background:var(--ds-color-surface-canvas)}
.chart h4{margin:0 0 8px;font-size:var(--ds-type-label-font-size)}
.chart svg{width:100%;height:180px;display:block}
.chart text{font-family:var(--ds-font-family-sans);font-size:11px;fill:var(--ds-color-text-secondary)}
.legend{display:flex;flex-wrap:wrap;gap:10px;margin-top:8px;font-size:12px;color:var(--ds-color-text-secondary)}
.legend span{display:inline-flex;align-items:center;gap:5px}
.legend i{width:10px;height:10px;border-radius:2px;display:inline-block}
.slo{grid-column:1/5;display:flex;gap:16px;flex-wrap:wrap}
.slo .badge{height:28px;padding:0 10px;font-size:13px}
.b-ok{background:var(--ds-color-success-subtle);color:var(--ds-color-success-fg)}
.b-warn{background:var(--ds-color-warning-subtle);color:var(--ds-color-warning-fg)}
.b-bad{background:var(--ds-color-danger-subtle);color:var(--ds-color-danger-fg)}
.b-none{background:var(--ds-color-surface-sunken);color:var(--ds-color-text-secondary)}
"""

SCRIPT = r"""
(function(){
  var p=new URLSearchParams(location.search), root=document.documentElement;
  var view=p.get('view'), theme=p.get('theme'), density=p.get('density');
  if(theme) root.setAttribute('data-theme',theme);
  if(density) root.setAttribute('data-density',density);
  if(view){document.body.classList.add('single');
    document.querySelectorAll('section.pv').forEach(function(s){ if(s.id!==view) s.remove(); });}
  var t=document.getElementById('pv-theme'), d=document.getElementById('pv-density'), m=document.getElementById('pv-motion');
  if(t){t.value=theme||'system'; t.onchange=function(){ if(t.value==='system') root.removeAttribute('data-theme'); else root.setAttribute('data-theme',t.value); };}
  if(d){d.value=density||'comfortable'; d.onchange=function(){ root.setAttribute('data-density',d.value); };}
  if(m){m.onchange=function(){ if(m.value==='system') root.removeAttribute('data-motion'); else root.setAttribute('data-motion',m.value); };}
})();
"""


def _swatch_text(hex_value: str) -> str:
    return "#ffffff" if color.contrast(hex_value, "#ffffff") >= color.contrast(hex_value, "#0b1416") else "#0b1416"


def _palette_section(model: Model) -> str:
    scales: dict[str, list[tuple[str, str]]] = {}
    for path, token in model.base.items():
        if path.startswith("palette."):
            _, scale, step = path.split(".")
            scales.setdefault(scale, []).append((step, token.hex))
    rows = []
    for scale, steps in scales.items():
        steps.sort(key=lambda s: int(s[0]))
        cells = "".join(
            f'<div class="sw" style="background:{hx};color:{_swatch_text(hx)}"><b>{st}</b>{hx}</div>'
            for st, hx in steps
        )
        rows.append(
            f'<div class="scale" style="grid-template-columns:96px repeat({len(steps)},1fr)"><div class="name">{scale}</div>{cells}</div>'
        )
    roles = []
    for name, bg, label in (
        ("surface.sidebar", "var(--ds-color-surface-sidebar)", "Brand surface"),
        ("surface.list", "var(--ds-color-surface-list)", "List"),
        ("surface.canvas", "var(--ds-color-surface-canvas)", "History"),
        ("surface.raised", "var(--ds-color-surface-raised)", "Live box, card"),
    ):
        fg = "var(--ds-color-text-on-sidebar)" if "sidebar" in name else "var(--ds-color-text-primary)"
        fg2 = "var(--ds-color-text-on-sidebar-secondary)" if "sidebar" in name else "var(--ds-color-text-secondary)"
        fg3 = "var(--ds-color-text-on-sidebar-muted)" if "sidebar" in name else "var(--ds-color-text-muted)"
        roles.append(
            f'<div class="role" style="background:{bg}"><h4 style="color:{fg}">{name}</h4>'
            f'<div style="color:{fg}">Primary text {label}</div><div style="color:{fg2}">Secondary text</div>'
            f'<div style="color:{fg3}">Muted text</div></div>'
        )
    status = []
    for label, var in (
        ("Waiting for you", "attention"),
        ("Done", "success"),
        ("Stalled", "warning"),
        ("Failed", "danger"),
        ("Follow-up delivered", "info"),
        ("Running", "accent"),
    ):
        bg = f"var(--ds-color-{var}-subtle)"
        fg = f"var(--ds-color-{var}-fg)"
        status.append(f'<div class="status" style="background:{bg};color:{fg}">{label}</div>')
    tags = "".join(
        f'<span class="chip tag-{c}">{c}</span>'
        for c in ("spruce", "loch", "heather", "rowan", "amber", "gorse", "moss", "granite")
    )
    return (
        '<section class="pv" id="palette"><h2 class="t-title">Palette</h2>'
        '<p class="pv-note">Tonal scales in OKLCH (values in docs/design/palette.md). Below them, the semantic roles in the current theme.</p>'
        + "".join(rows)
        + '<div class="roles">'
        + "".join(roles)
        + "</div>"
        + '<div class="statusrow">'
        + "".join(status)
        + "</div>"
        + f'<div class="chips" style="margin-top:16px">{tags}</div></section>'
    )


def _sidebar(c: _Ctx) -> str:
    return f"""
<aside class="side" aria-label="Spaces and channels">
  <div class="brand"><span class="mark" aria-hidden="true"><svg viewBox="0 0 24 24"><path d="M3 13.5 12 5l9 8.5" fill="none" stroke="var(--ds-color-surface-sidebar)" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round"/><path d="M7.5 12v7h9v-7" fill="none" stroke="var(--ds-color-surface-sidebar)" stroke-width="2.6" stroke-linejoin="round"/></svg></span><span class="name">Bothy</span></div>
  <div class="jump">{c.ic("action.search", "sm")}Jump to<kbd>⌘K</kbd></div>
  <div class="running">{c.ic("run.running", "sm")}Running now<span class="count cnt">2</span></div>
  <nav class="tree">
    <div class="space">{c.ic("action.expand", "caret")}<span class="sico" style="background:color-mix(in oklab,var(--ds-color-category-heather-on-sidebar) 22%,transparent);color:var(--ds-color-category-heather-on-sidebar)">{c.ic("headset")}</span><span class="t-section">Support</span></div>
    <div class="ch">{c.ic("channel.text")}general</div>
    <div class="ch unread">{c.ic("channel.forum")}requests<span class="end"><span class="livedot" title="Run active"></span><span class="cnt">3</span></span></div>
    <div class="ch mute">{c.ic("channel.text")}reports<span class="end">{c.ic("action.mute", "sm")}</span></div>
    <div class="space">{c.ic("action.expand", "caret")}<span class="sico" style="background:color-mix(in oklab,var(--ds-color-category-loch-on-sidebar) 22%,transparent);color:var(--ds-color-category-loch-on-sidebar)">{c.ic("code")}</span><span class="t-section">Engineering</span></div>
    <div class="ch sel">{c.ic("channel.forum")}server<span class="end"><span class="livedot"></span></span></div>
    <div class="ch hover">{c.ic("channel.forum")}apps</div>
    <div class="ch">{c.ic("channel.text")}releases</div>
    <div class="space">{c.ic("action.collapse", "caret")}<span class="sico" style="background:color-mix(in oklab,var(--ds-color-category-amber-on-sidebar) 22%,transparent);color:var(--ds-color-category-amber-on-sidebar)">{c.ic("megaphone")}</span><span class="t-section">Marketing</span><span class="dot"></span></div>
    <div class="space">{c.ic("action.collapse", "caret")}<span class="sico" style="background:color-mix(in oklab,var(--ds-color-category-moss-on-sidebar) 22%,transparent);color:var(--ds-color-category-moss-on-sidebar)">{c.ic("flask")}</span><span class="t-section">Research</span></div>
  </nav>
  <div class="me"><span class="av sm c-heather">AL</span><span class="t-body-sm" style="font-weight:600">Alex</span>{c.ic("action.settings")}</div>
</aside>"""


def _post(c: _Ctx, state: str, icon: str, title: str, meta: str, when: str, cls: str = "", label: str = "") -> str:
    return (
        f'<div class="post {cls}">{c.ic(icon, "sm c-" + state, label)}'
        f'<div class="ti">{title}</div><div class="when">{when}</div>'
        f'<div class="meta">{meta}</div></div>'
    )


def _list(c: _Ctx) -> str:
    posts = [
        _post(
            c,
            "running",
            "run.running",
            "Gateway: resume after network change",
            f'{c.ic("agent", "sm")}dev-runner <span class="chip tag-loch">sync</span>',
            "running 4 min",
            "sel unread",
            "Running",
        ),
        _post(
            c,
            "waiting",
            "run.waiting",
            "Check migration 0012 before the merge",
            '<span class="badge b-attn">Waiting for you</span><span class="chip tag-rowan">database</span>',
            "12:41",
            "unread",
            "Waiting for you",
        ),
        _post(
            c,
            "attn",
            "state.info",
            "Review: grant wizard, plain text",
            'Sam mentions you <span class="chip tag-heather">permissions</span>',
            "11:58",
            "unread hover",
            "Mention",
        ),
        _post(
            c,
            "queued",
            "run.queued",
            "Search: highlight hits in the transcript",
            '<span class="chip tag-moss">search</span>',
            "11:20",
            "",
            "Queued",
        ),
        _post(
            c,
            "completed",
            "run.completed",
            "Costs per space in the statistics",
            'Robin <span class="chip tag-amber">statistics</span>',
            "2 h ago",
            "",
            "Done",
        ),
        _post(
            c,
            "failed",
            "run.failed",
            "Upload limit for voice messages",
            '<span style="color:var(--ds-color-danger-fg)">failed: project not ready</span>',
            "yesterday",
            "",
            "Failed",
        ),
        _post(
            c,
            "cancelled",
            "run.cancelled",
            "Old draft: export format",
            '<span class="chip tag-granite">export</span>',
            "Mon",
            "",
            "Canceled",
        ),
    ]
    return f"""
<section class="list" aria-label="Posts in the forum server">
  <div class="colhead">{c.ic("channel.forum", "c-accent")}<span class="t-heading">server</span><span class="grow"></span><span class="btn primary">{c.ic("action.new")}New post</span></div>
  <div class="filters"><span class="chip f on">Open</span><span class="chip f">Waiting for me</span><span class="chip f">{c.ic("action.filter", "sm")}Tags</span><span class="chip f">Last activity</span></div>
  <div class="posts">{"".join(posts)}</div>
  <div class="done-sec">{c.ic("action.collapse", "sm")}Done (14)</div>
</section>"""


def _livebox_running(c: _Ctx, actions: bool = True) -> str:
    foot = (
        f'<div class="lb-foot"><span class="btn danger-out">{c.ic("action.stop")}Stop</span>'
        f'<span class="t-caption muted">New messages in this thread go to the run as a follow-up.</span></div>'
        if actions
        else ""
    )
    return f"""
<div class="lb s-running" role="status" aria-label="Run of dev-runner: running, for 4 minutes 12 seconds, context 62 percent">
  <div class="lb-head"><span class="lb-state c-running">{c.ic("run.running", "sm")}Running</span>
    <span class="metrics t-code-sm"><span>4:12</span><span>{c.ic("context", "sm")}62%</span><span>48.1k tokens</span><span>{c.ic("cost", "sm")}USD 0.62</span></span>
    <span class="grow"></span><span class="badge b-org">{c.ic("billing.organization")}Organization</span>{c.ic("action.expand", "sm muted")}</div>
  <div class="statusline">Checking if <code class="inl">resume</code> restores the focus after the network change.</div>
  <div class="steps">
    <div class="step"><span class="tool">{c.ic("step.tool_call", "c-completed")}<span class="t-code-sm">Read</span></span><span class="t-code-sm secondary">internal/sync/gateway.go</span><span class="muted">0.4 s</span></div>
    <div class="step"><span class="tool">{c.ic("step.tool_call", "c-completed")}<span class="t-code-sm">Grep</span></span><span class="t-code-sm secondary">resume_window</span><span class="muted">0.2 s</span></div>
    <div class="step"><span class="tool">{c.ic("step.subagent", "c-running")}<span class="t-code-sm">Subagent</span></span><span class="secondary" style="font-size:13px">Write test cases for 4006</span></div>
    <div class="step child"><span class="tool">{c.ic("run.running", "c-running")}<span class="t-code-sm">Bash</span></span><span class="t-code-sm secondary">go test ./internal/sync/...</span><span class="muted">running 38 s</span></div>
  </div>
  <div class="excerpt">On resume, the focus is taken over from the gateway session; it is still open whether expired threads are dropped silently.</div>
  {foot}
</div>"""


def _conv(c: _Ctx) -> str:
    return f"""
<main class="conv" aria-label="Thread Gateway: resume after network change">
  <div class="thead">{c.ic("thread", "c-accent")}<span class="t-heading">Gateway: resume after network change</span>
    <span class="badge b-org">{c.ic("billing.organization")}Organization</span><span class="t-caption muted">USD 1.84 estimate</span>
    <span style="flex:1"></span><span class="btn secondary">{c.ic("action.close-thread")}Done</span><span class="iconbtn">{c.ic("action.pin")}</span><span class="iconbtn on">{c.ic("action.toggle-panel")}</span></div>
  <div class="scroll">
    <div class="pinned">{_livebox_running(c)}</div>
    <div class="msgs">
      <div class="msg agent"><span class="av agent c-spruce">{c.ic("agent")}</span><div><div class="who"><b>dev-runner</b><span class="badge b-agent">AGENT</span><span class="ts">10:14</span></div>
        <div class="body"><p>Cause: The window <code class="inl">sync.resume_window</code> is measured from the last heartbeat, not from the disconnect.</p>
<span class="more">Show more</span>
        <div class="lb-line" style="margin-top:8px">{c.ic("run.completed", "sm c-completed")}<b>Ran 12 min</b><span>3 tools</span><span>2 subagents</span><span class="t-code-sm">USD 0.74 estimate</span><span class="grow"></span>{c.ic("action.collapse", "sm")}</div>
        <div class="reacts"><span class="react mine">👍 2</span><span class="react">🎯 1</span></div></div></div></div>
      <div class="msg agent"><span class="av agent c-spruce">{c.ic("agent")}</span><div><div class="who"><b>dev-runner</b><span class="badge b-agent">AGENT</span><span class="ts">10:16</span></div>
        <div class="card"><div class="hd">{c.ic("security.step-up", "c-attn")}Approval needed</div>
          <div class="t-body-sm"><code class="inl">go test</code> wants to use the network for the test database.</div>
          <div class="row"><span class="btn primary">Allow once</span><span class="btn secondary">For this session</span><span class="btn secondary">Always</span><span class="btn ghost">Deny</span></div>
          <div class="note">{c.ic("security.lock", "sm")}Only Alex or an owner can approve this.</div></div></div></div>
      <div class="msg mention"><span class="av c-loch">SA</span><div><div class="who"><b>Sam</b><span class="ts">10:21</span></div>
        <div class="body"><p><span class="pill">@Alex</span> can you take over the approval? I only have write permission on the channel.</p></div></div></div>
      <div class="msg"><span class="av c-heather">AL</span><div><div class="who"><b>Alex</b><span class="ts">10:22</span>{c.ic("trigger.working", "tr-working", "Working")}</div>
        <div class="body"><p><span class="pill">@dev-runner</span> Please also check whether expired threads silently drop out of the focus on resume.</p></div></div></div>
    </div>
    <div class="composer-wrap">
      <div class="typing">{c.ic("action.more", "sm")}Sam is typing</div>
      <div class="composer">{c.ic("action.attach")}<span class="txt ph">Follow-up to dev-runner, / for commands</span>{c.ic("action.voice")}{c.ic("action.send", "c-accent")}</div>
    </div>
  </div>
</main>"""


def _session(c: _Ctx) -> str:
    return f"""
<aside class="sess" aria-label="Session">
  <div class="colhead"><span class="t-heading">Session</span><span class="grow"></span>{c.ic("action.dismiss", "sm secondary")}</div>
  <div class="blk"><div class="agentcard"><span class="av agent c-spruce">{c.ic("agent")}</span><div><div class="name">dev-runner</div><span class="presence"><i></i>online</span></div></div>
    <dl class="kv"><dt>Backend</dt><dd><span class="badge b-org">Organization</span></dd><dt>Model</dt><dd class="t-code-sm">model-large</dd><dt>Session</dt><dd class="t-code-sm">0193f2a7-8c41</dd><dt>Project</dt><dd>server, branch main</dd></dl></div>
  <div class="blk"><h5>CONTEXT</h5><div class="gauge" role="meter" aria-valuenow="62" aria-valuemin="0" aria-valuemax="100" aria-label="Context 62 percent"><i style="width:62%"></i></div>
    <dl class="kv"><dt>Context</dt><dd>62%</dd><dt>Thread cost</dt><dd>USD 1.84 <span class="muted">estimate</span></dd><dt>Runs</dt><dd>5</dd></dl></div>
  <div class="blk"><h5>EARLIER SESSIONS</h5><div class="prev">{c.ic("thread", "sm")}<span class="t-code-sm">0193e01c-55d2</span><span class="muted">started over</span></div><div class="prev">{c.ic("thread", "sm")}<span class="t-code-sm">0193d8b4-01aa</span><span class="muted">import</span></div></div>
  <div class="blk"><h5>PINNED (2)</h5><div class="prev">{c.ic("action.pin", "sm")}Resume flow, sketch</div><div class="prev">{c.ic("action.pin", "sm")}Test plan 4006</div></div>
</aside>"""


def _layout_section(c: _Ctx) -> str:
    return (
        '<section class="pv" id="layout"><div class="app">'
        + _sidebar(c)
        + _list(c)
        + _conv(c)
        + _session(c)
        + "</div></section>"
    )


def _components_section(c: _Ctx) -> str:
    trig = "".join(
        f'<span class="trig">{c.ic(k, cls, lab)}{lab}</span>'
        for k, cls, lab in (
            ("trigger.received", "tr-received", "Received"),
            ("trigger.queued", "tr-queued", "Queued"),
            ("trigger.working", "tr-working", "Working"),
            ("trigger.done", "tr-done", "Done"),
            ("trigger.failed", "tr-failed", "Failed"),
            ("trigger.followup", "tr-followup", "Follow-up delivered"),
        )
    )
    runs = "".join(
        f'<span class="trig">{c.ic(k, cls, lab)}{lab}</span>'
        for k, cls, lab in (
            # Words from docs/design/components.md, section 2.1 (never the raw enum value).
            ("run.queued", "c-queued", "Queued"),
            ("run.running", "c-running", "Running"),
            ("run.waiting", "c-waiting", "Waiting for you"),
            ("run.completed", "c-completed", "Done"),
            ("run.failed", "c-failed", "Failed"),
            ("run.cancelled", "c-cancelled", "Canceled"),
        )
    )
    waiting = f"""
<div class="lb s-waiting" role="status" aria-label="Run is waiting for your answer">
  <div class="lb-head"><span class="lb-state c-waiting">{c.ic("run.waiting", "sm")}Waiting for you</span><span class="metrics t-code-sm"><span>2:03</span><span>USD 0.18</span></span><span class="grow"></span><span class="badge b-pers">{c.ic("billing.personal")}Personal</span></div>
  <div class="statusline">Which database should the test use?</div>
  <div class="row" style="display:flex;gap:10px;align-items:flex-end;flex-wrap:wrap">
    <div class="field"><label>Database</label><div class="input select" style="width:220px">Local container{c.ic("action.expand")}</div></div>
    <div class="field"><label>Port</label><div class="input" style="width:110px"><span class="ph">55432</span></div></div>
    <span class="btn primary">Answer</span></div>
</div>"""
    failed = f"""
<div class="lb s-failed" role="status"><div class="lb-head"><span class="lb-state c-failed">{c.ic("run.failed", "sm")}Failed</span><span class="t-caption secondary">Backend does not match: the adapter reports organization, the configured mode is personal.</span></div>
<div class="lb-foot"><span class="btn secondary">{c.ic("action.reopen")}Try again</span><span class="btn ghost">Details</span></div></div>"""
    return f"""
<section class="pv" id="components"><h2 class="t-title">Components</h2>
<div class="sheet">
  <div class="grp"><h3>BUTTONS</h3>
    <div class="row"><span class="btn primary">Primary</span><span class="btn primary hover">Primary hover</span><span class="btn secondary">Secondary</span><span class="btn ghost">Quiet</span></div>
    <div class="row"><span class="btn success">Approve</span><span class="btn danger">Delete</span><span class="btn danger-out">{c.ic("action.stop")}Stop</span><span class="btn primary disabled">Disabled</span><span class="btn secondary focus">With focus</span></div>
    <h3 style="margin-top:14px">BADGES</h3>
    <div class="row"><span class="badge b-pers">{c.ic("billing.personal")}Personal</span><span class="badge b-org">{c.ic("billing.organization")}Organization</span><span class="badge b-attn">Waiting for you</span><span class="badge b-agent">AGENT</span><span class="cnt">3</span><span class="t-caption muted">USD 0.62 estimate</span></div>
    <h3 style="margin-top:14px">TAGS</h3>
    <div class="chips"><span class="chip tag-spruce">sync</span><span class="chip tag-loch">server</span><span class="chip tag-heather">permissions</span><span class="chip tag-rowan">database</span><span class="chip tag-amber">statistics</span><span class="chip tag-gorse">question</span><span class="chip tag-moss">search</span><span class="chip tag-granite">export</span></div>
  </div>
  <div class="grp"><h3>TRIGGER STATE (ON THE MESSAGE)</h3><div class="row">{trig}</div>
    <h3 style="margin-top:6px">RUN STATE</h3><div class="row">{runs}</div>
    <h3 style="margin-top:6px">BANNERS</h3>
    <div class="stack">
      <div class="banner warn">{c.ic("state.offline")}<span class="grow">Offline. 3 messages waiting to be sent.</span><span class="btn ghost">Details</span></div>
      <div class="banner info">{c.ic("state.reconnecting")}<span class="grow">Reconnecting, catching up on missed events.</span></div>
      <div class="banner err">{c.ic("state.error")}<span class="grow">No access to this channel anymore.</span></div>
      <div class="banner attn">{c.ic("security.step-up")}<span class="grow">This change needs a fresh confirmation.</span><span class="btn primary">Confirm</span></div>
    </div>
  </div>
  <div class="grp"><h3>LIVE BOX</h3><div class="stack">{_livebox_running(c)}{waiting}{failed}
    <div class="lb-line">{c.ic("run.completed", "sm c-completed")}<b>Ran 28 min</b><span>3 agents</span><span class="t-code-sm">USD 1.20 estimate</span><span class="grow"></span>{c.ic("action.collapse", "sm")}</div>
    <div class="lb-line">{c.ic("run.cancelled", "sm c-cancelled")}<b>Canceled after 1 min</b><span>by Alex</span><span class="grow"></span>{c.ic("action.collapse", "sm")}</div>
  </div></div>
  <div class="grp"><h3>COMPONENTS ON MESSAGES</h3><div class="stack">
    <div class="card" style="margin:0"><div class="hd">{c.ic("security.step-up", "c-attn")}Approve draft</div><div class="t-body-sm">Post for the announcements channel, 3 paragraphs.</div>
      <div class="row"><span class="btn success">Approve</span><span class="btn secondary">Edit</span><span class="btn ghost">Discard</span></div></div>
    <div class="card" style="margin:0"><div class="hd">{c.ic("context", "c-accent")}Choose model</div>
      <div class="input select" style="width:260px">model-large{c.ic("action.expand")}</div>
      <div class="menu" style="margin-top:6px"><div class="it sel">{c.ic("trigger.received", "sm c-accent")}model-large</div><div class="it">model-fast</div><div class="it">model-small</div></div></div>
    <pre class="code" style="margin:0">if now.Sub(s.lastHeartbeat) &gt; cfg.ResumeWindow {{
    return closeWith(4006)
}}</pre>
    <div class="card" style="margin:0"><div class="hd">{c.ic("trigger.done", "sm tr-done")}Approved</div><div class="used-line">{c.ic("person", "sm")}Alex chose "Allow once" at 10:24. The buttons are used up.</div></div>
  </div></div>
  <div class="grp"><h3>SPACE AND CHANNEL TREE</h3><div class="sidedemo">
    <div class="space">{c.ic("action.expand", "caret")}<span class="sico" style="background:color-mix(in oklab,var(--ds-color-category-heather-on-sidebar) 22%,transparent);color:var(--ds-color-category-heather-on-sidebar)">{c.ic("headset")}</span><span class="t-section">Support</span></div>
    <div class="ch unread">{c.ic("channel.forum")}requests<span class="end"><span class="livedot"></span><span class="cnt">3</span></span></div>
    <div class="ch sel">{c.ic("channel.text")}general</div>
    <div class="ch mute">{c.ic("channel.text")}reports<span class="end">{c.ic("action.mute", "sm")}</span></div>
    <div class="space">{c.ic("action.collapse", "caret")}<span class="sico" style="background:color-mix(in oklab,var(--ds-color-category-moss-on-sidebar) 22%,transparent);color:var(--ds-color-category-moss-on-sidebar)">{c.ic("flask")}</span><span class="t-section">Research</span><span class="dot"></span></div>
  </div></div>
  <div class="grp"><h3>QUICK SWITCHER (⌘K / CTRL+K)</h3>
    <div class="qs"><div class="q">{c.ic("action.search")}<span>serv<span class="textcaret"></span></span><span style="margin-left:auto"><kbd>esc</kbd></span></div>
      <div class="res"><div class="r sel">{c.ic("channel.forum", "c-accent")}server<span class="path">Engineering</span></div><div class="r">{c.ic("thread")}Gateway: resume after network change<span class="path">server</span></div><div class="r">{c.ic("agent")}dev-runner<span class="path">Agent</span></div></div></div>
  </div>
  <div class="grp" style="grid-column:1/3"><h3>INPUT WITH SLASH SUGGESTIONS</h3>
    <div class="composer-wrap" style="padding:150px 0 0">
      <div class="suggest" role="listbox" aria-label="Commands" style="left:0;right:0;bottom:56px">
        <div class="hd">Commands from dev-runner, operator of this channel</div>
        <div class="sug sel"><span class="cmd">/status</span><span class="desc">State of the current run</span><span class="src">Control command</span></div>
        <div class="sug"><span class="cmd">/stop</span><span class="desc">Cancel the run, never goes to the model</span><span class="src">Control command</span></div>
        <div class="sug"><span class="cmd">/stats</span><span class="desc">Channel metrics</span><span class="src">research-bot</span></div>
      </div>
      <div class="composer">{c.ic("action.attach")}<span class="txt">/st<span class="textcaret"></span></span>{c.ic("action.voice")}{c.ic("action.send", "c-accent")}</div>
    </div>
  </div>
</div></section>"""


def _mobile_section(c: _Ctx) -> str:
    left = f"""
<div class="phone" data-density="touch"><div class="m-side"><div class="sb"></div>
  <div class="brand" style="padding:6px 4px 10px"><span class="mark" aria-hidden="true"><svg viewBox="0 0 24 24"><path d="M3 13.5 12 5l9 8.5" fill="none" stroke="var(--ds-color-surface-sidebar)" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round"/><path d="M7.5 12v7h9v-7" fill="none" stroke="var(--ds-color-surface-sidebar)" stroke-width="2.6" stroke-linejoin="round"/></svg></span><span class="name">Bothy</span><span style="margin-left:auto">{c.ic("action.search")}</span></div>
  <div class="running" style="margin:0">{c.ic("run.running", "sm")}Running now<span class="count cnt">2</span></div>
  <div class="space">{c.ic("action.expand", "caret")}<span class="sico" style="background:color-mix(in oklab,var(--ds-color-category-heather-on-sidebar) 22%,transparent);color:var(--ds-color-category-heather-on-sidebar)">{c.ic("headset")}</span><span class="t-section">Support</span></div>
  <div class="ch">{c.ic("channel.text")}general</div>
  <div class="ch unread">{c.ic("channel.forum")}requests<span class="end"><span class="livedot"></span><span class="cnt">3</span></span></div>
  <div class="ch mute">{c.ic("channel.text")}reports</div>
  <div class="space">{c.ic("action.expand", "caret")}<span class="sico" style="background:color-mix(in oklab,var(--ds-color-category-loch-on-sidebar) 22%,transparent);color:var(--ds-color-category-loch-on-sidebar)">{c.ic("code")}</span><span class="t-section">Engineering</span></div>
  <div class="ch">{c.ic("channel.forum")}server<span class="end"><span class="livedot"></span></span></div>
  <div class="ch">{c.ic("channel.forum")}apps</div>
  <div class="space">{c.ic("action.collapse", "caret")}<span class="sico" style="background:color-mix(in oklab,var(--ds-color-category-amber-on-sidebar) 22%,transparent);color:var(--ds-color-category-amber-on-sidebar)">{c.ic("megaphone")}</span><span class="t-section">Marketing</span><span class="dot"></span></div>
  </div>
  <div class="m-tabbar"><div class="on">{c.ic("channel.forum")}Spaces</div><div>{c.ic("action.notifications")}Mentions</div><div>{c.ic("run.running")}Running</div><div>{c.ic("person")}Me</div></div>
</div>"""
    right = f"""
<div class="phone" data-density="touch"><div class="sb"></div>
  <div class="nav"><span class="back">{c.ic("action.back", "lg", "Back")}</span><span class="t-heading">Gateway: resume after network change</span>{c.ic("action.more", "lg")}</div>
  <div class="offline">{c.ic("state.offline", "sm")}Offline. 1 message waiting.</div>
  <div class="m-lb" role="status" aria-label="Running for 4 minutes, stop possible">{c.ic("run.running", "c-running")}<div class="grow"><div class="t-label">Running 4:12 · 62%</div><div class="stl">go test ./internal/sync/...</div></div><span class="m-stop" role="button" aria-label="Stop run">{c.ic("action.stop")}</span></div>
  <div class="msgs" style="flex:1;justify-content:flex-end">
    <div class="msg agent"><span class="av agent c-spruce">{c.ic("agent")}</span><div><div class="who"><b>dev-runner</b><span class="ts">10:16</span></div>
      <div class="card"><div class="hd">{c.ic("security.step-up", "c-attn")}Approval needed</div><div class="t-body-sm"><code class="inl">go test</code> wants to use the network.</div>
      <div class="row" style="flex-direction:column;align-items:stretch"><span class="btn primary" style="justify-content:center">Allow once</span><span class="btn secondary" style="justify-content:center">For this session</span><span class="btn ghost" style="justify-content:center">Deny</span></div></div></div></div>
    <div class="msg"><span class="av c-heather">AL</span><div><div class="who"><b>Alex</b><span class="ts">10:22</span>{c.ic("trigger.working", "tr-working", "Working")}</div><div class="body"><p>Please also check expired threads.</p></div></div></div>
    <div class="msg cont pending"><span class="av c-heather">AL</span><div><div class="body"><p>And then please summarize.</p></div><div class="who"><span class="ts">{c.ic("trigger.queued", "sm tr-queued")}sending</span></div></div></div>
  </div>
  <div class="composer">{c.ic("action.attach", "lg")}<span class="txt ph">Follow-up to dev-runner</span>{c.ic("action.voice", "lg")}</div>
</div>"""
    return (
        '<section class="pv" id="mobile"><h2 class="t-title">iPhone and Android</h2><div class="phones">'
        + left
        + right
        + "</div></section>"
    )


def _terminal_section(c: _Ctx) -> str:
    names = ("black", "red", "green", "yellow", "blue", "magenta", "cyan", "white")
    swatches = "".join(
        f'<div class="a{i}"><i></i>{i} {("bright-" if i > 7 else "") + names[i % 8]}</div>' for i in range(16)
    )
    screen = (
        '<span class="a6">~/src/server</span> <span class="a8">main</span> $ go test ./internal/sync/...\n'
        '<span class="a2">ok</span>      internal/sync           4.212s\n'
        '<span class="a2">ok</span>      internal/sync/gateway   1.874s\n'
        '<span class="a1">FAIL</span>    internal/sync/outbox    <span class="a3">0.912s</span>\n'
        '<span class="a8">--- resume_test.go:88: expected seq 41, got 40</span>\n'
        '<span class="a6">~/src/server</span> <span class="a8">main</span> $ git status --short\n'
        ' <span class="a1">M</span> internal/sync/gateway.go\n'
        '<span class="a2">A</span>  internal/sync/outbox/<span class="sel">resume_test.go</span>\n'
        '<span class="a5">??</span> notes/<span class="a4">plan.md</span>\n'
        '<span class="a6">~/src/server</span> <span class="a8">main</span> $ <span class="cur"> </span>'
    )
    desktop = f"""
<div class="term">
  <div class="term-head">{c.ic("terminal-window")}<span class="t-heading">Build host</span><span class="presence"><i></i>online</span><span class="badge b-org">{c.ic("billing.organization")}Organization</span><span class="fp">Fp 7KQ2 9XTA</span><span class="grow"></span><span class="t-caption secondary">Left 11:42</span></div>
  <div class="term-tabs"><span class="term-tab on">shell 1</span><span class="term-tab">claude: server</span><span class="term-tab">{c.ic("action.new", "sm")}New session</span><span class="grow"></span><span class="btn secondary">Detach</span><span class="btn danger-out">End</span></div>
  <div class="screen" role="log" aria-label="Terminal shell 1 on Build host">{screen}</div>
  <div class="term-foot"><span>{c.ic("security.lock", "sm")}End-to-end encrypted</span><span>Profile shell</span><span>Idle timeout active</span></div>
</div>
<div class="ansi" aria-label="ANSI colors 0 to 15">{swatches}</div>
<div class="dlgs">
  <div class="dlg"><div class="hd">{c.ic("security.step-up", "c-attn")}Open terminal</div>
    <dl class="kv"><dt>Host</dt><dd>Build host</dd><dt>Profile</dt><dd>shell</dd><dt>Login</dt><dd><span class="badge b-org">{c.ic("billing.organization")}Organization</span></dd></dl>
    <div class="t-caption muted" style="margin-top:8px">Every opening is logged.</div>
    <div class="row"><span class="btn ghost">Cancel</span><span class="btn primary">{c.ic("security.key")}Confirm with passkey</span></div></div>
  <div class="dlg warn"><div class="hd">{c.ic("state.warning", "c-failed")}The host's key has changed</div>
    <dl class="kv"><dt>Stored</dt><dd class="mono">7KQ2 9XTA M4PB 1C8D</dd><dt>Now</dt><dd class="mono">Q0ZJ 55RW 2HNE K7TS</dd></dl>
    <div class="t-caption secondary" style="margin-top:8px">Without re-pairing by an owner, this is a sign of an attack.</div>
    <div class="row"><span class="btn secondary">Accept new key</span><span class="btn danger">Do not connect</span></div></div>
</div>"""
    keys = "".join(
        f'<span class="key{on}">{k}</span>'
        for k, on in (
            ("Esc", ""),
            ("Tab", ""),
            ("Ctrl", " on"),
            ("Alt", ""),
            ("&#8592;", ""),
            ("&#8593;", ""),
            ("&#8595;", ""),
            ("&#8594;", ""),
            ("|", ""),
            ("~", ""),
            ("/", ""),
            ("-", ""),
        )
    )
    phone = f"""
<div class="phone" data-density="touch"><div class="sb"></div>
  <div class="nav"><span class="back">{c.ic("action.back", "lg", "Back")}</span><span class="t-heading">Build host</span><span class="badge b-org">Org.</span></div>
  <div class="term-tabs"><span class="term-tab on">shell 1</span><span class="grow"></span><span class="t-caption secondary">Left 11:42</span><span class="btn secondary">Detach</span></div>
  <div class="screen" role="log" aria-label="Terminal shell 1">{screen.replace("internal/sync/", "").replace("resume_test.go:88: expected seq 41, got 40", "resume_test.go:88: seq 41 != 40")}</div>
  <div class="keys" role="toolbar" aria-label="Extra keys">{keys}</div>
</div>"""
    return (
        '<section class="pv" id="terminal"><h2 class="t-title">Terminal</h2><div class="tw"><div>'
        + desktop
        + "</div>"
        + phone
        + "</div></section>"
    )


SLO_FILE = Path(__file__).resolve().parents[3] / "infra" / "observability" / "slo" / "sync.yaml"
SLO_NAME = "bothy-sync/delivery-latency"
_SLO_ENTRY = re.compile(r'^  - name: "delivery-latency"\n(?P<body>(?:(?!  - name:).*\n)*)', re.M)


def delivery_slo(path: Path = SLO_FILE) -> tuple[float, float]:
    """Return (objective in percent, threshold in seconds) of the delivery-latency SLO.

    Reads the Sloth file that also generates the alert rules, so the preview shows the real
    target. The threshold is the `le` bucket of the error query. Raises ValueError if the
    SLO, its objective or its threshold is missing.
    """
    entry = _SLO_ENTRY.search(path.read_text(encoding="utf-8"))
    if entry is None:
        raise ValueError(f"{path}: no SLO delivery-latency")
    objective = re.search(r"^\s+objective:\s*([0-9.]+)\s*$", entry.group("body"), re.M)
    threshold = re.search(r'le="([0-9.]+)"', entry.group("body"))
    if objective is None or threshold is None:
        raise ValueError(f"{path}: delivery-latency lacks objective or le threshold")
    return float(objective.group(1)), float(threshold.group(1))


def _num(value: float) -> str:
    """Format a number the English way, without trailing zeros (99.5 -> '99.5', 500.0 -> '500', 2941.0 -> '2,941')."""
    return f"{value:,.10g}"


def _viz_section(c: _Ctx) -> str:
    bars = []
    days = 7
    for d in range(days):
        y = 150
        for k in range(1, 9):
            h = 4 + ((d * 7 + k * 5) % 13)
            y -= h
            bars.append(
                f'<rect x="{44 + d * 62}" y="{y}" width="40" height="{h}" fill="var(--ds-color-viz-series-{k})"/>'
            )
    grid = "".join(
        f'<line x1="36" x2="480" y1="{y}" y2="{y}" stroke="var(--ds-color-viz-grid)"/>' for y in (30, 70, 110)
    )
    axis = '<line x1="36" x2="480" y1="150" y2="150" stroke="var(--ds-color-viz-axis)"/><line x1="36" x2="36" y1="20" y2="150" stroke="var(--ds-color-viz-axis)"/>'
    labels = "".join(
        f'<text x="{56 + d * 62}" y="166">{t}</text>'
        for d, t in enumerate(("Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"))
    )
    names = (
        "dev-runner",
        "review-bot",
        "support-agent",
        "research-bot",
        "release-bot",
        "docs-bot",
        "import-bot",
        "others",
    )
    legend = "".join(
        f'<span><i style="background:var(--ds-color-viz-series-{k})"></i>{n}</span>' for k, n in enumerate(names, 1)
    )
    # Share of live deliveries within the threshold of the SLO, per hour. Target and threshold
    # come from the SLO file (25 "Delivery and sync live"); the sample values are invented.
    objective, threshold_s = delivery_slo()
    low = objective - (100 - objective)

    def y_of(value: float) -> float:
        return round(150 - (value - low) / (100 - low) * 120, 1)

    shares = (99.95, 99.9, 99.8, 99.92, 99.7, 99.85, 99.4, 99.75, 99.9, 99.97, 99.88, 99.93, 99.9)
    pts = [(56 + i * 35, y_of(v)) for i, v in enumerate(shares)]
    line = " ".join(f"{x},{y}" for x, y in pts)
    area = f"M56,150 L{line.replace(' ', ' L')} L476,150 Z"
    y_axis = "".join(
        f'<line x1="56" x2="480" y1="{y_of(v)}" y2="{y_of(v)}" stroke="var(--ds-color-viz-grid)"/>'
        f'<text x="50" y="{y_of(v) + 4}" text-anchor="end">{_num(v)}</text>'
        for v in (low, 100)
    )
    target = (
        f'<line x1="56" x2="480" y1="{y_of(objective)}" y2="{y_of(objective)}" '
        'stroke="var(--ds-color-viz-axis)" stroke-dasharray="6 4"/>'
        f'<text x="50" y="{y_of(objective) + 4}" text-anchor="end">{_num(objective)}</text>'
    )
    share_axis = (
        '<line x1="56" x2="480" y1="150" y2="150" stroke="var(--ds-color-viz-axis)"/>'
        '<line x1="56" x2="56" y1="20" y2="150" stroke="var(--ds-color-viz-axis)"/>'
    )
    tiles = "".join(
        f'<div class="tile"><div class="lab">{lab}</div><div class="num">{num}</div><div class="cmp">{c.ic(ic, "sm")}{cmp}</div></div>'
        for lab, num, ic, cmp in (
            ("Runs today", "128", "trigger.followup", "12 more than yesterday"),
            ("Tool calls", "2,941", "trigger.followup", "8% more than yesterday"),
            ("Costs today", "USD 14.20", "cost", "Estimate, API mode only"),
            ("Delivery on target", "99.9%", "run.completed", f"Target {_num(objective)}%"),
        )
    )
    lanes = f"""
<div class="lanes" role="table" aria-label="Live topology">
  <div class="lane"><div class="lh"><span class="sico" style="color:var(--ds-color-category-loch-fg)">{c.ic("code")}</span>Engineering</div><div class="cards">
    <div class="rc s-running"><div class="top">{c.ic("run.running", "sm c-running")}dev-runner<span class="t-caption muted">4:12</span></div><div class="bot"><span class="tool">{c.ic("step.tool_call")}Bash</span><span class="t-code-sm">go test</span></div><div class="bot"><span class="badge b-org">Org.</span><span class="t-code-sm">USD 0.62</span></div></div>
    <div class="rc s-completed"><div class="top">{c.ic("run.completed", "sm c-completed")}review-bot<span class="t-caption muted">2 min ago</span></div><div class="bot"><span class="tool">{c.ic("step.tool_call")}Read</span><span class="t-code-sm">3 files</span></div><div class="bot"><span class="badge b-org">Org.</span><span class="t-code-sm">USD 0.08</span></div></div>
  </div></div>
  <div class="lane"><div class="lh"><span class="sico" style="color:var(--ds-color-category-heather-fg)">{c.ic("headset")}</span>Support</div><div class="cards">
    <div class="rc s-waiting"><div class="top">{c.ic("run.waiting", "sm c-waiting")}support-agent<span class="t-caption muted">waiting</span></div><div class="bot">Approval waiting</div><div class="bot"><span class="badge b-pers">Personal</span><span class="t-caption muted">information only</span></div></div>
  </div></div>
</div>"""
    charts = f"""
<div class="chart"><h4>Runs per agent, last 7 days</h4><svg viewBox="0 0 490 172" role="img" aria-label="Stacked bars of the runs per agent">{grid}{"".join(bars)}{axis}{labels}</svg><div class="legend">{legend}</div></div>
<div class="chart"><h4>Delivery today: share within {_num(threshold_s * 1000)} ms</h4><svg viewBox="0 0 490 172" role="img" aria-label="Area of the share of deliveries within the threshold, with a target line">{y_axis}<path d="{area}" fill="var(--ds-color-viz-area-fill)"/><polyline points="{line}" fill="none" stroke="var(--ds-color-viz-series-1)" stroke-width="2"/>{target}{share_axis}</svg><div class="legend"><span><i style="background:var(--ds-color-viz-series-1)"></i>Share in %, per hour</span><span>dashed: target {_num(objective)}% (SLO {SLO_NAME})</span></div></div>"""
    slo = f"""
<div class="slo"><span class="badge b-ok">{c.ic("run.completed", "sm")}Delivery: on target</span><span class="badge b-warn">{c.ic("state.warning", "sm")}Reconnect: budget tight</span><span class="badge b-bad">{c.ic("state.error", "sm")}Push: target missed</span><span class="badge b-none">{c.ic("state.info", "sm")}Agent runs: not enough data</span><span class="t-caption muted" style="align-self:center">own measurement</span></div>"""
    return (
        '<section class="pv" id="viz"><h2 class="t-title">Statistics and live views</h2><div class="vz">'
        + tiles
        + lanes
        + charts
        + slo
        + "</div></section>"
    )


def render(model: Model, vendor: Path, header: str) -> str:
    """Render the self-contained HTML preview of all components in both themes."""
    c = _Ctx(model)
    tokens_css = emit_css.render(model, header)
    return (
        "<!doctype html>\n"
        f"<!-- {header} -->\n"
        "<!-- Fonts: Atkinson Hyperlegible Next and Mono, SIL Open Font License 1.1 (see tools/design-tokens/vendor/fonts). Icons: Phosphor, MIT (see tools/design-tokens/vendor/phosphor/LICENSE). -->\n"
        '<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">'
        "<title>Design system: preview</title>\n<style>\n"
        + _font_faces(vendor)
        + "\n"
        + tokens_css
        + CSS
        + "</style></head><body>\n"
        + _sprite(model, vendor)
        + '\n<div class="pv-bar"><b>Design system</b><span class="grow"></span>'
        '<label>Theme <select id="pv-theme"><option value="system">System</option><option value="light">Light</option><option value="dark">Dark</option></select></label>'
        '<label>Density <select id="pv-density"><option value="comfortable">comfortable</option><option value="compact">compact</option><option value="touch">touch</option></select></label>'
        '<label>Motion <select id="pv-motion"><option value="system">System</option><option value="full">full</option><option value="reduced">reduced</option></select></label></div>\n'
        + _layout_section(c)
        + _components_section(c)
        + _mobile_section(c)
        + _terminal_section(c)
        + _viz_section(c)
        + _palette_section(model)
        + "<script>"
        + SCRIPT
        + "</script></body></html>\n"
    )
