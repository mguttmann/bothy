# SPDX-License-Identifier: Apache-2.0
"""The third-party images an instance runs from the Compose files, one reference per line.

The daily image scan of `advisories.yml` (job `images`, docs/quality/gates.md "Advisories",
#908) runs `make trivy-image` over the images the product builds and over these. A Compose
file names a third-party image as `image: <name>:${BOTHY_<X>_VERSION:-<default>}`, the form
the Renovate manager of `renovate.json` reads; the scan takes the default, the version an
instance gets without an override in its `.env`. The images of the product itself
(`${BOTHY_REGISTRY}/...`) are left out: the job builds them from the checkout.

Usage: compose_images.py [<compose file> ...]   (default: the files of COMPOSE_FILES)
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]
COMPOSE_FILES = ("infra/compose/compose.yaml", "infra/compose/compose.multi-node.yaml")
_PINNED = re.compile(
    r"^\s*image:\s*(?P<name>[a-z0-9][a-z0-9./_-]*):\$\{BOTHY_[A-Z0-9_]+_VERSION:-(?P<tag>[^}\s]+)\}\s*$"
)
_IMAGE = re.compile(r"^\s*image:\s*(?P<ref>\S+)")


class ComposeError(Exception):
    """A third-party image of a Compose file has no version default the scan can take."""


def images(text: str, source: str) -> list[str]:
    """Return the third-party images of one Compose file as `<name>:<default>`, sorted.

    Raises:
        ComposeError: an image line is neither one of the product (`${BOTHY_REGISTRY}`) nor of the
            pinned form, so the scan would miss it.
    """
    found: set[str] = set()
    for number, line in enumerate(text.splitlines(), start=1):
        image = _IMAGE.match(line)
        if image is None or image.group("ref").startswith("${BOTHY_REGISTRY"):
            continue
        pinned = _PINNED.match(line)
        if pinned is None:
            raise ComposeError(
                f"{source}:{number}: {image.group('ref')} is not of the form "
                "<name>:${BOTHY_<X>_VERSION:-<default>}"
            )
        found.add(f"{pinned.group('name')}:{pinned.group('tag')}")
    return sorted(found)


def main(argv: list[str] | None = None) -> int:
    """Print the third-party images of the Compose files; 1 on an image without a default."""
    files = (argv if argv is not None else sys.argv[1:]) or [str(REPO / f) for f in COMPOSE_FILES]
    found: set[str] = set()
    try:
        for name in files:
            found.update(images(Path(name).read_text(encoding="utf-8"), name))
    except (OSError, ComposeError) as err:
        print(f"compose_images: {err}", file=sys.stderr)
        return 1
    for image in sorted(found):
        print(image)
    return 0


if __name__ == "__main__":
    sys.exit(main())
