# SPDX-License-Identifier: Apache-2.0
"""The daily image scan covers the images of an instance (#908, docs/quality/gates.md "Advisories").

tools/ci/compose_images.py lists the third-party images of the Compose files at their default
versions; the job `images` of advisories.yml builds the two images of the product and runs
`make trivy-image` over them and over that list.
"""

from __future__ import annotations

import re
from pathlib import Path

import compose_images
import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]


def test_the_real_compose_files_list_every_third_party_image_with_its_default() -> None:
    listed = set()
    for name in compose_images.COMPOSE_FILES:
        listed.update(compose_images.images((REPO / name).read_text(encoding="utf-8"), name))
    texts = "\n".join((REPO / n).read_text(encoding="utf-8") for n in compose_images.COMPOSE_FILES)
    names = {
        m.group(1) for m in re.finditer(r"^\s*image:\s*([^$\s:][^:\s]*):", texts, re.MULTILINE)
    }
    assert names, "no third-party image in the Compose files"
    assert {image.rsplit(":", 1)[0] for image in listed} == names
    assert all(":" in image and "$" not in image for image in listed)


def test_an_image_of_bothy_is_left_out_and_a_default_is_taken() -> None:
    text = (
        "x: &server\n  image: ${BOTHY_REGISTRY:?missing}/bothy-server:${BOTHY_VERSION:?missing}\n"
        "services:\n"
        "  db:\n    image: ${BOTHY_REGISTRY}/bothy-postgres:pg18-${BOTHY_VERSION}\n"
        "  prom:\n    image: prom/prometheus:${BOTHY_PROMETHEUS_VERSION:-v3.15.0}\n"
    )
    assert compose_images.images(text, "c.yaml") == ["prom/prometheus:v3.15.0"]


@pytest.mark.parametrize(
    "line",
    [
        "    image: redis:7",
        "    image: redis:${REDIS_VERSION}",
        "    image: redis:${BOTHY_REDIS_VERSION}",
    ],
)
def test_a_third_party_image_without_a_default_fails(line: str) -> None:
    with pytest.raises(compose_images.ComposeError, match=r"c\.yaml:2: "):
        compose_images.images(f"services:\n{line}\n", "c.yaml")


def test_main_prints_one_image_per_line(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    compose = tmp_path / "c.yaml"
    compose.write_text(
        "  a:\n    image: b/b:${BOTHY_B_VERSION:-2}\n  c:\n    image: a/a:${BOTHY_A_VERSION:-1}\n"
    )
    assert compose_images.main([str(compose)]) == 0
    assert capsys.readouterr().out == "a/a:1\nb/b:2\n"
    bad = tmp_path / "bad.yaml"
    bad.write_text("    image: redis:7\n")
    assert compose_images.main([str(bad)]) == 1


def test_the_daily_workflow_scans_the_built_and_the_third_party_images() -> None:
    wf = yaml.safe_load((REPO / ".github/workflows/advisories.yml").read_text(encoding="utf-8"))
    on = wf[True]  # YAML 1.1 reads the key `on` as true
    assert "schedule" in on
    job = wf["jobs"]["images"]
    assert job["if"] == wf["jobs"]["scan"]["if"]
    runs = "\n".join(step.get("run", "") for step in job["steps"])
    assert "make image " in runs
    assert "make postgres-image " in runs
    assert "tools/ci/compose_images.py" in runs
    assert 'make trivy-image IMAGE="$image"' in runs
    # The scan of each image runs before the step fails, so one run lists every finding.
    assert "exit 1" in runs.split("done", 1)[1]
