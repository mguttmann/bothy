# SPDX-License-Identifier: Apache-2.0
"""Builds the token model: theme-independent base tokens plus one token map per modifier context."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path

from . import dtcg, resolver


@dataclass
class Model:
    document: resolver.ResolverDocument
    defaults: dict[str, str]
    base: dict[str, dtcg.Token]
    contexts: dict[str, dict[str, dict[str, dtcg.Token]]]
    full: dict[str, dict[str, dict[str, dtcg.Token]]]

    def all_paths(self) -> list[str]:
        paths = set(self.base)
        for per_ctx in self.contexts.values():
            for tokens in per_ctx.values():
                paths |= set(tokens)
        return sorted(paths)


def _fingerprint(token: dtcg.Token) -> str:
    return repr((token.type, token.hex, token.value))


def build(resolver_path: Path) -> Model:
    doc = resolver.load(resolver_path)
    defaults = doc.default_input()
    default_tokens = doc.resolve(defaults)
    full: dict[str, dict[str, dict[str, dtcg.Token]]] = {}
    dependent: dict[str, set[str]] = {}
    for name, contexts in doc.modifiers().items():
        full[name] = {ctx: doc.resolve({**defaults, name: ctx}) for ctx in contexts}
        owned: set[str] = set()
        for ctx in contexts:
            sources = doc.data["modifiers"][name]["contexts"][ctx]
            for source in sources:
                owned |= set(dtcg.flatten(doc._load_source(source)))
        varying = {
            path for path in default_tokens if len({_fingerprint(full[name][ctx][path]) for ctx in contexts}) > 1
        }
        dependent[name] = owned | varying
    names = list(dependent)
    for i, a in enumerate(names):
        for b in names[i + 1 :]:
            both = dependent[a] & dependent[b]
            if both:
                raise dtcg.TokenError(f"tokens depend on two modifiers ({a}, {b}): {sorted(both)[:5]}")
    every_dependent = set().union(*dependent.values()) if dependent else set()
    base = {p: t for p, t in default_tokens.items() if p not in every_dependent}
    contexts_out = {
        name: {ctx: {p: full[name][ctx][p] for p in sorted(dependent[name])} for ctx in full[name]} for name in full
    }
    return Model(document=doc, defaults=defaults, base=base, contexts=contexts_out, full=full)
