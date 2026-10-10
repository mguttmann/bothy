# SPDX-License-Identifier: Apache-2.0
"""Tests for colour conversion and WCAG contrast maths."""

import unittest

from tokenkit import color


class OklchToSrgbTest(unittest.TestCase):
    def test_pure_red_round_trips_to_ff0000(self):
        rgb = color.oklch_to_srgb(0.6279553606, 0.2576833077, 29.2338851923)
        self.assertEqual(color.to_hex(rgb), "#ff0000")

    def test_pure_blue_round_trips_to_0000ff(self):
        rgb = color.oklch_to_srgb(0.4520137184, 0.3132143886, 264.0520206381)
        self.assertEqual(color.to_hex(rgb), "#0000ff")

    def test_white_with_zero_chroma(self):
        rgb = color.oklch_to_srgb(1.0, 0.0, 0.0)
        self.assertEqual(color.to_hex(rgb), "#ffffff")

    def test_out_of_gamut_is_reported(self):
        self.assertFalse(color.in_srgb_gamut(color.oklch_to_srgb(0.9, 0.4, 140)))
        self.assertTrue(color.in_srgb_gamut(color.oklch_to_srgb(0.5, 0.05, 200)))

    def test_srgb_to_oklch_inverts(self):
        lightness, chroma, hue = color.srgb_to_oklch(color.hex_to_srgb("#5865f2"))
        back = color.to_hex(color.oklch_to_srgb(lightness, chroma, hue))
        self.assertEqual(back, "#5865f2")


class ContrastTest(unittest.TestCase):
    def test_black_on_white_is_21(self):
        self.assertAlmostEqual(color.contrast("#000000", "#ffffff"), 21.0, places=6)

    def test_contrast_is_symmetric(self):
        self.assertAlmostEqual(color.contrast("#336699", "#eeeeee"), color.contrast("#eeeeee", "#336699"))

    def test_767676_on_white_passes_aa_and_777777_fails(self):
        self.assertGreaterEqual(color.contrast("#767676", "#ffffff"), 4.5)
        self.assertLess(color.contrast("#777777", "#ffffff"), 4.5)


class DeltaETest(unittest.TestCase):
    def test_identical_colours_have_zero_distance(self):
        self.assertAlmostEqual(color.delta_e_ok("#5865f2", "#5865f2"), 0.0)

    def test_distinct_colours_have_positive_distance(self):
        self.assertGreater(color.delta_e_ok("#5865f2", "#0b7a75"), 0.1)


if __name__ == "__main__":
    unittest.main()
