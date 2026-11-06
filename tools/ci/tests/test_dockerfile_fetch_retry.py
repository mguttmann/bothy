# SPDX-License-Identifier: Apache-2.0
"""The curl and go mod download calls of the tracked Dockerfiles are retried (PL/33).

A transient error of a download host turned the image jobs of main red: `go mod download` on a
stream error of the Go module proxy (`postgres-image (amd64)`, run 40012832148) and the restic
download on an HTTP 403 of GitHub's release host (`postgres-image (arm64)`, run 40065171991).
These tests hold the real tracked Dockerfiles to the rule: every `curl` call retries every error
(`--retry` with `--retry-all-errors`, because `-f` turns an HTTP error into a curl error that a
plain `--retry` does not repeat), and every `go mod download` runs in a loop that leaves on
success (`&& break`). A RUN that installs from snapshot.debian.org runs `apt-get update` and
`apt-get install` in a loop that leaves on success and turns APT's own retries off
(docs/decisions.d/FIX-apt-snapshot-retry.md): the snapshot failed the host image of the terminal
suite in main runs 42082056885 and 42081964628 with HTTP 503 and 504 for more than ten minutes,
and APT 3.0 with its own retries repeats "Tried to start delayed item" until the kernel kills it.
apt against deb.debian.org is not checked: no image job failed on it.
Not covered: other downloads (`git clone`, `flutter`, `ADD <url>`) and calls written in another
form than a simple command (exec form, heredoc, `$(...)`, `if`); docs/DECISIONS.md PL/33 lists them.
"""

from __future__ import annotations

import re
from pathlib import Path

import dockerfile_pins as dp
import pytest

REPO = Path(__file__).resolve().parents[3]

#: An APT source on snapshot.debian.org in a RUN command.
SNAPSHOT_SOURCE = re.compile(r"\bdeb https?://snapshot\.debian\.org/")

#: Splits a RUN command into simple commands at `;`, `&&`, `||` and `|`.
SEPARATORS = re.compile(r"\s*(?:;|&&|\|\||\|)\s*")


def _commands(instruction: str) -> list[str]:
    """Simple commands of one RUN instruction, without the keyword and `VAR=value` assignments."""
    out: list[str] = []
    # The keyword and its options (`--mount=...`) are no part of the shell command.
    command = re.sub(r"^RUN(?:\s+--\S+)*\s+", "", instruction, count=1, flags=re.IGNORECASE)
    for part in SEPARATORS.split(command):
        words = [w for w in part.split() if not re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*=\S*", w)]
        if words:
            out.append(" ".join(words))
    return out


def fetch_findings(path: str, text: str) -> list[str]:
    """Return `path:line: reason` for every download of `text` that is not retried."""
    findings: list[str] = []
    for number, instruction in dp._instructions(text):
        if not instruction.lower().startswith("run "):
            continue
        for command in _commands(instruction):
            words = command.split()
            if words[0] == "curl" and "--retry-all-errors" not in words:
                findings.append(f"{path}:{number}: curl without --retry-all-errors")
            if words[0] == "curl" and "--retry" not in words:
                findings.append(f"{path}:{number}: curl without --retry")
        if "go mod download" in instruction and not re.search(
            r"go mod download(?: [^;|&]*)?\s*&&\s*break\b", instruction
        ):
            findings.append(f"{path}:{number}: go mod download outside a retry loop")
        if SNAPSHOT_SOURCE.search(instruction) and "apt-get install" in instruction:
            where = f"{path}:{number}: apt-get from snapshot.debian.org"
            if not re.search(
                r"\bfor \w+ in [^;]+; do\b.*apt-get update.*apt-get install[^;]*&&\s*break\b",
                instruction,
            ):
                findings.append(f"{where} outside a retry loop")
            if 'Acquire::Retries "0"' not in instruction:
                findings.append(f"{where} with APT retries on")
    return findings


def test_curl_without_retry_is_found() -> None:
    text = "FROM x AS a\nRUN apt-get install -y curl; \\\n    curl -fsSL -o /tmp/a https://h/a\n"
    assert fetch_findings("Dockerfile", text) == [
        "Dockerfile:2: curl without --retry-all-errors",
        "Dockerfile:2: curl without --retry",
    ]


@pytest.mark.parametrize(
    "run", ["RUN curl", "run curl", "RUN --mount=type=cache,target=/x --network=default curl"]
)
def test_curl_as_the_first_command_of_run_is_found(run: str) -> None:
    assert fetch_findings("Dockerfile", f"{run} -fsSL -o /tmp/a https://h/a\n") == [
        "Dockerfile:1: curl without --retry-all-errors",
        "Dockerfile:1: curl without --retry",
    ]


def test_curl_with_retry_passes_and_a_package_named_curl_is_no_call() -> None:
    text = "RUN apt-get install -y curl && curl -fsSL --retry 5 --retry-all-errors -o a https://h\n"
    assert fetch_findings("Dockerfile", text) == []


def test_go_mod_download_outside_a_loop_is_found() -> None:
    text = (
        "RUN --mount=type=cache,target=/go/pkg/mod \\\n    GOFLAGS=-mod=readonly go mod download\n"
    )
    assert fetch_findings("Dockerfile", text) == [
        "Dockerfile:1: go mod download outside a retry loop"
    ]


def test_go_mod_download_in_a_loop_passes() -> None:
    text = (
        "RUN for attempt in 1 2 3; do \\\n"
        "      GOFLAGS=-mod=readonly go mod download && break; \\\n"
        '      [ "$attempt" = 3 ] && exit 1; sleep 10; \\\n'
        "    done\n"
    )
    assert fetch_findings("Dockerfile", text) == []


SNAPSHOT_SOURCES = (
    "printf 'deb http://snapshot.debian.org/archive/debian/x trixie main' > /etc/apt/sources.list"
)


def test_apt_from_snapshot_outside_a_loop_is_found() -> None:
    text = (
        f"RUN {SNAPSHOT_SOURCES} \\\n"
        "    && printf 'Acquire::Retries \"5\";' > /etc/apt/apt.conf.d/99snapshot \\\n"
        "    && apt-get update -qq && apt-get install -y -qq systemd\n"
    )
    assert fetch_findings("Dockerfile", text) == [
        "Dockerfile:1: apt-get from snapshot.debian.org outside a retry loop",
        "Dockerfile:1: apt-get from snapshot.debian.org with APT retries on",
    ]


def test_apt_from_snapshot_in_a_loop_passes_and_other_mirrors_are_not_checked() -> None:
    text = (
        f"RUN {SNAPSHOT_SOURCES} \\\n"
        "    && printf 'Acquire::Retries \"0\";' > /etc/apt/apt.conf.d/99snapshot \\\n"
        "    && for attempt in 1 2 3; do \\\n"
        "         apt-get update -qq && apt-get install -y -qq systemd && break; \\\n"
        '         [ "$attempt" = 3 ] && exit 1; sleep 10; \\\n'
        "       done\n"
        "RUN apt-get update -qq && apt-get install -y -qq unzip\n"
    )
    assert fetch_findings("Dockerfile", text) == []


def test_repository_dockerfiles_retry_every_download() -> None:
    files = dp.tracked_dockerfiles(REPO)
    assert "services/server/images/bothy-postgres/Dockerfile" in files
    assert "infra/compose/test/host/Dockerfile" in files
    findings = [f for name in files for f in fetch_findings(name, (REPO / name).read_text())]
    assert findings == []
