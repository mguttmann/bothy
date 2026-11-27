from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.host_computer_helper import HostComputerHelper
from ..models.host_computer_signature import HostComputerSignature

if TYPE_CHECKING:
    from ..models.host_computer_permissions import HostComputerPermissions


T = TypeVar("T", bound="HostComputer")


@_attrs_define
class HostComputer:
    """The state of the computer helper of a host (schema `host_computer.v1`, docs/spec/27-computer-use.md 12.3): reported
    by the host service in `report.computer`, stored in `hosts.reported_computer`. Never a path, a key or screen
    content.

        Attributes:
            helper (HostComputerHelper): The computer helper is connected, absent, or refused by the peer rule (27 9.4).
            enabled (bool): The switch "Allow computer control" of the helper is on.
            generation (int): The enablement generation of the host (27 4).
            os_account (str): The operating system account of the desktop session the helper runs in.
            permissions (HostComputerPermissions): The operating system permissions of the computer helper (27 12.3).
            helper_version (str): Version of the computer helper.
            signature (HostComputerSignature): `verified` when the build of the host service carries a code signing
                requirement for the helper, else `development`.
            locked (bool): The screen of the desktop is locked.
    """

    helper: HostComputerHelper
    enabled: bool
    generation: int
    os_account: str
    permissions: HostComputerPermissions
    helper_version: str
    signature: HostComputerSignature
    locked: bool

    def to_dict(self) -> dict[str, Any]:
        helper = self.helper.value

        enabled = self.enabled

        generation = self.generation

        os_account = self.os_account

        permissions = self.permissions.to_dict()

        helper_version = self.helper_version

        signature = self.signature.value

        locked = self.locked

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "helper": helper,
                "enabled": enabled,
                "generation": generation,
                "os_account": os_account,
                "permissions": permissions,
                "helper_version": helper_version,
                "signature": signature,
                "locked": locked,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.host_computer_permissions import HostComputerPermissions

        d = dict(src_dict)
        helper = HostComputerHelper(d.pop("helper"))

        enabled = d.pop("enabled")

        generation = d.pop("generation")

        os_account = d.pop("os_account")

        permissions = HostComputerPermissions.from_dict(d.pop("permissions"))

        helper_version = d.pop("helper_version")

        signature = HostComputerSignature(d.pop("signature"))

        locked = d.pop("locked")

        host_computer = cls(
            helper=helper,
            enabled=enabled,
            generation=generation,
            os_account=os_account,
            permissions=permissions,
            helper_version=helper_version,
            signature=signature,
            locked=locked,
        )

        return host_computer
