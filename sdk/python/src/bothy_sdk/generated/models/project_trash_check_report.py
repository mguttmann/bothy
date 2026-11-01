from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="ProjectTrashCheckReport")


@_attrs_define
class ProjectTrashCheckReport:
    """The folder check a runner reports in its readiness entry (09 18.15).

    Attributes:
        ok (bool):
        reason (str | Unset): Why not, with `ok` false; a code of 09 18.15.
    """

    ok: bool
    reason: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        ok = self.ok

        reason = self.reason

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "ok": ok,
            }
        )
        if reason is not UNSET:
            field_dict["reason"] = reason

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        ok = d.pop("ok")

        reason = d.pop("reason", UNSET)

        project_trash_check_report = cls(
            ok=ok,
            reason=reason,
        )

        return project_trash_check_report
