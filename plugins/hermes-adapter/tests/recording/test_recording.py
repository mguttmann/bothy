# SPDX-License-Identifier: Apache-2.0
"""Live recording against the pinned Hermes Agent version (needs network for the pins).

Every scenario's call pattern must equal its fixture, the computed observations must equal
``observations.json``, nothing may listen or reach a provider, and the pinned checkout and the
installed files must be unchanged. ``test_pinned_hermes_is_unchanged`` stays last.
"""

from __future__ import annotations

import pytest
from hermes_harness.calls import RECORD_HINT, compare_patterns
from hermes_harness.fixtures import OBSERVATIONS, RecordingRun, load_fixture
from hermes_harness.pin import PIN_FILE, load_pin
from hermes_harness.provision import REPO_ROOT
from hermes_harness.scenarios import load_scenarios

SCENARIOS = [s.name for s in load_scenarios()]
COMPARED = ("plugin_discovery", "surface", "egress", "points")


@pytest.mark.parametrize("name", SCENARIOS)
def test_recording(recording: RecordingRun, name: str) -> None:
    result = recording.results[name]
    assert not result.failures, result.failures
    try:
        fixture = load_fixture(f"{name}.json")
    except FileNotFoundError:
        pytest.fail(f"no fixture for {name}; {RECORD_HINT}")
    computed = recording.fixtures[f"{name}.json"]
    assert compare_patterns(fixture["pattern"], computed["pattern"]) == []


def test_observations_match_the_fixture(recording: RecordingRun) -> None:
    try:
        fixture = load_fixture(OBSERVATIONS)
    except FileNotFoundError:
        pytest.fail(f"no {OBSERVATIONS}; {RECORD_HINT}")
    computed = recording.fixtures[OBSERVATIONS]
    for key in COMPARED:
        assert computed[key] == fixture[key], f"{key} differs; {RECORD_HINT}"


def test_hermes_opened_no_listener(recording: RecordingRun) -> None:
    listeners = {name: r.listeners for name, r in recording.results.items() if r.listeners}
    assert listeners == {}
    assert all(any(c["method"] == "connect" for c in r.calls) for r in recording.results.values())


def test_no_unexpected_credential_and_no_model_provider_contacted(recording: RecordingRun) -> None:
    for name, result in recording.results.items():
        bad = [f for f in result.failures if "credential" in f or "provider" in f]
        assert bad == [], name
        kinds = {str(r.get("credential")) for r in result.model_requests}
        assert "unexpected" not in kinds, name
    # Store and service tools were only ever reached through the refusing shims.
    for result in recording.results.values():
        assert all(
            c.split()[0] in {"security", "secret-tool", "launchctl", "systemctl"}
            for c in result.store_calls
        )


def test_pinned_hermes_is_unchanged(recording: RecordingRun) -> None:
    pin = load_pin(PIN_FILE)
    install = recording.install
    assert recording.checkout_after.head == pin.commit
    assert recording.checkout_after.status == ""
    assert recording.integrity_errors == []
    assert recording.checkout_after.digest == install.checkout_state.digest
    assert recording.installed_after == install.installed
    assert not install.root.is_relative_to(REPO_ROOT)
