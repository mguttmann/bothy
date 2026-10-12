# SPDX-License-Identifier: Apache-2.0
"""Pytest option ``--record`` and the session fixture of the live recording."""

from __future__ import annotations

from collections.abc import Iterator

import pytest
from hermes_harness.fixtures import (
    RecordingRun,
    finish_recording,
    make_root,
    run_recording,
    write_fixtures,
)
from hermes_harness.pin import PIN_FILE, load_pin


def pytest_addoption(parser: pytest.Parser) -> None:
    parser.addoption(
        "--record",
        action="store_true",
        default=False,
        help="write the fixtures of tests/fixtures/recording after a complete, clean recording",
    )


@pytest.fixture(scope="session")
def recording(request: pytest.FixtureRequest) -> Iterator[RecordingRun]:
    """Provision the pinned Hermes once, run every scenario and (with --record) write fixtures.

    Without network the provisioning fails, so every recording test fails; it never skips.
    The teardown removes the temporary root and fails if anything remains.
    """
    root = make_root()
    run: RecordingRun | None = None
    try:
        run = run_recording(load_pin(PIN_FILE), root)
        if request.config.getoption("--record"):
            if run.failures or run.integrity_errors:
                pytest.fail(
                    f"not recorded: scenario failures {run.failures}, "
                    f"integrity {run.integrity_errors}"
                )
            write_fixtures(run.fixtures)
        yield run
    finally:
        if run is not None:
            remained = finish_recording(run)
        else:
            import shutil

            from hermes_harness.integrity import restore_writable

            if (root / "i" / "checkout").exists():
                restore_writable(root / "i" / "checkout")
            shutil.rmtree(root, ignore_errors=True)
            remained = [str(root)] if root.exists() else []
        if remained:
            pytest.fail(f"temporary root not removed: {remained}")
