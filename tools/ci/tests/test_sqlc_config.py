# SPDX-License-Identifier: Apache-2.0
"""Every sql block of services/server/sqlc.yaml writes the same kind of generated code.

Generated Go files end in .gen.go (docs/DECISIONS.md M1-05/7): lint, analysis and coverage
recognise them by that suffix. Each block reads the whole migrations directory as its schema,
so without omit_unused_structs every module's models file would hold a struct for every
table of the server, including tables of other modules.
"""

from __future__ import annotations

from pathlib import Path
from typing import Any

import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]
SQLC = REPO / "services" / "server" / "sqlc.yaml"

EXPECTED = {
    "output_db_file_name": "db.gen.go",
    "output_models_file_name": "models.gen.go",
    "output_querier_file_name": "querier.gen.go",
    "output_files_suffix": ".gen",
    "omit_unused_structs": True,
}


def _blocks() -> list[dict[str, Any]]:
    config = yaml.safe_load(SQLC.read_text())
    blocks: list[dict[str, Any]] = config["sql"]
    assert blocks, "sqlc.yaml has no sql block"
    return blocks


@pytest.mark.parametrize("block", _blocks(), ids=lambda b: str(b["queries"]))
def test_block_writes_gen_go_files_and_only_used_structs(block: dict[str, Any]) -> None:
    go = block["gen"]["go"]
    wrong = {k: go.get(k) for k, v in EXPECTED.items() if go.get(k) != v}
    assert wrong == {}, f"{block['queries']}: expected {EXPECTED}"


def test_generated_go_files_end_in_gen_go() -> None:
    """The generated output on disk follows the configuration (no stale *.go file left).

    Hand-written tests may sit next to the generated code (messaging has one); they are skipped.
    """
    server = SQLC.parent
    stale = [
        str(p.relative_to(server))
        for b in _blocks()
        for p in sorted((server / b["gen"]["go"]["out"]).glob("*.go"))
        if not p.name.endswith((".gen.go", "_test.go"))
    ]
    assert stale == []
