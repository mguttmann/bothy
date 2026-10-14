from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="FieldError")


@_attrs_define
class FieldError:
    """
    Attributes:
        pointer (str): Where the violation is: a JSON Pointer (RFC 6901) to the offending member of the request body
            (for an unknown or missing member the pointer names that member, e.g. `/name`), or `#path/<name>`,
            `#query/<name>`, `#header/<name>` for parameters; empty for a rule of the whole request or the body as a whole.
        code (str): Machine-readable rule the field violates. Schema validation uses `required`, `unknown_member` (W7:
            request schemas are closed), `invalid_type`, `invalid_format`, `invalid_enum`, `too_long`, `too_short`,
            `too_large`, `too_small`, `too_many_items`, `too_few_items`, `pattern_mismatch` and `invalid` for any other
            rule; use cases add `limit_reached` and `not_enabled` (06 section 5). Clients treat unknown codes like
            `invalid`.
        detail (str | Unset): Short English explanation; never repeats the submitted value.
    """

    pointer: str
    code: str
    detail: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        pointer = self.pointer

        code = self.code

        detail = self.detail

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "pointer": pointer,
                "code": code,
            }
        )
        if detail is not UNSET:
            field_dict["detail"] = detail

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        pointer = d.pop("pointer")

        code = d.pop("code")

        detail = d.pop("detail", UNSET)

        field_error = cls(
            pointer=pointer,
            code=code,
            detail=detail,
        )

        field_error.additional_properties = d
        return field_error

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
