from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..models.run_wake_fire_outcome import RunWakeFireOutcome
from ..types import UNSET, Unset

T = TypeVar("T", bound="RunWakeFire")


@_attrs_define
class RunWakeFire:
    """
    Attributes:
        outcome (RunWakeFireOutcome):
        result_text (str | Unset): The result of `fired`; at most `message.max_content_bytes` bytes, no control or bidi
            characters except line feed and tab.
        reason (str | Unset): The reason of `failed` (the last failure, for example `gh_missing`); absent means
            `wake_check_failed`.
        run_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    outcome: RunWakeFireOutcome
    result_text: str | Unset = UNSET
    reason: str | Unset = UNSET
    run_id: UUID | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        outcome = self.outcome.value

        result_text = self.result_text

        reason = self.reason

        run_id: str | Unset = UNSET
        if not isinstance(self.run_id, Unset):
            run_id = str(self.run_id)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "outcome": outcome,
            }
        )
        if result_text is not UNSET:
            field_dict["result_text"] = result_text
        if reason is not UNSET:
            field_dict["reason"] = reason
        if run_id is not UNSET:
            field_dict["run_id"] = run_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        outcome = RunWakeFireOutcome(d.pop("outcome"))

        result_text = d.pop("result_text", UNSET)

        reason = d.pop("reason", UNSET)

        _run_id = d.pop("run_id", UNSET)
        run_id: UUID | Unset
        if isinstance(_run_id, Unset):
            run_id = UNSET
        else:
            run_id = UUID(_run_id)

        run_wake_fire = cls(
            outcome=outcome,
            result_text=result_text,
            reason=reason,
            run_id=run_id,
        )

        return run_wake_fire
