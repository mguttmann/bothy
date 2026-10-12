from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.json_patch_operation import JsonPatchOperation
    from ..models.run import Run


T = TypeVar("T", bound="RunUpdatedPayload")


@_attrs_define
class RunUpdatedPayload:
    """Coalesced live-box patch. Either `ops` (from `base_version` to `version`) or a full `snapshot`.

    Attributes:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        version (int):
        base_version (int | Unset):
        ops (list[JsonPatchOperation] | Unset):
        snapshot (Run | Unset): Run document (`run_document.v1`), the content of the live box.
    """

    run_id: UUID
    version: int
    base_version: int | Unset = UNSET
    ops: list[JsonPatchOperation] | Unset = UNSET
    snapshot: Run | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        run_id = str(self.run_id)

        version = self.version

        base_version = self.base_version

        ops: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.ops, Unset):
            ops = []
            for ops_item_data in self.ops:
                ops_item = ops_item_data.to_dict()
                ops.append(ops_item)

        snapshot: dict[str, Any] | Unset = UNSET
        if not isinstance(self.snapshot, Unset):
            snapshot = self.snapshot.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "run_id": run_id,
                "version": version,
            }
        )
        if base_version is not UNSET:
            field_dict["base_version"] = base_version
        if ops is not UNSET:
            field_dict["ops"] = ops
        if snapshot is not UNSET:
            field_dict["snapshot"] = snapshot

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.json_patch_operation import JsonPatchOperation
        from ..models.run import Run

        d = dict(src_dict)
        run_id = UUID(d.pop("run_id"))

        version = d.pop("version")

        base_version = d.pop("base_version", UNSET)

        _ops = d.pop("ops", UNSET)
        ops: list[JsonPatchOperation] | Unset = UNSET
        if _ops is not UNSET:
            ops = []
            for ops_item_data in _ops:
                ops_item = JsonPatchOperation.from_dict(ops_item_data)

                ops.append(ops_item)

        _snapshot = d.pop("snapshot", UNSET)
        snapshot: Run | Unset
        if isinstance(_snapshot, Unset):
            snapshot = UNSET
        else:
            snapshot = Run.from_dict(_snapshot)

        run_updated_payload = cls(
            run_id=run_id,
            version=version,
            base_version=base_version,
            ops=ops,
            snapshot=snapshot,
        )

        run_updated_payload.additional_properties = d
        return run_updated_payload

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
