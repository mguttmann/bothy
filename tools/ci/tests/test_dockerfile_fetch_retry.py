# SPDX-License-Identifier: Apache-2.0
"""Every download of a tracked Dockerfile is retried (docs/DECISIONS.md PL/33).

A transient error of a download host turned the image jobs of main red: `go mod download` on a
stream error of the Go module proxy (`postgres-image (amd64)`, run 40012832148) and the restic
download on an HTTP 403 of GitHub's release host (`postgres-image (arm64)`, run 40065171991).
These tests hold the real tracked Dockerfiles to the rule: every `curl` call retries every error
(`--retry` with `--retry-all-errors`, because `-f` turns an HTTP error into a curl error that a
plain `--retry` does not repeat), and every `go mod download` runs in a loop that leaves on
success (`&& break`). apt is not checked: it retries a failed fetch itself (`Acquire::Retries`).
"""

from __future__ import annotations

import re
from pathlib import Path

import dockerfile_pins as dp

REPO = Path(__file__).resolve().parents[3]

#: Splits a RUN command into simple commands at `;`, `&&`, `||` and `|`.
SEPARATORS = re.compile(r"\s*(?:;|&&|\|\||\|)\s*")


def _commands(instruction: str) -> list[str]:
    """Simple commands of one RUN instruction, without leading `VAR=value` assignments."""
    out: list[str] = []
    for part in SEPARATORS.split(instruction):
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
    return findings


def test_curl_without_retry_is_found() -> None:
    text = "FROM x AS a\nRUN apt-get install -y curl; \\\n    curl -fsSL -o /tmp/a https://h/a\n"
    assert fetch_findings("Dockerfile", text) == [
        "Dockerfile:2: curl without --retry-all-errors",
        "Dockerfile:2: curl without --retry",
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


def test_repository_dockerfiles_retry_every_download() -> None:
    files = dp.tracked_dockerfiles(REPO)
    assert "services/server/images/bothy-postgres/Dockerfile" in files
    findings = [f for name in files for f in fetch_findings(name, (REPO / name).read_text())]
    assert findings == []
