from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.flag_rollout import FlagRollout


T = TypeVar("T", bound="FeatureFlagPut")


@_attrs_define
class FeatureFlagPut:
    """
    Attributes:
        enabled (bool):
        reason (str):
        rollout (FlagRollout | Unset): Principals and groups for which the flag has the opposite value of `enabled`.
    """

    enabled: bool
    reason: str
    rollout: FlagRollout | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        enabled = self.enabled

        reason = self.reason

        rollout: dict[str, Any] | Unset = UNSET
        if not isinstance(self.rollout, Unset):
            rollout = self.rollout.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "enabled": enabled,
                "reason": reason,
            }
        )
        if rollout is not UNSET:
            field_dict["rollout"] = rollout

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.flag_rollout import FlagRollout

        d = dict(src_dict)
        enabled = d.pop("enabled")

        reason = d.pop("reason")

        _rollout = d.pop("rollout", UNSET)
        rollout: FlagRollout | Unset
        if isinstance(_rollout, Unset):
            rollout = UNSET
        else:
            rollout = FlagRollout.from_dict(_rollout)

        feature_flag_put = cls(
            enabled=enabled,
            reason=reason,
            rollout=rollout,
        )

        return feature_flag_put
