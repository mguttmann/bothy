# SPDX-License-Identifier: Apache-2.0
"""Tests for dockerfile_pins: every base image of a tracked Dockerfile is pinned by digest.

The synthetic tests prove each rule of `check_text` on small Dockerfiles; the repository tests
apply the check to the real tracked Dockerfiles and hold the builder image of the server image
equal to the Go toolchain of `services/server/go.mod` (brief M1-23, scope 1).
"""

from __future__ import annotations

import re
import subprocess
from pathlib import Path

import dockerfile_pins as dp
import pytest

DIGEST = "sha256:" + "a" * 64
REPO = Path(__file__).resolve().parents[3]


def _refs(text: str) -> list[tuple[int, str]]:
    return [(f.line, f.reference) for f in dp.check_text("Dockerfile", text)]


def test_pinned_from_passes() -> None:
    assert _refs(f"FROM golang:1.27.2-trixie@{DIGEST}\n") == []


def test_digest_without_tag_passes() -> None:
    assert _refs(f"FROM gcr.io/distroless/static-debian13@{DIGEST}\n") == []


def test_tag_without_digest_fails() -> None:
    assert _refs("FROM golang:1.27.2-trixie\n") == [(1, "golang:1.27.2-trixie")]


def test_latest_fails() -> None:
    assert _refs("FROM alpine:latest\n") == [(1, "alpine:latest")]


def test_bare_name_fails() -> None:
    assert _refs("FROM alpine\n") == [(1, "alpine")]


@pytest.mark.parametrize(
    "digest", ["sha256:" + "a" * 63, "sha256:" + "a" * 65, "sha256:" + "g" * 64]
)
def test_malformed_digest_fails(digest: str) -> None:
    assert _refs(f"FROM alpine@{digest}\n") == [(1, f"alpine@{digest}")]


def test_upper_case_digest_fails() -> None:
    ref = "alpine@sha256:" + "A" * 64
    assert _refs(f"FROM {ref}\n") == [(1, ref)]


def test_stage_reference_and_scratch_pass() -> None:
    text = f"FROM golang:1@{DIGEST} AS build\nFROM build AS second\nFROM scratch\nFROM Second\n"
    assert _refs(text) == []


def test_stage_reference_before_its_definition_fails() -> None:
    text = f"FROM later\nFROM golang:1@{DIGEST} AS later\n"
    assert _refs(text) == [(1, "later")]


def test_platform_flag_and_as_are_parsed() -> None:
    text = (
        f"FROM --platform=$BUILDPLATFORM golang:1@{DIGEST} as build\n"
        "from --platform=linux/arm64 build\n"
        "FROM --platform=$TARGETPLATFORM debian:13 AS runtime\n"
    )
    assert _refs(text) == [(3, "debian:13")]


def test_line_continuation_is_joined_and_keeps_the_first_line() -> None:
    text = "ARG X=1\nFROM \\\n  --platform=$BUILDPLATFORM \\\n  debian:13 \\\n  AS base\n"
    assert _refs(text) == [(2, "debian:13")]


def test_variable_reference_without_digest_fails() -> None:
    assert _refs("ARG BASE\nFROM ${BASE}\n") == [(2, "${BASE}")]


def test_from_without_image_fails() -> None:
    findings = dp.check_text("Dockerfile", "FROM --platform=linux/amd64\n")
    assert [(f.line, f.reference) for f in findings] == [(1, "")]
    assert "no image" in findings[0].message


def test_file_without_from_passes() -> None:
    assert _refs("# just a comment\nRUN true\n") == []


def test_comments_and_instructions_mentioning_from_are_ignored() -> None:
    text = f"FROM alpine@{DIGEST}\n# FROM alpine:latest\nRUN echo FROM alpine\n"
    assert _refs(text) == []


def test_unpinned_syntax_directive_fails() -> None:
    text = f"# syntax=docker/dockerfile:1\nFROM alpine@{DIGEST}\n"
    assert _refs(text) == [(1, "docker/dockerfile:1")]


def test_pinned_syntax_directive_passes() -> None:
    text = f"# syntax = docker/dockerfile:1.10@{DIGEST}\n# check=error=true\nFROM alpine@{DIGEST}\n"
    assert _refs(text) == []


def test_syntax_comment_after_the_directives_is_not_a_directive() -> None:
    text = f"FROM alpine@{DIGEST}\n# syntax=docker/dockerfile:1\n"
    assert _refs(text) == []


def test_syntax_after_a_blank_line_is_not_a_directive() -> None:
    text = f"\n# syntax=docker/dockerfile:1\nFROM alpine@{DIGEST}\n"
    assert _refs(text) == []


def test_copy_from_an_unpinned_image_fails() -> None:
    text = (
        f"FROM golang:1@{DIGEST} AS build\n"
        "COPY --from=build /out /out\n"
        "COPY --from=0 /out /out\n"
        f"COPY --from=alpine@{DIGEST} /etc/ssl /etc/ssl\n"
        "COPY --chown=0:0 --from=alpine:3 /bin/sh /bin/sh\n"
    )
    assert _refs(text) == [(5, "alpine:3")]


def test_run_mount_from_an_unpinned_image_fails() -> None:
    text = (
        f"FROM golang:1@{DIGEST} AS build\n"
        "RUN --mount=type=bind,from=build,target=/b true\n"
        "RUN --mount=type=cache,target=/c --mount=from=busybox:latest,target=/x true\n"
        f"RUN --mount=type=bind,source=/,from=alpine@{DIGEST},target=/a true\n"
    )
    assert _refs(text) == [(3, "busybox:latest")]


def test_finding_names_file_line_and_reference() -> None:
    finding = dp.check_text("services/x/Dockerfile", "FROM alpine:3\n")[0]
    assert str(finding).startswith("services/x/Dockerfile:1: alpine:3: ")


def test_is_dockerfile_selects_dockerfiles_but_not_ignore_files() -> None:
    assert dp.is_dockerfile("services/server/Dockerfile")
    assert dp.is_dockerfile("tools/x/Dockerfile.dev")
    assert dp.is_dockerfile("tools/x/dev.Dockerfile")
    assert not dp.is_dockerfile("services/server/Dockerfile.dockerignore")
    assert not dp.is_dockerfile(".dockerignore")
    assert not dp.is_dockerfile("README.md")
    assert dp.is_dockerfile("tools/x/build.dockerfile")
    assert dp.is_dockerfile("tools/x/Containerfile")
    assert not dp.is_dockerfile("tools/x/Containerfile.dockerignore")
    assert not dp.is_dockerfile("tools/ci/dockerfile_pins.py")
    assert not dp.is_dockerfile("docs/dockerfiles.md")


def _git(repo: Path, *args: str) -> None:
    subprocess.run(["git", "-C", str(repo), *args], check=True, capture_output=True)


@pytest.fixture
def repo(tmp_path: Path) -> Path:
    root = tmp_path / "repo"
    (root / "svc").mkdir(parents=True)
    _git(root, "init", "-q", "-b", "main")
    (root / "svc" / "Dockerfile").write_text(f"FROM alpine@{DIGEST}\n")
    (root / "svc" / "Dockerfile.dockerignore").write_text("FROM alpine:latest\n")
    (root / "svc" / "dev.dockerfile").write_text(f"FROM alpine@{DIGEST}\n")
    _git(root, "add", ".")
    # Untracked files are not part of the gate (tracked files only, like CI).
    (root / "Dockerfile").write_text("FROM alpine:latest\n")
    return root


def test_tracked_dockerfiles_lists_tracked_files_only(repo: Path) -> None:
    assert dp.tracked_dockerfiles(repo) == ["svc/Dockerfile", "svc/dev.dockerfile"]


def test_main_passes_on_pinned_tracked_files(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    assert dp.main(["--repo", str(repo)]) == 0
    assert "2 Dockerfile(s)" in capsys.readouterr().out


def test_main_fails_and_names_file_and_line(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    (repo / "svc" / "Dockerfile").write_text(f"FROM alpine@{DIGEST} AS a\nFROM debian:13\n")
    assert dp.main(["--repo", str(repo)]) == 1
    err = capsys.readouterr().err
    assert "svc/Dockerfile:2: debian:13:" in err


def test_main_checks_named_files(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    f = tmp_path / "Dockerfile"
    f.write_text("FROM alpine:3\n")
    assert dp.main(["--repo", str(tmp_path), "Dockerfile"]) == 1
    assert "Dockerfile:1: alpine:3:" in capsys.readouterr().err


def test_main_reports_a_missing_file(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    assert dp.main(["--repo", str(tmp_path), "nope/Dockerfile"]) == 2
    assert "nope/Dockerfile" in capsys.readouterr().err


def test_main_with_no_dockerfile(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    _git(tmp_path, "init", "-q", "-b", "main")
    assert dp.main(["--repo", str(tmp_path)]) == 0
    assert "0 Dockerfile(s)" in capsys.readouterr().out


# ---- the real repository ----------------------------------------------------------------------


def test_repository_dockerfiles_are_pinned() -> None:
    files = dp.tracked_dockerfiles(REPO)
    assert "services/server/Dockerfile" in files
    findings = [f for name in files for f in dp.check_text(name, (REPO / name).read_text())]
    assert findings == []


def test_server_builder_image_follows_the_go_toolchain() -> None:
    gomod = (REPO / "services/server/go.mod").read_text()
    m = re.search(r"^toolchain go(\S+)$", gomod, re.M) or re.search(r"^go (\S+)$", gomod, re.M)
    assert m is not None
    dockerfile = (REPO / "services/server/Dockerfile").read_text()
    assert re.search(
        rf"^FROM --platform=\$BUILDPLATFORM golang:{re.escape(m.group(1))}-[a-z]+"
        r"@sha256:[0-9a-f]{64} AS build$",
        dockerfile,
        re.M,
    ), "the builder image tag must equal the toolchain of services/server/go.mod"


def test_server_readme_names_the_pinned_base_images() -> None:
    dockerfile = (REPO / "services/server/Dockerfile").read_text()
    # The console stage reuses the builder image (brief M6-02), so two images for three stages.
    refs = sorted(set(re.findall(r"^FROM (?:--\S+ )*(\S+@sha256:[0-9a-f]{64})", dockerfile, re.M)))
    assert len(refs) == 2
    readme = (REPO / "services/server/README.md").read_text()
    for ref in refs:
        tag, digest = ref.split("@")
        assert f"| `{tag}` | `{digest}` |" in readme, f"services/server/README.md lacks {ref}"
