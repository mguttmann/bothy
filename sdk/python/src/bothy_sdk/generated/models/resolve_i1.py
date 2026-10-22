from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

T = TypeVar("T", bound="ResolveI1")


@_attrs_define
class ResolveI1:
    """Guided flow (05 rule 1.6, 10 section 11): switch these agents to `organization` first, in the same transaction as
    the change.

        Attributes:
            switch_to_organization (list[UUID]):
            reason (str): Reason of the switch, in the audit entry of every switched agent; at most `audit.reason_max_chars`
                characters, no control or bidi characters.
    """

    switch_to_organization: list[UUID]
    reason: str

    def to_dict(self) -> dict[str, Any]:
        switch_to_organization = []
        for switch_to_organization_item_data in self.switch_to_organization:
            switch_to_organization_item = str(switch_to_organization_item_data)
            switch_to_organization.append(switch_to_organization_item)

        reason = self.reason

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "switch_to_organization": switch_to_organization,
                "reason": reason,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        switch_to_organization = []
        _switch_to_organization = d.pop("switch_to_organization")
        for switch_to_organization_item_data in _switch_to_organization:
            switch_to_organization_item = UUID(switch_to_organization_item_data)

            switch_to_organization.append(switch_to_organization_item)

        reason = d.pop("reason")

        resolve_i1 = cls(
            switch_to_organization=switch_to_organization,
            reason=reason,
        )

        return resolve_i1
