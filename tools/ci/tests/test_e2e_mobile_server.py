# SPDX-License-Identifier: Apache-2.0
"""The server side of the mobile device E2E test (brief M5-11 Design note 1).

The Dockerfile tests hold that only the stage of the E2E server build passes the Go build tag
`e2e`, that the targets `e2e-server` and `fakepush` reuse the pinned runtime base and the layout of
the production stage, and that the production stage stays last, so a build without `--target` and
every release image are unchanged. The overlay tests hold the Compose overlay
`infra/compose/test/compose.e2e-mobile.yaml` to the contract of the parts: the E2E images with
build sections, the fake on the Compose network with only its capture port published on
127.0.0.1, the hardening of the other overlay services, and the push configuration of api and
worker.
"""

from __future__ import annotations

import re
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

import yaml

REPO = Path(__file__).resolve().parents[3]
DOCKERFILE = REPO / "services" / "server" / "Dockerfile"
DOCKERIGNORE = REPO / "services" / "server" / "Dockerfile.dockerignore"
OVERLAY = REPO / "infra" / "compose" / "test" / "compose.e2e-mobile.yaml"
README = REPO / "infra" / "compose" / "test" / "README.md"

# ---- Dockerfile ---------------------------------------------------------------------------------


@dataclass
class Stage:
    """One stage of a Dockerfile: its base, its name (empty without AS) and its instructions."""

    base: str
    name: str
    lines: list[str] = field(default_factory=list)


def _instructions(text: str) -> list[str]:
    """Return the instructions of a Dockerfile with continuation lines joined, without comments."""
    out: list[str] = []
    current = ""
    for raw in text.splitlines():
        line = raw.strip()
        if not current and (not line or line.startswith("#")):
            continue
        if line.startswith("#"):
            continue
        if line.endswith("\\"):
            current += line[:-1] + " "
            continue
        out.append(current + line)
        current = ""
    if current:
        out.append(current)
    return out


def stages(text: str) -> list[Stage]:
    """Split a Dockerfile into its stages in order."""
    result: list[Stage] = []
    for ins in _instructions(text):
        m = re.match(r"(?i)^FROM\s+(?:--\S+\s+)*(\S+)(?:\s+AS\s+(\S+))?\s*$", ins)
        if m:
            result.append(Stage(base=m.group(1), name=(m.group(2) or "").lower()))
        elif result:
            result[-1].lines.append(ins)
    return result


def _go_builds(stage: Stage) -> list[str]:
    return [ins for ins in stage.lines if re.search(r"\bgo build\b", ins)]


def _by_name(all_stages: list[Stage], name: str) -> Stage:
    found = [s for s in all_stages if s.name == name]
    assert len(found) == 1, f"stage {name} is missing or repeated"
    return found[0]


def test_stages_splits_names_and_bases() -> None:
    text = (
        "# c\nFROM --platform=$BUILDPLATFORM golang:1@sha256:x AS Build\nRUN go build \\\n"
        "  -tags e2e ./x\nFROM build AS e2e\nFROM scratch\nCOPY a b\n"
    )
    got = stages(text)
    assert [(s.base, s.name) for s in got] == [
        ("golang:1@sha256:x", "build"),
        ("build", "e2e"),
        ("scratch", ""),
    ]
    assert got[0].lines == ["RUN go build  -tags e2e ./x"]
    assert got[2].lines == ["COPY a b"]


def test_only_the_e2e_build_stage_passes_the_build_tag_e2e() -> None:
    all_stages = stages(DOCKERFILE.read_text())
    tagged = [s.name for s in all_stages for b in _go_builds(s) if re.search(r"-tags[= ]", b)]
    assert tagged == ["e2e-build"]
    (build,) = _go_builds(_by_name(all_stages, "e2e-build"))
    assert re.search(r"-tags[= ]e2e\b", build)
    assert build.rstrip().endswith("./cmd/bothy")
    assert "e2e" not in " ".join(_go_builds(_by_name(all_stages, "build")))
    # No other line of the file names the tag, for example through GOFLAGS.
    assert len(re.findall(r"-tags[= ]e2e\b", DOCKERFILE.read_text())) == 1


def test_production_stage_stays_last_and_unchanged() -> None:
    all_stages = stages(DOCKERFILE.read_text())
    last = all_stages[-1]
    assert last.name == ""
    assert last.base.startswith("gcr.io/distroless/static-debian13:nonroot@sha256:")
    copy_release = "COPY --from=build --chown=0:0 --chmod=0555 /out/bothy /usr/local/bin/bothy"
    assert copy_release in last.lines
    assert not any("e2e" in ins or "fakepush" in ins for ins in last.lines)
    names = [s.name for s in all_stages]
    assert names.index("e2e-server") < len(names) - 1
    assert names.index("fakepush") < len(names) - 1


def test_e2e_server_has_the_layout_of_the_production_stage() -> None:
    all_stages = stages(DOCKERFILE.read_text())
    prod, e2e = all_stages[-1], _by_name(all_stages, "e2e-server")
    assert e2e.base == prod.base
    copy_e2e = "COPY --from=e2e-build --chown=0:0 --chmod=0555 /out/bothy-e2e /usr/local/bin/bothy"
    assert copy_e2e in e2e.lines
    for ins in (
        "COPY --from=build /out/lib/ /var/lib/bothy/",
        "USER 65532:65532",
        'ENTRYPOINT ["/usr/local/bin/bothy"]',
        'CMD ["serve"]',
    ):
        assert ins in prod.lines, ins
        assert ins in e2e.lines, ins
    prod_check = [i for i in prod.lines if i.startswith("HEALTHCHECK")]
    assert prod_check
    assert [i for i in e2e.lines if i.startswith("HEALTHCHECK")] == prod_check


def test_fakepush_runs_as_nonroot_with_its_volume_mount_point() -> None:
    all_stages = stages(DOCKERFILE.read_text())
    build, image = _by_name(all_stages, "fakepush-build"), _by_name(all_stages, "fakepush")
    assert build.base == "build"
    assert image.base == all_stages[-1].base
    assert "COPY services/server/test/fakepush ./test/fakepush" in build.lines
    (go,) = _go_builds(build)
    assert go.rstrip().endswith("./test/fakepush/cmd/fakepush")
    prep = " ".join(build.lines)
    assert "chmod 0700 /out/fakepush-root/push" in prep
    assert "chown 65532:65532 /out/fakepush-root/push" in prep
    assert "COPY --from=fakepush-build /out/fakepush-root/ /" in image.lines
    assert "USER 65532:65532" in image.lines
    assert 'ENTRYPOINT ["/usr/local/bin/fakepush"]' in image.lines


def test_build_context_lets_the_fake_in_without_its_tests() -> None:
    lines = DOCKERIGNORE.read_text().splitlines()
    assert "!services/server/test/fakepush/" in lines
    assert "**/*_test.go" in lines


# ---- Compose overlay ----------------------------------------------------------------------------


class ComposeLoader(yaml.SafeLoader):
    """SafeLoader that accepts the Compose tags `!reset` and `!override` as their plain value."""


def _plain(loader: yaml.SafeLoader, node: yaml.Node) -> Any:
    if isinstance(node, yaml.MappingNode):
        return loader.construct_mapping(node, deep=True)
    if isinstance(node, yaml.SequenceNode):
        return loader.construct_sequence(node, deep=True)
    return loader.construct_scalar(node)  # type: ignore[arg-type]


ComposeLoader.add_constructor("!reset", _plain)
ComposeLoader.add_constructor("!override", _plain)


def _overlay() -> dict[str, Any]:
    data = yaml.load(OVERLAY.read_text(), Loader=ComposeLoader)  # noqa: S506
    assert isinstance(data, dict)
    return data


def test_overlay_header_marks_it_test_only() -> None:
    head = OVERLAY.read_text().split("\n\n", 1)[0]
    assert head.startswith("# SPDX-License-Identifier: Apache-2.0")
    assert "TEST ONLY" in head
    assert "M5-11" in head


def test_api_and_worker_run_the_e2e_server_image() -> None:
    services = _overlay()["services"]
    for name in ("api", "worker"):
        svc = services[name]
        assert svc["image"] == "bothy-e2e-server:${SMOKE_RUN_ID:?set by e2e.sh}", name
        assert svc["build"]["dockerfile"] == "services/server/Dockerfile"
        assert svc["build"]["target"] == "e2e-server"
        assert svc["build"]["context"] == "../.."
        assert svc["depends_on"]["fakepush"]["condition"] == "service_healthy"
        mounts = [v for v in svc["volumes"] if isinstance(v, str) and v.startswith("fakepush-run:")]
        assert mounts == ["fakepush-run:/run/bothy-e2e/push:ro"], name


def test_api_and_worker_configure_push_to_the_fake() -> None:
    services = _overlay()["services"]
    for name in ("api", "worker"):
        env = services[name]["environment"]
        assert env["E2E_PUSH_APNS_URL"] == "https://fakepush:8443"
        assert env["E2E_PUSH_FCM_URL"] == "https://fakepush:8444"
        assert env["SSL_CERT_FILE"] == "/run/bothy-e2e/push/ca.pem"
        assert env["BOTHY_PUSH__APNS__PRIVATE_KEY_REF"] == "file:/run/bothy-e2e/push/apns_key.p8"
        assert (
            env["BOTHY_PUSH__FCM__SERVICE_ACCOUNT_REF"]
            == "file:/run/bothy-e2e/push/fcm_service_account.json"
        )
        assert env["BOTHY_PUSH__APNS__TOPICS"] == "com.example.chat"
        assert env["BOTHY_NOTIFY__DEFAULT_PREVIEW_MODE"] == "full"
        rules = env["BOTHY_EGRESS__ALLOWED_HOSTS"].split(",")
        assert "fakepush:8443;allow_private" in rules
        assert "fakepush:8444;allow_private" in rules
        assert env["BOTHY_NOTIFY__COALESCE_WINDOW"].endswith("s")


def test_fakepush_publishes_only_its_capture_port_on_loopback() -> None:
    overlay = _overlay()
    fake = overlay["services"]["fakepush"]
    assert fake["image"] == "bothy-fakepush:${SMOKE_RUN_ID:?set by e2e.sh}"
    assert fake["build"]["target"] == "fakepush"
    assert fake["ports"] == [
        "127.0.0.1:${MOBILE_E2E_FAKEPUSH_PORT:?set by tools/e2e/mobile_e2e.py}:8090"
    ]
    assert "network_mode" not in fake
    assert fake["read_only"] is True
    assert fake["cap_drop"] == ["ALL"]
    assert fake["security_opt"] == ["no-new-privileges:true"]
    assert fake["healthcheck"]["test"] == ["CMD", "/usr/local/bin/fakepush", "health"]
    assert fake["volumes"] == ["fakepush-run:/push"]
    assert "fakepush-run" in overlay["volumes"]
    # No other service of the overlay publishes a port.
    published = [n for n, s in overlay["services"].items() if "ports" in s]
    assert published == ["fakepush"]


def test_overlay_keys_exist_in_the_configuration_reference() -> None:
    reference = (REPO / "docs" / "operations" / "configuration.md").read_text(encoding="utf-8")
    services = _overlay()["services"]
    env = services["api"]["environment"]
    assert env == services["worker"]["environment"]
    server_keys = [k for k in env if k.startswith("BOTHY_")]
    for key in server_keys:
        assert f"`{key}`" in reference, key
    # Besides server keys only the two variables of the E2E server build and the trust store.
    assert sorted(set(env) - set(server_keys)) == [
        "E2E_PUSH_APNS_URL",
        "E2E_PUSH_FCM_URL",
        "SSL_CERT_FILE",
    ]


def test_readme_lists_the_overlay() -> None:
    assert "| `compose.e2e-mobile.yaml` |" in README.read_text()
