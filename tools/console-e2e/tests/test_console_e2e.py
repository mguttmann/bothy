# SPDX-License-Identifier: Apache-2.0
"""Tests of the runner of ``make console-e2e`` (briefs M6-03 and M6-04): teardown after a failure,
the preconditions, the pins of the overlays, the CI job, phase 2 (the patrol suites of the console
administration: bundle build, overlay, order) and the pure helpers of headless/admin_suites.py. The
browser flows themselves run only against the stack (``make console-e2e``)."""

from __future__ import annotations

import base64
import datetime
import hashlib
import json
import re
import subprocess
import sys
import tomllib
import uuid
from pathlib import Path

import console_e2e as e2e
import pytest
import yaml
from cryptography import x509
from cryptography.hazmat.primitives import serialization

ROOT = Path(__file__).resolve().parents[3]
OVERLAY = ROOT / "infra" / "compose" / "test" / "compose.console-e2e.yaml"
ADMIN_OVERLAY = ROOT / "infra" / "compose" / "test" / "compose.console-e2e-admin.yaml"
CI = ROOT / ".github" / "workflows" / "ci.yml"
DOCKERFILE = ROOT / "services" / "server" / "Dockerfile"
DOCKERIGNORE = ROOT / "services" / "server" / "Dockerfile.dockerignore"

# The headless modules run in the browser image with their directory on sys.path; the pure
# helpers of admin_suites are tested here the same way.
sys.path.insert(0, str(ROOT / "tools" / "console-e2e" / "headless"))
import admin_suites as admin  # noqa: E402

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
    """Records every command and its environment; ``fail_on`` makes the compose subcommand of
    that name fail, ``fail_admin`` only in phase 2 (calls with the admin overlay), ``fail_bundle``
    the build of the admin bundle."""

    def __init__(
        self,
        fail_on: str | None = None,
        image: bool = True,
        leftovers: str = "",
        fail_admin: str | None = None,
        fail_bundle: bool = False,
    ) -> None:
        self.calls: list[list[str]] = []
        self.envs: list[dict[str, str]] = []
        self.fail_on = fail_on
        self.fail_admin = fail_admin
        self.fail_bundle = fail_bundle
        self.image = image
        self.leftovers = leftovers

    def __call__(self, args, **kwargs):
        self.calls.append(list(args))
        self.envs.append(dict(kwargs.get("env") or {}))
        out, rc = "", 0
        if args[:3] == ["docker", "buildx", "build"]:
            rc = 1 if self.fail_bundle else 0
            if not self.fail_bundle:  # the local exporter writes the bundle
                dest = Path(args[args.index("--output") + 1].split("dest=", 1)[1])
                dest.mkdir(parents=True)
                (dest / "index.html").write_text("<html></html>", encoding="utf-8")
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
            if sub == self.fail_admin and str(ADMIN_OVERLAY) in args:
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
    assert "' apps '" in job["if"]
    assert "' server '" in job["if"]
    for step in job["steps"]:
        if "uses" in step:
            assert re.search(r"@[0-9a-f]{40}$", step["uses"]), step["uses"]
    assert any(step.get("run") == "make console-e2e" for step in job["steps"])
    ok = wf["jobs"]["ci-ok"]
    assert "console-e2e" in ok["needs"]
    script = next(s["run"] for s in ok["steps"] if "run" in s)
    assert 'case "$CONSOLE_E2E" in success|skipped) ;; *) fail=1 ;; esac' in script


def test_make_target_outside_gates() -> None:
    makefile = (ROOT / "Makefile").read_text(encoding="utf-8")
    assert re.search(r"^console-e2e: ##", makefile, re.M)
    for line in re.findall(r"^gates:(?! ##).*(?:\\\n.*)*", makefile, re.M):
        assert "console-e2e" not in line


# ---- phase 2: the patrol suites of the console administration (brief M6-04) ---------------


def _compose_calls(fake: FakeDocker) -> list[tuple[list[str], dict[str, str]]]:
    return [
        (c, e)
        for c, e in zip(fake.calls, fake.envs, strict=True)
        if c[:2] == ["docker", "compose"] and "version" not in c
    ]


def test_bundle_build_command_line(tmp_path: Path) -> None:
    assert e2e.bundle_build_args(tmp_path, "1.2.3") == [
        "docker",
        "buildx",
        "build",
        "--quiet",
        "--file",
        str(ROOT / "services" / "server" / "Dockerfile"),
        "--target",
        "console-e2e-web",
        "--build-arg",
        "VERSION=1.2.3",
        "--output",
        f"type=local,dest={tmp_path / 'admin-bundle'}",
        str(ROOT),
    ]


def test_bundle_is_built_in_the_build_step(environ: dict[str, str]) -> None:
    fake = FakeDocker()
    assert e2e.main([], environ, fake) == 0
    builds = [i for i, c in enumerate(fake.calls) if c[:3] == ["docker", "buildx", "build"]]
    assert len(builds) == 1
    first_up = next(
        i for i, c in enumerate(fake.calls) if c[:2] == ["docker", "compose"] and "up" in c
    )
    assert builds[0] < first_up
    version = (ROOT / "services" / "server" / "VERSION").read_text(encoding="utf-8").strip()
    assert f"VERSION={version}" in fake.calls[builds[0]]


def test_phase_two_adds_the_admin_overlay_after_phase_one(environ: dict[str, str]) -> None:
    fake = FakeDocker()
    assert e2e.main([], environ, fake) == 0
    calls = _compose_calls(fake)
    with_overlay = [i for i, (c, _) in enumerate(calls) if str(ADMIN_OVERLAY) in c]
    # The report check of phase 1 (logs api) comes before the first call of phase 2.
    report = next(i for i, (c, _) in enumerate(calls) if c[-2:] == ["--no-color", "api"])
    assert with_overlay
    assert min(with_overlay) > report
    # Phase 2 keeps every file of phase 1 and appends the overlay last.
    first = calls[with_overlay[0]][0]
    files = [first[i + 1] for i, a in enumerate(first) if a == "--file"]
    assert files == [str(f) for f in e2e.Stack(project="x", env={}).files] + [str(ADMIN_OVERLAY)]
    phase2 = [c for c, _ in calls if str(ADMIN_OVERLAY) in c]
    ups = [c for c in phase2 if "up" in c]
    # api first (with --wait), then the sidecar in its network namespace; both recreated.
    assert ups[0][-1] == "api"
    assert "--wait" in ups[0]
    assert "--force-recreate" in ups[0]
    assert ups[1][-1] == "tls"
    assert "--force-recreate" in ups[1]
    assert phase2.index(ups[0]) < phase2.index(ups[1])
    runs = [(c, e) for c, e in calls if str(ADMIN_OVERLAY) in c and "run" in c]
    assert len(runs) == 1
    assert runs[0][0][-1] == "console-browser"
    assert runs[0][1]["E2E_MODE"] == "admin"
    assert phase2.index(ups[1]) < phase2.index(runs[0][0])
    # Phase 1 never sees E2E_MODE=admin.
    for c, env in calls:
        if str(ADMIN_OVERLAY) not in c and "run" in c:
            assert env.get("E2E_MODE") != "admin"
    assert "down" in calls[-1][0]


def test_phase_one_failure_skips_phase_two(environ: dict[str, str]) -> None:
    fake = FakeDocker(fail_on="run")
    assert e2e.main([], environ, fake) == 1
    assert not any(str(ADMIN_OVERLAY) in c and "up" in c for c in fake.calls)


@pytest.mark.parametrize("sub", ["up", "run"])
def test_teardown_after_a_phase_two_failure(
    environ: dict[str, str], tmp_path: Path, sub: str
) -> None:
    fake = FakeDocker(fail_admin=sub)
    assert e2e.main([], environ, fake) == 1
    assert "down" in fake.compose_subcommands()
    assert any(c[:3] == ["docker", "image", "rm"] for c in fake.calls)
    assert list(tmp_path.iterdir()) == []  # the bundle lives in the temp directory, gone too


def test_bundle_build_failure_fails_and_tears_down(environ: dict[str, str], tmp_path: Path) -> None:
    fake = FakeDocker(fail_bundle=True)
    assert e2e.main([], environ, fake) == 1
    subs = fake.compose_subcommands()
    assert "up" not in subs
    assert "down" in subs
    assert list(tmp_path.iterdir()) == []


def test_stack_env_names_the_bundle_inside_the_temp_directory(tmp_path: Path) -> None:
    env = e2e.stack_env({}, tmp_path, "img:1", "abcd")
    assert env["E2E_ADMIN_BUNDLE"] == str(tmp_path / "admin-bundle")
    assert "E2E_MODE" not in env


def test_share_bundle_makes_the_files_readable(tmp_path: Path) -> None:
    bundle = tmp_path / "admin-bundle"
    (bundle / "assets").mkdir(parents=True)
    (bundle / "index.html").write_text("x", encoding="utf-8")
    (bundle / "index.html").chmod(0o600)
    (bundle / "assets").chmod(0o700)
    e2e.share_bundle(bundle)
    assert (bundle / "index.html").stat().st_mode & 0o777 == 0o644
    assert (bundle / "assets").stat().st_mode & 0o777 == 0o755
    assert bundle.stat().st_mode & 0o777 == 0o755


def test_share_bundle_requires_an_index(tmp_path: Path) -> None:
    (tmp_path / "admin-bundle").mkdir()
    with pytest.raises(e2e.StepError):
        e2e.share_bundle(tmp_path / "admin-bundle")


def test_admin_overlay_pins() -> None:
    text = ADMIN_OVERLAY.read_text(encoding="utf-8")
    assert "TEST ONLY" in text
    data = yaml.safe_load(text)
    assert set(data["services"]) == {"api", "console-browser"}
    for name, svc in data["services"].items():
        assert "ports" not in svc, name
    api = data["services"]["api"]
    assert api["environment"] == {
        "BOTHY_CONSOLE__ASSETS_DIR": "/usr/share/bothy/console-e2e",
        "BOTHY_AUTH__STEP_UP_MAX_AGE": "3s",
    }
    assert api["volumes"] == [
        "${E2E_ADMIN_BUNDLE:?set by console_e2e.py}:/usr/share/bothy/console-e2e:ro"
    ]
    assert data["services"]["console-browser"]["environment"] == {"E2E_MODE": "${E2E_MODE:-}"}


def test_admin_overlay_sets_only_existing_configuration_keys() -> None:
    data = yaml.safe_load(ADMIN_OVERLAY.read_text(encoding="utf-8"))
    keys = ROOT / "services" / "server" / "internal" / "wire"
    names = "".join(p.read_text(encoding="utf-8") for p in keys.glob("configkeys_*.go"))
    for var in data["services"]["api"]["environment"]:
        key = var.removeprefix("BOTHY_").lower().replace("__", ".")
        assert f'Name: "{key}"' in names, key


def _stages(text: str) -> list[tuple[str, str]]:
    return re.findall(r"^FROM (?:--platform=\S+ )?(\S+)(?: AS (\S+))?$", text, re.M)


def test_dockerfile_console_e2e_stages() -> None:
    text = DOCKERFILE.read_text(encoding="utf-8")
    stages = _stages(text)
    names = [alias for _, alias in stages]
    assert ("console", "console-e2e-build") in stages
    assert ("scratch", "console-e2e-web") in stages
    assert (
        names.index("console") < names.index("console-e2e-build") < names.index("console-e2e-web")
    )
    assert stages[-1][1] == ""  # the production stage stays last
    console = text.split(" AS console\n", 1)[1].split("\nFROM ", 1)[0]
    assert "COPY packages/admin_ui/lib packages/admin_ui/lib" in console
    build = text.split(" AS console-e2e-build\n", 1)[1].split("\nFROM ", 1)[0]
    flags = "--base-href /console/ --no-web-resources-cdn --no-pub --no-wasm-dry-run"
    assert flags in console
    assert flags in build
    assert "--target integration_test/admin/admin_bundle.dart" in build
    assert "tool/version_json.sh" in build
    assert "COPY" not in build  # the context already holds the suites
    web = text.split(" AS console-e2e-web\n", 1)[1].split("\nFROM ", 1)[0]
    assert re.findall(r"^COPY .*$", web, re.M) == [
        "COPY --from=console-e2e-build /src/apps/console/build/admin-bundle /"
    ]
    production = text.rsplit("\nFROM ", 1)[1]
    assert "admin-bundle" not in production
    assert "console-e2e" not in production


def test_dockerignore_admits_admin_ui_and_the_suites_but_no_unit_tests() -> None:
    lines = DOCKERIGNORE.read_text(encoding="utf-8").splitlines()
    assert "!packages/admin_ui/lib/" in lines
    assert "!apps/console/" in lines
    assert "apps/console/test/" in lines
    assert not any("integration_test" in line for line in lines if not line.startswith("#"))


# ---- pure helpers of headless/admin_suites.py ---------------------------------------------


def test_flatten_tests_names_like_patrol_and_skips_the_explorer() -> None:
    tree = {
        "type": "group",
        "name": "",
        "skip": False,
        "entries": [
            {"type": "test", "name": "patrol_test_explorer", "entries": [], "skip": False},
            {
                "type": "group",
                "name": "a1_people_test",
                "skip": False,
                "entries": [
                    {
                        "type": "test",
                        "name": "A1 create a person and approve a waiting device",
                        "entries": [],
                        "skip": False,
                    }
                ],
            },
            {
                "type": "group",
                "name": "a2",
                "skip": True,
                "entries": [
                    {
                        "type": "group",
                        "name": "inner",
                        "skip": False,
                        "entries": [{"type": "test", "name": "t", "entries": [], "skip": False}],
                    }
                ],
            },
        ],
    }
    assert admin.flatten_tests(tree) == [
        admin.PatrolTest("a1_people_test A1 create a person and approve a waiting device", False),
        admin.PatrolTest("a2 inner t", True),
    ]


def test_proof_claims_follow_the_rules_of_the_server() -> None:
    claims = admin.proof_claims(
        "POST", "https://localhost:8443/api/v1/me", nonce="", access_token="", now=7, jti="j"
    )
    assert claims == {
        "jti": "j",
        "htm": "POST",
        "htu": "https://localhost:8443/api/v1/me",
        "iat": 7,
    }
    with_token = admin.proof_claims(
        "GET", "https://localhost:8443/api/v1/me", nonce="n1", access_token="tok", now=7, jti="j"
    )
    assert with_token["nonce"] == "n1"
    want = base64.urlsafe_b64encode(hashlib.sha256(b"tok").digest()).rstrip(b"=").decode()
    assert with_token["ath"] == want
    assert "=" not in with_token["ath"]


def test_request_htu_drops_the_query_and_keeps_the_public_origin() -> None:
    assert (
        admin.request_htu("https://localhost:8443/", "/api/v1/devices/x?a=1#f")
        == "https://localhost:8443/api/v1/devices/x"
    )
    with pytest.raises(ValueError, match="absolute"):
        admin.request_htu("https://localhost:8443", "https://other.example.com/api/v1/me")
    with pytest.raises(ValueError, match="absolute"):
        admin.request_htu("https://localhost:8443", "api/v1/me")


def test_proof_header_carries_only_the_public_key() -> None:
    jwk = {"kty": "EC", "crv": "P-256", "x": "X", "y": "Y", "d": "secret", "ext": True}
    assert admin.proof_header(jwk) == {
        "typ": "dpop+jwt",
        "alg": "ES256",
        "jwk": {"kty": "EC", "crv": "P-256", "x": "X", "y": "Y"},
    }


def test_uuid7_has_version_and_variant() -> None:
    u = uuid.UUID(admin.uuid7(now_ms=0x0123456789AB))
    assert u.version == 7
    assert u.variant == uuid.RFC_4122
    assert u.hex.startswith("0123456789ab")
    assert admin.uuid7() != admin.uuid7()


def test_totp_secret_reads_the_otpauth_uri() -> None:
    uri = "otpauth://totp/Bothy:sam?secret=jbswy3dpehpk3pxp%3D%3D&issuer=Bothy"
    assert admin.totp_secret(uri) == "JBSWY3DPEHPK3PXP"
    with pytest.raises(admin.CheckError):
        admin.totp_secret("otpauth://totp/Bothy:sam?issuer=Bothy")


def _seed() -> dict[str, object]:
    obj = {"id": "00000000-0000-7000-8000-000000000000", "name": "n"}
    return {
        "owner": dict(obj, name="Alex"),
        "sam": dict(obj, name="Sam"),
        "pendingDevice": dict(obj, name="Sam laptop", code="123456"),
        "spaces": {k: dict(obj) for k in ("support", "marketing", "engineering")},
        "channels": {
            k: dict(obj)
            for k in (
                "supportGeneral",
                "supportRequests",
                "marketingCampaigns",
                "marketingInternal",
                "engineeringBuilds",
            )
        },
        "agents": {"supportAgent": dict(obj), "texter": dict(obj)},
        "group": dict(obj, name="Support Team"),
    }


def test_check_seed_accepts_the_shape_the_dart_tests_read() -> None:
    seed = _seed()
    admin.check_seed(seed)
    assert json.loads(json.dumps(seed)) == seed


@pytest.mark.parametrize(
    ("path", "value"),
    [
        (("extra",), {"id": "x", "name": "y"}),
        (("spaces", "support"), {"id": "x"}),
        (("channels", "supportGeneral"), {"id": "", "name": "general"}),
        (("pendingDevice", "code"), "12a4"),
        (("agents", "other"), {"id": "x", "name": "y"}),
    ],
)
def test_check_seed_rejects_a_wrong_shape(path: tuple[str, ...], value: object) -> None:
    seed = _seed()
    target = seed
    for key in path[:-1]:
        target = target[key]  # type: ignore[assignment]
    target[path[-1]] = value  # type: ignore[index]
    with pytest.raises(admin.CheckError):
        admin.check_seed(seed)


def test_named_takes_the_display_name_where_asked() -> None:
    assert admin.named({"id": 1, "name": "Support"}) == {"id": "1", "name": "Support"}
    assert admin.named({"id": "a", "display_name": "Sam"}, "display_name") == {
        "id": "a",
        "name": "Sam",
    }


def test_csv_export_seen() -> None:
    rec = admin.DownloadRecord
    assert admin.csv_export_seen([rec("audit.csv", "occurred_at,actor,action")])
    assert admin.csv_export_seen([rec("x.txt", ""), rec("Audit.CSV", "id,occurred_at")])
    assert not admin.csv_export_seen([rec("audit.csv", "id,actor")])
    assert not admin.csv_export_seen([rec("audit.json", "occurred_at")])
    assert not admin.csv_export_seen([])


def test_problem_code() -> None:
    assert admin.problem_code({"json": {"code": "step_up_required"}}) == "step_up_required"
    assert admin.problem_code({"json": None}) == ""
    assert admin.problem_code({"json": ["x"]}) == ""


class _FakeContext:
    def __init__(self) -> None:
        self.granted: list[tuple[list[str], str]] = []

    def grant_permissions(self, permissions: list[str], origin: str) -> None:
        self.granted.append((permissions, origin))


class _FakePage:
    def __init__(self) -> None:
        self.context = _FakeContext()
        self.evaluated: list[tuple[str, object]] = []
        self.media: list[str] = []

    def evaluate(self, script: str, arg: object = None) -> object:
        self.evaluated.append((script, arg))
        return "clip"

    def emulate_media(self, color_scheme: str) -> None:
        self.media.append(color_scheme)


def test_platform_handler_answers_lifecycle_and_rejects_unknown_actions() -> None:
    page = _FakePage()
    lines: list[str] = []
    handler = admin.PlatformHandler(page, lines.append)  # type: ignore[arg-type]
    handler.downloads.append("old.csv")
    assert handler(None, {"action": "startTest", "params": {}}) is None
    assert handler.downloads == []
    handler.downloads.append("audit.csv")
    assert handler(None, {"action": "verifyFileDownloads", "params": {}}) == ["audit.csv"]
    assert handler(None, {"action": "getClipboard", "params": {}}) == "clip"
    assert handler(None, {"action": "setClipboard", "params": {"text": "t"}}) is None
    assert page.evaluated[-1][1] == "t"
    assert handler(None, {"action": "grantPermissions", "params": {"permissions": ["a"]}}) is None
    assert page.context.granted == [(["a"], admin.ORIGIN)]
    assert handler(None, {"action": "enableDarkMode", "params": {}}) is None
    assert page.media == ["dark"]
    with pytest.raises(admin.CheckError, match="tap"):
        handler(None, {"action": "tap", "params": {}})
    assert handler.unknown == ["tap"]
    assert lines[0] == "platform action startTest"


def test_judge_fails_on_every_kind_of_failure() -> None:
    ok = [{"name": "a", "result": "success"}]
    csv = [admin.DownloadRecord("audit.csv", "occurred_at,action")]
    admin.judge(ok, [], csv)
    with pytest.raises(admin.CheckError, match="no patrol test"):
        admin.judge([], [], csv)
    with pytest.raises(admin.CheckError, match="tap"):
        admin.judge(ok, ["tap"], csv)
    with pytest.raises(admin.CheckError, match="b"):
        admin.judge([*ok, {"name": "b", "result": "skipped"}], [], csv)
    with pytest.raises(admin.CheckError, match="CSV"):
        admin.judge(ok, [], [])


def test_problem_text_names_code_and_reason_only() -> None:
    answer = {"json": {"code": "conflict", "reason": "billing_mode_not_personal", "detail": "x"}}
    assert admin.problem_text(answer) == "conflict billing_mode_not_personal"
    assert admin.problem_text({"json": {"code": "forbidden"}}) == "forbidden"
    assert admin.problem_text({"json": None}) == ""
