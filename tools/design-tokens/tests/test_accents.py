# SPDX-License-Identifier: Apache-2.0
"""Tests for the accent variants: derivation, step search and validation of the rules member."""

import json
import unittest
from pathlib import Path

from tokenkit import accents, dtcg, model, rules

HERE = Path(__file__).resolve().parents[1]
ROOT = HERE.parents[1]


def _components(hex_value):
    hex_value = hex_value.lstrip("#")
    return [int(hex_value[i : i + 2], 16) / 255 for i in (0, 2, 4)]


def _theme(palette, roles):
    """Resolved tokens of one synthetic theme.

    Args:
        palette: mapping scale -> mapping step -> hex.
        roles: mapping role path -> alias target ("palette.x.700") or a hex value.
    """
    doc: dict = {"palette": {"$type": "color"}}
    for scale, steps in palette.items():
        doc["palette"][scale] = {
            step: {"$value": {"colorSpace": "srgb", "components": _components(h)}} for step, h in steps.items()
        }
    for path, value in roles.items():
        node = doc
        parts = path.split(".")
        for part in parts[:-1]:
            node = node.setdefault(part, {})
        if value.startswith("#"):
            node[parts[-1]] = {"$type": "color", "$value": {"colorSpace": "srgb", "components": _components(value)}}
        else:
            node[parts[-1]] = {"$type": "color", "$value": "{" + value + "}"}
    return dtcg.resolve(dtcg.flatten(doc))


# Greys with the same steps on both scales; contrast to white rises with the step number.
_GREYS = {"500": "#949494", "600": "#767676", "700": "#595959", "800": "#3b3b3b", "900": "#1e1e1e"}


def _rules(accent_list, roles, pairs, default="spruce"):
    return {
        "contrast_pairs": pairs,
        "accent_variants": {"default": default, "accents": accent_list, "roles": roles},
    }


_ON_WHITE = [{"usage": "test", "kind": "text", "fg": "color.accent.bg", "bg": "color.surface.canvas"}]


class DeriveTest(unittest.TestCase):
    def _themes(self, palette, light_role="palette.spruce.700", dark_role="palette.spruce.700"):
        return {
            "light": _theme(palette, {"color.accent.bg": light_role, "color.surface.canvas": "#ffffff"}),
            "dark": _theme(palette, {"color.accent.bg": dark_role, "color.surface.canvas": "#ffffff"}),
        }

    def test_same_step_on_other_scale(self):
        themes = self._themes({"spruce": _GREYS, "loch": _GREYS})
        out = accents.derive(themes, _rules(["spruce", "loch"], ["color.accent.bg"], _ON_WHITE))
        variant = out[("light", "loch")]["color.accent.bg"]
        self.assertEqual((variant.scale, variant.step, variant.moved_from), ("loch", "700", None))
        self.assertEqual(variant.hex, "#595959")
        self.assertTrue(variant.swapped)

    def test_missing_step_takes_nearest(self):
        spruce = {**_GREYS, "950": "#101010", "975": "#080808"}
        loch = {**_GREYS, "950": "#101010"}
        themes = self._themes({"spruce": spruce, "loch": loch}, "palette.spruce.975", "palette.spruce.975")
        out = accents.derive(themes, _rules(["spruce", "loch"], ["color.accent.bg"], _ON_WHITE))
        self.assertEqual(out[("light", "loch")]["color.accent.bg"].step, "950")
        self.assertEqual(out[("dark", "loch")]["color.accent.bg"].step, "950")
        self.assertIsNone(out[("light", "loch")]["color.accent.bg"].moved_from)

    def test_missing_step_tie_goes_darker_in_light_and_lighter_in_dark(self):
        spruce = {**_GREYS, "850": "#2c2c2c"}
        themes = self._themes({"spruce": spruce, "loch": _GREYS}, "palette.spruce.850", "palette.spruce.850")
        out = accents.derive(themes, _rules(["spruce", "loch"], ["color.accent.bg"], _ON_WHITE))
        self.assertEqual(out[("light", "loch")]["color.accent.bg"].step, "900")
        self.assertEqual(out[("dark", "loch")]["color.accent.bg"].step, "800")

    def test_role_not_aliasing_spruce_is_kept(self):
        model_ = model.build(ROOT / "design" / "tokens" / "theme.resolver.json")
        data = json.loads((ROOT / "design" / "design-rules.json").read_text(encoding="utf-8"))
        out = accents.derive(model_.full["theme"], data)
        base = model_.full["theme"]["dark"]["color.focus.ring"]
        self.assertEqual(base.alias_of, "palette.gorse.300")
        for accent in data["accent_variants"]["accents"]:
            variant = out[("dark", accent)]["color.focus.ring"]
            self.assertFalse(variant.swapped)
            self.assertEqual((variant.scale, variant.step, variant.hex), ("gorse", "300", base.hex))

    def test_default_accent_equals_base_roles(self):
        model_ = model.build(ROOT / "design" / "tokens" / "theme.resolver.json")
        data = json.loads((ROOT / "design" / "design-rules.json").read_text(encoding="utf-8"))
        out = accents.derive(model_.full["theme"], data)
        for theme, tokens in model_.full["theme"].items():
            for role in data["accent_variants"]["roles"]:
                variant = out[(theme, "spruce")][role]
                self.assertEqual(variant.hex, tokens[role].hex, f"{theme} {role}")
                self.assertEqual(f"palette.{variant.scale}.{variant.step}", tokens[role].alias_of)
                self.assertIsNone(variant.moved_from)

    def test_failing_variant_moves_to_passing_step(self):
        # loch.600 (#808080) has 3.95:1 on white, loch.700 passes 4.5.
        loch = {**_GREYS, "600": "#808080", "700": "#666666"}
        themes = self._themes({"spruce": _GREYS, "loch": loch}, "palette.spruce.600", "palette.spruce.600")
        out = accents.derive(themes, _rules(["spruce", "loch"], ["color.accent.bg"], _ON_WHITE))
        variant = out[("light", "loch")]["color.accent.bg"]
        self.assertEqual((variant.step, variant.moved_from), ("700", "600"))

    def test_moves_lighter_when_role_is_the_lighter_colour(self):
        pairs = [{"usage": "test", "kind": "text", "fg": "color.accent.bg", "bg": "color.surface.canvas"}]
        loch = {"500": "#cccccc", "600": "#b0b0b0", "700": "#606060", "800": "#3b3b3b"}
        spruce = {"500": "#cccccc", "600": "#c0c0c0", "700": "#b8b8b8", "800": "#3b3b3b"}
        themes = {
            t: _theme(
                {"spruce": spruce, "loch": loch},
                {"color.accent.bg": "palette.spruce.700", "color.surface.canvas": "#000000"},
            )
            for t in ("light", "dark")
        }
        out = accents.derive(themes, _rules(["spruce", "loch"], ["color.accent.bg"], pairs))
        self.assertEqual(out[("light", "loch")]["color.accent.bg"].step, "600")

    def test_no_passing_step_fails_build(self):
        loch = {"500": "#d0d0d0", "600": "#c0c0c0", "700": "#b0b0b0", "800": "#a0a0a0", "900": "#909090"}
        themes = self._themes({"spruce": _GREYS, "loch": loch})
        with self.assertRaises(accents.AccentContrastError) as ctx:
            accents.derive(themes, _rules(["spruce", "loch"], ["color.accent.bg"], _ON_WHITE))
        self.assertIsInstance(ctx.exception, rules.RuleError)
        message = str(ctx.exception)
        for part in ("accent loch", "theme light", "no step of loch", "color.accent.bg on color.surface.canvas"):
            self.assertIn(part, message)

    def test_deterministic(self):
        model_ = model.build(ROOT / "design" / "tokens" / "theme.resolver.json")
        data = json.loads((ROOT / "design" / "design-rules.json").read_text(encoding="utf-8"))
        self.assertEqual(accents.derive(model_.full["theme"], data), accents.derive(model_.full["theme"], data))

    def test_rules_validation(self):
        themes = self._themes({"spruce": _GREYS, "loch": _GREYS})
        cases = {
            "default": _rules(["spruce", "loch"], ["color.accent.bg"], _ON_WHITE, default="gorse"),
            "accents": _rules(["spruce", "nope"], ["color.accent.bg"], _ON_WHITE),
            "roles": _rules(["spruce", "loch"], ["color.accent.missing"], _ON_WHITE),
        }
        for member, data in cases.items():
            with self.subTest(member=member), self.assertRaises(rules.RuleError) as ctx:
                accents.derive(themes, data)
            self.assertIn(f"accent_variants.{member}", str(ctx.exception))
            self.assertNotIsInstance(ctx.exception, accents.AccentContrastError)


if __name__ == "__main__":
    unittest.main()
