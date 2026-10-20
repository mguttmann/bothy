# SPDX-License-Identifier: Apache-2.0
"""Tests for the register check (gate `register-check`, ticket M1-06, ADR 0050).

Each test builds a small repository in `tmp_path` with three modules (`alpha`, `beta`,
`gamma`), puts one defect into one source and expects the rule id and the item in the output.
The tests at the end hold the class "the check reads what the site shows" against the real
artefacts: they render 02, 03 and the module READMEs with the Markdown configuration of
mkdocs.yml and compare every text the check reads with its static reading.
"""

from __future__ import annotations

import functools
import os
import re
import subprocess
import sys
import time
from html.parser import HTMLParser
from pathlib import Path
from typing import Any

import markdown
import pytest
import register_check
import render_diagrams
import yaml
from mkdocs.config import load_config

REPO = Path(__file__).resolve().parents[3]
SCRIPT = REPO / "tools" / "doccheck" / "register_check.py"

REGISTER = "services/server/architecture/modules.yaml"
CATALOGUE = "docs/spec/actions.yaml"
CH02 = "docs/spec/02-architecture.md"
CH03 = "docs/spec/03-data-model.md"
DIAGRAM = "docs/architecture/c4-3-server-components.mmd"

REGISTER_TEXT = """\
# SPDX-License-Identifier: Apache-2.0
version: 1
exclude: [tools]
kernel:
  core: [ids]
  ports: [authorize]
platform: {}
wire: {vendors: []}
cmd: {vendors: []}
testsupport: {paths: [test]}
modules:
  alpha:
    tables: [alpha_items, alpha_keys]
    actions: [alpha.read, alpha.update]
    events: [alpha.updated]
    ephemeral_events: []
    ws_ops: []
    depends_on: []
    adapter_vendors: []
  beta:
    tables: [beta_items]
    actions: [beta.read]
    events: [beta.created]
    ephemeral_events: [beta.pinged]
    ws_ops: [ping]
    depends_on: [alpha]
    adapter_vendors: []
  gamma:
    tables: []
    actions: [gamma.connect]
    events: []
    ephemeral_events: []
    ws_ops: [identify]
    depends_on: [alpha, beta]
    adapter_vendors: []
vendors: {}
"""

_ACTION = (
    "section: main, description: Text, right: read, owner: false, step_up: false, "
    "audit: never, rights_change: false, rl_class: rl.read"
)
CATALOGUE_TEXT = f"""\
# SPDX-License-Identifier: Apache-2.0
version: 1
rate_limit_classes:
- id: rl.read
  key: principal
  purpose: read
sections:
- main
actions:
- {{id: alpha.read, resource: alpha, effect: read, {_ACTION}}}
- {{id: alpha.update, resource: alpha, effect: write, {_ACTION}}}
- {{id: beta.read, resource: beta, effect: read, {_ACTION}}}
- {{id: gamma.connect, resource: instance, effect: read, {_ACTION}}}
events:
- {{type: alpha.updated, kind: durable, resource: alpha, receive_action: alpha.read,
   resource_version: false}}
- {{type: beta.created, kind: durable, resource: beta, receive_action: beta.read,
   resource_version: false}}
- {{type: beta.pinged, kind: ephemeral, resource: beta, receive_action: beta.read,
   resource_version: false}}
ws_ops:
- {{op: identify, direction: client_to_server, action: gamma.connect}}
- {{op: ping, direction: agent_to_server, action: beta.read}}
- {{op: hello, direction: server_to_client, action: gamma.connect}}
channel_ops: []
"""

CH03_TEXT = """\
# 03 Data model

## 1. Principles

Every table belongs to exactly one module.

## 2. Modules

| Module | Responsibility | Tables |
|---|---|---|
| `alpha` | Foundation | yes |
| `beta` | Middle, uses `alpha` | yes |
| `gamma` | Edge | none |

## 4. Tables

### 4.1 Alpha (`alpha`)

#### `alpha_items`

Purpose: entries. Module: `alpha`.

| Field | Type |
|---|---|
| `id` | `uuid` |

#### `alpha_keys`

Purpose: keys, read by `beta`. Module:
`alpha`.

| Field | Type |
|---|---|
| `id` | `uuid` |

### 4.2 Beta (`beta`)

#### `beta_items`

Purpose: entries. Module: `beta`.

| Field | Type |
|---|---|
| `id` | `uuid` |

### 4.3 Technical tables

#### `river_job`

Belongs to the tool, to no module.

| Tables | Owner |
|---|---|
| River (`river_job`) | `internal/platform/jobs` |

## 5. Search vectors

Text.
"""

CH02_TEXT = """\
# 02 Architecture

## 4. Modules of the monolith

### 4.1 Rules

1. Every table belongs to exactly one module.

### 4.2 Module list

| Module | Responsibility | Owns tables | Chapter |
|---|---|---|---|
| `alpha` | Foundation with `alpha.read` | `alpha_items`, `alpha_keys` | 03 |
| `beta` | Middle | `beta_items` | 03 |
| `gamma` | Edge | none | 04 |

### 4.4 Allowed dependencies

Example of a register line:

```yaml
# no section, only a comment in the code block
gamma: {depends_on: [alpha, beta]}
```

| Module | May use | Reason per edge |
|---|---|---|
| `alpha` | nothing | Base |
| `beta` | `alpha` | reads `alpha_items` |
| `gamma` | `alpha`, `beta` | both |

Levels (from the bottom): `alpha` · `beta` · `gamma`.

### 4.5 Cross-cutting ports (`internal/kernel`)

| Package | Purpose | Implemented by | Used by |
|---|---|---|---|
| `kernel/ids` | IDs | kernel itself | all |
| `kernel/authorize` | Permissions | `alpha` | all |

### 4.6 Actions per module

| Module | Actions |
|---|---|
| `alpha` | `alpha.read`, `alpha.update` |
| `beta` | `beta.read` |
| `gamma` | `gamma.connect` |

### 4.7 Event types and WebSocket ops per module

| Module | Durable events | Ephemeral events | WebSocket ops (client to server) |
|---|---|---|---|
| `alpha` | `alpha.updated` | | |
| `beta` | `beta.created` | `beta.pinged` | `ping` (through the hub) |
| `gamma` | | none on the shared socket; own connections | `identify` |

## 7. Automatic check of the rules

### 7.1 Module register as the source

Text.
"""

DIAGRAM_TEXT = """\
---
config:
  layout: elk
---
%% Arrow A --> B means: A may call the port of B.
flowchart TB
  classDef top fill:#5e35b1,stroke:#3b1f78,color:#ffffff
  subgraph edge["Edge: connections"]
    gamma["<b>gamma</b><br/>Edge"]:::top
  end
  beta["<b>beta</b><br/>Middle"]
  alpha["<b>alpha</b><br/>Foundation"]
  kernel["<b>internal/kernel</b>"]
  platform["<b>internal/platform</b>"]

  gamma --> alpha
  gamma --> beta
  beta --> alpha
  edge ~~~ kernel
"""


def _names(items: list[str]) -> str:
    return ", ".join(f"`{i}`" for i in items) + "." if items else "none."


def _readme(
    name: str,
    deps: list[str],
    tables: list[str],
    actions: list[str],
    events: tuple[list[str], list[str], list[str]],
) -> str:
    durable, ephemeral, ops = events
    tables_text = _names(tables) if tables else "None. The module reads through foreign ports."
    return f"""\
# Module `{name}`

## Purpose

Part of the test instance.

## Dependencies

May use the ports of these modules: {_names(deps)}

Reason: test.

## Tables

{tables_text}

Only this module writes and reads these tables through SQL.

## Actions (05-actions.md)

{_names(actions)}

## Events and WebSocket ops

- Durable: {_names(durable)}
- Ephemeral: {_names(ephemeral)}
- WebSocket ops (client to server): {_names(ops)}

## Chapters

Text.
"""


READMES = {
    "alpha": _readme(
        "alpha",
        [],
        ["alpha_items", "alpha_keys"],
        ["alpha.read", "alpha.update"],
        (["alpha.updated"], [], []),
    ),
    "beta": _readme(
        "beta",
        ["alpha"],
        ["beta_items"],
        ["beta.read"],
        (["beta.created"], ["beta.pinged"], ["ping"]),
    ),
    "gamma": _readme("gamma", ["alpha", "beta"], [], ["gamma.connect"], ([], [], ["identify"])),
}


def readme_path(module: str) -> str:
    return f"services/server/internal/{module}/README.md"


def build_repo(tmp_path: Path) -> Path:
    """A clean three-module repository: every source agrees with the register."""
    root = tmp_path / "repo"
    files = {
        REGISTER: REGISTER_TEXT,
        CATALOGUE: CATALOGUE_TEXT,
        CH02: CH02_TEXT,
        CH03: CH03_TEXT,
        DIAGRAM: DIAGRAM_TEXT,
    }
    files.update({readme_path(m): text for m, text in READMES.items()})
    for rel, text in files.items():
        (root / rel).parent.mkdir(parents=True, exist_ok=True)
        (root / rel).write_text(text, encoding="utf-8")
    return root


def edit(root: Path, rel: str, old: str, new: str) -> None:
    path = root / rel
    text = path.read_text(encoding="utf-8")
    assert text.count(old) == 1, f"{old!r} must occur once in {rel}"
    path.write_text(text.replace(old, new), encoding="utf-8")


def run(root: Path, capsys: pytest.CaptureFixture[str]) -> tuple[int, list[str]]:
    code = register_check.main(["--repo", str(root)])
    captured = capsys.readouterr()
    return code, (captured.out + captured.err).splitlines()


def assert_finding(
    root: Path, capsys: pytest.CaptureFixture[str], rel: str, rule: str, message: str
) -> list[str]:
    code, lines = run(root, capsys)
    assert code == 1, lines
    assert f"register-check: {rel}: {rule}: {message}" in lines, lines
    return lines


@pytest.fixture
def repo(tmp_path: Path) -> Path:
    return build_repo(tmp_path)


# ---- clean fixture and output form ---------------------------------------------------------


def test_clean_fixture_passes(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    code, lines = run(repo, capsys)
    assert code == 0, lines
    assert lines == ["register-check: ok (3 modules, 3 tables, 4 actions, 3 event types, 2 ops)"]


def test_findings_are_sorted_one_per_line(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, REGISTER, "actions: [gamma.connect]", "actions: []")
    edit(repo, REGISTER, "events: [alpha.updated]", "events: []")
    code, lines = run(repo, capsys)
    assert code == 1
    assert lines == sorted(lines)
    assert all(line.startswith("register-check: ") for line in lines)


# ---- C1: table owners of 03 ------------------------------------------------------------------


def test_table_from_03_without_module_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, REGISTER, "tables: [alpha_items, alpha_keys]", "tables: [alpha_items]")
    assert_finding(repo, capsys, REGISTER, "C1", "table alpha_keys is in no module's tables")


def test_table_in_two_modules_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, REGISTER, "tables: [beta_items]", "tables: [beta_items, alpha_items]")
    assert_finding(
        repo, capsys, REGISTER, "C1", "table alpha_items is in tables of modules alpha, beta"
    )


def test_table_owner_differs_from_03_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, CH03, "Purpose: entries. Module: `alpha`.", "Purpose: entries. Module: `beta`.")
    assert_finding(
        repo, capsys, REGISTER, "C1", "table alpha_items: register module alpha, 03 module beta"
    )


def test_table_heading_without_owner_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, CH03, "Purpose: entries. Module: `beta`.", "Purpose: entries.")
    assert_finding(repo, capsys, CH03, "C1", "table beta_items has no line 'Module: `<module>`'")


def test_register_table_unknown_to_03_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, REGISTER, "tables: [beta_items]", "tables: [beta_items, river_job]")
    assert_finding(
        repo, capsys, REGISTER, "C1", "module beta: tables names river_job, unknown to 03"
    )


def test_table_heading_without_space_is_read(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    # Python-Markdown renders `####x` as a heading too; the table must not drop out unseen.
    edit(
        repo,
        CH03,
        "### 4.3 Technical tables",
        "####`beta_extra`\n\nModule: `beta`.\n\n### 4.3 Technical tables",
    )
    assert_finding(repo, capsys, REGISTER, "C1", "table beta_extra is in no module's tables")


@pytest.mark.parametrize("title", ["`Beta_Extra`", "`beta_extra` (new)", "Table beta_extra"])
def test_other_level_4_heading_in_tables_fails(
    repo: Path, capsys: pytest.CaptureFixture[str], title: str
) -> None:
    edit(
        repo,
        CH03,
        "### 4.3 Technical tables",
        f"#### {title}\n\nModule: `beta`.\n\n### 4.3 Technical tables",
    )
    message = f"heading '#### {title}' is not of the form '#### `<table>`'"
    assert_finding(repo, capsys, CH03, "C1", message)


def test_table_heading_twice_in_03_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(
        repo,
        CH03,
        "### 4.3 Technical tables",
        "#### `beta_items`\n\nModule: `beta`.\n\n### 4.3 Technical tables",
    )
    assert_finding(repo, capsys, CH03, "C1", "table beta_items has 2 headings")


# ---- C2: module set and table ownership of 03 "Modules" ---------------------------------------


@pytest.mark.parametrize(
    ("old", "new", "message"),
    [
        ("| `gamma` | Edge | none |\n", "", 'module gamma is not in 03 "Modules"'),
        (
            "| `gamma` | Edge | none |\n",
            "| `gamma` | Edge | none |\n| `delta` | New | none |\n",
            'module delta of 03 "Modules" is not in the register',
        ),
    ],
)
def test_module_set_differs_from_03_fails(
    repo: Path, capsys: pytest.CaptureFixture[str], old: str, new: str, message: str
) -> None:
    edit(repo, CH03, old, new)
    assert_finding(repo, capsys, REGISTER, "C2", message)


def test_tableless_module_with_tables_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, CH03, "| `beta` | Middle, uses `alpha` | yes |", "| `beta` | Middle | none |")
    assert_finding(
        repo,
        capsys,
        REGISTER,
        "C2",
        'module beta: 03 "Modules" says none, the register lists beta_items',
    )


def test_module_with_tables_but_none_listed_fails(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    edit(repo, CH03, "| `gamma` | Edge | none |", "| `gamma` | Edge | yes |")
    assert_finding(
        repo,
        capsys,
        REGISTER,
        "C2",
        'module gamma: 03 "Modules" says yes, the register lists no table',
    )


def test_tables_column_other_than_yes_or_none_fails(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    edit(repo, CH03, "| `gamma` | Edge | none |", "| `gamma` | Edge | later |")
    assert_finding(
        repo,
        capsys,
        CH03,
        "C2",
        "module gamma: column 'Tables' is 'later', expected yes or none",
    )


# ---- C3: actions of the catalogue -----------------------------------------------------------


def test_action_twice_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, REGISTER, "actions: [beta.read]", "actions: [beta.read, alpha.read]")
    assert_finding(
        repo, capsys, REGISTER, "C3", "action alpha.read is in actions of modules alpha, beta"
    )


def test_action_without_module_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, REGISTER, "actions: [gamma.connect]", "actions: []")
    assert_finding(repo, capsys, REGISTER, "C3", "action gamma.connect is in no module's actions")


def test_register_action_unknown_to_catalogue_fails(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    edit(repo, REGISTER, "actions: [beta.read]", "actions: [beta.read, beta.write]")
    assert_finding(
        repo,
        capsys,
        REGISTER,
        "C3",
        "module beta: actions names beta.write, unknown to the catalogue",
    )


# ---- C4: event types ------------------------------------------------------------------------


def test_event_type_without_producer_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, REGISTER, "events: [alpha.updated]", "events: []")
    assert_finding(
        repo, capsys, REGISTER, "C4", "durable event alpha.updated is in no module's events"
    )


def test_event_type_two_producers_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, REGISTER, "events: [alpha.updated]", "events: [alpha.updated, beta.created]")
    assert_finding(
        repo,
        capsys,
        REGISTER,
        "C4",
        "durable event beta.created is in events of modules alpha, beta",
    )


def test_ephemeral_event_listed_as_durable_fails(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    edit(repo, REGISTER, "events: [beta.created]", "events: [beta.created, beta.pinged]")
    edit(repo, REGISTER, "ephemeral_events: [beta.pinged]", "ephemeral_events: []")
    lines = assert_finding(
        repo,
        capsys,
        REGISTER,
        "C4",
        "module beta: events names beta.pinged, an ephemeral event type",
    )
    expected = f"register-check: {REGISTER}: C4: ephemeral event beta.pinged is in no module's"
    assert f"{expected} ephemeral_events" in lines


def test_unknown_event_type_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, REGISTER, "ephemeral_events: [beta.pinged]", "ephemeral_events: [beta.pinged, x.y]")
    assert_finding(
        repo,
        capsys,
        REGISTER,
        "C4",
        "module beta: ephemeral_events names x.y, unknown to the catalogue",
    )


# ---- C5: WebSocket ops ------------------------------------------------------------------------


def test_client_op_without_module_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, REGISTER, "ws_ops: [ping]", "ws_ops: []")
    assert_finding(repo, capsys, REGISTER, "C5", "op ping is in no module's ws_ops")


def test_server_frame_in_register_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, REGISTER, "ws_ops: [identify]", "ws_ops: [identify, hello]")
    assert_finding(
        repo, capsys, REGISTER, "C5", "module gamma: ws_ops names hello, a server-to-client frame"
    )


def test_op_in_two_modules_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, REGISTER, "ws_ops: [ping]", "ws_ops: [ping, identify]")
    assert_finding(repo, capsys, REGISTER, "C5", "op identify is in ws_ops of modules beta, gamma")


# ---- C6 to C10: tables of chapter 02 ----------------------------------------------------------


def test_module_list_row_differs_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, CH02, "| `alpha_items`, `alpha_keys` |", "| `alpha_items` |")
    assert_finding(repo, capsys, CH02, "C6", "module alpha: table alpha_keys missing")


def test_module_list_module_missing_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, CH02, "| `gamma` | Edge | none | 04 |\n", "")
    assert_finding(repo, capsys, CH02, "C6", "module gamma missing")


def test_dependency_row_differs_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, CH02, "| `beta` | `alpha` | reads `alpha_items` |", "| `beta` | nothing | reads |")
    lines = assert_finding(repo, capsys, CH02, "C7", "module beta: dependency alpha missing")
    # The prose column "Reason per edge" is not compared, although it names a table.
    assert not [line for line in lines if "alpha_items" in line], lines


def test_actions_row_differs_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, CH02, "| `alpha` | `alpha.read`, `alpha.update` |", "| `alpha` | `alpha.read` |")
    edit(repo, CH02, "| `beta` | `beta.read` |", "| `beta` | `beta.read`, `alpha.update` |")
    lines = assert_finding(repo, capsys, CH02, "C8", "module alpha: action alpha.update missing")
    expected = f"register-check: {CH02}: C8: module beta: action alpha.update not in the register"
    assert expected in lines


def test_events_row_differs_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, CH02, "| `beta.created` | `beta.pinged` |", "| `beta.created` | |")
    assert_finding(repo, capsys, CH02, "C9", "module beta: ephemeral event beta.pinged missing")


def test_ops_cell_differs_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, CH02, "| `identify` |", "| `identify`, `hello` |")
    assert_finding(repo, capsys, CH02, "C9", "module gamma: op hello not in the register")


def test_kernel_packages_differ_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, CH02, "| `kernel/authorize` |", "| `kernel/txn` |")
    lines = assert_finding(repo, capsys, CH02, "C10", "kernel package authorize missing")
    assert f"register-check: {CH02}: C10: kernel package txn not in the register" in lines


def test_kernel_row_without_prefix_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, CH02, "| `kernel/ids` |", "| `ids` |")
    assert_finding(repo, capsys, CH02, "C10", "row names 'ids', expected 'kernel/<package>'")


def test_prose_cell_without_names_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, CH02, "| `beta` | `alpha` | reads `alpha_items` |", "| `beta` | all | reads |")
    assert_finding(
        repo,
        capsys,
        CH02,
        "C7",
        "module beta: cell 'May use' names nothing and is not empty, nothing or none",
    )


def test_duplicate_module_row_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, CH02, "| `beta` | `beta.read` |", "| `beta` | `beta.read` |\n| `beta` | |")
    assert_finding(repo, capsys, CH02, "C8", "module beta has 2 rows")


def test_row_without_module_name_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, CH02, "| `gamma` | `gamma.connect` |", "| gamma | `gamma.connect` |")
    code, lines = run(repo, capsys)
    assert code == 1
    assert any(
        re.fullmatch(
            rf"register-check: {CH02}: C8: line [0-9]+: first cell names no single module", line
        )
        for line in lines
    ), lines


# ---- C11: component diagram -------------------------------------------------------------------


def test_module_edge_missing_in_diagram_fails(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    edit(repo, DIAGRAM, "  gamma --> beta\n", "")
    assert_finding(repo, capsys, DIAGRAM, "C11", "edge gamma --> beta missing")


def test_extra_edge_in_diagram_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, DIAGRAM, "  beta --> alpha\n", "  beta --> alpha\n  alpha --> beta\n")
    assert_finding(repo, capsys, DIAGRAM, "C11", "edge alpha --> beta not in the register")


def test_module_without_node_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, DIAGRAM, '  beta["<b>beta</b><br/>Middle"]\n', "")
    assert_finding(repo, capsys, DIAGRAM, "C11", "module beta has no node")


def test_node_that_is_no_module_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, DIAGRAM, "  kernel[", '  delta["<b>delta</b>"]\n  kernel[')
    assert_finding(repo, capsys, DIAGRAM, "C11", "node delta is not a register module")


def test_edge_to_unknown_node_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, DIAGRAM, "  beta --> alpha\n", "  beta --> alpha\n  beta --> alpah\n")
    assert_finding(
        repo, capsys, DIAGRAM, "C11", "edge beta --> alpah: alpah is not a register module"
    )


@pytest.mark.parametrize(
    "link",
    ["gamma --> alpha & beta", "gamma -->|calls| alpha", "beta -.-> alpha", "beta==>alpha"],
)
def test_other_link_syntax_fails(repo: Path, capsys: pytest.CaptureFixture[str], link: str) -> None:
    edit(repo, DIAGRAM, "  edge ~~~ kernel\n", f"  edge ~~~ kernel\n  {link}\n")
    code, lines = run(repo, capsys)
    assert code == 1
    reason = "more than one statement" if "&" in link else "link is not of the form 'a --> b'"
    expected = f"register-check: {DIAGRAM}: C11: line 20: {reason}: {link}"
    assert expected in lines, lines


# ---- C12: module READMEs ----------------------------------------------------------------------


def test_readme_tables_differ_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, readme_path("beta"), "\n`beta_items`.\n", "\n`beta_items`, `beta_extra`.\n")
    assert_finding(
        repo,
        capsys,
        readme_path("beta"),
        "C12",
        "module beta: table beta_extra not in the register",
    )


def test_readme_dependencies_differ_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(
        repo,
        readme_path("gamma"),
        "May use the ports of these modules: `alpha`, `beta`.",
        "May use the ports of these modules: `alpha`.",
    )
    assert_finding(
        repo, capsys, readme_path("gamma"), "C12", "module gamma: dependency beta missing"
    )


def test_readme_dependency_line_missing_fails(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    edit(
        repo,
        readme_path("alpha"),
        "May use the ports of these modules: none.",
        "Must not use the ports of any other module.",
    )
    assert_finding(
        repo,
        capsys,
        readme_path("alpha"),
        "C12",
        "module alpha: line 'May use the ports of these modules:' appears 0 times, expected once",
    )


def test_readme_actions_and_events_differ_fails(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    rel = readme_path("beta")
    edit(repo, rel, "- Ephemeral: `beta.pinged`.", "- Ephemeral: none.")
    edit(repo, rel, "\n`beta.read`.\n", "\n`beta.read`, `beta.write`.\n")
    lines = assert_finding(
        repo, capsys, rel, "C12", "module beta: ephemeral event beta.pinged missing"
    )
    assert (
        f"register-check: {rel}: C12: module beta: action beta.write not in the register" in lines
    )


def test_readme_bullet_missing_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, readme_path("gamma"), "- WebSocket ops (client to server): `identify`.\n", "")
    assert_finding(
        repo,
        capsys,
        readme_path("gamma"),
        "C12",
        "module gamma: line '- WebSocket ops (client to server):' appears 0 times, expected once",
    )


# ---- sources missing or malformed: exit 2 -----------------------------------------------------


def _exit_2(root: Path, capsys: pytest.CaptureFixture[str], *needles: str) -> None:
    code, lines = run(root, capsys)
    assert code == 2, lines
    text = "\n".join(lines)
    for needle in needles:
        assert needle in text, lines


def test_missing_section_exits_2(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, CH02, "### 4.6 Actions per module\n", "")
    _exit_2(repo, capsys, CH02, "section 'Actions per module' missing")


def test_missing_file_exits_2(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    (repo / DIAGRAM).unlink()
    _exit_2(repo, capsys, f"register-check: {DIAGRAM}: file missing")


def test_missing_readme_exits_2(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    (repo / readme_path("gamma")).unlink()
    _exit_2(repo, capsys, f"register-check: {readme_path('gamma')}: file missing")


def test_missing_column_exits_2(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(
        repo, CH02, "| Module | May use | Reason per edge |", "| Module | Uses | Reason per edge |"
    )
    _exit_2(repo, capsys, CH02, "section 'Allowed dependencies'", "no column 'May use'")


def test_section_title_twice_exits_2(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, CH02, "## 7. Automatic", "### 4.8 Actions per module\n\nText.\n\n## 7. Automatic")
    _exit_2(repo, capsys, CH02, "section 'Actions per module' appears 2 times")


def test_heading_inside_code_fence_is_not_a_section(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    edit(repo, CH02, "Text.\n", "```markdown\n### 4.6 Actions per module\n## End\n```\n")
    code, lines = run(repo, capsys)
    assert code == 0, lines


def test_section_without_table_exits_2(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, CH02, "| `kernel/ids` | IDs | kernel itself | all |\n", "")
    edit(repo, CH02, "| `kernel/authorize` | Permissions | `alpha` | all |\n", "")
    edit(repo, CH02, "| Package | Purpose | Implemented by | Used by |\n|---|---|---|---|\n", "")
    _exit_2(repo, capsys, CH02, "section 'Cross-cutting ports (`internal/kernel`)' holds no table")


def test_duplicate_register_key_exits_2(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, REGISTER, "vendors: {}\n", "  alpha:\n    tables: []\nvendors: {}\n")
    _exit_2(repo, capsys, REGISTER, "duplicate key 'alpha'")


@pytest.mark.parametrize(
    ("old", "new", "needle"),
    [
        ("tables: [beta_items]", "tables: beta_items", "'modules.beta.tables'"),
        ("    ws_ops: [identify]\n", "", "'modules.gamma.ws_ops'"),
        ("  ports: [authorize]", "  ports: authorize", "'kernel.ports'"),
        ("modules:\n", "modules_old:\n", "key 'modules'"),
        ("  gamma:\n", "  gamma: []\n  gamma_old:\n", "key 'modules.gamma' must be a mapping"),
        ("modules:\n", "modules: [\n", "cannot parse"),
    ],
)
def test_malformed_register_exits_2(
    repo: Path, capsys: pytest.CaptureFixture[str], old: str, new: str, needle: str
) -> None:
    edit(repo, REGISTER, old, new)
    _exit_2(repo, capsys, REGISTER, needle)


@pytest.mark.parametrize(
    ("line", "problem"),
    [
        ("reviewed: 2026-02-30", "cannot parse: day 30 must be in range 1..28"),
        ("size: 1" + "0" * 5000, "cannot parse: Exceeds the limit (4300 digits)"),
        ("deep: " + "[" * 5000 + "]" * 5000, "cannot parse: nested too deeply"),
        ("loop: &loop [*loop]", "recursive alias"),
    ],
)
def test_register_value_python_cannot_build_exits_2(
    repo: Path, capsys: pytest.CaptureFixture[str], line: str, problem: str
) -> None:
    # Like tools/catalog (DECISIONS M1-02/16): one line and exit code 2, no traceback.
    with (repo / REGISTER).open("a", encoding="utf-8") as register:
        register.write(line + "\n")
    code, lines = run(repo, capsys)
    assert code == 2, lines
    assert len(lines) == 1, lines
    assert lines[0].startswith(f"register-check: {REGISTER}: "), lines
    assert problem in lines[0], lines


def test_nested_register_aliases_are_read_in_linear_time(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    # A walk that follows every alias anew visits 2**26 nodes here, one that visits each node
    # once only 26 lists (as tools/catalog reads them).
    ladder = ["a0: &a0 [x, x]", *(f"a{n}: &a{n} [*a{n - 1}, *a{n - 1}]" for n in range(1, 26))]
    with (repo / REGISTER).open("a", encoding="utf-8") as register:
        register.write("".join(f"{line}\n" for line in ladder))
    start = time.process_time()
    code, lines = run(repo, capsys)
    assert time.process_time() - start < 10
    assert code == 0, lines


def test_register_that_is_no_mapping_exits_2(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    (repo / REGISTER).write_text("- alpha\n", encoding="utf-8")
    _exit_2(repo, capsys, f"register-check: {REGISTER}: not a mapping")


def test_invalid_catalogue_exits_2(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, CATALOGUE, "version: 1\n", "version: 2\n")
    _exit_2(repo, capsys, f"register-check: {CATALOGUE}: invalid catalogue")


def test_missing_catalogue_exits_2(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    (repo / CATALOGUE).unlink()
    _exit_2(repo, capsys, f"register-check: {CATALOGUE}: file missing")


def test_unreadable_source_exits_2(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    (repo / readme_path("beta")).write_bytes(b"# Module \xff\n")
    _exit_2(repo, capsys, f"register-check: {readme_path('beta')}: cannot read")


def test_table_without_separator_exits_2(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    # Without its separator row the site shows a paragraph, not a table.
    edit(repo, CH02, "| Module | Actions |\n|---|---|\n", "| Module | Actions |\n")
    _exit_2(repo, capsys, CH02, "section 'Actions per module' holds no table")


def test_second_table_in_section_exits_2(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(
        repo,
        CH02,
        "| `gamma` | `gamma.connect` |\n",
        "| `gamma` | `gamma.connect` |\n\n| x |\n|---|\n",
    )
    _exit_2(repo, capsys, CH02, "section 'Actions per module' holds 2 tables, expected one")


# ---- robustness of the readers ------------------------------------------------------------------


def test_row_without_closing_pipe_is_read(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, CH02, "| `beta` | `beta.read` |", "| `beta` | `beta.read`")
    code, lines = run(repo, capsys)
    assert code == 0, lines


def test_kernel_cell_without_name_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, CH02, "| `kernel/ids` |", "| kernel/ids |")
    code, lines = run(repo, capsys)
    assert code == 1
    assert any(
        re.fullmatch(
            rf"register-check: {CH02}: C10: line [0-9]+: cell 'Package' names no single package",
            line,
        )
        for line in lines
    ), lines


def test_kernel_package_twice_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, CH02, "| `kernel/ids` | IDs |", "| `kernel/ids` | IDs |\n| `kernel/ids` | IDs |")
    assert_finding(repo, capsys, CH02, "C10", "kernel package ids has 2 rows")


def test_readme_line_with_prose_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(
        repo,
        readme_path("beta"),
        "May use the ports of these modules: `alpha`.",
        "May use the ports of these modules: all base modules.",
    )
    message = (
        "module beta: line 'May use the ports of these modules:' names nothing and is not "
        "nothing or none"
    )
    assert_finding(repo, capsys, readme_path("beta"), "C12", message)


def test_readme_paragraph_with_prose_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, readme_path("gamma"), "None. The module reads", "See below. The module reads")
    message = "module gamma: section 'Tables' names nothing and is not nothing or none"
    assert_finding(repo, capsys, readme_path("gamma"), "C12", message)


def test_unclosed_front_matter_hides_no_edge(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    # Without its closing line the whole file is front matter: every module lacks its node.
    edit(repo, DIAGRAM, "  layout: elk\n---\n", "  layout: elk\n")
    assert_finding(repo, capsys, DIAGRAM, "C11", "module alpha has no node")


# ---- closing review: no name may drop out of the comparison unseen ----------------------------


def test_readme_dependency_line_continues_on_next_line(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    edit(
        repo,
        readme_path("beta"),
        "May use the ports of these modules: `alpha`.",
        "May use the ports of these modules: `alpha`,\n`gamma`.",
    )
    message = "module beta: dependency gamma not in the register"
    assert_finding(repo, capsys, readme_path("beta"), "C12", message)


def test_readme_dependency_list_is_read(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    # Python-Markdown needs a blank line before a list: these lines continue the paragraph.
    edit(
        repo,
        readme_path("alpha"),
        "May use the ports of these modules: none.",
        "May use the ports of these modules:\n- `beta`",
    )
    message = "module alpha: dependency beta not in the register"
    assert_finding(repo, capsys, readme_path("alpha"), "C12", message)


def test_readme_line_without_names_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(
        repo,
        readme_path("alpha"),
        "May use the ports of these modules: none.",
        "May use the ports of these modules:\n\n- `beta`",
    )
    message = (
        "module alpha: line 'May use the ports of these modules:' names nothing and is not "
        "nothing or none"
    )
    assert_finding(repo, capsys, readme_path("alpha"), "C12", message)


def test_readme_bullet_continues_on_next_line(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    edit(
        repo,
        readme_path("beta"),
        "- Durable: `beta.created`.",
        "- Durable: `beta.created`,\n  `x.y`.",
    )
    message = "module beta: durable event x.y not in the register"
    assert_finding(repo, capsys, readme_path("beta"), "C12", message)


def test_readme_later_paragraph_of_tables_is_read(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    edit(repo, readme_path("beta"), "\n`beta_items`.\n", "\n`beta_items`.\n\nAlso `beta_more`.\n")
    message = "module beta: table beta_more not in the register"
    assert_finding(repo, capsys, readme_path("beta"), "C12", message)


@pytest.mark.parametrize(
    ("old", "new", "rule", "message"),
    [
        (
            "| `beta` | `alpha` | reads `alpha_items` |",
            "| `beta` | `alpha`, gamma | reads |",
            "C7",
            "module beta: cell 'May use' names gamma without backticks",
        ),
        (
            "| `beta` | `alpha` | reads `alpha_items` |",
            "| `beta` | `alpha`, <code>gamma</code> | reads |",
            "C7",
            "module beta: dependency gamma not in the register",
        ),
        (
            "| `alpha` | nothing | Base |",
            "| `alpha` | nothing but beta | Base |",
            "C7",
            "module alpha: cell 'May use' names beta without backticks",
        ),
        (
            "| `beta` | `beta.read` |",
            "| `beta` | `beta.read`, alpha.update |",
            "C8",
            "module beta: cell 'Actions' names alpha.update without backticks",
        ),
    ],
)
def test_name_without_backticks_in_02_fails(
    repo: Path, capsys: pytest.CaptureFixture[str], old: str, new: str, rule: str, message: str
) -> None:
    edit(repo, CH02, old, new)
    assert_finding(repo, capsys, CH02, rule, message)


def test_name_without_backticks_in_readme_fails(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    edit(
        repo,
        readme_path("beta"),
        "May use the ports of these modules: `alpha`.",
        "May use the ports of these modules: `alpha` and gamma.",
    )
    message = (
        "module beta: line 'May use the ports of these modules:' names gamma without backticks"
    )
    assert_finding(repo, capsys, readme_path("beta"), "C12", message)


def test_empty_word_must_be_a_whole_word(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, CH02, "| `alpha` | nothing | Base |", "| `alpha` | nothingness | Base |")
    message = "module alpha: cell 'May use' names nothing and is not empty, nothing or none"
    assert_finding(repo, capsys, CH02, "C7", message)


def test_html_comment_is_not_read(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    # The site hides a comment: a name inside one is not stated, a table inside one is gone.
    edit(repo, CH02, "| `beta` | `alpha` | reads", "| `beta` | `alpha`<!--, `gamma` --> | reads")
    edit(repo, CH03, "#### `beta_items`\n", "<!--\n#### `beta_items`\n")
    edit(repo, CH03, "### 4.3 Technical tables", "-->\n\n### 4.3 Technical tables")
    code, lines = run(repo, capsys)
    assert code == 1
    expected = (
        f"register-check: {REGISTER}: C1: module beta: tables names beta_items, unknown to 03"
    )
    assert lines == [expected], lines


def test_table_row_without_leading_pipe_is_read(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    edit(
        repo,
        CH02,
        "| `gamma` | `gamma.connect` |\n",
        "| `gamma` | `gamma.connect` |\n`delta` | |\n",
    )
    assert_finding(repo, capsys, CH02, "C8", "module delta not in the register")


@pytest.mark.parametrize("hashes", ["###", "#####", "######"])
def test_table_heading_at_other_level_fails(
    repo: Path, capsys: pytest.CaptureFixture[str], hashes: str
) -> None:
    edit(
        repo,
        CH03,
        "### 4.3 Technical tables",
        f"{hashes} `beta_extra`\n\nModule: `beta`.\n\n### 4.3 Technical tables",
    )
    message = (
        f"heading '{hashes} `beta_extra`' names a table, but table headings are '#### `<table>`'"
    )
    assert_finding(repo, capsys, CH03, "C1", message)


@pytest.mark.parametrize(
    "declaration",
    [
        'delta("New")',
        "delta[delta]",
        "delta{New}",
        'delta(["New"])',
        "delta>New]",
        'delta@{ shape: rect, label: "New" }',
    ],
)
def test_node_in_other_shape_fails(
    repo: Path, capsys: pytest.CaptureFixture[str], declaration: str
) -> None:
    edit(repo, DIAGRAM, "  kernel[", f"  {declaration}\n  kernel[")
    assert_finding(repo, capsys, DIAGRAM, "C11", "node delta is not a register module")


@pytest.mark.parametrize(
    ("link", "shared"), [("kernel --> alpha", "kernel"), ("gamma --> platform", "platform")]
)
def test_edge_to_kernel_or_platform_fails(
    repo: Path, capsys: pytest.CaptureFixture[str], link: str, shared: str
) -> None:
    edit(repo, DIAGRAM, "  beta --> alpha\n", f"  beta --> alpha\n  {link}\n")
    message = f"edge {link}: {shared} is not a register module"
    assert_finding(repo, capsys, DIAGRAM, "C11", message)


def test_source_error_is_one_line(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(repo, REGISTER, "modules:\n", "modules: [\n")
    code, lines = run(repo, capsys)
    assert code == 2
    assert len(lines) == 1, lines
    assert lines[0].startswith(f"register-check: {REGISTER}: cannot parse: "), lines


# ---- red team and re-review: the check reads what the site shows ----------------------------


def test_unknown_name_in_code_tag_is_compared(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    # The site shows <code> like a code span, so its name is a stated name.
    edit(
        repo,
        CH02,
        "| `beta` | `beta.read` |",
        "| `beta` | `beta.read`, <code>beta.reindex</code> |",
    )
    assert_finding(repo, capsys, CH02, "C8", "module beta: action beta.reindex not in the register")


@pytest.mark.parametrize(
    ("rel", "old", "new", "message"),
    [
        (
            CH02,
            "| `alpha` | nothing | Base |",
            "| `alpha` | nothing (formerly `beta`) | Base |",
            "module alpha: cell 'May use' starts with nothing but names beta",
        ),
        (
            "services/server/internal/alpha/README.md",
            "May use the ports of these modules: none.",
            "May use the ports of these modules: none (until M2 `beta`).",
            "module alpha: line 'May use the ports of these modules:' starts with none but "
            "names beta",
        ),
    ],
)
def test_empty_word_with_names_fails(
    repo: Path, capsys: pytest.CaptureFixture[str], rel: str, old: str, new: str, message: str
) -> None:
    edit(repo, rel, old, new)
    rule = "C7" if rel == CH02 else "C12"
    assert_finding(repo, capsys, rel, rule, message)


def test_comment_mark_in_code_span_is_text(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    # `<!--` inside a code span is shown as code, so it hides nothing after it.
    edit(repo, CH02, "| `beta` | `beta.read` |", "| `beta` | `beta.read`, `<!--` |")
    edit(repo, CH02, "| `gamma` | `gamma.connect` |", "| `gamma` | `gamma.connect` --> |")
    assert_finding(repo, capsys, CH02, "C8", "module beta: action <!-- not in the register")


@pytest.mark.parametrize(
    ("cell", "missing"),
    [("`beta.read`, `a|b`", "a|b"), ("`beta.read` `|` `alpha.read`", "alpha.read")],
)
def test_pipe_in_code_span_does_not_split_a_cell(
    repo: Path, capsys: pytest.CaptureFixture[str], cell: str, missing: str
) -> None:
    edit(repo, CH02, "| `beta` | `beta.read` |", f"| `beta` | {cell} |")
    message = f"module beta: action {missing} not in the register"
    assert_finding(repo, capsys, CH02, "C8", message)


def test_row_starting_with_backticks_is_no_fence(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    edit(
        repo,
        CH02,
        "| `gamma` | | none on the shared socket",
        "```gamma``` | `x.y` | | |\n| `gamma` | | none on the shared socket",
    )
    assert_finding(repo, capsys, CH02, "C9", "module gamma has 2 rows")


def test_separator_with_colons_only_makes_a_table(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    edit(
        repo,
        CH02,
        "| `gamma` | `gamma.connect` |\n",
        "| `gamma` | `gamma.connect` |\n\n| a | b |\n|:|:|\n| `x` | `y` |\n",
    )
    _exit_2(repo, capsys, CH02, "section 'Actions per module' holds 2 tables, expected one")


def test_readme_nested_list_under_bullet_is_read(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    edit(repo, readme_path("gamma"), "- Durable: none.\n", "- Durable: none.\n    - `x.y`\n")
    message = "module gamma: line '- Durable:' starts with none but names x.y"
    assert_finding(repo, capsys, readme_path("gamma"), "C12", message)


def test_readme_bullet_second_paragraph_is_read(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    edit(
        repo,
        readme_path("beta"),
        "- Durable: `beta.created`.\n",
        "- Durable: `beta.created`.\n\n    Also `x.y`.\n\n",
    )
    message = "module beta: durable event x.y not in the register"
    assert_finding(repo, capsys, readme_path("beta"), "C12", message)


def test_readme_raw_html_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(
        repo,
        readme_path("beta"),
        "\n`beta_items`.\n",
        "\n`beta_items`.<span hidden>`alpha_items`</span>\n",
    )
    message = "module beta: section 'Tables' holds raw HTML, which the site may hide"
    assert_finding(repo, capsys, readme_path("beta"), "C12", message)


def test_readme_other_line_in_events_section_fails(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    rel = readme_path("beta")
    edit(
        repo,
        rel,
        "- Ephemeral: `beta.pinged`.\n",
        "- Ephemeral: `beta.pinged`.\n- Further: `x.y`.\n",
    )
    code, lines = run(repo, capsys)
    assert code == 1
    expected = (
        rf"register-check: {re.escape(rel)}: C12: module beta: section 'Events and "
        r"WebSocket ops': line [0-9]+ belongs to none of the three bullets"
    )
    assert any(re.fullmatch(expected, line) for line in lines), lines


def test_readme_of_module_outside_register_fails(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    rel = readme_path("delta")
    (repo / rel).parent.mkdir(parents=True)
    (repo / rel).write_text("# Module `delta`\n", encoding="utf-8")
    message = "README of module delta, which is not in the register"
    assert_finding(repo, capsys, rel, "C12", message)


@pytest.mark.parametrize(
    "line",
    [
        "delta",
        "delta:::top",
        'push-relay["Relay"]',
        "alpha & delta",
        "beta; delta",
        'gamma["x"] & delta["y"]',
    ],
)
def test_diagram_line_that_is_no_node_edge_or_layout_fails(
    repo: Path, capsys: pytest.CaptureFixture[str], line: str
) -> None:
    edit(repo, DIAGRAM, "  kernel[", f"  {line}\n  kernel[")
    code, lines = run(repo, capsys)
    assert code == 1
    expected = {
        f"register-check: {DIAGRAM}: C11: node delta is not a register module",
        f"register-check: {DIAGRAM}: C11: line 13: not a node, edge or layout line: {line}",
        f"register-check: {DIAGRAM}: C11: line 13: more than one statement: {line}",
    }
    assert expected & set(lines), lines


@pytest.mark.parametrize("heading", ["### `beta_extra` (new)", "##### beta_extra", "###### Note"])
def test_heading_off_level_in_another_form_fails(
    repo: Path, capsys: pytest.CaptureFixture[str], heading: str
) -> None:
    edit(
        repo,
        CH03,
        "### 4.3 Technical tables",
        f"{heading}\n\nModule: `beta`.\n\n### 4.3 Technical tables",
    )
    code, lines = run(repo, capsys)
    assert code == 1
    prefix = f"register-check: {CH03}: C1: heading '{heading}'"
    assert any(line.startswith(prefix) for line in lines), lines


# ---- re-review round 2 and red team delta ----------------------------------------------------


@pytest.mark.parametrize(
    ("line", "message"),
    [
        ("  style delta fill:#fff", "node delta is not a register module"),
        ("  class delta,alpha top", "node delta is not a register module"),
        ("  alpha ~~~ delta", "node delta is not a register module"),
        ("  end; alpha --> beta", "line 13: more than one statement: end; alpha --> beta"),
        (
            "  style alpha fill:#fff; gamma --> beta",
            "line 13: more than one statement: style alpha fill:#fff; gamma --> beta",
        ),
    ],
)
def test_diagram_line_that_makes_a_node_or_hides_an_edge_fails(
    repo: Path, capsys: pytest.CaptureFixture[str], line: str, message: str
) -> None:
    edit(repo, DIAGRAM, "  kernel[", f"{line}\n  kernel[")
    assert_finding(repo, capsys, DIAGRAM, "C11", message)


def test_emphasised_empty_word_with_names_fails(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    edit(
        repo,
        readme_path("alpha"),
        "May use the ports of these modules: none.",
        "May use the ports of these modules: **none** (until M2 `beta`).",
    )
    message = (
        "module alpha: line 'May use the ports of these modules:' starts with none but names beta"
    )
    assert_finding(repo, capsys, readme_path("alpha"), "C12", message)


def test_readme_code_span_over_a_line_break_is_read(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    edit(repo, readme_path("beta"), "\n`beta_items`.\n", "\n`beta_items`, `beta_more\n`.\n")
    message = "module beta: table beta_more not in the register"
    assert_finding(repo, capsys, readme_path("beta"), "C12", message)


def test_level_3_heading_with_owner_line_fails(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    edit(
        repo,
        CH03,
        "### 4.3 Technical tables",
        "### beta_extra\n\nModule: `beta`.\n\n| Field | Type |\n|---|---|\n\n"
        "### 4.3 Technical tables",
    )
    message = (
        "heading '### beta_extra' has a 'Module:' line, but table headings are '#### `<table>`'"
    )
    assert_finding(repo, capsys, CH03, "C1", message)


# ---- the class "the check reads what the site shows", held against the real artefacts --------
#
# The static reader approximates Python-Markdown; each review found another form it read
# differently (comments, code spans, fences, separators, lists). Instead of one more patch per
# form, these tests render every source the check reads with the configuration of the docs site
# and require the same reading as the check: an edit that the two read differently fails `make
# test`, whatever its form. The READMEs are not part of the site; GitHub renders them, so for
# them Python-Markdown stands in for the renderer (docs/DECISIONS.md M1-06/11).

_HEADING_LEVEL = {f"h{level}": level for level in range(1, 7)}
_VOID = frozenset({"br", "hr", "img", "input", "meta", "link", "wbr"})


@functools.cache
def _site_markdown() -> tuple[tuple[Any, ...], dict[str, Any]]:
    """Markdown extensions and their options exactly as mkdocs.yml gives them to the site."""
    config = load_config(config_file=str(REPO / "mkdocs.yml"))
    return tuple(config["markdown_extensions"]), dict(config["mdx_configs"])


class _Node:
    def __init__(self, tag: str, attrs: dict[str, str | None]) -> None:
        self.tag = tag
        self.attrs = attrs
        self.children: list[_Node | str] = []

    def find(self, *tags: str) -> list[_Node]:
        """Descendants with one of `tags`, in document order."""
        out: list[_Node] = []
        for child in self.children:
            if isinstance(child, _Node):
                out += [child] if child.tag in tags else []
                out += child.find(*tags)
        return out


class _Tree(HTMLParser):
    def __init__(self) -> None:
        super().__init__(convert_charrefs=True)
        self.root = _Node("root", {})
        self._stack = [self.root]

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        node = _Node(tag, dict(attrs))
        self._stack[-1].children.append(node)
        if tag not in _VOID:
            self._stack.append(node)

    def handle_startendtag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        self._stack[-1].children.append(_Node(tag, dict(attrs)))

    def handle_endtag(self, tag: str) -> None:
        for index in range(len(self._stack) - 1, 0, -1):
            if self._stack[index].tag == tag:
                del self._stack[index:]
                return

    def handle_data(self, data: str) -> None:
        self._stack[-1].children.append(data)


def _render(text: str) -> _Node:
    extensions, configs = _site_markdown()
    md = markdown.Markdown(extensions=list(extensions), extension_configs=configs)
    tree = _Tree()
    tree.feed(md.convert(text))
    tree.close()
    return tree.root


def _plain(node: _Node) -> str:
    return "".join(c if isinstance(c, str) else _plain(c) for c in node.children)


def _shown(node: _Node) -> str:
    """Text as the site shows it, code written as a code span; hidden elements left out."""
    parts: list[str] = []
    for child in node.children:
        if isinstance(child, str):
            parts.append(child)
        elif "hidden" in child.attrs or "headerlink" in (child.attrs.get("class") or ""):
            continue
        elif child.tag == "code":
            code = _plain(child)
            ticks = "`" * (max((len(r) for r in re.findall("`+", code)), default=0) + 1)
            parts.append(f"{ticks}{code}{ticks}")
        elif child.tag == "br":
            parts.append(" ")
        else:
            parts.append(_shown(child))
    return "".join(parts)


def _blocks(root: _Node) -> list[_Node]:
    return [c for c in root.children if isinstance(c, _Node)]


def _title(node: _Node) -> str:
    return register_check._NUMBER.sub("", _shown(node).strip(), count=1)


def _rendered_section(blocks: list[_Node], title: str) -> list[_Node] | str:
    """Top-level blocks after the one heading titled `title`, up to a heading as high."""
    heads = [
        (i, _HEADING_LEVEL[b.tag], _title(b))
        for i, b in enumerate(blocks)
        if b.tag in _HEADING_LEVEL
    ]
    found = [h for h in heads if h[2] == title]
    if len(found) != 1:
        return f"title {title!r} appears {len(found)} times on the page"
    start, level, _ = found[0]
    end = next((i for i, lv, _ in heads if i > start and lv <= level), len(blocks))
    return blocks[start + 1 : end]


def _cell_view(text: str) -> tuple[tuple[str, ...], bool, tuple[str, ...]]:
    """What the check takes from a cell: names, a leading nothing/none, words outside code."""
    words = register_check._NAME.findall(register_check.without_code(text))
    return (
        tuple(register_check.ticked_names(text)),
        register_check._EMPTY.match(text.strip()) is not None,
        tuple(sorted(set(words))),
    )


def _table_differences(
    doc: register_check.Doc, root: _Node, title: str, read: list[str]
) -> list[str]:
    static = register_check.table(doc, title)
    section = _rendered_section(_blocks(root), title)
    if isinstance(section, str):
        return [f"{doc.path}: {section}"]
    tables = [t for b in section for t in ([b] if b.tag == "table" else b.find("table"))]
    if len(tables) != 1:
        return [f"{doc.path}: section {title!r}: {len(tables)} tables on the page"]
    header = tuple(_shown(th).strip() for th in tables[0].find("th"))
    if header != static.header:
        return [f"{doc.path}: section {title!r}: header {header} on the page, {static.header} read"]
    rows = [tuple(_shown(td).strip() for td in tr.find("td")) for tr in tables[0].find("tr")]
    rows = [row for row in rows if row]
    index = [static.header.index(name) for name in read]
    shown = [tuple(_cell_view(r[i] if i < len(r) else "") for i in index) for r in rows]
    taken = [tuple(_cell_view(row.cell(i)) for i in index) for row in static.rows]
    return (
        []
        if shown == taken
        else [f"{doc.path}: section {title!r}: {shown} on the page, {taken} read"]
    )


def _tables_section_differences(doc: register_check.Doc, root: _Node) -> list[str]:
    body = register_check.section(doc, "Tables")
    taken_heads = [(h.level, h.title) for h in doc.headings if h.index in body]
    taken_owners, _ = register_check.tables_of_03(doc)
    section = _rendered_section(_blocks(root), "Tables")
    if isinstance(section, str):
        return [f"{doc.path}: {section}"]
    if [n for b in section if b.tag not in _HEADING_LEVEL for n in b.find(*_HEADING_LEVEL)]:
        return [f"{doc.path}: section 'Tables': a heading inside another block"]
    heads = [
        (i, _HEADING_LEVEL[b.tag], _title(b))
        for i, b in enumerate(section)
        if b.tag in _HEADING_LEVEL
    ]
    owners: dict[str, str | None] = {}
    technical = False
    for pos, (start, level, title) in enumerate(heads):
        technical = title == "Technical tables" if level <= 3 else technical
        name = register_check._TABLE_TITLE.fullmatch(title)
        if level != 4 or technical or name is None:
            continue
        end = heads[pos + 1][0] if pos + 1 < len(heads) else len(section)
        text = []
        for block in section[start + 1 : end]:
            if block.tag == "table" or block.find("table"):
                break
            text.append(_shown(block))
        owner = register_check._OWNER.search("\n".join(text))
        owners.setdefault(name["name"], owner["module"] if owner else None)
    shown_heads = [(level, title) for _, level, title in heads]
    out = (
        []
        if shown_heads == taken_heads
        else [f"{doc.path}: headings {shown_heads} != {taken_heads}"]
    )
    return out + (
        [] if owners == taken_owners else [f"{doc.path}: owners {owners} != {taken_owners}"]
    )


def _readme_differences(doc: register_check.Doc, root: _Node) -> list[str]:
    out: list[str] = []
    line = register_check.DEPENDENCY_LINE
    starts = [i for i, x in enumerate(doc.lines) if x.startswith(line)]
    taken = register_check._continued(doc, starts[0], len(doc.lines))[0] if len(starts) == 1 else ""
    shown = [_shown(b) for b in root.find("p", "li") if _shown(b).startswith(line)]
    views = [_cell_view(s[len(line) :]) for s in shown]
    if views != [_cell_view(taken[len(line) :])]:
        out.append(f"{doc.path}: dependency line {shown} on the page, {taken!r} read")
    blocks = _blocks(root)
    for title in ("Tables", "Actions (05-actions.md)", "Events and WebSocket ops"):
        section = _rendered_section(blocks, title)
        if isinstance(section, str):
            out.append(f"{doc.path}: {section}")
            continue
        page = "\n".join(_shown(b) for b in section)
        read = register_check.section_text(doc, title)
        if _cell_view(page) != _cell_view(read):
            out.append(f"{doc.path}: section {title!r}: {page!r} on the page, {read!r} read")
        if not title.startswith("Events"):
            continue
        body = register_check.section(doc, title)
        for col in register_check.README_BULLETS:
            label = col.header.removeprefix("- ")
            items = [_shown(li) for b in section for li in b.find("li")]
            items = [item[len(label) :] for item in items if item.startswith(label)]
            begin = [i for i in body if doc.lines[i].startswith(col.header)]
            text = register_check._continued(doc, begin[0], body.stop)[0] if begin else ""
            if [_cell_view(i) for i in items] != [_cell_view(text[len(col.header) :])]:
                out.append(f"{doc.path}: bullet {label!r}: {items} on the page, {text!r} read")
    return out


def site_view_differences(root: Path) -> list[str]:
    """Every place where the rendered page and the check read a source differently."""
    sources = register_check.load_sources(root)
    out: list[str] = []
    chapter_02 = _render((root / register_check.CHAPTER_02).read_text(encoding="utf-8"))
    for _, title, columns in register_check.CHAPTER_02_TABLES:
        read = ["Module", *(c.header for c in columns)]
        out += _table_differences(sources.chapter_02, chapter_02, title, read)
    out += _table_differences(
        sources.chapter_02, chapter_02, register_check.KERNEL_TITLE, ["Package"]
    )
    chapter_03 = _render((root / register_check.CHAPTER_03).read_text(encoding="utf-8"))
    out += _table_differences(sources.chapter_03, chapter_03, "Modules", ["Module", "Tables"])
    out += _tables_section_differences(sources.chapter_03, chapter_03)
    for name, doc in sources.readmes.items():
        text = (root / register_check.README.format(module=name)).read_text(encoding="utf-8")
        out += _readme_differences(doc, _render(text))
    return out


def test_real_artefacts_read_as_the_site_shows_them() -> None:
    assert site_view_differences(REPO) == []


def test_clean_fixture_reads_as_the_site_shows_it(repo: Path) -> None:
    assert site_view_differences(repo) == []


@pytest.mark.parametrize(
    ("rel", "old", "new"),
    [
        # A Setext heading: the page has a second section of the same title, the check one.
        (
            CH02,
            "## 7. Automatic",
            "Actions per module\n-----------------\n\nText.\n\n## 7. Automatic",
        ),
        # A table inside a quote: the page shows a second table, the check reads none of it.
        (
            CH02,
            "| `gamma` | `gamma.connect` |\n",
            "| `gamma` | `gamma.connect` |\n\n"
            "> | Module | Actions |\n> |---|---|\n> | `beta` | `x.y` |\n",
        ),
        # A hidden span in a README: the check reads a name the page does not show.
        (
            "services/server/internal/beta/README.md",
            "\n`beta_items`.\n",
            "\n`beta_items`.<span hidden>`alpha_items`</span>\n",
        ),
    ],
)
def test_reading_unlike_the_site_is_caught(repo: Path, rel: str, old: str, new: str) -> None:
    edit(repo, rel, old, new)
    assert site_view_differences(repo) != []


# The diagram is held the same way: the pinned mermaid-cli of `make diagrams` draws it, and the
# nodes, edges and layout links in its SVG must equal what `parse_diagram` reads (M1-06/6).

_SVG_TAG = re.compile(r"<(?P<tag>g|path)\b(?P<attrs>[^>]*)>")
_SVG_ATTR = re.compile(r'([\w:-]+)="([^"]*)"')


def _mermaid_view(
    text: str, work: Path
) -> tuple[set[str], set[tuple[str, str]], set[tuple[str, str]]]:
    """Nodes, drawn edges and invisible layout links as mermaid-cli renders `text`."""
    source, svg = work / "diagram.mmd", work / "diagram.svg"
    source.write_text(text, encoding="utf-8")
    command = render_diagrams.mmdc_command(render_diagrams.DEFAULT_MMDC, source, svg)
    proc = subprocess.run(command, capture_output=True, text=True, timeout=600, check=False)
    assert proc.returncode == 0, proc.stderr
    nodes: set[str] = set()
    clusters: set[str] = set()
    links: list[tuple[str, bool]] = []
    for tag in _SVG_TAG.finditer(svg.read_text(encoding="utf-8")):
        attrs = dict(_SVG_ATTR.findall(tag["attrs"]))
        classes = attrs.get("class", "").split()
        if tag["tag"] == "g" and "node" in classes:
            nodes.add(re.sub(r"^.*?flowchart-(.+)-[0-9]+$", r"\1", attrs["id"]))
        elif tag["tag"] == "g" and "cluster" in classes:
            clusters.add(attrs["id"].removeprefix("my-svg-"))
        elif tag["tag"] == "path" and attrs.get("data-edge") == "true":
            links.append((attrs["data-id"], "edge-thickness-invisible" in classes))
    ends = nodes | clusters
    drawn: set[tuple[str, str]] = set()
    layout: set[tuple[str, str]] = set()
    for link_id, invisible in links:
        middle = re.sub(r"_[0-9]+$", "", link_id.removeprefix("L_"))
        pairs = [
            (middle[:i], middle[i + 1 :])
            for i in range(len(middle))
            if middle[i] == "_" and middle[:i] in ends and middle[i + 1 :] in ends
        ]
        assert len(pairs) == 1, link_id
        (layout if invisible else drawn).add(pairs[0])
    return nodes, drawn, layout


def diagram_differences(text: str, work: Path) -> list[str]:
    """Where mermaid-cli and `parse_diagram` read the diagram differently."""
    static = register_check.parse_diagram(text)
    nodes, drawn, layout = _mermaid_view(text, work)
    out = [] if nodes == set(static.mentioned) else [f"nodes {nodes} != {set(static.mentioned)}"]
    edges = {(a, b) for _, a, b in static.edges}
    out += [] if drawn == edges else [f"edges {drawn} != {edges}"]
    return out + (
        [] if layout == set(static.layout) else [f"layout {layout} != {set(static.layout)}"]
    )


def test_real_diagram_reads_as_mermaid_draws_it(tmp_path: Path) -> None:
    assert diagram_differences((REPO / DIAGRAM).read_text(encoding="utf-8"), tmp_path) == []


def test_fixture_diagram_reads_as_mermaid_draws_it(tmp_path: Path) -> None:
    assert diagram_differences(DIAGRAM_TEXT, tmp_path) == []


def test_diagram_reading_unlike_mermaid_is_caught(tmp_path: Path) -> None:
    # A chain draws two edges; the check reads none of them (and reports the line).
    text = DIAGRAM_TEXT.replace("  beta --> alpha\n", "  beta --> alpha --> gamma\n")
    assert diagram_differences(text, tmp_path) != []


WORKFLOWS = REPO / ".github" / "workflows"
PUPPETEER_CONFIG = "MMDC_PUPPETEER_CONFIG"


def _env(*scopes: dict[str, Any]) -> dict[str, Any]:
    """The environment of the last scope: workflow, job and step `env`, the later one wins."""
    out: dict[str, Any] = {}
    for scope in scopes:
        out.update(scope.get("env") or {})
    return out


def _ci_puppeteer_config(workflow: Path = WORKFLOWS / "ci.yml") -> str:
    """The `MMDC_PUPPETEER_CONFIG` that CI sets.

    Every `env` of the workflow, a job or a step that sets it must give the same value; YAML
    reads the file, so quotes around the value are no part of it.
    """
    data = yaml.safe_load(workflow.read_text(encoding="utf-8"))
    jobs = list(data["jobs"].values())
    scopes = [data, *jobs, *(step for job in jobs for step in job.get("steps", []))]
    values = [_env(scope)[PUPPETEER_CONFIG] for scope in scopes if PUPPETEER_CONFIG in _env(scope)]
    assert values, f"{workflow.name} sets no {PUPPETEER_CONFIG}"
    assert len(set(values)) == 1, values
    return str(values[0])


_TWO_JOBS = """\
jobs:
  gates:
    steps:
      - name: make gates
        env:
          MMDC_PUPPETEER_CONFIG: tools/node/puppeteer-ci.json
        run: make gates
  sonar:
    steps:
      - name: make sonar
        env:
          COMPONENTS: all
        run: make sonar
"""
_SONAR_ENV = "          COMPONENTS: all\n"


@pytest.mark.parametrize(
    ("old", "new"),
    [
        # A second place with the same value, as the sonar job needs it (red team H1).
        (
            _SONAR_ENV,
            _SONAR_ENV + "          MMDC_PUPPETEER_CONFIG: tools/node/puppeteer-ci.json\n",
        ),
        ("jobs:\n", "env:\n  MMDC_PUPPETEER_CONFIG: tools/node/puppeteer-ci.json\njobs:\n"),
        # YAML quotes are syntax, not part of the path.
        (": tools/node/puppeteer-ci.json\n", ': "tools/node/puppeteer-ci.json"\n'),
        (": tools/node/puppeteer-ci.json\n", ": 'tools/node/puppeteer-ci.json'\n"),
    ],
)
def test_ci_puppeteer_config_reads_every_place_that_sets_it(
    tmp_path: Path, old: str, new: str
) -> None:
    (tmp_path / "ci.yml").write_text(_TWO_JOBS, encoding="utf-8")
    edit(tmp_path, "ci.yml", old, new)
    assert _ci_puppeteer_config(tmp_path / "ci.yml") == "tools/node/puppeteer-ci.json"


@pytest.mark.parametrize(
    ("old", "new"),
    [
        (_SONAR_ENV, _SONAR_ENV + "          MMDC_PUPPETEER_CONFIG: tools/node/other.json\n"),
        ("          MMDC_PUPPETEER_CONFIG: tools/node/puppeteer-ci.json\n", ""),
    ],
)
def test_ci_puppeteer_config_fails_unless_ci_sets_one_value(
    tmp_path: Path, old: str, new: str
) -> None:
    (tmp_path / "ci.yml").write_text(_TWO_JOBS, encoding="utf-8")
    edit(tmp_path, "ci.yml", old, new)
    with pytest.raises(AssertionError):
        _ci_puppeteer_config(tmp_path / "ci.yml")


# The make targets that start Chromium: `diagrams` renders, `test` runs the diagram tests above.
_CHROMIUM_TARGETS = frozenset({"diagrams", "test"})
_MAKE_CALL = re.compile(r"(?:^|[;&|(]|\bthen|\bdo)\s*make\b([^;&|)\n]*)", re.MULTILINE)


@functools.cache
def _make_prerequisites() -> dict[str, tuple[str, ...]]:
    """Target to prerequisites as `make -p` prints the root Makefile; `""` is the default goal."""
    proc = subprocess.run(
        ["make", "-pq", "help"],
        cwd=REPO,
        env=_clean_env(),
        capture_output=True,
        text=True,
        timeout=120,
        check=False,
    )
    rules: dict[str, tuple[str, ...]] = {}
    for line in proc.stdout.splitlines():
        if rule := re.fullmatch(r"([A-Za-z0-9_.-]+):(?![:=])(.*)", line):
            rules[rule[1]] = tuple(t for t in rule[2].split() if t != "|")
        elif goal := re.fullmatch(r"\.DEFAULT_GOAL :?= (\S+)", line):
            rules[""] = (goal[1],)
    assert {"gates", "test", "diagrams", ""} <= rules.keys(), sorted(rules)
    return rules


def _starts_chromium(call: str) -> bool:
    """Whether `make <call>` in the repository root reaches a target of `_CHROMIUM_TARGETS`."""
    words = call.split()
    if any(word.startswith("-C") for word in words):
        return False  # another Makefile
    todo = [w for w in words if not w.startswith("-") and "=" not in w] or [""]
    seen: set[str] = set()
    while todo:
        target = todo.pop()
        if target in _CHROMIUM_TARGETS:
            return True
        if target not in seen:
            seen.add(target)
            todo += _make_prerequisites().get(target, ())
    return False


def test_every_ci_step_that_starts_chromium_sets_the_puppeteer_config() -> None:
    # Without it Chromium starts with its sandbox, which a CI runner may block (red team H1).
    # Steps of all workflows, ci.yml's value. The job sonar runs `make sonar-scan`, no tests.
    config = _ci_puppeteer_config()
    checked: set[str] = set()
    wrong: list[str] = []
    for path in sorted(WORKFLOWS.glob("*.yml")):
        data = yaml.safe_load(path.read_text(encoding="utf-8"))
        for job_id, job in data["jobs"].items():
            for step in job.get("steps", []):
                calls = _MAKE_CALL.findall(step.get("run", ""))
                if not any(_starts_chromium(call) for call in calls):
                    continue
                where = f"{path.name}: {job_id}: {step.get('name', step['run'])}"
                checked.add(f"{path.name}: {job_id}")
                if _env(data, job, step).get(PUPPETEER_CONFIG) != config:
                    wrong.append(where)
    assert "ci.yml: gates" in checked, checked
    assert "ci.yml: sonar" not in checked, checked
    assert wrong == []


def test_mermaid_view_renders_in_the_ci_environment(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    # `make test` runs pytest in tools/, while CI gives the Puppeteer config relative to the
    # repository root. A config already in effect stays (a developer's own Chromium, say);
    # without one the test takes the one CI sets.
    config = os.environ.get(PUPPETEER_CONFIG) or _ci_puppeteer_config()
    monkeypatch.setenv(PUPPETEER_CONFIG, config)
    monkeypatch.chdir(REPO / "tools")
    nodes, drawn, _ = _mermaid_view("flowchart TB\n  alpha --> beta\n", tmp_path)
    assert (nodes, drawn) == ({"alpha", "beta"}, {("alpha", "beta")})


# ---- the real repository and the make target ---------------------------------------------------


def test_real_repository_passes(capsys: pytest.CaptureFixture[str]) -> None:
    # Default --repo is the repository of the script: the real register, 02, 03, catalogue,
    # diagram and READMEs.
    code = register_check.main([])
    captured = capsys.readouterr()
    assert code == 0, captured.out + captured.err
    assert re.fullmatch(
        r"register-check: ok \([0-9]+ modules, [0-9]+ tables, [0-9]+ actions, "
        r"[0-9]+ event types, [0-9]+ ops\)\n",
        captured.out,
    ), captured.out


def _clean_env() -> dict[str, str]:
    # The test may run under `make test`; a nested make must not inherit its flags.
    env = {k: v for k, v in os.environ.items() if k not in ("MAKEFLAGS", "MAKELEVEL", "MFLAGS")}
    env.pop("PYTHONPATH", None)
    # The environment and interpreter pytest runs in, so that uv neither creates one elsewhere
    # nor rebuilds this one for the Python of tools/.python-version.
    env["UV_PROJECT_ENVIRONMENT"] = sys.prefix
    env["UV_PYTHON"] = sys.executable
    return env


def test_nested_make_keeps_the_running_environment() -> None:
    # Without the interpreter, uv rebuilds sys.prefix for the Python of tools/.python-version
    # when pytest runs under another one (red team M4); test_action_catalog.py pins both too.
    env = _clean_env()
    assert (env["UV_PROJECT_ENVIRONMENT"], env["UV_PYTHON"]) == (sys.prefix, sys.executable)


def test_script_runs_outside_pytest(repo: Path) -> None:
    # As `make register-check` runs it: a plain script, without the pytest pythonpath.
    edit(repo, REGISTER, "ws_ops: [ping]", "ws_ops: []")
    proc = subprocess.run(
        [sys.executable, str(SCRIPT), "--repo", str(repo)],
        cwd=repo,
        env=_clean_env(),
        capture_output=True,
        text=True,
        timeout=120,
        check=False,
    )
    assert proc.returncode == 1, proc.stdout + proc.stderr
    assert f"register-check: {REGISTER}: C5: op ping is in no module's ws_ops" in proc.stderr


def test_make_register_check_is_a_gate() -> None:
    proc = subprocess.run(
        ["make", "-pq", "gates"],
        cwd=REPO,
        env=_clean_env(),
        capture_output=True,
        text=True,
        timeout=120,
        check=False,
    )
    database = proc.stdout.splitlines()
    [gates] = [line for line in database if line.startswith("gates:")]
    [phony] = [line for line in database if line.startswith(".PHONY:")]
    assert "register-check" in gates.split(":", 1)[1].split()
    assert "register-check" in phony.split(":", 1)[1].split()
    run_target = subprocess.run(
        ["make", "register-check"],
        cwd=REPO,
        env=_clean_env(),
        capture_output=True,
        text=True,
        timeout=300,
        check=False,
    )
    assert run_target.returncode == 0, run_target.stdout + run_target.stderr
    assert "register-check: ok (" in run_target.stdout
