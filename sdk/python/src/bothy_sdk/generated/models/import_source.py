from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.import_kind import ImportKind
from ..models.import_source_state import ImportSourceState
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="ImportSource")


@_attrs_define
class ImportSource:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        kind (ImportKind):
        name (str):
        config (JsonObject): Free-form JSON object whose schema is named in the field description.
        state (ImportSourceState): A dry run writes nothing and keeps the state.
        stats (JsonObject): Free-form JSON object whose schema is named in the field description.
        created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        cursor (JsonObject | Unset): Free-form JSON object whose schema is named in the field description.
        started_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        finished_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    id: UUID
    kind: ImportKind
    name: str
    config: JsonObject
    state: ImportSourceState
    stats: JsonObject
    created_at: datetime.datetime
    cursor: JsonObject | Unset = UNSET
    started_at: datetime.datetime | Unset = UNSET
    finished_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        kind = self.kind.value

        name = self.name

        config = self.config.to_dict()

        state = self.state.value

        stats = self.stats.to_dict()

        created_at = self.created_at.isoformat()

        cursor: dict[str, Any] | Unset = UNSET
        if not isinstance(self.cursor, Unset):
            cursor = self.cursor.to_dict()

        started_at: str | Unset = UNSET
        if not isinstance(self.started_at, Unset):
            started_at = self.started_at.isoformat()

        finished_at: str | Unset = UNSET
        if not isinstance(self.finished_at, Unset):
            finished_at = self.finished_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "kind": kind,
                "name": name,
                "config": config,
                "state": state,
                "stats": stats,
                "created_at": created_at,
            }
        )
        if cursor is not UNSET:
            field_dict["cursor"] = cursor
        if started_at is not UNSET:
            field_dict["started_at"] = started_at
        if finished_at is not UNSET:
            field_dict["finished_at"] = finished_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        id = UUID(d.pop("id"))

        kind = ImportKind(d.pop("kind"))

        name = d.pop("name")

        config = JsonObject.from_dict(d.pop("config"))

        state = ImportSourceState(d.pop("state"))

        stats = JsonObject.from_dict(d.pop("stats"))

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        _cursor = d.pop("cursor", UNSET)
        cursor: JsonObject | Unset
        if isinstance(_cursor, Unset):
            cursor = UNSET
        else:
            cursor = JsonObject.from_dict(_cursor)

        _started_at = d.pop("started_at", UNSET)
        started_at: datetime.datetime | Unset
        if isinstance(_started_at, Unset):
            started_at = UNSET
        else:
            started_at = datetime.datetime.fromisoformat(_started_at)

        _finished_at = d.pop("finished_at", UNSET)
        finished_at: datetime.datetime | Unset
        if isinstance(_finished_at, Unset):
            finished_at = UNSET
        else:
            finished_at = datetime.datetime.fromisoformat(_finished_at)

        import_source = cls(
            id=id,
            kind=kind,
            name=name,
            config=config,
            state=state,
            stats=stats,
            created_at=created_at,
            cursor=cursor,
            started_at=started_at,
            finished_at=finished_at,
        )

        import_source.additional_properties = d
        return import_source

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
