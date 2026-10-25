# SPDX-License-Identifier: Apache-2.0
"""Step 2: create the adapter's key and print its thumbprint for the owner.

The owner binds the agent token to this key (``dpop_jkt``), so a copied token is worthless
without the key file. The key never leaves the adapter's folder.
"""

from __future__ import annotations

import argparse
from pathlib import Path

from connect import KEY_FILE

from bothy_sdk.auth import FileDpopKey


def main(argv: list[str] | None = None) -> None:
    """Write ``adapter.key`` (mode ``0600``, never overwritten) and print ``dpop_jkt``.

    Args:
        argv: The command line without the program name.
    """
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--adapter-dir", type=Path, required=True)
    args = parser.parse_args(argv)
    args.adapter_dir.mkdir(mode=0o700, parents=True, exist_ok=True)
    try:
        key = FileDpopKey.generate(args.adapter_dir / KEY_FILE)
    except FileExistsError:
        parser.exit(1, f"{args.adapter_dir / KEY_FILE} exists already; keeping it\n")
    print(f"dpop_jkt: {key.jkt}", flush=True)


if __name__ == "__main__":
    main()
