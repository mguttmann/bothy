# SPDX-License-Identifier: Apache-2.0
"""Top-level module names of the pinned Hermes Agent distribution (``top_level.txt``).

Recorded from the installation of the pin in ``tests/hermes_pin.toml``; the Fake-Hermes test
``test_hermes_top_level_matches_pinned_version`` holds this list equal to the provisioned
installation, so ``test_only_two_modules_import_hermes`` judges against the real names.
"""

from __future__ import annotations

from typing import Final

HERMES_TOP_LEVEL: Final = frozenset(
    [
        "acp_adapter",
        "agent",
        "batch_runner",
        "cli",
        "cron",
        "gateway",
        "hermes_agent",
        "hermes_bootstrap",
        "hermes_cli",
        "hermes_constants",
        "hermes_constants_scratch",
        "hermes_constants_scratch_detach",
        "hermes_constants_scratch_stuck",
        "hermes_dir_acl",
        "hermes_logging",
        "hermes_platform",
        "hermes_scope",
        "hermes_startup_watchdog",
        "hermes_state",
        "hermes_state_common",
        "hermes_state_loop_guard",
        "hermes_state_registry",
        "hermes_time",
        "hermes_yaml",
        "lineage_adoption",
        "model_tools",
        "persistence",
        "persistence_assembly",
        "plugins",
        "pm",
        "process_identity",
        "providers",
        "registration_lifecycle",
        "run_agent",
        "session_keys",
        "session_rescue",
        "session_retention",
        "session_rules",
        "sqlite_support",
        "storage_errors",
        "storage_limits",
        "storage_user_copy",
        "tools",
        "toolset_distributions",
        "toolsets",
        "trajectory_compressor",
        "tui_gateway",
        "utils",
    ]
)
