# SPDX-License-Identifier: Apache-2.0
"""The recording run as a whole, its fixtures and the digest of its inputs.

:func:`run_recording` provisions the pinned version once, runs every scenario, takes the
integrity snapshots again and builds the scenario fixtures and ``observations.json``.
:func:`write_fixtures` replaces the fixture directory only through a complete temporary copy,
so a failed run never leaves partial fixtures. :func:`inputs_sha256` digests the inputs that
wrote the fixtures; a change to any of them without recording again fails the offline tests.

Example:
    >>> run = run_recording(load_pin(PIN_FILE), Path(tempfile.mkdtemp(prefix="h")))
    >>> write_fixtures(run.fixtures)
"""

from __future__ import annotations

import getpass
import hashlib
import json
import os
import shutil
import socket
import tempfile
from collections.abc import Mapping
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Final

from hermes_harness.calls import TIMER_METHODS, Redactions, derive_pattern, normalize
from hermes_harness.integrity import (
    CheckoutState,
    IntegrityError,
    assert_unchanged,
    installed_digest,
    snapshot_checkout,
)
from hermes_harness.observations import (
    discovery_facts,
    egress_facts,
    observe,
    store_call_facts,
    surface_facts,
)
from hermes_harness.pin import PIN_FILE, HermesPin
from hermes_harness.provision import REPO_ROOT, HermesInstall, provision, release
from hermes_harness.sandbox import account_home
from hermes_harness.scenarios import (
    PROBE_DIR,
    SCENARIO_DIR,
    TESTS_DIR,
    Scenario,
    ScenarioResult,
    ScenarioRunner,
    load_scenarios,
    root_fits,
)

FIXTURE_DIR: Final = TESTS_DIR / "fixtures" / "recording"
OBSERVATIONS: Final = "observations.json"
HARNESS_DIR: Final = Path(__file__).resolve().parent
#: Hand-written files of the fixture directory that a new recording keeps.
KEPT_FILES: Final = ("README.md",)


def _input_files() -> list[Path]:
    files = [PIN_FILE]
    for directory, pattern in ((PROBE_DIR, "*"), (SCENARIO_DIR, "*.json"), (HARNESS_DIR, "*.py")):
        files += [
            p for p in directory.rglob(pattern) if p.is_file() and "__pycache__" not in p.parts
        ]
    return sorted(set(files))


def inputs_sha256() -> str:
    """SHA-256 over the pin, the probe plugin, the scenarios and the harness modules."""
    digest = hashlib.sha256()
    for path in _input_files():
        rel = path.relative_to(TESTS_DIR).as_posix()
        digest.update(
            rel.encode() + b"\0" + hashlib.sha256(path.read_bytes()).hexdigest().encode() + b"\n"
        )
    return digest.hexdigest()


def canonical_json(data: Any) -> str:
    """JSON with sorted keys, two-space indent, ASCII only and a final newline."""
    return json.dumps(data, sort_keys=True, indent=2, ensure_ascii=True) + "\n"


def redactions_for(root: Path, scenario: Scenario | None = None) -> Redactions:
    """The redactions of a run: its temporary root, the real home, host and OS user."""
    texts: tuple[str, ...] = ()
    if scenario is not None:
        commands = [str(s["command"]) for s in scenario.steps if "command" in s]
        texts = (*scenario.texts.values(), *commands)
    return Redactions(
        tmp_root=str(root.resolve()),
        home=str(account_home()),
        host=socket.gethostname(),
        user=getpass.getuser(),
        scenario_texts=texts,
    )


def _rendered(scenario: Scenario, result: ScenarioResult) -> tuple[str, ...]:
    rendered = []
    for step in scenario.steps:
        if "command" not in step:
            continue
        text = str(step["command"])
        for key, value in scenario.texts.items():
            text = text.replace("{text:" + key + "}", value)
        for key, value in result.sessions.items():
            text = text.replace("{session:" + key + "}", value)
        rendered.append(text)
    return tuple(rendered)


def normalized_result(result: ScenarioResult, scenario: Scenario, root: Path) -> dict[str, Any]:
    """Calls, model requests and commands of one scenario, normalized with one ID registry."""
    red = redactions_for(root)
    red = Redactions(
        tmp_root=red.tmp_root,
        home=red.home,
        host=red.host,
        user=red.user,
        scenario_texts=(*scenario.texts.values(), *_rendered(scenario, result)),
    )
    calls = [c for c in result.calls if c["method"] not in TIMER_METHODS]
    bundle = [
        *calls,
        {"method": "<model>", "args": {"requests": result.model_requests}},
        {"method": "<commands>", "args": {"items": result.commands}},
    ]
    out = normalize(bundle, red)
    return {
        "calls": out[: len(calls)],
        "model_requests": out[len(calls)]["args"]["requests"],
        "commands": out[len(calls) + 1]["args"]["items"],
    }


def scenario_fixture(
    result: ScenarioResult, scenario: Scenario, pin: HermesPin, root: Path, inputs: str
) -> dict[str, Any]:
    """The fixture of one scenario (see the fixture README for the fields)."""
    normalized = normalized_result(result, scenario, root)
    return {
        "scenario": scenario.name,
        "pin": {"tag": pin.tag, "commit": pin.commit},
        "inputs_sha256": inputs,
        "calls": normalized["calls"],
        "model_requests": normalized["model_requests"],
        "pattern": derive_pattern(normalized["calls"]),
    }


def observations_fixture(
    results: Mapping[str, ScenarioResult],
    scenarios: Mapping[str, Scenario],
    pin: HermesPin,
    inputs: str,
) -> dict[str, Any]:
    """``observations.json``: pin, inputs, discovery, surface, egress, store calls, points."""
    return {
        "pin": {"tag": pin.tag, "commit": pin.commit},
        "inputs_sha256": inputs,
        "recorded_on": pin.recorded_on.isoformat(),
        "plugin_discovery": discovery_facts(results),
        "surface": surface_facts(results),
        "egress": egress_facts(results),
        "credential_store_calls": store_call_facts(results),
        "points": observe(results, scenarios),
    }


@dataclass
class RecordingRun:
    """Everything one recording produced.

    Attributes:
        install: The provisioned version.
        results: Raw recordings by scenario name.
        scenarios: The scenarios by name.
        fixtures: File name to fixture data (scenario fixtures and ``observations.json``).
        checkout_after: Snapshot of the checkout after the last scenario.
        installed_after: Installed digest after the last scenario.
        integrity_errors: Differences between the snapshots before and after.
    """

    install: HermesInstall
    results: dict[str, ScenarioResult]
    scenarios: dict[str, Scenario]
    fixtures: dict[str, Any]
    checkout_after: CheckoutState
    installed_after: str
    integrity_errors: list[str] = field(default_factory=list)

    @property
    def failures(self) -> dict[str, list[str]]:
        """Failures per scenario, only the failing ones."""
        return {name: r.failures for name, r in self.results.items() if r.failures}


def make_root() -> Path:
    """A fresh temporary root outside the repository (``mktemp -d``).

    It is made in ``TMPDIR``; when that path is too long for the Unix sockets of the Hermes
    homes (the default ``TMPDIR`` of macOS is), it is made in ``/tmp`` instead.

    Raises:
        IntegrityError: The root would lie inside the repository.
    """
    root = Path(tempfile.mkdtemp(prefix="h")).resolve()
    if not root_fits(root):
        root.rmdir()
        root = Path(tempfile.mkdtemp(prefix="h", dir="/tmp")).resolve()
    if root.is_relative_to(REPO_ROOT):
        shutil.rmtree(root)
        raise IntegrityError(f"{root}: TMPDIR lies inside the repository")
    return root


def run_recording(pin: HermesPin, root: Path) -> RecordingRun:
    """Provision, run every scenario, snapshot again and build the fixtures.

    Args:
        pin: The pinned version.
        root: A fresh temporary root (:func:`make_root`).

    Returns:
        The run; the caller releases it with :func:`finish_recording`.
    """
    install = provision(pin, root / "i")
    scenarios = {s.name: s for s in load_scenarios()}
    results = ScenarioRunner(install, root=root).run_all(scenarios.values())
    checkout_after = snapshot_checkout(install.checkout)
    installed_after = installed_digest(install.venv)
    errors = []
    for before, after in (
        (install.checkout_state, checkout_after),
        (install.installed, installed_after),
    ):
        try:
            assert_unchanged(before, after)
        except IntegrityError as exc:
            errors.append(str(exc))
    inputs = inputs_sha256()
    fixtures: dict[str, Any] = {
        f"{name}.json": scenario_fixture(results[name], scenarios[name], pin, root, inputs)
        for name in sorted(results)
    }
    fixtures[OBSERVATIONS] = observations_fixture(results, scenarios, pin, inputs)
    return RecordingRun(
        install, results, scenarios, fixtures, checkout_after, installed_after, errors
    )


def finish_recording(run: RecordingRun) -> list[str]:
    """Release the checkout and remove the temporary root; return what remained."""
    release(run.install)
    root = run.install.root.parent
    shutil.rmtree(root, ignore_errors=True)
    return [str(root)] if root.exists() else []


def write_fixtures(fixtures: Mapping[str, Any], directory: Path = FIXTURE_DIR) -> None:
    """Replace the fixture directory with ``fixtures`` (the hand-written files are kept).

    The new files are written into a temporary directory outside the repository first and
    swapped in only when every file is complete.
    """
    staging = Path(tempfile.mkdtemp(prefix="fx")).resolve()
    try:
        for name, data in fixtures.items():
            (staging / name).write_text(canonical_json(data), encoding="utf-8")
        for name in KEPT_FILES:
            if (directory / name).is_file():
                shutil.copy2(directory / name, staging / name)
        old = Path(tempfile.mkdtemp(prefix="fo")).resolve() / "old"
        if directory.exists():
            shutil.move(str(directory), str(old))
        shutil.move(str(staging), str(directory))
        shutil.rmtree(old.parent, ignore_errors=True)
    finally:
        if staging.exists():
            shutil.rmtree(staging, ignore_errors=True)


def load_fixture(name: str, directory: Path = FIXTURE_DIR) -> dict[str, Any]:
    """Read one fixture file.

    Raises:
        FileNotFoundError: The file is missing (record first).
    """
    data: dict[str, Any] = json.loads((directory / name).read_text(encoding="utf-8"))
    return data


def environment_summary() -> dict[str, str]:
    """The platform the recording ran on (for the report, never written to a fixture)."""
    return {"platform": os.uname().sysname}
