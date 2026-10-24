# SPDX-License-Identifier: Apache-2.0
# Stub of the pinned Hermes Agent version (tests/hermes_pin.toml): exactly the symbols the
# adapter uses; test_hermes_stubs_match_pinned_version holds the signatures equal.

import os
from pathlib import Path

def load_hermes_dotenv(
    *,
    hermes_home: str | os.PathLike[str] | None = ...,
    project_env: str | os.PathLike[str] | None = ...,
    load_external_secrets: bool = ...,
) -> list[Path]: ...
