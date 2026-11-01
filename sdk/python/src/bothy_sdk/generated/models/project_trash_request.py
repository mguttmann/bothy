from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="ProjectTrashRequest")


@_attrs_define
class ProjectTrashRequest:
    """A project folder the runner moves to the trash (09 18.15).

    Attributes:
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        workspace_ref (str):
    """

    channel_id: UUID
    workspace_ref: str
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        channel_id = str(self.channel_id)

        workspace_ref = self.workspace_ref

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "channel_id": channel_id,
                "workspace_ref": workspace_ref,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        channel_id = UUID(d.pop("channel_id"))

        workspace_ref = d.pop("workspace_ref")

        project_trash_request = cls(
            channel_id=channel_id,
            workspace_ref=workspace_ref,
        )

        project_trash_request.additional_properties = d
        return project_trash_request

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
