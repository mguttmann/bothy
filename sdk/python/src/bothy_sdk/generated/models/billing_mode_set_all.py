from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..models.billing_mode import BillingMode
from ..types import UNSET, Unset

T = TypeVar("T", bound="BillingModeSetAll")


@_attrs_define
class BillingModeSetAll:
    """Switches several agents in one transaction, all or nothing (10, 9.1).

    Attributes:
        billing_mode (BillingMode): Backend policy of an agent. `personal` runs on the subscription of exactly one
            person.
        reason (str):
        credential_holder_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        agent_ids (list[UUID] | Unset): Required for `personal`. Without it `organization` affects every agent not yet
            on `organization`.
    """

    billing_mode: BillingMode
    reason: str
    credential_holder_id: UUID | Unset = UNSET
    agent_ids: list[UUID] | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        billing_mode = self.billing_mode.value

        reason = self.reason

        credential_holder_id: str | Unset = UNSET
        if not isinstance(self.credential_holder_id, Unset):
            credential_holder_id = str(self.credential_holder_id)

        agent_ids: list[str] | Unset = UNSET
        if not isinstance(self.agent_ids, Unset):
            agent_ids = []
            for agent_ids_item_data in self.agent_ids:
                agent_ids_item = str(agent_ids_item_data)
                agent_ids.append(agent_ids_item)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "billing_mode": billing_mode,
                "reason": reason,
            }
        )
        if credential_holder_id is not UNSET:
            field_dict["credential_holder_id"] = credential_holder_id
        if agent_ids is not UNSET:
            field_dict["agent_ids"] = agent_ids

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        billing_mode = BillingMode(d.pop("billing_mode"))

        reason = d.pop("reason")

        _credential_holder_id = d.pop("credential_holder_id", UNSET)
        credential_holder_id: UUID | Unset
        if isinstance(_credential_holder_id, Unset):
            credential_holder_id = UNSET
        else:
            credential_holder_id = UUID(_credential_holder_id)

        _agent_ids = d.pop("agent_ids", UNSET)
        agent_ids: list[UUID] | Unset = UNSET
        if _agent_ids is not UNSET:
            agent_ids = []
            for agent_ids_item_data in _agent_ids:
                agent_ids_item = UUID(agent_ids_item_data)

                agent_ids.append(agent_ids_item)

        billing_mode_set_all = cls(
            billing_mode=billing_mode,
            reason=reason,
            credential_holder_id=credential_holder_id,
            agent_ids=agent_ids,
        )

        return billing_mode_set_all
