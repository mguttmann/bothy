# SPDX-License-Identifier: Apache-2.0
"""Tests of the OpenTofu module generic-vm (brief M7-15): the rendered host setup per edge_mode.

The module is rendered with the pinned `tofu` through the test root
infra/tofu/test/generic-vm (`tofu plan` and `tofu show -json`), so the assertions read the real
user data and the real plan, not a copy of the templates. The local VM test that applies the
user data is `make tofu-vm-test` (tools/ci/tofu_vm_test.py).
"""

from __future__ import annotations

import json
import os
import re
import subprocess
from pathlib import Path
from typing import Any

import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]
ROOT = REPO / "infra" / "tofu" / "test" / "generic-vm"
CATALOG = REPO / "services/server/internal/platform/secrets/stack/catalog.go"
LOCK = REPO / "tools" / "bin" / "tools.lock.json"
SETUP = "/usr/local/lib/bothy/bothy-setup"
CONFIG = "/etc/bothy/setup.env"
OPTIONS = "/etc/bothy/install-options"
FIREWALL = "/etc/bothy/firewall.nft"
RELEASE_KEY = "/etc/bothy/release.pub"
# A syntactically valid public key; only its presence in the user data is checked.
PUBLIC_KEY = (
    "-----BEGIN PUBLIC KEY-----\n"
    "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEexampleexampleexampleexampleexam\n"
    "pleexampleexampleexampleexampleexampleexampleexampleexample==\n"
    "-----END PUBLIC KEY-----\n"
)
# Patterns of key material that must never be part of the user data or the plan.
KEY_MATERIAL = [
    re.compile(r"-----BEGIN [A-Z ]*PRIVATE KEY-----"),
    re.compile(r"tskey-[A-Za-z0-9-]+"),
    re.compile(r"AKIA[0-9A-Z]{16}"),
]

BASE_VARS = {
    "bothy_version": "1.2.3",
    "image_registry": "ghcr.io/alex",
    "public_hostname": "chat.example.com",
    "release_repository": "alex/bothy",
    "owner_handle": "alex",
    "owner_name": "Alex Example",
    "ssh_host": "192.0.2.10",
}


@pytest.fixture(scope="module")
def tofu() -> str:
    """Path of the pinned tofu binary (tools/bin/tools.lock.json)."""
    import fetch_tools

    lock = fetch_tools.load_lock(fetch_tools.DEFAULT_LOCK)
    return str(
        fetch_tools.ensure(lock, "tofu", fetch_tools.platform_key(), fetch_tools.DEFAULT_CACHE)
    )


@pytest.fixture(scope="module")
def data_dir(tofu: str, tmp_path_factory: pytest.TempPathFactory) -> Path:
    """An initialised data directory of the test root; the only provider source is an empty
    local mirror, as in `make tofu-check`, so init never downloads."""
    data = tmp_path_factory.mktemp("tofu-data")
    (data / "mirror").mkdir()
    (data / "offline.tfrc").write_text(
        f'provider_installation {{\n  filesystem_mirror {{\n    path = "{data / "mirror"}"\n'
        "  }\n}\n",
        encoding="utf-8",
    )
    proc = _tofu(tofu, data, "init", "-backend=false", "-input=false")
    assert proc.returncode == 0, proc.stdout + proc.stderr
    return data


def _tofu(tofu: str, data: Path, *args: str) -> subprocess.CompletedProcess[str]:
    env = {
        **os.environ,
        "TF_DATA_DIR": str(data),
        "TF_IN_AUTOMATION": "1",
        "TF_CLI_CONFIG_FILE": str(data / "offline.tfrc"),
    }
    env.pop("TF_CLI_ARGS", None)
    return subprocess.run(
        [tofu, f"-chdir={ROOT}", *args, "-no-color"],
        env=env,
        capture_output=True,
        text=True,
        check=False,
    )


def _var_args(values: dict[str, Any]) -> list[str]:
    args = []
    for key, value in values.items():
        text = value if isinstance(value, str) else json.dumps(value)
        args += ["-var", f"{key}={text}"]
    return args


def plan(tofu: str, data: Path, tmp_path: Path, **extra: Any) -> tuple[dict[str, Any], str]:
    """Plan the test root; return the planned outputs and the JSON text of the plan."""
    out = tmp_path / "plan.bin"
    proc = _tofu(
        tofu, data, "plan", "-input=false", "-lock=false", f"-out={out}",
        *_var_args({**BASE_VARS, **extra}),
    )  # fmt: skip
    assert proc.returncode == 0, proc.stdout + proc.stderr
    shown = _tofu(tofu, data, "show", "-json", str(out))
    assert shown.returncode == 0, shown.stderr
    doc = json.loads(shown.stdout)
    outputs = {k: v.get("value") for k, v in doc["planned_values"]["outputs"].items()}
    return outputs, shown.stdout


def plan_error(tofu: str, data: Path, **extra: Any) -> str:
    """Plan with values the module must refuse; return the error text."""
    proc = _tofu(
        tofu, data, "plan", "-input=false", "-lock=false", *_var_args({**BASE_VARS, **extra})
    )
    assert proc.returncode != 0, "the plan must fail"
    return proc.stderr


def files(user_data: str) -> dict[str, str]:
    """The write_files of the cloud-config, path to content."""
    assert user_data.startswith("#cloud-config\n")
    doc = yaml.safe_load(user_data)
    return {f["path"]: f["content"] for f in doc["write_files"]}


def config(text: str) -> dict[str, str]:
    """KEY='value' lines of setup.env."""
    pairs = re.findall(r"^([A-Z0-9_]+)='([^'\n]*)'$", text, re.M)
    assert len(pairs) == len([line for line in text.splitlines() if line and line[0] != "#"])
    return dict(pairs)


def catalogue_names() -> list[str]:
    """Every secret file name of the stack catalogue (the code, not a copy).

    Directory entries (`offsite/`, `pgbackrest_tls/`) name folders, not secrets; of a pattern
    entry (`oidc/<provider>_client_secret`) the fixed part after the placeholder counts.
    """
    names = re.findall(r'\{Name: "([^"]+)"', CATALOG.read_text(encoding="utf-8"))
    assert len(names) > 10, "the catalogue moved; update CATALOG"
    return [n.rsplit(">", 1)[-1] for n in names if not n.endswith("/")]


@pytest.mark.parametrize(
    ("edge_mode", "edge", "public_port"),
    [("reverse_proxy", "direct", "443"), ("tailscale", "tailscale", ""), ("private", "direct", "")],
)
def test_user_data_per_edge_mode(
    tofu: str, data_dir: Path, tmp_path: Path, edge_mode: str, edge: str, public_port: str
) -> None:
    outputs, plan_json = plan(tofu, data_dir, tmp_path, edge_mode=edge_mode)
    rendered = files(outputs["user_data"])
    assert set(rendered) == {SETUP, CONFIG, OPTIONS, FIREWALL}
    cfg = config(rendered[CONFIG])
    assert cfg["SETUP_EDGE"] == edge
    assert cfg["SETUP_EDGE_PORT"] == public_port
    assert cfg["SETUP_BASE_URL"] == "https://github.com/alex/bothy/releases/download/v1.2.3/"
    assert cfg["SETUP_SIGNING"] == "keyless"
    assert cfg["SETUP_IDENTITY_REGEXP"] == (
        r"^https://github\.com/alex/bothy/\.github/workflows/release\.yml@refs/tags/v"
    )
    # Options of install.sh (16 3.1): one per line, derived from the inputs only.
    assert rendered[OPTIONS].splitlines() == [
        "--yes",
        "--public-url",
        "https://chat.example.com",
        "--edge",
        edge,
        "--storage",
        "fs",
        "--observability",
        "off",
        "--owner-handle",
        "alex",
        "--owner-name",
        "Alex Example",
    ]
    # Only the edge port is public (infra/tofu/README.md rule 3); SSH stays for maintenance.
    ports = re.findall(r"tcp dport (\d+) accept", rendered[FIREWALL])
    assert ports == (["22", "443"] if public_port else ["22"])
    assert "policy drop" in rendered[FIREWALL]
    for text in (outputs["user_data"], plan_json):
        for name in catalogue_names():
            assert name not in text, f"secret name {name} in the rendered output"
        for pattern in KEY_MATERIAL:
            assert not pattern.search(text), pattern.pattern
    assert outputs["public_url"] == "https://chat.example.com"
    assert outputs["ssh_endpoint"] == "bothy@192.0.2.10"
    assert outputs["database_endpoint"] == ""
    assert outputs["object_storage_bucket"] == ""
    assert outputs["backup_offsite_location"] == ""
    assert outputs["secret_store_ref"] == ""


def test_setup_verifies_before_install_sh_unpacks(
    tofu: str, data_dir: Path, tmp_path: Path
) -> None:
    outputs, _ = plan(tofu, data_dir, tmp_path)
    script = files(outputs["user_data"])[SETUP]
    main = script[script.index("\nmain() {") :]
    steps = re.findall(r"^\tstep_([a-z_]+)$", main, re.M)
    assert steps == [
        "preflight",
        "packages",
        "docker",
        "user",
        "cosign",
        "fetch",
        "verify",
        "firewall",
        "install",
        "service",
    ]
    verify = script[script.index("\nstep_verify() {") :]
    verify = verify[: verify.index("\n}\n")]
    # The signature of SHA256SUMS first, then the checksums, then the files' own signatures.
    assert verify.index("verify_blob SHA256SUMS") < verify.index("sha256sum")
    assert verify.index("sha256sum") < verify.index('verify_blob "$bundle"')
    # The enrollment code of install.sh never reaches the cloud-init log.
    assert "Enrollment code (shown once)" in script


def test_cosign_pin_equals_the_tool_lock(tofu: str, data_dir: Path, tmp_path: Path) -> None:
    outputs, _ = plan(tofu, data_dir, tmp_path)
    cfg = config(files(outputs["user_data"])[CONFIG])
    lock = json.loads(LOCK.read_text(encoding="utf-8"))["tools"]["cosign"]
    assert cfg["SETUP_COSIGN_VERSION"] == lock["version"]
    assert cfg["SETUP_COSIGN_SHA256_AMD64"] == lock["assets"]["linux-amd64"]["sha256"]
    assert cfg["SETUP_COSIGN_SHA256_ARM64"] == lock["assets"]["linux-arm64"]["sha256"]


def test_options_follow_the_inputs(tofu: str, data_dir: Path, tmp_path: Path) -> None:
    outputs, _ = plan(
        tofu,
        data_dir,
        tmp_path,
        object_storage_mode="seaweedfs",
        telemetry_exporters=["local"],
        virus_scan="off",
        backup_offsite={"type": "s3", "location": "https://s3.example.com/bothy-backup/i1"},
        release_public_key=PUBLIC_KEY,
        release_base_url="http://127.0.0.1:8000/",
        admin_ssh_public_keys=["ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExample alex@example.com"],
    )
    rendered = files(outputs["user_data"])
    options = rendered[OPTIONS].splitlines()
    assert options[options.index("--storage") + 1] == "s3"
    assert options[options.index("--observability") + 1] == "on"
    assert options[options.index("--scan") + 1] == "off"
    assert options[options.index("--offsite") + 1] == "s3:https://s3.example.com/bothy-backup/i1"
    assert outputs["backup_offsite_location"] == "s3:https://s3.example.com/bothy-backup/i1"
    assert rendered[RELEASE_KEY] == PUBLIC_KEY
    cfg = config(rendered[CONFIG])
    assert cfg["SETUP_SIGNING"] == "key"
    assert cfg["SETUP_BASE_URL"] == "http://127.0.0.1:8000/"
    users = yaml.safe_load(outputs["user_data"])["users"]
    assert users[0]["name"] == "bothy"
    assert users[0]["ssh_authorized_keys"] == [
        "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExample alex@example.com"
    ]


@pytest.mark.parametrize(
    ("values", "message"),
    [
        ({"database_mode": "managed"}, "database_mode"),
        ({"object_storage_mode": "s3"}, "object_storage_mode"),
        ({"telemetry_exporters": ["newrelic"]}, "telemetry_exporters"),
        ({"backup_offsite": {"type": "sftp", "location": "https://backup.example.com/b"}}, "sftp"),
        ({"owner_name": "Alex 'x'"}, "owner_name"),
        ({"release_base_url": "ftp://example.com/"}, "release_base_url"),
    ],
)
def test_unsupported_inputs_stop_the_plan(
    tofu: str, data_dir: Path, values: dict[str, Any], message: str
) -> None:
    assert message in plan_error(tofu, data_dir, **values)


def test_ssh_delivery_plans_one_provisioner(tofu: str, data_dir: Path, tmp_path: Path) -> None:
    _, plan_json = plan(tofu, data_dir, tmp_path, provision_over_ssh=True)
    doc = json.loads(plan_json)
    changes = [c for c in doc["resource_changes"] if c["type"] == "terraform_data"]
    assert [c["change"]["actions"] for c in changes] == [["create"]]
    _, plan_json = plan(tofu, data_dir, tmp_path)
    changes = json.loads(plan_json).get("resource_changes", [])
    assert not [c for c in changes if c["type"] == "terraform_data"]
