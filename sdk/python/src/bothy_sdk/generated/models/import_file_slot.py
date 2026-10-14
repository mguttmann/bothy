from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.import_item import ImportItem
    from ..models.upload_session import UploadSession


T = TypeVar("T", bound="ImportFileSlot")


@_attrs_define
class ImportFileSlot:
    """The `pending` item reserves the file ID; a repeated call with the same key returns the same ID (22, 7.2).

    Attributes:
        item (ImportItem):
        upload (UploadSession | Unset): Upload plan. In mode `direct` the part URLs point to this server; in mode
            `presigned` they
            point to the object store. The client uploads all parts, then calls complete.
    """

    item: ImportItem
    upload: UploadSession | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        item = self.item.to_dict()

        upload: dict[str, Any] | Unset = UNSET
        if not isinstance(self.upload, Unset):
            upload = self.upload.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "item": item,
            }
        )
        if upload is not UNSET:
            field_dict["upload"] = upload

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.import_item import ImportItem
        from ..models.upload_session import UploadSession

        d = dict(src_dict)
        item = ImportItem.from_dict(d.pop("item"))

        _upload = d.pop("upload", UNSET)
        upload: UploadSession | Unset
        if isinstance(_upload, Unset):
            upload = UNSET
        else:
            upload = UploadSession.from_dict(_upload)

        import_file_slot = cls(
            item=item,
            upload=upload,
        )

        import_file_slot.additional_properties = d
        return import_file_slot

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
