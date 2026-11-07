# SPDX-License-Identifier: Apache-2.0
"""The pin of the Hermes Agent version that the recording runs against."""

from __future__ import annotations

import datetime
import re
from pathlib import Path

import pytest
from hermes_harness.pin import PIN_FILE, HermesPin, PinError, load_pin
from pin_rules import pin_reference_problems, ruling_rows

REPO = Path(__file__).resolve().parents[4]

VALID = """\
# SPDX-License-Identifier: Apache-2.0
repository = "https://github.com/NousResearch/hermes-agent"
tag = "v2026.9.24"
commit = "f97608f178d1ffeca59860195ab7da295f7c8e5f"
python = "3.13"
install = "uv-lock"
extras = ["anthropic"]
recorded_on = 2026-10-12
"""


def _write(tmp_path: Path, text: str) -> Path:
    path = tmp_path / "pin.toml"
    path.write_text(text, encoding="utf-8")
    return path


def test_pin_file_is_complete_and_valid() -> None:
    pin = load_pin(PIN_FILE)
    chapter = (REPO / "docs" / "spec" / "09-agents.md").read_text(encoding="utf-8")
    section = chapter.split("### 17.1", 1)[1].split("### 17.2", 1)[0]
    assert f"({pin.repository})" in section or f"{pin.repository})" in section
    assert re.fullmatch(r"[0-9a-f]{40}", pin.commit)
    assert re.fullmatch(r"3\.\d+", pin.python)
    assert pin.install in {"uv-lock", "requirements"}
    assert isinstance(pin.recorded_on, datetime.date)
    if pin.tag is None:
        text = PIN_FILE.read_text(encoding="utf-8")
        assert any(pin.commit in row for r, row in ruling_rows().items() if r in text), (
            "a pin without a release tag must name the ruling that names its commit"
        )
    assert PIN_FILE.read_text(encoding="utf-8").startswith("# SPDX-License-Identifier: Apache-2.0")


def test_load_pin_reads_every_field(tmp_path: Path) -> None:
    pin = load_pin(_write(tmp_path, VALID))
    assert pin == HermesPin(
        repository="https://github.com/NousResearch/hermes-agent",
        tag="v2026.9.24",
        commit="f97608f178d1ffeca59860195ab7da295f7c8e5f",
        python="3.13",
        install="uv-lock",
        extras=("anthropic",),
        recorded_on=datetime.date(2026, 10, 12),
    )


COMMIT = "a8b0bc15053c1d71fb2bfea39007847c9e1c5673"
ROWS = {
    "D-X-1": f"| D-X-1 | 2026-11-07 | Pin the main commit `{COMMIT}` without a tag. | Owner. |",
    "D-X-2": "| D-X-2 | 2026-11-07 | Some other ruling. | Reason. |",
}


def _pin(tag: str | None) -> HermesPin:
    return HermesPin(
        repository="https://github.com/NousResearch/hermes-agent",
        tag=tag,
        commit=COMMIT,
        python="3.14",
        install="uv-lock",
        extras=(),
        recorded_on=datetime.date(2026, 11, 7),
    )


@pytest.mark.parametrize(
    ("tag", "pin_text", "intro", "ok"),
    [
        pytest.param("v0.23.0", "", "against `v0.23.0`, commit `a8b0bc15053c`", True, id="tagged"),
        pytest.param("v0.23.0", "", "against commit `a8b0bc15053c`", False, id="tag-missing"),
        pytest.param("v0.23.0", "", "against `v0.23.0`", False, id="tagged-commit-missing"),
        pytest.param(None, "# D-X-1", "main commit `a8b0bc15053c` (D-X-1)", True, id="tagless"),
        pytest.param(None, "# D-X-1", "main commit `a8b0bc15053c`", False, id="no-ruling-in-17.10"),
        pytest.param(None, "", "main commit `a8b0bc15053c` (D-X-1)", False, id="no-ruling-in-pin"),
        pytest.param(None, "# D-X-2", "commit `a8b0bc15053c` (D-X-2)", False, id="no-commit-row"),
        pytest.param(None, "# D-X-9", "commit `a8b0bc15053c` (D-X-9)", False, id="unknown-ruling"),
        pytest.param(None, "# D-X-1", "main commit (D-X-1)", False, id="tagless-commit-missing"),
    ],
)
def test_pin_references_accept_a_tag_or_a_ruling_that_names_the_commit(
    tag: str | None, pin_text: str, intro: str, ok: bool
) -> None:
    problems = pin_reference_problems(_pin(tag), pin_text, intro, ROWS)
    assert (problems == []) is ok, problems


def test_ruling_rows_find_the_rulings_of_the_hermes_decision_files() -> None:
    rows = ruling_rows()
    assert "bf0203a794a07aac3ff764972ab9087baa4e4f4e" in rows["D-HERMES-COMPAT-3"]
    assert "a8b0bc15053c1d71fb2bfea39007847c9e1c5673" in rows["D-HERMES-MAIN-1"]
    assert "D-M2-32-3" not in rows


def test_load_pin_accepts_a_tagless_pin(tmp_path: Path) -> None:
    text = VALID.replace('tag = "v2026.9.24"\n', "")
    assert load_pin(_write(tmp_path, text)).tag is None


@pytest.mark.parametrize(
    ("old", "new"),
    [
        pytest.param(
            'commit = "f97608f178d1ffeca59860195ab7da295f7c8e5f"', 'commit = "f97608f"', id="short"
        ),
        pytest.param(
            'commit = "f97608f178d1ffeca59860195ab7da295f7c8e5f"',
            'commit = "F97608F178D1FFECA59860195AB7DA295F7C8E5F"',
            id="upper-case",
        ),
        pytest.param('tag = "v2026.9.24"', 'tag = "main"', id="branch-as-tag"),
        pytest.param('tag = "v2026.9.24"', 'tag = "release/2026.9"', id="branch-path-as-tag"),
        pytest.param('install = "uv-lock"', 'install = "pip"', id="unknown-install"),
        pytest.param('python = "3.13"\n', "", id="missing-python"),
        pytest.param("recorded_on = 2026-10-12\n", "", id="missing-date"),
        pytest.param('python = "3.13"', 'python = "3"', id="python-without-minor"),
        pytest.param('extras = ["anthropic"]', 'extras = "anthropic"', id="extras-not-a-list"),
        pytest.param("recorded_on = 2026-10-12", 'recorded_on = "yesterday"', id="date-not-a-date"),
        pytest.param(
            'repository = "https://github.com/NousResearch/hermes-agent"',
            'repository = ""',
            id="empty-repository",
        ),
    ],
)
def test_load_pin_rejects_invalid_pins(tmp_path: Path, old: str, new: str) -> None:
    assert old in VALID
    with pytest.raises(PinError):
        load_pin(_write(tmp_path, VALID.replace(old, new)))


def test_load_pin_rejects_unknown_fields_and_broken_toml(tmp_path: Path) -> None:
    with pytest.raises(PinError, match="unknown"):
        load_pin(_write(tmp_path, VALID + 'branch = "main"\n'))
    with pytest.raises(PinError, match="TOML"):
        load_pin(_write(tmp_path, "commit = \n"))
    with pytest.raises(PinError, match="missing"):
        load_pin(tmp_path / "absent.toml")
