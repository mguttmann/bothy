from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..models.import_channel_data_kind import ImportChannelDataKind
from ..types import UNSET, Unset

T = TypeVar("T", bound="ImportChannelData")


@_attrs_define
class ImportChannelData:
    """`data` of a `channel` item (origin key `channel:<channel_id>`). The space is the target of the posted item
    `space_origin_key` or, without it, of the category named in `defaults.uncategorized_space`; `existing_space_id` and
    `existing_channel_id` must equal the stored mapping. `read_only` is not a member: the server derives it from the
    stored `lead`.

        Attributes:
            kind (ImportChannelDataKind):
            sort_key (str): Fractional index key of the order in the space.
            space_origin_key (str | Unset):
            existing_space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            name (str | Unset):
            topic (str | Unset):
            existing_channel_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    kind: ImportChannelDataKind
    sort_key: str
    space_origin_key: str | Unset = UNSET
    existing_space_id: UUID | Unset = UNSET
    name: str | Unset = UNSET
    topic: str | Unset = UNSET
    existing_channel_id: UUID | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        kind = self.kind.value

        sort_key = self.sort_key

        space_origin_key = self.space_origin_key

        existing_space_id: str | Unset = UNSET
        if not isinstance(self.existing_space_id, Unset):
            existing_space_id = str(self.existing_space_id)

        name = self.name

        topic = self.topic

        existing_channel_id: str | Unset = UNSET
        if not isinstance(self.existing_channel_id, Unset):
            existing_channel_id = str(self.existing_channel_id)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "kind": kind,
                "sort_key": sort_key,
            }
        )
        if space_origin_key is not UNSET:
            field_dict["space_origin_key"] = space_origin_key
        if existing_space_id is not UNSET:
            field_dict["existing_space_id"] = existing_space_id
        if name is not UNSET:
            field_dict["name"] = name
        if topic is not UNSET:
            field_dict["topic"] = topic
        if existing_channel_id is not UNSET:
            field_dict["existing_channel_id"] = existing_channel_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        kind = ImportChannelDataKind(d.pop("kind"))

        sort_key = d.pop("sort_key")

        space_origin_key = d.pop("space_origin_key", UNSET)

        _existing_space_id = d.pop("existing_space_id", UNSET)
        existing_space_id: UUID | Unset
        if isinstance(_existing_space_id, Unset):
            existing_space_id = UNSET
        else:
            existing_space_id = UUID(_existing_space_id)

        name = d.pop("name", UNSET)

        topic = d.pop("topic", UNSET)

        _existing_channel_id = d.pop("existing_channel_id", UNSET)
        existing_channel_id: UUID | Unset
        if isinstance(_existing_channel_id, Unset):
            existing_channel_id = UNSET
        else:
            existing_channel_id = UUID(_existing_channel_id)

        import_channel_data = cls(
            kind=kind,
            sort_key=sort_key,
            space_origin_key=space_origin_key,
            existing_space_id=existing_space_id,
            name=name,
            topic=topic,
            existing_channel_id=existing_channel_id,
        )

        return import_channel_data
