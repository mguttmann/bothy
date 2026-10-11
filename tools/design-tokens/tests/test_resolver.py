# SPDX-License-Identifier: Apache-2.0
"""Tests for the DTCG Resolver Module 2025.10 subset used by the token pipeline."""

import json
import tempfile
import unittest
from pathlib import Path

from tokenkit import resolver


def _write(path: Path, data) -> None:
    path.write_text(json.dumps(data), encoding="utf-8")


class ResolverTest(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.dir = Path(self._tmp.name)
        _write(self.dir / "base.tokens.json", {"n": {"$type": "number", "a": {"$value": 1}}})
        _write(self.dir / "light.tokens.json", {"bg": {"$type": "number", "$value": "{n.a}"}})
        _write(self.dir / "dark.tokens.json", {"bg": {"$type": "number", "$value": 2}})
        _write(
            self.dir / "theme.resolver.json",
            {
                "version": "2025.10",
                "sets": {"base": {"sources": [{"$ref": "base.tokens.json"}]}},
                "modifiers": {
                    "theme": {
                        "contexts": {
                            "light": [{"$ref": "light.tokens.json"}],
                            "dark": [{"$ref": "dark.tokens.json"}],
                        },
                        "default": "light",
                    }
                },
                "resolutionOrder": [{"$ref": "#/sets/base"}, {"$ref": "#/modifiers/theme"}],
            },
        )
        self.doc = resolver.load(self.dir / "theme.resolver.json")

    def tearDown(self):
        self._tmp.cleanup()

    def test_default_context_is_used_without_input(self):
        tokens = self.doc.resolve({})
        self.assertEqual(tokens["bg"].value, 1)

    def test_input_selects_context(self):
        tokens = self.doc.resolve({"theme": "dark"})
        self.assertEqual(tokens["bg"].value, 2)

    def test_unknown_context_is_rejected(self):
        with self.assertRaises(resolver.ResolverError):
            self.doc.resolve({"theme": "sepia"})

    def test_modifier_contexts_are_listed(self):
        self.assertEqual(self.doc.modifiers(), {"theme": ["light", "dark"]})
        self.assertEqual(self.doc.default_input(), {"theme": "light"})

    def test_wrong_version_is_rejected(self):
        _write(self.dir / "bad.resolver.json", {"version": "1.0", "resolutionOrder": []})
        with self.assertRaises(resolver.ResolverError):
            resolver.load(self.dir / "bad.resolver.json")


if __name__ == "__main__":
    unittest.main()
