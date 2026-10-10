"""Tests for DTCG 2025.10 parsing, type inheritance and alias resolution."""

import unittest

from tokenkit import dtcg


def _px(v):
    return {"value": v, "unit": "px"}


class FlattenTest(unittest.TestCase):
    def test_group_type_is_inherited(self):
        doc = {"space": {"$type": "dimension", "s4": {"$value": _px(4)}}}
        tokens = dtcg.flatten(doc)
        self.assertEqual(tokens["space.s4"].type, "dimension")

    def test_token_type_overrides_group_type(self):
        doc = {"g": {"$type": "dimension", "n": {"$type": "number", "$value": 2}}}
        self.assertEqual(dtcg.flatten(doc)["g.n"].type, "number")

    def test_object_with_value_and_children_is_rejected(self):
        doc = {"a": {"$value": 1, "$type": "number", "b": {"$value": 2, "$type": "number"}}}
        with self.assertRaises(dtcg.TokenError):
            dtcg.flatten(doc)

    def test_invalid_name_is_rejected(self):
        doc = {"a": {"b{c": {"$value": 1, "$type": "number"}}}
        with self.assertRaises(dtcg.TokenError):
            dtcg.flatten(doc)

    def test_root_token_keeps_dollar_root_segment(self):
        doc = {"accent": {"$type": "number", "$root": {"$value": 1}, "hover": {"$value": 2}}}
        tokens = dtcg.flatten(doc)
        self.assertIn("accent.$root", tokens)
        self.assertIn("accent.hover", tokens)

    def test_extensions_and_description_are_kept(self):
        doc = {"n": {"$type": "number", "$value": 1, "$description": "d", "$extensions": {"x.y": 1}}}
        token = dtcg.flatten(doc)["n"]
        self.assertEqual(token.description, "d")
        self.assertEqual(token.extensions, {"x.y": 1})


class ResolveTest(unittest.TestCase):
    def test_alias_chain_resolves_value_and_type(self):
        doc = {
            "base": {"$type": "number", "one": {"$value": 1}},
            "mid": {"$value": "{base.one}"},
            "top": {"$value": "{mid}"},
        }
        resolved = dtcg.resolve(dtcg.flatten(doc))
        self.assertEqual(resolved["top"].value, 1)
        self.assertEqual(resolved["top"].type, "number")
        self.assertEqual(resolved["top"].alias_of, "mid")

    def test_alias_inside_composite_value_resolves(self):
        doc = {
            "font": {"$type": "fontFamily", "sans": {"$value": ["A", "sans-serif"]}},
            "size": {"$type": "dimension", "m": {"$value": _px(15)}},
            "type": {
                "$type": "typography",
                "body": {
                    "$value": {
                        "fontFamily": "{font.sans}",
                        "fontSize": "{size.m}",
                        "fontWeight": 400,
                        "lineHeight": 1.4,
                        "letterSpacing": _px(0),
                    }
                },
            },
        }
        body = dtcg.resolve(dtcg.flatten(doc))["type.body"].value
        self.assertEqual(body["fontFamily"], ["A", "sans-serif"])
        self.assertEqual(body["fontSize"], _px(15))

    def test_cycle_is_detected(self):
        doc = {"a": {"$value": "{b}"}, "b": {"$value": "{a}"}}
        with self.assertRaises(dtcg.TokenError):
            dtcg.resolve(dtcg.flatten(doc))

    def test_unknown_reference_is_rejected(self):
        doc = {"a": {"$type": "number", "$value": "{missing}"}}
        with self.assertRaises(dtcg.TokenError):
            dtcg.resolve(dtcg.flatten(doc))

    def test_token_without_type_is_rejected(self):
        with self.assertRaises(dtcg.TokenError):
            dtcg.resolve(dtcg.flatten({"a": {"$value": 1}}))


class ValueValidationTest(unittest.TestCase):
    def _resolve_one(self, type_, value):
        return dtcg.resolve(dtcg.flatten({"t": {"$type": type_, "$value": value}}))["t"]

    def test_oklch_colour_is_converted_to_hex(self):
        token = self._resolve_one(
            "color", {"colorSpace": "oklch", "components": [0.6279553606, 0.2576833077, 29.2338851923]}
        )
        self.assertEqual(dtcg.color_hex(token), "#ff0000")

    def test_out_of_gamut_colour_is_rejected(self):
        with self.assertRaises(dtcg.TokenError):
            self._resolve_one("color", {"colorSpace": "oklch", "components": [0.9, 0.4, 140]})

    def test_unknown_colour_space_is_rejected(self):
        with self.assertRaises(dtcg.TokenError):
            self._resolve_one("color", {"colorSpace": "cmyk", "components": [0, 0, 0, 0]})

    def test_dimension_requires_unit(self):
        with self.assertRaises(dtcg.TokenError):
            self._resolve_one("dimension", {"value": 4})

    def test_cubic_bezier_x_must_be_in_unit_interval(self):
        with self.assertRaises(dtcg.TokenError):
            self._resolve_one("cubicBezier", [1.5, 0, 0, 1])

    def test_font_weight_alias_is_accepted(self):
        self.assertEqual(self._resolve_one("fontWeight", "bold").value, "bold")
        self.assertEqual(dtcg.font_weight_number("semi-bold"), 600)

    def test_unknown_type_is_rejected(self):
        with self.assertRaises(dtcg.TokenError):
            self._resolve_one("string", "x")


class MergeTest(unittest.TestCase):
    def test_later_source_wins_and_groups_merge_deeply(self):
        a = {"c": {"$type": "number", "x": {"$value": 1}, "y": {"$value": 2}}}
        b = {"c": {"y": {"$value": 3}}}
        merged = dtcg.merge([a, b])
        tokens = dtcg.resolve(dtcg.flatten(merged))
        self.assertEqual(tokens["c.x"].value, 1)
        self.assertEqual(tokens["c.y"].value, 3)


if __name__ == "__main__":
    unittest.main()
