from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.host_platform import HostPlatform

T = TypeVar("T", bound="ComputerDesktop")


@_attrs_define
class ComputerDesktop:
    """A host of the owner that carries a computer helper, as `computer.desktops.list` shows it to the agent (27 5.1).

    Attributes:
        host_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        name (str): The name of the host (`hosts.name`), by which the owner names the desktop.
        platform (HostPlatform):
        online (bool): The host service holds a control connection to the server.
        locked (bool): The screen is locked (`report.computer.locked`).
        busy (bool): Another computer session on the host is not closed.
    """

    host_id: UUID
    name: str
    platform: HostPlatform
    online: bool
    locked: bool
    busy: bool
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        host_id = str(self.host_id)

        name = self.name

        platform = self.platform.value

        online = self.online

        locked = self.locked

        busy = self.busy

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "host_id": host_id,
                "name": name,
                "platform": platform,
                "online": online,
                "locked": locked,
                "busy": busy,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        host_id = UUID(d.pop("host_id"))

        name = d.pop("name")

        platform = HostPlatform(d.pop("platform"))

        online = d.pop("online")

        locked = d.pop("locked")

        busy = d.pop("busy")

        computer_desktop = cls(
            host_id=host_id,
            name=name,
            platform=platform,
            online=online,
            locked=locked,
            busy=busy,
        )

        computer_desktop.additional_properties = d
        return computer_desktop

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
