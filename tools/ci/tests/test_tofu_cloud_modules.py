# SPDX-License-Identifier: Apache-2.0
"""Static tests of the OpenTofu cloud modules hetzner, aws and azure (brief M7-16).

They read the configuration files, not a plan: the interface of every module (requirement 13),
the shape of the network rules that `tofu test` checks with mocked providers (requirement 14,
infra/tofu/test/cloud/*.tftest.hcl, run by `make tofu-check`), the absence of secrets in inputs
and outputs and the enforced state encryption of the example root (requirement 15), and the
provider pins against the lock files. The files are formatted by `tofu fmt` (checked by
`make tofu-check`), so a top-level block starts at the beginning of a line and ends with the
first line that is a lone `}`.
"""

from __future__ import annotations

import re
from pathlib import Path

import pytest

REPO = Path(__file__).resolve().parents[3]
TOFU = REPO / "infra" / "tofu"
MODULES = TOFU / "modules"
CLOUD = ("hetzner", "aws", "azure")
LOCKS = sorted((TOFU / "test").glob("*/.terraform.lock.hcl"))
EXAMPLE_ROOT = TOFU / "test" / "validate" / "main.tf"

# The contract of infra/tofu/README.md "Structure" items 2 and 3.
SHARED_INPUTS = {
    "instance_name",
    "environment",
    "bothy_version",
    "image_registry",
    "public_hostname",
    "edge_mode",
    "database_mode",
    "object_storage_mode",
    "backup_offsite",
    "telemetry_exporters",
    "admin_ssh_public_keys",
    "tags",
}
# Inputs of generic-vm that every cloud module passes on to the host setup unchanged.
SETUP_INPUTS = {
    "release_repository",
    "release_base_url",
    "release_public_key",
    "owner_handle",
    "owner_name",
    "virus_scan",
}
# Inputs every cloud module has for its network.
CLOUD_INPUTS = {"ssh_allowed_cidrs"}
TARGET_INPUTS = {
    "hetzner": {"location", "server_type", "image"},
    "aws": {
        "region",
        "instance_type",
        "root_volume_size",
        "offsite_region",
        "offsite_retention_days",
    },
    "azure": {"location", "resource_group_name", "vm_size", "image_sku"},
}
SHARED_OUTPUTS = {
    "public_url",
    "ssh_endpoint",
    "database_endpoint",
    "object_storage_bucket",
    "backup_offsite_location",
    "secret_store_ref",
}
# Resource types that admit traffic into a module's network; each module has exactly one
# resource of its type, the one the tftest files assert over.
INGRESS_TYPES = {
    "hetzner": {"hcloud_firewall": "host"},
    "aws": {"aws_vpc_security_group_ingress_rule": "host"},
    "azure": {"azurerm_network_security_group": "host"},
}
# Further types that can open a port and must not appear: separate rule resources and
# attachments the tests would not see.
FORBIDDEN_TYPES = {
    "aws_security_group_rule",
    "azurerm_network_security_rule",
    "hcloud_firewall_attachment",
    "hcloud_load_balancer",
    "azurerm_application_gateway",
    "aws_lb",
}
SECRET_NAME = re.compile(r"password|secret|token|private|credential|access_key|auth_key")
# Names of the contract that match SECRET_NAME without holding a secret: the prefix of secret
# references (for example a Key Vault URI), never a value.
NOT_SECRETS = {"secret_store_ref"}

_BLOCK = re.compile(
    r'^(variable|output|resource|data|module) "([^"]+)"(?: "([^"]+)")? \{\n(.*?)^\}\n', re.M | re.S
)


def blocks(module: str, kind: str) -> dict[str, str]:
    """Top-level blocks of one kind in a module, keyed by name (resources by "type.name")."""
    found: dict[str, str] = {}
    for path in sorted((MODULES / module).glob("*.tf")):
        for m in _BLOCK.finditer(path.read_text(encoding="utf-8")):
            if m.group(1) != kind:
                continue
            key = f"{m.group(2)}.{m.group(3)}" if m.group(3) else m.group(2)
            assert key not in found, f"{module}: {kind} {key} defined twice"
            found[key] = m.group(4)
    return found


def lock_versions(path: Path) -> dict[str, str]:
    """Provider source to version in a lock file."""
    text = path.read_text(encoding="utf-8")
    return {
        m.group(1): m.group(2)
        for m in re.finditer(
            r'^provider "registry\.opentofu\.org/([^"]+)" \{\n\s+version\s+= "([^"]+)"', text, re.M
        )
    }


def pinned_providers(module: str) -> dict[str, str]:
    """Provider source to exact version in `required_providers` of a module."""
    text = (MODULES / module / "versions.tf").read_text(encoding="utf-8")
    pins = re.findall(r'source\s+= "([^"]+)"\n\s+version\s+= "= ([0-9.]+)"', text)
    return dict(pins)


# ---- requirement 13: the interface ---------------------------------------------------------


@pytest.mark.parametrize("module", CLOUD)
def test_inputs_are_the_contract_plus_the_target_inputs(module: str) -> None:
    names = set(blocks(module, "variable"))
    assert names == SHARED_INPUTS | SETUP_INPUTS | CLOUD_INPUTS | TARGET_INPUTS[module]


@pytest.mark.parametrize("module", CLOUD)
def test_outputs_are_exactly_the_shared_outputs(module: str) -> None:
    assert set(blocks(module, "output")) == SHARED_OUTPUTS


def test_generic_vm_has_the_contract_and_the_setup_inputs() -> None:
    names = set(blocks("generic-vm", "variable"))
    assert names >= SHARED_INPUTS | SETUP_INPUTS
    assert set(blocks("generic-vm", "output")) == SHARED_OUTPUTS | {"user_data"}


@pytest.mark.parametrize("module", [*CLOUD, "generic-vm"])
def test_every_input_and_output_has_a_description(module: str) -> None:
    for kind in ("variable", "output"):
        for name, body in blocks(module, kind).items():
            assert re.search(r'^\s+description\s+= "[^"]+', body, re.M), f"{module}: {kind} {name}"


@pytest.mark.parametrize("module", CLOUD)
def test_shared_inputs_keep_the_types_of_generic_vm(module: str) -> None:
    """The same type per shared input, so a root can move between targets unchanged."""
    reference = blocks("generic-vm", "variable")
    own = blocks(module, "variable")
    for name in SHARED_INPUTS | SETUP_INPUTS:
        expected = re.search(
            r"^  type\s+= (.*?)\n  (?:default|description|validation|\})",
            reference[name],
            re.M | re.S,
        )
        actual = re.search(
            r"^  type\s+= (.*?)\n  (?:default|description|validation|\})", own[name], re.M | re.S
        )
        assert expected, f"generic-vm: {name} has no type"
        assert actual, f"{module}: {name} has no type"
        assert actual.group(1) == expected.group(1), f"{module}: {name}"


@pytest.mark.parametrize("module", CLOUD)
def test_every_module_calls_generic_vm(module: str) -> None:
    [body] = blocks(module, "module").values()
    assert 'source = "../generic-vm"' in body
    for name in SHARED_INPUTS | SETUP_INPUTS:
        assert re.search(rf"^\s+{name}\s+= ", body, re.M), f"{module}: {name} is not passed on"


# ---- requirement 14: only the edge port is public ------------------------------------------


@pytest.mark.parametrize("module", CLOUD)
def test_one_resource_admits_traffic(module: str) -> None:
    """The tftest files assert over this one resource; no other resource may open a port."""
    resources = blocks(module, "resource")
    types = {key.split(".")[0] for key in resources}
    assert not types & FORBIDDEN_TYPES
    [(kind, name)] = INGRESS_TYPES[module].items()
    assert [key for key in resources if key.startswith(f"{kind}.")] == [f"{kind}.{name}"]
    if module == "aws":
        group = resources["aws_security_group.host"]
        assert "ingress" not in group, "inline rules of the security group escape the test"


@pytest.mark.parametrize("module", CLOUD)
def test_the_network_test_covers_the_module(module: str) -> None:
    text = (TOFU / "test" / "cloud" / f"{module}.tftest.hcl").read_text(encoding="utf-8")
    assert f'source = "../../modules/{module}"' in text
    assert 'run "edge_port_only_public"' in text
    assert 'run "no_public_port_without_reverse_proxy"' in text
    assert 'run "ssh_from_anywhere_is_refused"' in text


# ---- requirement 15: no secret in inputs, outputs or state ---------------------------------


@pytest.mark.parametrize("module", CLOUD)
def test_no_input_or_output_is_a_secret(module: str) -> None:
    for kind in ("variable", "output"):
        for name, body in blocks(module, kind).items():
            assert name in NOT_SECRETS or not SECRET_NAME.search(name), (
                f"{module}: {kind} {name} looks like a secret"
            )
            assert "sensitive" not in body, f"{module}: {kind} {name} is marked sensitive"


@pytest.mark.parametrize("module", CLOUD)
def test_no_resource_keeps_a_secret_in_the_state(module: str) -> None:
    """Resources whose state holds a generated secret are never used (requirement 15)."""
    resources = blocks(module, "resource")
    for key, body in resources.items():
        kind = key.split(".")[0]
        assert kind not in {
            "aws_iam_access_key",
            "random_password",
            "tls_private_key",
            "azurerm_storage_account",
        }, f"{module}: {key}"
        assert not re.search(r"^\s+(admin_password|password|secret)\s+=", body, re.M), key


def test_the_example_root_requires_state_encryption() -> None:
    text = EXAMPLE_ROOT.read_text(encoding="utf-8")
    encryption = re.search(r"^  encryption \{\n(.*?)^  \}\n", text, re.M | re.S)
    assert encryption, "the example root has no encryption block"
    body = encryption.group(1)
    for target in ("state", "plan"):
        block = re.search(rf"^    {target} \{{\n(.*?)^    \}}\n", body, re.M | re.S)
        assert block, f"no {target} block"
        assert re.search(r"^\s+enforced\s+= true$", block.group(1), re.M), f"{target} not enforced"
        assert "fallback" not in block.group(1), f"{target} may fall back to plain text"


# ---- provider pins ---------------------------------------------------------------------------


@pytest.mark.parametrize("module", CLOUD)
def test_provider_pinned_to_the_version_of_the_lock_files(module: str) -> None:
    pins = pinned_providers(module)
    assert len(pins) == 1, f"{module}: one provider, pinned with = <version>"
    assert LOCKS, "no lock file under infra/tofu/test/"
    for lock in LOCKS:
        versions = lock_versions(lock)
        for source, version in pins.items():
            assert versions.get(source) == version, f"{lock.relative_to(REPO)}: {source}"


def test_lock_files_are_equal() -> None:
    texts = {lock.read_text(encoding="utf-8") for lock in LOCKS}
    assert len(LOCKS) >= 2
    assert len(texts) == 1


def test_lock_files_hold_every_platform_of_the_gates() -> None:
    """The zip hash per platform is what tools/bin/tofu_providers.py verifies a download by."""
    text = LOCKS[0].read_text(encoding="utf-8")
    for block in re.findall(r'^provider "[^"]+" \{\n(.*?)^\}\n', text, re.M | re.S):
        assert len(re.findall(r'"zh:[0-9a-f]{64}"', block)) >= 4, block
