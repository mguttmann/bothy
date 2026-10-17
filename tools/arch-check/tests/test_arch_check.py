# SPDX-License-Identifier: Apache-2.0
"""Tests of the architecture test of the Dart workspace (tools/arch-check, brief M4-01).

Each case writes a throwaway workspace with the fourteen members of the real `rules.yaml`
(manifests with the edges of the diagram, one library each) and changes one thing in it. The
last two cases run the real rules against the real repository.
"""

from __future__ import annotations

import re
from pathlib import Path

import arch_check
import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]
RULES = REPO / "tools" / "arch-check" / "rules.yaml"


class Tree:
    """A fake workspace below `root`, built from the real rules."""

    def __init__(self, root: Path) -> None:
        self.root = root
        self.rules = arch_check.load_rules(RULES)
        for member in self.rules.members.values():
            self.pubspec(member.name, dependencies=list(member.edges))
            self.write(f"{member.path}/lib/{member.name}.dart", "library;\n")
        workspace = [m.path for m in self.rules.members.values()]
        self.write(
            "pubspec.yaml",
            yaml.safe_dump({"name": "bothy_workspace", "workspace": workspace}),
        )

    def path_of(self, name: str) -> str:
        return self.rules.members[name].path

    def write(self, rel: str, text: str) -> Path:
        path = self.root / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
        return path

    def pubspec(
        self,
        name: str,
        *,
        dependencies: list[str],
        dev_dependencies: list[str] | None = None,
        extra: dict[str, object] | None = None,
    ) -> None:
        data: dict[str, object] = {"name": name, "resolution": "workspace"}
        data["dependencies"] = dict.fromkeys(dependencies, "any")
        if dev_dependencies:
            data["dev_dependencies"] = dict.fromkeys(dev_dependencies, "any")
        data.update(extra or {})
        self.write(f"{self.path_of(name)}/pubspec.yaml", yaml.safe_dump(data))

    def source(self, name: str, rel: str, text: str) -> str:
        path = f"{self.path_of(name)}/{rel}"
        self.write(path, text)
        return path

    def run(self) -> tuple[int, list[str]]:
        findings, members, files = arch_check.check(self.root, self.rules)
        return (1 if findings else 0), [str(f) for f in findings] + [f"{members} {files}"]


@pytest.fixture
def tree(tmp_path: Path) -> Tree:
    return Tree(tmp_path / "ws")


def _only(lines: list[str]) -> list[str]:
    return lines[:-1]


def test_clean_workspace_passes(tree: Tree) -> None:
    # apps/client reaches domain only through sync_engine (and others): a transitive import.
    tree.source("client", "lib/main.dart", "import 'package:domain/domain.dart';\n")
    tree.source("sync_engine", "lib/src/x.dart", "import 'package:auth/auth.dart';\n")
    tree.source("bothy_noise", "test/x_test.dart", "import 'dart:io';\n")
    code, lines = tree.run()
    assert code == 0, lines
    assert lines[-1].startswith(f"{len(arch_check.load_rules(RULES).members)} ")


def test_flutter_import_in_domain_fails(tree: Tree) -> None:
    path = tree.source(
        "domain", "lib/src/x.dart", "// SPDX\n\nimport 'package:flutter/widgets.dart';\n"
    )
    code, lines = tree.run()
    assert code == 1
    assert _only(lines) == [
        f"{path}:3: rule 1 (15 3.1): domain must not import package:flutter/widgets.dart"
    ]


def test_dart_ui_import_in_sync_engine_fails(tree: Tree) -> None:
    path = tree.source("sync_engine", "lib/x.dart", "import 'dart:ui' as ui;\n")
    code, lines = tree.run()
    assert code == 1
    assert _only(lines) == [f"{path}:1: rule 1 (15 3.1): sync_engine must not import dart:ui"]


def test_flutter_sdk_dependency_of_pure_dart_member_fails(tree: Tree) -> None:
    tree.pubspec("auth", dependencies=["api_client", "domain"])
    data = yaml.safe_load((tree.root / "packages/auth/pubspec.yaml").read_text())
    data["dev_dependencies"] = {"flutter_test": {"sdk": "flutter"}}
    tree.write("packages/auth/pubspec.yaml", yaml.safe_dump(data))
    code, lines = tree.run()
    assert code == 1
    assert _only(lines) == [
        "packages/auth/pubspec.yaml:1: rule 1 (15 3.1): auth must not depend on the Flutter SDK"
        " (flutter_test)"
    ]


def test_console_depends_on_sync_engine_fails(tree: Tree) -> None:
    tree.pubspec("console", dependencies=["admin_ui", "device_key", "terminal_ui", "sync_engine"])
    code, lines = tree.run()
    assert code == 1
    assert _only(lines) == [
        "apps/console/pubspec.yaml:1: rule 3 (15 3.1): console must not depend on sync_engine",
        "apps/console/pubspec.yaml:1: rule 3 (15 3.1): console must not depend on local_store"
        " (through sync_engine)",
    ]


def test_console_imports_local_store_through_admin_ui_fails(tree: Tree) -> None:
    tree.pubspec("admin_ui", dependencies=["design_system", "auth", "api_client", "local_store"])
    code, lines = tree.run()
    assert code == 1
    assert _only(lines) == [
        "apps/console/pubspec.yaml:1: rule 3 (15 3.1): console must not depend on local_store"
        " (through admin_ui)",
        "packages/admin_ui/pubspec.yaml:1: rule 8 (15 3.1): admin_ui may not depend on"
        " local_store; the diagram does not reach it",
    ]


def test_dependency_not_reached_fails(tree: Tree) -> None:
    path = tree.source("auth", "lib/src/x.dart", "import 'package:local_store/local_store.dart';\n")
    code, lines = tree.run()
    assert code == 1
    assert _only(lines) == [
        f"{path}:1: rule 8 (15 3.1): auth may not import local_store; the diagram does not reach it"
    ]


def test_dev_dependency_follows_the_reach(tree: Tree) -> None:
    tree.pubspec("domain", dependencies=[], dev_dependencies=["auth"])
    code, lines = tree.run()
    assert code == 1
    assert _only(lines) == [
        "packages/domain/pubspec.yaml:1: rule 8 (15 3.1): domain may not depend on auth;"
        " the diagram does not reach it"
    ]


def test_package_importing_an_app_fails(tree: Tree) -> None:
    path = tree.source("domain", "lib/x.dart", "export 'package:client/main.dart';\n")
    code, lines = tree.run()
    assert code == 1
    assert _only(lines) == [f"{path}:1: rule 2 (15 3.1): domain must not import the app client"]


def test_relative_import_leaving_member_fails(tree: Tree) -> None:
    path = tree.source("auth", "lib/x.dart", "import '../../domain/lib/x.dart';\n")
    tree.source("auth", "lib/src/y.dart", "import '../auth.dart';\nimport 'x.dart';\n")
    code, lines = tree.run()
    assert code == 1
    assert _only(lines) == [
        f"{path}:1: rule 2 (15 3.1): auth imports ../../domain/lib/x.dart, which leaves the member"
    ]


def test_conditional_and_multiline_imports_are_read(tree: Tree) -> None:
    path = tree.source(
        "domain",
        "lib/src/x.dart",
        "// import 'package:flutter/widgets.dart';\n"
        "import 'stub.dart'\n"
        "    if (dart.library.io) 'package:local_store/io.dart'\n"
        '    if (dart.library.js_interop) "dart:ui";\n'
        'export\n  "package:flutter/foundation.dart"\n  show kDebugMode;\n',
    )
    code, lines = tree.run()
    assert code == 1
    assert _only(lines) == [
        f"{path}:3: rule 8 (15 3.1): domain may not import local_store; the diagram does not"
        " reach it",
        f"{path}:4: rule 1 (15 3.1): domain must not import dart:ui",
        f"{path}:6: rule 1 (15 3.1): domain must not import package:flutter/foundation.dart",
    ]


def test_test_library_outside_test_folders_fails(tree: Tree) -> None:
    # Rule 10 (brief M4-17): package:<member>/testing.dart and its src/testing/ folder are test
    # support; production code, also of the own member, never reaches them.
    app = tree.source("client", "lib/main.dart", "import 'package:sync_engine/testing.dart';\n")
    deep = tree.source(
        "client", "bin/tool.dart", "import 'package:sync_engine/src/testing/world.dart';\n"
    )
    own = tree.source("sync_engine", "lib/sync_engine.dart", "export 'testing.dart';\n")
    inner = tree.source("sync_engine", "lib/src/engine.dart", "import 'testing/world.dart';\n")
    ds = tree.source("admin_ui", "lib/x.dart", "import 'package:design_system/testing.dart';\n")
    code, lines = tree.run()
    assert code == 1
    reach = "outside integration_test/ and test/"
    assert _only(lines) == [
        f"{deep}:1: rule 10 (15 3.1): client reaches the test library"
        f" package:sync_engine/testing.dart {reach}",
        f"{app}:1: rule 10 (15 3.1): client reaches the test library"
        f" package:sync_engine/testing.dart {reach}",
        f"{ds}:1: rule 10 (15 3.1): admin_ui reaches the test library"
        f" package:design_system/testing.dart {reach}",
        f"{inner}:1: rule 10 (15 3.1): sync_engine reaches the test library"
        f" package:sync_engine/testing.dart {reach}",
        f"{own}:1: rule 10 (15 3.1): sync_engine reaches the test library"
        f" package:sync_engine/testing.dart {reach}",
    ]


def test_test_library_inside_test_folders_passes(tree: Tree) -> None:
    tree.source("client", "test/x_test.dart", "import 'package:sync_engine/testing.dart';\n")
    tree.source(
        "client", "integration_test/app_test.dart", "import 'package:sync_engine/testing.dart';\n"
    )
    tree.source("sync_engine", "lib/testing.dart", "export 'src/testing/world.dart';\n")
    tree.source(
        "sync_engine",
        "lib/src/testing/world.dart",
        "import '../engine.dart';\nimport 'faults.dart';\n",
    )
    tree.source("sync_engine", "test/x_test.dart", "import '../lib/src/testing/world.dart';\n")
    code, lines = tree.run()
    assert code == 0, lines


def test_test_libraries_entry_is_checked(tmp_path: Path) -> None:
    data = yaml.safe_load(RULES.read_text(encoding="utf-8"))
    data["test_libraries"] = {"rule": 10, "library": "testing"}
    bad = tmp_path / "rules.yaml"
    bad.write_text(yaml.safe_dump(data), encoding="utf-8")
    with pytest.raises(arch_check.RulesError, match="test_libraries"):
        arch_check.load_rules(bad)


@pytest.mark.parametrize("folder", ["test", "integration_test", "bin"])
def test_every_source_folder_is_read(tree: Tree, folder: str) -> None:
    path = tree.source("domain", f"{folder}/x.dart", "import 'package:flutter/widgets.dart';\n")
    code, lines = tree.run()
    assert code == 1
    assert _only(lines) == [
        f"{path}:1: rule 1 (15 3.1): domain must not import package:flutter/widgets.dart"
    ]


def test_native_source_outside_allowed_places_fails(tree: Tree) -> None:
    tree.source("design_system", "macos/Classes/x.swift", "// native\n")
    tree.source("client", "linux/runner/main.cc", "// native\n")
    tree.source("console", "web/x.h", "// native\n")
    code, lines = tree.run()
    assert code == 1
    assert _only(lines) == [
        "apps/client/linux/runner/main.cc:1: rule 4 (15 3.1): native source outside"
        " device_key and apps/client/{ios,macos,android,windows}",
        "apps/console/web/x.h:1: rule 4 (15 3.1): native source outside"
        " device_key and apps/client/{ios,macos,android,windows}",
        "packages/design_system/macos/Classes/x.swift:1: rule 4 (15 3.1): native source outside"
        " device_key and apps/client/{ios,macos,android,windows}",
    ]


def test_native_source_in_device_key_passes(tree: Tree) -> None:
    tree.source("device_key", "macos/Classes/DeviceKeyPlugin.swift", "// native\n")
    tree.source("device_key", "windows/device_key_plugin.cpp", "// native\n")
    tree.source("client", "macos/Runner/AppDelegate.swift", "// native\n")
    tree.source("client", "windows/runner/main.cpp", "// native\n")
    code, lines = tree.run()
    assert code == 0, lines


def test_build_output_is_not_read(tree: Tree) -> None:
    tree.source("domain", "build/x.dart", "import 'package:flutter/widgets.dart';\n")
    tree.source("design_system", "build/macos/x.swift", "// built\n")
    tree.source("domain", ".dart_tool/x/pubspec.yaml", "name: x\n")
    code, lines = tree.run()
    assert code == 0, lines


def test_bothy_noise_extra_dependency_fails(tree: Tree) -> None:
    tree.pubspec("bothy_noise", dependencies=["cryptography", "pointycastle"])
    code, lines = tree.run()
    assert code == 1
    assert _only(lines) == [
        "packages/bothy_noise/pubspec.yaml:1: rule 7 (15 3.1): bothy_noise may depend at"
        " runtime only on cryptography, not on pointycastle"
    ]


def test_bothy_noise_dart_io_in_lib_fails(tree: Tree) -> None:
    path = tree.source("bothy_noise", "lib/src/x.dart", "import 'dart:io';\n")
    code, lines = tree.run()
    assert code == 1
    assert _only(lines) == [f"{path}:1: rule 7 (15 3.1): bothy_noise must not import dart:io"]


def test_bothy_noise_dart_io_in_test_passes(tree: Tree) -> None:
    tree.source("bothy_noise", "test/vectors_test.dart", "import 'dart:io';\n")
    tree.source("bothy_noise", "lib/src/x.dart", "import 'dart:isolate';\n")
    code, lines = tree.run()
    assert code == 0, lines


def test_unlisted_member_fails(tree: Tree) -> None:
    tree.write("packages/extra/pubspec.yaml", "name: extra\n")
    tree.write("packages/device_key/example/pubspec.yaml", "name: device_key_example\n")
    code, lines = tree.run()
    assert code == 1
    assert _only(lines) == [
        "packages/device_key/example/pubspec.yaml:1: rule 9 (15 3.1): not a member of"
        " tools/arch-check/rules.yaml and the root workspace",
        "packages/extra/pubspec.yaml:1: rule 9 (15 3.1): not a member of"
        " tools/arch-check/rules.yaml and the root workspace",
    ]


def test_listed_member_missing_fails(tree: Tree) -> None:
    (tree.root / "packages/l10n/pubspec.yaml").unlink()
    code, lines = tree.run()
    assert code == 1
    assert _only(lines) == [
        "packages/l10n/pubspec.yaml:1: rule 9 (15 3.1): member l10n of"
        " tools/arch-check/rules.yaml has no pubspec.yaml"
    ]


def test_workspace_list_differs_fails(tree: Tree) -> None:
    root = yaml.safe_load((tree.root / "pubspec.yaml").read_text())
    root["workspace"] = [p for p in root["workspace"] if p != "packages/auth"] + ["packages/x"]
    tree.write("pubspec.yaml", yaml.safe_dump(root))
    code, lines = tree.run()
    assert code == 1
    assert _only(lines) == [
        "pubspec.yaml:1: rule 9 (15 3.1): workspace lists packages/x, which"
        " tools/arch-check/rules.yaml does not name",
        "pubspec.yaml:1: rule 9 (15 3.1): workspace does not list packages/auth",
    ]


def test_member_name_differs_from_rules_fails(tree: Tree) -> None:
    tree.write("packages/domain/pubspec.yaml", "name: core\n")
    code, lines = tree.run()
    assert code == 1
    assert _only(lines) == [
        "packages/domain/pubspec.yaml:1: rule 9 (15 3.1): name is core, rules.yaml says domain"
    ]


def test_invalid_rules_are_rejected(tmp_path: Path) -> None:
    bad = tmp_path / "rules.yaml"
    bad.write_text(
        "members:\n  - {name: a, path: packages/a, kind: dart, edges: [b]}\n", encoding="utf-8"
    )
    with pytest.raises(arch_check.RulesError, match="a: edge to unknown member b"):
        arch_check.load_rules(bad)
    bad.write_text("members:\n  - {name: a, path: packages/a, kind: rust}\n", encoding="utf-8")
    with pytest.raises(arch_check.RulesError, match="a: unknown kind rust"):
        arch_check.load_rules(bad)
    bad.write_text(
        "members:\n  - {name: a, path: packages/a, kind: dart, edges: [b]}\n"
        "  - {name: b, path: packages/b, kind: dart, edges: [a]}\n",
        encoding="utf-8",
    )
    with pytest.raises(arch_check.RulesError, match="cycle"):
        arch_check.load_rules(bad)


def test_reach_is_the_transitive_closure() -> None:
    rules = arch_check.load_rules(RULES)
    assert "domain" in rules.reach("client")
    assert "local_store" in rules.reach("client")
    assert "local_store" not in rules.reach("auth")
    assert rules.reach("console").isdisjoint({"sync_engine", "local_store", "client"})
    assert rules.reach("domain") == frozenset()


def test_rules_edges_equal_the_spec_diagram() -> None:
    """A new edge changes 15 3.1 and rules.yaml in the same change (docs/DECISIONS.md)."""
    [chapter] = sorted((REPO / "docs" / "spec").glob("15-*.md"))
    text = chapter.read_text(encoding="utf-8")
    section = text.split("### 3.1", 1)[1].split("```mermaid", 1)[1].split("```", 1)[0]
    labels: dict[str, str] = {}
    for node, label in re.findall(r'(\w+)\["([^"]+)"\]', section):
        labels[node] = label.removeprefix("apps/")
    edges = {
        (labels[a], labels[b])
        for a, b in re.findall(r"^\s*(\w+)\s*-->\s*(\w+)", section, re.MULTILINE)
    }
    rules = arch_check.load_rules(RULES)
    assert {(m.name, e) for m in rules.members.values() for e in m.edges} == edges


def test_rules_cover_every_member_of_the_repository() -> None:
    rules = arch_check.load_rules(RULES)
    root = yaml.safe_load((REPO / "pubspec.yaml").read_text(encoding="utf-8"))
    assert [m.path for m in rules.members.values()] == root["workspace"]
    tracked = sorted(
        str(p.parent.relative_to(REPO))
        for base in ("apps", "packages")
        # One level, plus the nested harness of bothy_viz (docs/DECISIONS.md D-M6-11-6).
        for pattern in ("*/pubspec.yaml", "*/harness/pubspec.yaml")
        for p in (REPO / base).glob(pattern)
    )
    assert tracked == sorted(m.path for m in rules.members.values())


def test_real_repository_passes(capsys: pytest.CaptureFixture[str]) -> None:
    assert arch_check.main(["--root", str(REPO)]) == 0
    out = capsys.readouterr().out
    members = len(arch_check.load_rules(RULES).members)
    assert re.fullmatch(rf"arch-check: {members} members, \d+ files, ok\n", out), out


def test_main_prints_findings_and_fails(tree: Tree, capsys: pytest.CaptureFixture[str]) -> None:
    tree.source("domain", "lib/x.dart", "import 'dart:ui';\n")
    assert arch_check.main(["--root", str(tree.root), "--rules", str(RULES)]) == 1
    captured = capsys.readouterr()
    assert captured.out == ""
    assert captured.err == (
        "packages/domain/lib/x.dart:1: rule 1 (15 3.1): domain must not import dart:ui\n"
        "arch-check: 1 finding\n"
    )


def test_main_reports_invalid_rules(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    bad = tmp_path / "rules.yaml"
    bad.write_text("members: nope\n", encoding="utf-8")
    assert arch_check.main(["--root", str(tmp_path), "--rules", str(bad)]) == 2
    assert "arch-check: rules.yaml: members must be a list" in capsys.readouterr().err


@pytest.mark.parametrize("member", ["sync_engine", "local_store"])
def test_real_console_importing_a_forbidden_member_fails(tree: Tree, member: str) -> None:
    """The real console member (manifest and sources, brief M6-02) with one import of a member
    of rule 3 fails, both through the import and as an undeclared dependency."""
    console = REPO / "apps" / "console"
    tree.write("apps/console/pubspec.yaml", (console / "pubspec.yaml").read_text(encoding="utf-8"))
    for source in sorted((console / "lib").rglob("*.dart")):
        rel = source.relative_to(REPO).as_posix()
        tree.write(rel, source.read_text(encoding="utf-8"))
    code, lines = tree.run()
    assert code == 0, lines
    app = tree.root / "apps" / "console" / "lib" / "app.dart"
    app.write_text(
        f"import 'package:{member}/{member}.dart';\n" + app.read_text(encoding="utf-8"),
        encoding="utf-8",
    )
    code, lines = tree.run()
    assert code == 1
    assert any(
        line.startswith("apps/console/lib/app.dart:1:") and member in line for line in _only(lines)
    ), lines
