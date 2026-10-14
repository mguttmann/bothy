from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.run_step_kind import RunStepKind
from ..models.run_step_status import RunStepStatus
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="RunStep")


@_attrs_define
class RunStep:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        ordinal (int):
        kind (RunStepKind):
        name (str):
        title (str):
        status (RunStepStatus):
        started_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        parent_step_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        detail (JsonObject | Unset): Free-form JSON object whose schema is named in the field description.
        finished_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    id: UUID
    ordinal: int
    kind: RunStepKind
    name: str
    title: str
    status: RunStepStatus
    started_at: datetime.datetime
    parent_step_id: UUID | Unset = UNSET
    detail: JsonObject | Unset = UNSET
    finished_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        ordinal = self.ordinal

        kind = self.kind.value

        name = self.name

        title = self.title

        status = self.status.value

        started_at = self.started_at.isoformat()

        parent_step_id: str | Unset = UNSET
        if not isinstance(self.parent_step_id, Unset):
            parent_step_id = str(self.parent_step_id)

        detail: dict[str, Any] | Unset = UNSET
        if not isinstance(self.detail, Unset):
            detail = self.detail.to_dict()

        finished_at: str | Unset = UNSET
        if not isinstance(self.finished_at, Unset):
            finished_at = self.finished_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "ordinal": ordinal,
                "kind": kind,
                "name": name,
                "title": title,
                "status": status,
                "started_at": started_at,
            }
        )
        if parent_step_id is not UNSET:
            field_dict["parent_step_id"] = parent_step_id
        if detail is not UNSET:
            field_dict["detail"] = detail
        if finished_at is not UNSET:
            field_dict["finished_at"] = finished_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        id = UUID(d.pop("id"))

        ordinal = d.pop("ordinal")

        kind = RunStepKind(d.pop("kind"))

        name = d.pop("name")

        title = d.pop("title")

        status = RunStepStatus(d.pop("status"))

        started_at = datetime.datetime.fromisoformat(d.pop("started_at"))

        _parent_step_id = d.pop("parent_step_id", UNSET)
        parent_step_id: UUID | Unset
        if isinstance(_parent_step_id, Unset):
            parent_step_id = UNSET
        else:
            parent_step_id = UUID(_parent_step_id)

        _detail = d.pop("detail", UNSET)
        detail: JsonObject | Unset
        if isinstance(_detail, Unset):
            detail = UNSET
        else:
            detail = JsonObject.from_dict(_detail)

        _finished_at = d.pop("finished_at", UNSET)
        finished_at: datetime.datetime | Unset
        if isinstance(_finished_at, Unset):
            finished_at = UNSET
        else:
            finished_at = datetime.datetime.fromisoformat(_finished_at)

        run_step = cls(
            id=id,
            ordinal=ordinal,
            kind=kind,
            name=name,
            title=title,
            status=status,
            started_at=started_at,
            parent_step_id=parent_step_id,
            detail=detail,
            finished_at=finished_at,
        )

        run_step.additional_properties = d
        return run_step

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
