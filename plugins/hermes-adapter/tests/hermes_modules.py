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
        "hermes_bootstrap",
        "hermes_cli",
        "hermes_constants",
        "hermes_constants_scratch",
        "hermes_logging",
        "hermes_platform",
        "hermes_startup_watchdog",
        "hermes_state",
        "hermes_state_common",
        "hermes_state_compression",
        "hermes_state_dbfile",
        "hermes_state_errors",
        "hermes_state_fts",
        "hermes_state_gateway",
        "hermes_state_guard",
        "hermes_state_health",
        "hermes_state_holders",
        "hermes_state_ids",
        "hermes_state_lockguard",
        "hermes_state_lockowners",
        "hermes_state_maintenance",
        "hermes_state_messages",
        "hermes_state_portability",
        "hermes_state_profile_repair",
        "hermes_state_readpool",
        "hermes_state_registry",
        "hermes_state_repair",
        "hermes_state_rewind",
        "hermes_state_schema",
        "hermes_state_search",
        "hermes_state_sessions",
        "hermes_state_telegram",
        "hermes_state_timeline",
        "hermes_state_titles",
        "hermes_state_usage",
        "hermes_state_user_copy",
        "hermes_state_wal",
        "hermes_time",
        "mcp_serve",
        "mini_swe_runner",
        "model_tools",
        "plugins",
        "providers",
        "registration_lifecycle",
        "run_agent",
        "tools",
        "toolset_distributions",
        "toolsets",
        "trajectory_compressor",
        "tui_gateway",
        "utils",
    ]
)
