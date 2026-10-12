from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.json_patch_operation_op import JsonPatchOperationOp
from ..types import UNSET, Unset

T = TypeVar("T", bound="JsonPatchOperation")


@_attrs_define
class JsonPatchOperation:
    """One RFC 6902 operation, restricted to `add`, `replace` and `remove`.

    Attributes:
        op (JsonPatchOperationOp):
        path (str): JSON Pointer (RFC 6901) into the run document.
        value (Any | Unset): Any JSON value except null. The schema of the value is defined by its key.
    """

    op: JsonPatchOperationOp
    path: str
    value: Any | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        op = self.op.value

        path = self.path

        value = self.value

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "op": op,
                "path": path,
            }
        )
        if value is not UNSET:
            field_dict["value"] = value

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        op = JsonPatchOperationOp(d.pop("op"))

        path = d.pop("path")

        value = d.pop("value", UNSET)

        json_patch_operation = cls(
            op=op,
            path=path,
            value=value,
        )

        return json_patch_operation
