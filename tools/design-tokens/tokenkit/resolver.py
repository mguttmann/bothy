# SPDX-License-Identifier: Apache-2.0
"""Subset of the DTCG Resolver Module 2025.10 (https://www.designtokens.org/tr/2025.10/resolver/).

Supported: root `version` (must be "2025.10"), `sets` with `sources`, `modifiers` with
`contexts` and `default`, and `resolutionOrder` entries that reference `#/sets/<name>` or
`#/modifiers/<name>`. Sources are `{"$ref": "<relative file>"}` objects or inline token
groups. Later entries override earlier ones (deep merge).
"""

from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path

from . import dtcg


class ResolverError(ValueError):
    """Raised for an invalid resolver document or input."""


@dataclass
class ResolverDocument:
    """A DTCG resolver document (version 2025.10) with its file path."""

    path: Path
    data: dict

    def modifiers(self) -> dict[str, list[str]]:
        """Return each modifier name with its context names."""
        return {name: list(mod["contexts"]) for name, mod in self.data.get("modifiers", {}).items()}

    def default_input(self) -> dict[str, str]:
        """Return the default context of each modifier."""
        out = {}
        for name, mod in self.data.get("modifiers", {}).items():
            out[name] = mod.get("default", next(iter(mod["contexts"])))
        return out

    def _load_source(self, source) -> dict:
        if isinstance(source, dict) and set(source) == {"$ref"}:
            ref = source["$ref"]
            if ref.startswith("#"):
                raise ResolverError(f"source references inside the document are not supported: {ref}")
            file = (self.path.parent / ref).resolve()
            try:
                return json.loads(file.read_text(encoding="utf-8"))
            except FileNotFoundError as exc:
                raise ResolverError(f"source file not found: {ref}") from exc
        if isinstance(source, dict):
            return source
        raise ResolverError(f"invalid source {source!r}")

    def source_files(self) -> list[Path]:
        """Return the token files the document references."""
        files = []
        for set_ in self.data.get("sets", {}).values():
            files += [s["$ref"] for s in set_["sources"] if isinstance(s, dict) and "$ref" in s]
        for mod in self.data.get("modifiers", {}).values():
            for sources in mod["contexts"].values():
                files += [s["$ref"] for s in sources if isinstance(s, dict) and "$ref" in s]
        return sorted({(self.path.parent / f).resolve() for f in files})

    def merged_document(self, inputs: dict[str, str]) -> dict:
        """Merge the sources in resolution order for the chosen contexts; raises ResolverError."""
        selection = {**self.default_input(), **inputs}
        unknown = set(inputs) - set(self.modifiers())
        if unknown:
            raise ResolverError(f"unknown modifiers {sorted(unknown)}")
        docs: list[dict] = []
        for entry in self.data["resolutionOrder"]:
            ref = entry.get("$ref") if isinstance(entry, dict) else None
            if not ref or not ref.startswith("#/"):
                raise ResolverError(f"resolutionOrder entry must reference #/sets or #/modifiers: {entry!r}")
            kind, _, name = ref[2:].partition("/")
            if kind == "sets":
                set_ = self.data.get("sets", {}).get(name)
                if set_ is None:
                    raise ResolverError(f"unknown set {name}")
                docs += [self._load_source(s) for s in set_["sources"]]
            elif kind == "modifiers":
                mod = self.data.get("modifiers", {}).get(name)
                if mod is None:
                    raise ResolverError(f"unknown modifier {name}")
                context = selection[name]
                if context not in mod["contexts"]:
                    raise ResolverError(f"modifier {name} has no context {context!r}")
                docs += [self._load_source(s) for s in mod["contexts"][context]]
            else:
                raise ResolverError(f"unsupported reference {ref}")
        return dtcg.merge(docs)

    def resolve(self, inputs: dict[str, str]) -> dict[str, dtcg.Token]:
        """Return the flat, alias-resolved tokens for the chosen contexts."""
        return dtcg.resolve(dtcg.flatten(self.merged_document(inputs)))


def load(path: Path) -> ResolverDocument:
    """Load and validate a resolver document; raises ResolverError for a wrong version or shape."""
    data = json.loads(Path(path).read_text(encoding="utf-8"))
    if data.get("version") != "2025.10":
        raise ResolverError(f"{path}: version must be 2025.10")
    if "resolutionOrder" not in data:
        raise ResolverError(f"{path}: resolutionOrder is required")
    for name, mod in data.get("modifiers", {}).items():
        contexts = mod.get("contexts")
        if not isinstance(contexts, dict) or not contexts:
            raise ResolverError(f"{path}: modifier {name} needs contexts")
        if "default" in mod and mod["default"] not in contexts:
            raise ResolverError(f"{path}: default of {name} is not a context")
    for name, set_ in data.get("sets", {}).items():
        if not isinstance(set_.get("sources"), list):
            raise ResolverError(f"{path}: set {name} needs a sources array")
    return ResolverDocument(path=Path(path), data=data)
