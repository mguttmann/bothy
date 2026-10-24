# SPDX-License-Identifier: Apache-2.0
"""The pinned Hermes version is unchanged after every scenario (M2 exit criterion 5).

Runs last in this directory (file order): the snapshots of the read-only checkout and of the
installed files, taken after provisioning and the adapter's additions, equal the state now.
"""

from __future__ import annotations

from fake_hermes_kit import HermesWorld
from hermes_harness.integrity import assert_unchanged, installed_digest, snapshot_checkout


def test_pinned_hermes_unchanged(world: HermesWorld) -> None:
    assert_unchanged(world.checkout, snapshot_checkout(world.install.checkout))
    assert_unchanged(world.digest, installed_digest(world.install.venv))
