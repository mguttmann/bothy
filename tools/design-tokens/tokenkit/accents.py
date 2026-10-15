# SPDX-License-Identifier: Apache-2.0
"""Accent variants: the accent roles of every accent an instance may choose, per theme.

An instance picks its accent from the checked palette (`branding.accent_color`, ADR 0044). The
rules member `accent_variants` of `design/design-rules.json` names the allowed accents, the
default and the swapped roles. A role that aliases `palette.<default>.<step>` in a theme is swapped
for the same step of the chosen scale ("equally light steps"); a role that aliases anything else
is kept. A variant that fails a contrast pairing of the rules moves step by step along its scale
until every pairing passes, or the derivation fails (`AccentContrastError`).

Example:
    >>> variants = derive(model.full["theme"], json.loads(RULES.read_text()))
    >>> variants[("light", "loch")]["color.accent.bg"].step
    '700'
"""

from __future__ import annotations

from dataclasses import dataclass

from . import color, dtcg
from .rules import THRESHOLDS, ContrastRow, RuleError, expand_pairs

MEMBER = "accent_variants"


class AccentContrastError(RuleError):
    """No step of a scale lets a swapped role meet one of its contrast pairings.

    A subclass of `RuleError` so callers that only know rule errors still stop; the generator
    reports it as a failed gate (exit 1) instead of an invalid rules file (exit 2).
    """


@dataclass(frozen=True)
class Variant:
    """The value of one accent role in one theme for one accent.

    Attributes:
        role: token path of the role, for example `color.accent.bg`.
        scale: palette scale the value comes from (`loch`), or None for an own value.
        step: step on that scale (`700`), or None for an own value.
        hex: resolved `#rrggbb` value.
        moved_from: start step when the step search moved the role, else None.
        swapped: True when the role follows the accent; False when it keeps its base value.
    """

    role: str
    scale: str | None
    step: str | None
    hex: str
    moved_from: str | None
    swapped: bool

    @property
    def alias(self) -> str | None:
        """Palette path of the value (`palette.loch.700`), or None for an own value."""
        return f"palette.{self.scale}.{self.step}" if self.scale else None


@dataclass(frozen=True)
class AccentRules:
    """The validated rules member `accent_variants`."""

    default: str
    accents: tuple[str, ...]
    roles: tuple[str, ...]


def _scales(tokens: dict[str, dtcg.Token]) -> dict[str, dict[str, str]]:
    """Palette scales of a theme's token map: scale -> step -> hex, steps in numeric order."""
    scales: dict[str, dict[str, str]] = {}
    for path in sorted(tokens, key=_path_key):
        parts = path.split(".")
        if len(parts) == 3 and parts[0] == "palette" and tokens[path].type == "color":
            scales.setdefault(parts[1], {})[parts[2]] = dtcg.color_hex(tokens[path])
    return scales


def _path_key(path: str):
    tail = path.rsplit(".", 1)[-1]
    return (path.rsplit(".", 1)[0], int(tail) if tail.isdigit() else 10**9, tail)


def validate(themes: dict[str, dict[str, dtcg.Token]], data: dict) -> AccentRules:
    """Check the rules member `accent_variants` against the themes.

    Args:
        themes: resolved token map per theme context (palette included).
        data: the whole rules document.

    Returns:
        The validated member.

    Raises:
        RuleError: naming the member (`accent_variants.default`, `.accents` or `.roles`) when the
            default is not an accent, an accent is not a palette scale, or a role is missing in a
            theme.
    """
    member = data.get(MEMBER)
    if not isinstance(member, dict):
        raise RuleError(f"{MEMBER} is missing in the rules file")
    accent_list = member.get("accents")
    roles = member.get("roles")
    if not isinstance(accent_list, list) or not accent_list or len(set(accent_list)) != len(accent_list):
        raise RuleError(f"{MEMBER}.accents must be a non-empty list without duplicates")
    if not isinstance(roles, list) or not roles or len(set(roles)) != len(roles):
        raise RuleError(f"{MEMBER}.roles must be a non-empty list without duplicates")
    if member.get("default") not in accent_list:
        raise RuleError(f"{MEMBER}.default {member.get('default')!r} is not one of {MEMBER}.accents")
    for theme, tokens in themes.items():
        scales = _scales(tokens)
        for accent in accent_list:
            if accent not in scales:
                raise RuleError(f"{MEMBER}.accents: {accent!r} is not a palette scale (theme {theme})")
        for role in roles:
            token = tokens.get(role)
            if token is None or token.type != "color":
                raise RuleError(f"{MEMBER}.roles: {role!r} is not a colour token in theme {theme}")
            if dtcg.color_alpha(token.value) < 1:
                raise RuleError(f"{MEMBER}.roles: {role!r} is translucent in theme {theme}")
    return AccentRules(default=member["default"], accents=tuple(accent_list), roles=tuple(roles))


def _nearest_step(steps: dict[str, str], wanted: str, darker: bool) -> str:
    """The step of `wanted` on a scale, else the nearest by number; a tie goes darker or lighter."""
    if wanted in steps:
        return wanted
    numeric = [s for s in steps if s.isdigit()]
    if not wanted.isdigit() or not numeric:
        raise RuleError(f"{MEMBER}: step {wanted!r} has no numeric neighbour")
    target = int(wanted)
    best = min(abs(int(s) - target) for s in numeric)
    ties = [s for s in numeric if abs(int(s) - target) == best]
    luminance = {s: color.relative_luminance(steps[s]) for s in ties}
    pick = min if darker else max
    return pick(ties, key=lambda s: (luminance[s], int(s)))


def _kept(role: str, token: dtcg.Token) -> Variant:
    parts = (token.alias_of or "").split(".")
    scale, step = (parts[1], parts[2]) if len(parts) == 3 and parts[0] == "palette" else (None, None)
    return Variant(role=role, scale=scale, step=step, hex=dtcg.color_hex(token), moved_from=None, swapped=False)


def _rows(theme, tokens, pairs, current: dict[str, Variant], role: str) -> list[ContrastRow]:
    """Every expanded pairing that contains `role`, evaluated with the current variant colours."""
    out = []
    for pair in pairs:
        if role not in (pair["fg"], pair["bg"]):
            continue
        hexes = []
        for path in (pair["fg"], pair["bg"]):
            if path in current:
                hexes.append(current[path].hex)
            else:
                token = tokens.get(path)
                if token is None or token.type != "color":
                    raise RuleError(f"contrast pair references unknown token {path!r} (theme {theme})")
                hexes.append(dtcg.color_hex(token))
        out.append(
            ContrastRow(
                theme=theme,
                fg=pair["fg"],
                bg=pair["bg"],
                kind=pair["kind"],
                fg_hex=hexes[0],
                bg_hex=hexes[1],
                ratio=color.contrast(hexes[0], hexes[1]),
                required=THRESHOLDS[pair["kind"]],
                usage=pair.get("usage", ""),
            )
        )
    return out


def _search(theme, accent, tokens, pairs, current, role, steps: dict[str, str], start: str) -> Variant:
    """Move `role` along its scale until all its pairings pass (Design note 2 of M4-03)."""
    order = list(steps)  # numeric order
    step = start
    seen = set()
    while True:
        seen.add(step)
        current[role] = Variant(
            role=role,
            scale=accent,
            step=step,
            hex=steps[step],
            moved_from=None if step == start else start,
            swapped=True,
        )
        failing = [r for r in _rows(theme, tokens, pairs, current, role) if not r.passed]
        if not failing:
            return current[role]
        first = failing[0]
        own = color.relative_luminance(steps[step])
        partner = first.bg_hex if first.fg == role else first.fg_hex
        go_darker = own <= color.relative_luminance(partner)
        index = order.index(step)
        neighbours = [order[i] for i in (index - 1, index + 1) if 0 <= i < len(order)]
        candidates = [
            s for s in neighbours if (lum := color.relative_luminance(steps[s])) != own and (lum < own) == go_darker
        ]
        if not candidates or candidates[0] in seen:
            raise AccentContrastError(
                f"accent {accent}, theme {theme}: no step of {accent} meets {first.fg} on {first.bg} "
                f"({first.kind} {first.required:g}, best start {accent}.{start}: {first.ratio:.2f})"
            )
        step = candidates[0]


def derive(themes: dict[str, dict[str, dtcg.Token]], data: dict) -> dict[tuple[str, str], dict[str, Variant]]:
    """Derive the accent roles for every accent and theme.

    Args:
        themes: resolved token map per theme context (`Model.full["theme"]`), palette included.
        data: the whole rules document (`contrast_pairs` and `accent_variants`).

    Returns:
        Mapping (theme, accent) -> role -> Variant, accents in rules order, then themes; every
        role of `accent_variants.roles` is present, kept roles with `swapped` False.

    Raises:
        RuleError: the rules member is invalid (see `validate`).
        AccentContrastError: a swapped role meets a pairing on no step of the chosen scale.
    """
    member = validate(themes, data)
    pairs = expand_pairs(data.get("contrast_pairs", []))
    prefix = f"palette.{member.default}."
    out: dict[tuple[str, str], dict[str, Variant]] = {}
    for accent in member.accents:
        for theme, tokens in themes.items():
            steps = _scales(tokens)[accent]
            darker = theme != "dark"
            current: dict[str, Variant] = {}
            swapped: list[tuple[str, str]] = []
            for role in member.roles:
                token = tokens[role]
                alias = token.alias_of or ""
                if alias.startswith(prefix) and alias.count(".") == 2:
                    start = _nearest_step(steps, alias.removeprefix(prefix), darker)
                    current[role] = Variant(role, accent, start, steps[start], None, True)
                    swapped.append((role, start))
                else:
                    current[role] = _kept(role, token)
            for role, start in swapped:
                _search(theme, accent, tokens, pairs, current, role, steps, start)
            out[(theme, accent)] = {role: current[role] for role in member.roles}
    return out
