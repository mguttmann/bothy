# SPDX-License-Identifier: Apache-2.0
"""The checked-in evidence of the recording (offline; red until the first recording)."""

from __future__ import annotations

import json
import re
import tomllib
from pathlib import Path
from typing import Any

import pytest
import yaml
from hermes_harness.fake_model import FakeCredentials, ScriptTurn
from hermes_harness.fixtures import (
    FIXTURE_DIR,
    OBSERVATIONS,
    canonical_json,
    inputs_sha256,
    load_fixture,
    write_fixtures,
)
from hermes_harness.observations import SLUGS
from hermes_harness.pin import PIN_FILE, load_pin
from hermes_harness.scenarios import PEOPLE, Scenario, load_scenarios, timing_problems

REPO = Path(__file__).resolve().parents[4]
CHAPTER = REPO / "docs" / "spec" / "09-agenten.md"
FIXTURE_FILES = sorted(p for p in FIXTURE_DIR.glob("*.json")) if FIXTURE_DIR.is_dir() else []
SCENARIOS = {s.name: s for s in load_scenarios()}
_MARKERS = re.compile(r"<(?:ws|partial|cursor|tmp|home|host|user|id:[0-9]+)>")
_IDENTIFIER = re.compile(r"[A-Za-z0-9_.:-]{1,64}")
_DIGEST = re.compile(r"sha256:[0-9a-f]{64}(?::[0-9]+)?")


def _strings(value: Any) -> list[str]:
    if isinstance(value, dict):
        return [s for k, v in value.items() for s in (str(k), *_strings(v))]
    if isinstance(value, list):
        return [s for v in value for s in _strings(v)]
    return [value] if isinstance(value, str) else []


def _section(text: str, start: str, end: str) -> str:
    return text.split(start, 1)[1].split(end, 1)[0]


def _max_file_kb() -> int:
    config = yaml.safe_load((REPO / ".pre-commit-config.yaml").read_text(encoding="utf-8"))
    for repo in config["repos"]:
        for hook in repo["hooks"]:
            if hook["id"] == "check-added-large-files":
                return int(next(a for a in hook["args"] if a.startswith("--maxkb=")).split("=")[1])
    raise AssertionError("check-added-large-files missing")


def _scenario_strings() -> set[str]:
    found: set[str] = set()
    for scenario in SCENARIOS.values():
        found |= set(scenario.texts.values())
        found |= {str(s["command"]) for s in scenario.steps if "command" in s}
    return found


def _command_patterns() -> list[re.Pattern[str]]:
    """Scenario commands with their session references as ID placeholders."""
    patterns = []
    for scenario in SCENARIOS.values():
        for step in scenario.steps:
            if "command" in step:
                text = re.escape(str(step["command"]))
                text = re.sub(r"\\\{session:\w+\\\}", r"<id:[0-9]+>", text)
                patterns.append(re.compile(text))
    return patterns


def test_every_point_has_an_observation() -> None:
    observations = load_fixture(OBSERVATIONS)
    assert sorted(observations["points"]) == sorted(SLUGS)
    for slug in SLUGS:
        assert set(observations["points"][slug]) == {"facts", "holds"}
        assert isinstance(observations["points"][slug]["holds"], bool)


def test_every_scenario_has_a_fixture() -> None:
    names = {p.stem for p in FIXTURE_FILES} - {Path(OBSERVATIONS).stem}
    assert names == set(SCENARIOS)


def test_fixtures_match_the_pin() -> None:
    pin = load_pin(PIN_FILE)
    assert FIXTURE_FILES
    for path in FIXTURE_FILES:
        assert load_fixture(path.name)["pin"] == {"tag": pin.tag, "commit": pin.commit}, path.name
    assert load_fixture(OBSERVATIONS)["recorded_on"] == pin.recorded_on.isoformat()


def test_fixtures_match_the_recording_inputs() -> None:
    current = inputs_sha256()
    for path in FIXTURE_FILES:
        assert load_fixture(path.name)["inputs_sha256"] == current, (
            f"{path.name} was recorded from other inputs; "
            "record again: make -C plugins/hermes-adapter record"
        )


def _table_rows() -> list[list[str]]:
    section = _section(CHAPTER.read_text(encoding="utf-8"), "### 17.10", "\n## 18.")
    rows = []
    for line in section.splitlines():
        if line.startswith("| `"):
            rows.append([cell.strip() for cell in line.strip().strip("|").split("|")])
    return rows


def test_section_17_10_matches_the_observations() -> None:
    observations = load_fixture(OBSERVATIONS)
    pin = load_pin(PIN_FILE)
    section = _section(CHAPTER.read_text(encoding="utf-8"), "### 17.10", "\n## 18.")
    intro = section.split("| Punkt", 1)[0]
    assert pin.tag is not None
    assert pin.tag in intro
    assert pin.commit[:12] in intro
    rows = _table_rows()
    slugs = [row[0].strip("`") for row in rows]
    assert slugs == list(SLUGS)
    for row in rows:
        slug, result, evidence, consequence = row[0].strip("`"), row[1], row[2], row[3]
        holds = observations["points"][slug]["holds"]
        assert result.startswith("bestätigt" if holds else "angepasst"), slug
        if not holds:
            assert re.search(r"\b\d+(\.\d+)?\b", consequence), slug
            assert consequence != "keine", slug
        files = re.findall(r"`([a-z0-9_]+\.json)`", evidence)
        assert files, slug
        for name in files:
            assert (FIXTURE_DIR / name).is_file(), name


def test_no_open_check_remains_in_section_17() -> None:
    section = _section(CHAPTER.read_text(encoding="utf-8"), "### 17.1 ", "### 17.10")
    assert "zu prüfen" not in section


def test_fixtures_hold_no_credential_and_no_third_party_text() -> None:
    rules = tomllib.loads((REPO / ".gitleaks.toml").read_text(encoding="utf-8"))["rules"]
    [rule] = [r for r in rules if r["id"] == "anthropic-credential-any"]
    pattern = re.compile(rule["regex"])
    creds = FakeCredentials.generate()
    assert pattern.search(creds.api_key)
    assert pattern.search(creds.oauth_token)
    allowed = _scenario_strings()
    commands = _command_patterns()
    for path in FIXTURE_FILES:
        text = path.read_text(encoding="utf-8")
        assert not pattern.search(text), path.name
        for value in _strings(json.loads(text)):
            core = _MARKERS.sub("", value)
            ok = (
                value in allowed
                or core in allowed
                or core == ""
                or _IDENTIFIER.fullmatch(value) is not None
                or _DIGEST.fullmatch(value) is not None
                or _MARKERS.fullmatch(value) is not None
                or (value.startswith(("<tmp>", "<home>")) and " " not in value)
                or _known_fact_text(value)
                or any(p.fullmatch(value) for p in commands)
            )
            assert ok, f"{path.name}: unexpected text {value[:60]!r}"


_FACT_TEXT = re.compile(
    r"(?:security|secret-tool|launchctl|systemctl) [a-z-]+"
    r"|(?:process environment|\.env of the Hermes home): [a-z_]+"
    r"|<HERMES_HOME>/plugins/[a-z_]+"
)


def _known_fact_text(value: str) -> bool:
    """Surface names and labels the harness itself writes into the facts."""
    from hermes_harness import observations

    labels = set(_strings({k: v for k, v in vars(observations).items() if k.isupper()}))
    return (
        value in labels
        or _FACT_TEXT.fullmatch(value) is not None
        or value.startswith(
            ("hermes_cli.", "agent.", "tools.", "BasePlatformAdapter", "/stop", "/new")
        )
    )


def _keyed_strings(value: Any, key: str = "") -> list[tuple[str, str]]:
    if isinstance(value, dict):
        return [pair for k, v in value.items() for pair in _keyed_strings(v, str(k))]
    if isinstance(value, list):
        return [pair for v in value for pair in _keyed_strings(v, key)]
    return [(key, value)] if isinstance(value, str) else []


def test_fixtures_are_white_label_and_canonical() -> None:
    people_ids = {p[0] for p in PEOPLE.values()}
    people_names = {p[1] for p in PEOPLE.values()}
    limit = _max_file_kb() * 1024
    for path in FIXTURE_FILES:
        text = path.read_text(encoding="utf-8")
        assert text == canonical_json(json.loads(text)), f"{path.name} is not canonical"
        assert len(text.encode("utf-8")) < limit, path.name
        for email in re.findall(r"[\w.+-]+@([\w-]+(?:\.[\w-]+)+)", text):
            assert email == "example.com", path.name
        assert not re.search(r'"/(?:Users|home|private|var|tmp|opt)/', text), path.name
        for key, value in _keyed_strings(json.loads(text)):
            if key == "user_id":
                assert value in people_ids, (path.name, value)
            elif key == "user_name":
                assert value in people_names, (path.name, value)


def test_write_fixtures_replaces_the_directory_completely(tmp_path: Path) -> None:
    target = tmp_path / "fixtures"
    target.mkdir()
    (target / "README.md").write_text("Beschreibung\n", encoding="utf-8")
    (target / "stale.json").write_text("{}\n", encoding="utf-8")
    write_fixtures({"a.json": {"b": 1, "a": [1]}}, target)
    assert sorted(p.name for p in target.iterdir()) == ["README.md", "a.json"]
    assert (target / "a.json").read_text(
        encoding="utf-8"
    ) == '{\n  "a": [\n    1\n  ],\n  "b": 1\n}\n'
    assert (target / "README.md").read_text(encoding="utf-8") == "Beschreibung\n"


def test_write_fixtures_leaves_the_old_fixtures_on_failure(tmp_path: Path) -> None:
    target = tmp_path / "fixtures"
    target.mkdir()
    (target / "old.json").write_text("{}\n", encoding="utf-8")
    with pytest.raises(TypeError):
        write_fixtures({"bad.json": {"x": object()}}, target)
    assert sorted(p.name for p in target.iterdir()) == ["old.json"]


def test_make_root_falls_back_to_tmp_for_a_long_tmpdir(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    import shutil
    import tempfile

    from hermes_harness.fixtures import make_root
    from hermes_harness.scenarios import root_fits

    long_dir = tmp_path / ("x" * 60)
    long_dir.mkdir()
    monkeypatch.setattr(tempfile, "tempdir", str(long_dir))
    root = make_root()
    try:
        assert root_fits(root)
        assert not root.is_relative_to(long_dir)
        assert root.parent in {Path("/tmp").resolve(), Path("/tmp")}
        assert list(long_dir.iterdir()) == []
    finally:
        shutil.rmtree(root)


def _scenario(point: str, config: dict[str, Any], *turns: ScriptTurn) -> Scenario:
    return Scenario(
        name="synthetic",
        point=point,
        provider="openai",
        credentials={},
        probe={},
        config=config,
        allowlist=("alex",),
        texts={},
        model_script=turns,
        steps=(),
    )


def test_timing_problems_name_unpaced_text_and_racing_progress() -> None:
    tool = ScriptTurn(tool_calls=({"name": "todo_list", "arguments": {}},))
    unpaced = ScriptTurn(text=("a ", "b"))
    paced = ScriptTurn(text=("a ", "b"), paced=True)
    off = {"display": {"platforms": {"recording_probe": {"tool_progress": "off"}}}}
    assert timing_problems(_scenario("steer", off, tool, paced)) == []
    assert timing_problems(_scenario("progress_hooks", {}, tool, paced)) == []
    unpaced_problems = timing_problems(_scenario("steer", off, tool, unpaced))
    assert len(unpaced_problems) == 1
    assert "turn 2" in unpaced_problems[0]
    assert "paced" in unpaced_problems[0]
    progress_problems = timing_problems(_scenario("steer", {}, tool, paced))
    assert len(progress_problems) == 1
    assert "tool_progress" in progress_problems[0]


def test_scenarios_do_not_depend_on_stream_timing() -> None:
    # Hermes drafts only the deltas that reach its stream consumer before the end of the
    # stream, and its progress task polls on its own clock; both race an unpaced answer.
    problems = {name: timing_problems(s) for name, s in SCENARIOS.items()}
    assert {name: p for name, p in problems.items() if p} == {}
