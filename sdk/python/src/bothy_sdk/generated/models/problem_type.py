from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="ProblemType")


@_attrs_define
class ProblemType:
    """Documentation of one problem type, served at its `type` URI.

    Attributes:
        code (str):
        status (int):
        title (str):
        description (str):
        client_action (str | Unset): What a client should do.
    """

    code: str
    status: int
    title: str
    description: str
    client_action: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        code = self.code

        status = self.status

        title = self.title

        description = self.description

        client_action = self.client_action

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "code": code,
                "status": status,
                "title": title,
                "description": description,
            }
        )
        if client_action is not UNSET:
            field_dict["client_action"] = client_action

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        code = d.pop("code")

        status = d.pop("status")

        title = d.pop("title")

        description = d.pop("description")

        client_action = d.pop("client_action", UNSET)

        problem_type = cls(
            code=code,
            status=status,
            title=title,
            description=description,
            client_action=client_action,
        )

        problem_type.additional_properties = d
        return problem_type

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
