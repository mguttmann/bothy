from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.host_computer_permissions_capture import HostComputerPermissionsCapture
from ..models.host_computer_permissions_input import HostComputerPermissionsInput

T = TypeVar("T", bound="HostComputerPermissions")


@_attrs_define
class HostComputerPermissions:
    """
    Attributes:
        capture (HostComputerPermissionsCapture):
        input_ (HostComputerPermissionsInput):
    """

    capture: HostComputerPermissionsCapture
    input_: HostComputerPermissionsInput

    def to_dict(self) -> dict[str, Any]:
        capture = self.capture.value

        input_ = self.input_.value

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "capture": capture,
                "input": input_,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        capture = HostComputerPermissionsCapture(d.pop("capture"))

        input_ = HostComputerPermissionsInput(d.pop("input"))

        host_computer_permissions = cls(
            capture=capture,
            input_=input_,
        )

        return host_computer_permissions
