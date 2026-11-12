# SPDX-License-Identifier: Apache-2.0
"""The texts about the platform runs of the host service follow the list of recorded runs.

`services/host/internal/buildinfo/buildinfo.go` lists the platforms whose recorded platform run
proved the peer check (`peerCheckProven`). The list is empty until a commit records a run
(docs/DECISIONS.md D-M2-42-19). While it is empty, docs/spec/24-terminal.md section 14 and
services/host/README.md say that no platform has a recorded passing run of the workflow
`host-platform.yml`, instead of naming its hosted runners as the place where the host-local
tests are proven (docs/decisions.d/AUD-misc-misc-01.md). The commit that records the first run
changes both texts together with the list.

The test lives with the tools tests because they run on every change, also on one that touches
only a chapter of the specification.
"""

from __future__ import annotations

import re
from pathlib import Path

import pytest

REPO = Path(__file__).resolve().parents[3]
BUILDINFO = REPO / "services" / "host" / "internal" / "buildinfo" / "buildinfo.go"
_PROVEN = re.compile(r"^var peerCheckProven = map\[string\]bool\{([^}]*)\}", re.MULTILINE)
# The sentence of each text, whitespace collapsed.
SENTENCES = {
    "docs/spec/24-terminal.md": "No platform has a recorded passing run of that workflow yet",
    "services/host/README.md": (
        "no platform has a recorded passing run of `host-platform.yml` yet"
    ),
}


def recorded_platforms(source: str) -> list[str]:
    """Return the platforms of `peerCheckProven` in the Go source of the package buildinfo.

    Example: `recorded_platforms('var peerCheckProven = map[string]bool{"linux": true}')`
    returns `["linux"]`; the empty map returns `[]`.

    Raises:
        ValueError: the source does not declare the list in the known form.
    """
    found = _PROVEN.search(source)
    if found is None:
        raise ValueError("buildinfo.go does not declare peerCheckProven as a map literal")
    return re.findall(r'"([a-z0-9]+)"\s*:\s*true', found.group(1))


def says_none_recorded(text: str, sentence: str) -> bool:
    """Return whether `text` holds `sentence`, however its lines are wrapped."""
    return sentence in " ".join(text.split())


def test_recorded_platforms_reads_the_list() -> None:
    assert recorded_platforms("var peerCheckProven = map[string]bool{}\n") == []
    source = 'var peerCheckProven = map[string]bool{\n\t"linux": true,\n\t"darwin": true,\n}\n'
    assert recorded_platforms(source) == ["linux", "darwin"]
    with pytest.raises(ValueError, match="peerCheckProven"):
        recorded_platforms("var other = 1\n")


def test_a_wrapped_sentence_is_found() -> None:
    assert says_none_recorded("a run.\nNo platform has\n  a recorded run yet.", "has a recorded")
    assert not says_none_recorded("A platform has a run.", "No platform has")


@pytest.mark.parametrize("name", sorted(SENTENCES))
def test_the_texts_follow_the_list_of_recorded_runs(name: str) -> None:
    recorded = recorded_platforms(BUILDINFO.read_text(encoding="utf-8"))
    named = says_none_recorded((REPO / name).read_text(encoding="utf-8"), SENTENCES[name])
    assert named != bool(recorded), (
        f"{name} says that no platform has a recorded passing run: {named}; "
        f"platforms with a recorded run in buildinfo.go: {recorded or 'none'}"
    )
