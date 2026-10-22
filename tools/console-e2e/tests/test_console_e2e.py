# SPDX-License-Identifier: Apache-2.0
"""Tests of the runner of ``make console-e2e`` (brief M6-03): teardown after a failure, the
preconditions, the pins of the overlay and the CI job. The browser flow itself runs only against
the stack (``make console-e2e``)."""

from __future__ import annotations

import base64
import datetime
import hashlib
import re
import subprocess
import tomllib
from pathlib import Path

import components
import console_e2e as e2e
import pytest
import yaml
from cryptography import x509
from cryptography.hazmat.primitives import serialization

ROOT = Path(__file__).resolve().parents[3]
OVERLAY = ROOT / "infra" / "compose" / "test" / "compose.console-e2e.yaml"
CI = ROOT / ".github" / "workflows" / "ci.yml"

REPORT_LINE = (
    '{"msg":"csp violation reported","bothy.csp.directive":"%s",'
    '"bothy.csp.blocked_origin":"inline"}'
)

EXPORT = """playwright==1.64.0 \\
    --hash=sha256:aaa \\
    --hash=sha256:bbb
    # via bothy-devtools
greenlet==3.5.6 \\
    --hash=sha256:ccc
    # via playwright
pyee==14.0.0 \\
    --hash=sha256:ddd
typing-extensions==4.16.0 \\
    --hash=sha256:eee
    # via
    #   pyee
pytest==9.1.1 \\
    --hash=sha256:fff
"""


class FakeDocker:
    """Records every command; ``fail_on`` makes the compose subcommand of that name fail."""

    def __init__(self, fail_on: str | None = None, image: bool = True, leftovers: str = "") -> None:
        self.calls: list[list[str]] = []
        self.fail_on = fail_on
        self.image = image
        self.leftovers = leftovers

    def __call__(self, args, **kwargs):
        self.calls.append(list(args))
        out, rc = "", 0
        if args[:2] == ["docker", "version"]:
            out = "29.5.2"
        elif args[:3] == ["docker", "compose", "version"]:
            out = "5.6.0"
        elif args[:3] == ["docker", "image", "inspect"]:
            rc = 0 if self.image else 1
        elif args[:1] == ["uv"]:
            out = EXPORT
        elif args[:2] == ["docker", "ps"]:
            out = self.leftovers
        elif args[:2] == ["docker", "compose"]:
            sub = next(
                (
                    a
                    for a in args[2:]
                    if a in {"config", "build", "pull", "up", "exec", "run", "logs", "down", "ps"}
                ),
                "",
            )
            if sub == self.fail_on:
                rc = 1
            if sub == "logs":
                out = REPORT_LINE % "script-src-elem"
        stdout = kwargs.get("stdout")
        if hasattr(stdout, "write"):
            stdout.write("")
        return subprocess.CompletedProcess(args, rc, stdout=out, stderr="")

    def compose_subcommands(self) -> list[str]:
        subs = []
        for c in self.calls:
            if c[:2] == ["docker", "compose"] and "version" not in c:
                subs.append(
                    next(
                        a
                        for a in c[2:]
                        if a
                        in {"config", "build", "pull", "up", "exec", "run", "logs", "down", "ps"}
                    )
                )
        return subs


@pytest.fixture
def environ(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> dict[str, str]:
    monkeypatch.setattr(e2e.shutil, "which", lambda _name: "/usr/bin/docker")
    return {
        "TMPDIR": str(tmp_path),
        "PATH": "/usr/bin",
        "BOTHY_PUBLIC_URL": "https://leak.example.com",
        "E2E_SERVER_IMAGE": "img:1",
    }


def test_pass_runs_every_step_and_tears_down(environ: dict[str, str], tmp_path: Path) -> None:
    fake = FakeDocker()
    assert e2e.main([], environ, fake) == 0
    subs = fake.compose_subcommands()
    assert subs[0] == "config"
    assert subs[-1] == "down"
    assert "run" in subs
    assert "exec" in subs
    assert list(tmp_path.iterdir()) == []  # the temp directory is gone
    # The variables of an installation never reach Compose.
    for c in fake.calls:
        assert "leak.example.com" not in " ".join(c)


def test_teardown_after_a_failure(environ: dict[str, str], tmp_path: Path) -> None:
    fake = FakeDocker(fail_on="up")
    assert e2e.main([], environ, fake) == 1
    subs = fake.compose_subcommands()
    assert "down" in subs
    assert "run" not in subs  # nothing after the failed step
    assert any(c[:3] == ["docker", "image", "rm"] for c in fake.calls)
    assert list(tmp_path.iterdir()) == []


def test_failed_browsers_save_results_and_tear_down(
    environ: dict[str, str], tmp_path: Path
) -> None:
    artifacts = tmp_path / "artifacts"
    environ["E2E_ARTIFACTS"] = str(artifacts)
    fake = FakeDocker(fail_on="run")
    assert e2e.main([], environ, fake) == 1
    assert "down" in fake.compose_subcommands()
    assert {p.name for p in tmp_path.iterdir()} <= {"artifacts"}  # the temp directory is gone


def test_leftovers_fail_the_run(environ: dict[str, str]) -> None:
    assert e2e.main([], environ, FakeDocker(leftovers="bothy-console-e2e-x-api-1")) == 1


def test_missing_image_is_a_precondition(environ: dict[str, str], tmp_path: Path) -> None:
    fake = FakeDocker(image=False)
    assert e2e.main([], environ, fake) == 2
    assert fake.compose_subcommands() == []
    assert list(tmp_path.iterdir()) == []


def test_browser_requirements_takes_the_pinned_entries() -> None:
    req = e2e.browser_requirements(EXPORT)
    assert req.splitlines()[0] == "playwright==1.64.0 \\"
    assert "pytest" not in req
    assert req.count("--hash=") == 5
    with pytest.raises(e2e.PreconditionError):
        e2e.browser_requirements("playwright==1.64.0\n")


def test_report_seen() -> None:
    assert e2e.report_seen("x " + REPORT_LINE % "script-src-elem")
    assert not e2e.report_seen(REPORT_LINE % "style-src-elem")
    assert not e2e.report_seen('{"msg":"http request"}')


def test_certs_name_the_origins_and_pin_the_leaf(tmp_path: Path) -> None:
    now = datetime.datetime(2026, 10, 20, tzinfo=datetime.UTC)
    certs = e2e.make_certs(tmp_path / "c", now)
    leaf, ca = x509.load_pem_x509_certificates((certs.directory / "leaf.pem").read_bytes())
    sans = leaf.extensions.get_extension_for_class(x509.SubjectAlternativeName).value
    assert set(sans.get_values_for_type(x509.DNSName)) == {"localhost", "api"}
    assert leaf.issuer == ca.subject
    spki = leaf.public_key().public_bytes(
        serialization.Encoding.DER, serialization.PublicFormat.SubjectPublicKeyInfo
    )
    assert certs.spki_sha256 == base64.b64encode(hashlib.sha256(spki).digest()).decode()
    assert (certs.directory / "leaf.key").stat().st_mode & 0o077 == 0
    assert leaf.not_valid_after_utc - now <= datetime.timedelta(days=1)


def test_overlay_images_are_pinned_and_match_the_lock() -> None:
    overlay = OVERLAY.read_text(encoding="utf-8")
    lock = tomllib.loads((ROOT / "tools" / "uv.lock").read_text(encoding="utf-8"))
    version = next(p["version"] for p in lock["package"] if p["name"] == "playwright")
    images = re.findall(r"(?:image:|FROM) +(\S+)", overlay)
    pinned = [i for i in images if "${" not in i]
    assert pinned, "no external image in the overlay"
    for image in pinned:
        assert re.search(r"@sha256:[0-9a-f]{64}$", image), f"{image} is not pinned by digest"
    assert any(i.startswith(f"mcr.microsoft.com/playwright/python:v{version}-") for i in pinned)
    data = yaml.safe_load(overlay)
    for name in ("tls", "console-seed", "console-browser"):
        svc = data["services"][name]
        assert svc["network_mode"] == "service:api", name
        assert "ports" not in svc, name
        assert svc["read_only"] is True, name
        assert svc["cap_drop"] == ["ALL"], name


def test_ci_job_is_pinned_read_only_and_feeds_ci_ok() -> None:
    wf = yaml.safe_load(CI.read_text(encoding="utf-8"))
    job = wf["jobs"]["console-e2e"]
    assert job["permissions"] == {"contents": "read"}
    # The plan decides (tools/ci/components.py CONSOLE_E2E_JOBS); no other condition skips it.
    assert job["if"] == "contains(format(' {0} ', needs.plan.outputs.jobs), ' console-e2e ')"
    for step in job["steps"]:
        if "uses" in step:
            assert re.search(r"@[0-9a-f]{40}$", step["uses"]), step["uses"]
    assert any(step.get("run") == "make console-e2e" for step in job["steps"])
    # ci-ok needs the job; tools/ci/ci_ok.py accepts a skip only when the plan left it out.
    assert "console-e2e" in wf["jobs"]["ci-ok"]["needs"]
    assert "console-e2e" in components.CONDITIONAL_JOBS


def test_make_target_outside_gates() -> None:
    makefile = (ROOT / "Makefile").read_text(encoding="utf-8")
    assert re.search(r"^console-e2e: ##", makefile, re.M)
    for line in re.findall(r"^gates:(?! ##).*(?:\\\n.*)*", makefile, re.M):
        assert "console-e2e" not in line
