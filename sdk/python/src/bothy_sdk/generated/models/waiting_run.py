from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..models.waiting_run_reason import WaitingRunReason

T = TypeVar("T", bound="WaitingRun")


@_attrs_define
class WaitingRun:
    """One run an adapter accepted that waits before its start (09 5.2.1).

    Attributes:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        reason (WaitingRunReason): `slots_full`: every slot of `runner.max_parallel_runs` is in use; `thread_busy`: the
            thread or session lock is held by another turn.
    """

    run_id: UUID
    reason: WaitingRunReason

    def to_dict(self) -> dict[str, Any]:
        run_id = str(self.run_id)

        reason = self.reason.value

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "run_id": run_id,
                "reason": reason,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        run_id = UUID(d.pop("run_id"))

        reason = WaitingRunReason(d.pop("reason"))

        waiting_run = cls(
            run_id=run_id,
            reason=reason,
        )

        return waiting_run
