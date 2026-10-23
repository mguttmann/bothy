# SPDX-License-Identifier: Apache-2.0
"""ADR 0026 lint: the hand layer defines no own type for an object the core generates.

No class defined in a module of ``bothy_sdk`` outside ``bothy_sdk.generated`` (``testing``
included) carries the name of a schema of ``docs/api/openapi.yaml``. A second test holds the
documentation duty of Design note 18: every public module of the hand layer and every public
class, function, method and property in it has a docstring.
"""

from __future__ import annotations

import importlib
import inspect
import pkgutil
from collections.abc import Iterator
from types import ModuleType

from conftest import spec

import bothy_sdk

_GENERATED = "bothy_sdk.generated"


def _hand_layer_modules() -> Iterator[ModuleType]:
    yield bothy_sdk
    for info in pkgutil.walk_packages(bothy_sdk.__path__, "bothy_sdk."):
        if info.name == _GENERATED or info.name.startswith(_GENERATED + "."):
            continue
        yield importlib.import_module(info.name)


def _own_classes(module: ModuleType) -> Iterator[type]:
    """Every class defined in ``module``, nested classes included."""
    pending = [v for v in vars(module).values() if inspect.isclass(v)]
    seen: set[int] = set()
    while pending:
        cls = pending.pop()
        if id(cls) in seen or cls.__module__ != module.__name__:
            continue
        seen.add(id(cls))
        yield cls
        pending.extend(v for v in vars(cls).values() if inspect.isclass(v))


def test_hand_layer_modules_found() -> None:
    names = {m.__name__ for m in _hand_layer_modules()}
    assert {"bothy_sdk.adapter", "bothy_sdk.outbox", "bothy_sdk.testing.fakeserver"} <= names
    assert not any(n.startswith(_GENERATED) for n in names)


def test_hand_layer_types() -> None:
    schemas = set(spec()["components"]["schemas"])
    clashes = sorted(
        f"{module.__name__}.{cls.__qualname__}"
        for module in _hand_layer_modules()
        for cls in _own_classes(module)
        if cls.__name__ in schemas
    )
    assert clashes == []


def _public(name: str) -> bool:
    return not name.startswith("_")


def _public_module(module: ModuleType) -> bool:
    return all(_public(part) for part in module.__name__.split("."))


def _exported(module: ModuleType) -> list[str]:
    names = getattr(module, "__all__", None)
    if names is not None:
        return list(names)
    return [n for n in vars(module) if _public(n)]


def _defined_here(module: ModuleType, value: object) -> bool:
    return getattr(value, "__module__", None) == module.__name__


def _missing_docstrings(module: ModuleType) -> Iterator[str]:
    if not inspect.getdoc(module):
        yield module.__name__
    for name in _exported(module):
        value = getattr(module, name, None)
        if not (inspect.isclass(value) or inspect.isfunction(value)):
            continue
        if not _defined_here(module, value):
            continue
        qualified = f"{module.__name__}.{name}"
        if not (value.__doc__ or "").strip():
            yield qualified
        if not inspect.isclass(value):
            continue
        for member, attr in vars(value).items():
            if not _public(member):
                continue
            target = attr.__func__ if isinstance(attr, classmethod | staticmethod) else attr
            if isinstance(target, property):
                target = target.fget
            if not inspect.isfunction(target):
                continue
            if not (target.__doc__ or "").strip():
                yield f"{qualified}.{member}"


def test_public_symbols_have_docstrings() -> None:
    public = [m for m in _hand_layer_modules() if _public_module(m)]
    assert len(public) > 10
    missing = sorted(name for module in public for name in _missing_docstrings(module))
    assert missing == []
