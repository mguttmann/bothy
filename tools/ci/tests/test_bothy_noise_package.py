# SPDX-License-Identifier: Apache-2.0
"""Guards of the Dart package packages/bothy_noise (docs/spec/15-client-architektur.md 3.1 rule 7
and 23.1): the only runtime dependency is `cryptography`, the package is never published, and
nothing under lib/ imports Flutter, dart:io, dart:html or a networking package, so the same code
runs in the apps and in the web console without network or storage of its own.
"""

from __future__ import annotations

import re
from pathlib import Path

import yaml

REPO = Path(__file__).resolve().parents[3]
PACKAGE = REPO / "packages" / "bothy_noise"
# Imports that would give the package Flutter, file or socket access, or a network client.
FORBIDDEN = re.compile(
    r"^\s*(?:import|export)\s+['\"]"
    r"(?:package:flutter[/_]|dart:io\b|dart:html\b|dart:ffi\b|dart:isolate\b|dart:web_socket|"
    r"package:(?:http|dio|web_socket_channel|web_socket|grpc|socket_io_client|path_provider|"
    r"shared_preferences|sqflite|drift|hive)\b)",
    re.MULTILINE,
)


def _pubspec() -> dict[str, object]:
    data = yaml.safe_load((PACKAGE / "pubspec.yaml").read_text(encoding="utf-8"))
    assert isinstance(data, dict)
    return data


def test_bothy_noise_dependencies() -> None:
    pubspec = _pubspec()
    assert pubspec["name"] == "bothy_noise"
    assert pubspec["publish_to"] == "none"
    dependencies = pubspec.get("dependencies")
    assert isinstance(dependencies, dict)
    assert sorted(dependencies) == ["cryptography"]


def test_bothy_noise_has_no_flutter_network_or_storage() -> None:
    files = sorted((PACKAGE / "lib").rglob("*.dart"))
    assert files, "packages/bothy_noise/lib has no Dart files"
    offenders = [
        f"{path.relative_to(REPO)}: {match.group(0).strip()}"
        for path in files
        for match in FORBIDDEN.finditer(path.read_text(encoding="utf-8"))
    ]
    assert offenders == []


def test_forbidden_imports_are_found() -> None:
    for line in (
        "import 'package:flutter/material.dart';",
        'import "dart:io";',
        "import 'dart:html' as html;",
        "import 'package:http/http.dart';",
        "export 'package:web_socket_channel/io.dart';",
    ):
        assert FORBIDDEN.search(line), line
    for line in ("import 'dart:typed_data';", "import 'package:cryptography/cryptography.dart';"):
        assert not FORBIDDEN.search(line), line


def test_responder_only_in_testing_library() -> None:
    public = (PACKAGE / "lib" / "bothy_noise.dart").read_text(encoding="utf-8")
    testing = (PACKAGE / "lib" / "testing.dart").read_text(encoding="utf-8")
    assert "NkResponder" not in public
    assert "show NkResponder;" in testing


def _go_version(module: str) -> str:
    text = (REPO / "services" / "host" / "go.mod").read_text(encoding="utf-8")
    match = re.search(rf"^\s*(?:require\s+)?{re.escape(module)} (v\S+)", text, re.MULTILINE)
    assert match, module
    return match.group(1)


def test_spec_names_the_pinned_library_versions() -> None:
    """docs/spec/24-terminal.md 5.2 names the pinned versions; they must equal the lock files."""
    spec = " ".join((REPO / "docs" / "spec" / "24-terminal.md").read_text(encoding="utf-8").split())
    for module in ("github.com/flynn/noise", "golang.org/x/crypto", "golang.org/x/sys"):
        assert f"`{module}` {_go_version(module)}" in spec, module
    # One pubspec.lock for the whole workspace (docs/DECISIONS.md D-M4-01-2).
    lock = yaml.safe_load((REPO / "pubspec.lock").read_text(encoding="utf-8"))
    version = lock["packages"]["cryptography"]["version"]
    assert f"Version {version} (https://pub.dev/packages/cryptography" in spec
