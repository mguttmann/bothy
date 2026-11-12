# SPDX-License-Identifier: Apache-2.0
"""A hosting runner as a process of its own, for a scenario that kills it (`hosting_world`).

``python hosting_runner.py <runner.toml> <origins>`` runs the runner of ``<runner.toml>`` with the
test seam ``hosted_url`` of `Runner`, so each hosted agent reaches the FakeServer of its own;
``<origins>`` is a JSON object from agent ID to origin. The process runs until it is killed.
"""

from __future__ import annotations

import asyncio
import json
import logging
import sys
from pathlib import Path
from uuid import UUID

from claude_code_runner.app import Runner
from claude_code_runner.config import load_config


def main(config_file: str, origins_json: str) -> None:
    """Run the runner until the process is killed."""
    origins: dict[str, str] = json.loads(origins_json)

    def hosted_url(agent_id: UUID) -> str:
        return origins[str(agent_id)]

    logging.basicConfig(level=logging.INFO, format="%(message)s")
    asyncio.run(Runner(load_config(Path(config_file)), hosted_url=hosted_url).run())


if __name__ == "__main__":
    main(sys.argv[1], sys.argv[2])
