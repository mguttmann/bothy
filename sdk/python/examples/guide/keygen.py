# SPDX-License-Identifier: Apache-2.0
"""Step 2: create the adapter's key and print its thumbprint for the owner.

The owner binds the agent token to this key (``dpop_jkt``), so a copied token is worthless
without the key file. The key never leaves the adapter's folder.
"""

from __future__ import annotations

import argparse
import stat
from pathlib import Path

from connect import KEY_FILE

from bothy_sdk.auth import FileDpopKey


def main(argv: list[str] | None = None) -> None:
    """Write ``adapter.key`` (mode ``0600``, never overwritten) and print ``dpop_jkt``.

    The adapter's folder must exist and be private (``mkdir -m 700``): the key and the token
    live there.

    Args:
        argv: The command line without the program name.
    """
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--adapter-dir", type=Path, required=True)
    args = parser.parse_args(argv)
    folder: Path = args.adapter_dir
    if not folder.is_dir() or folder.stat().st_mode & (stat.S_IRWXG | stat.S_IRWXO):
        parser.exit(1, f"{folder} must be a folder only the adapter may use (mkdir -m 700)\n")
    try:
        key = FileDpopKey.generate(args.adapter_dir / KEY_FILE)
    except FileExistsError:
        parser.exit(1, f"{args.adapter_dir / KEY_FILE} exists already; keeping it\n")
    print(f"dpop_jkt: {key.jkt}", flush=True)


if __name__ == "__main__":
    main()
