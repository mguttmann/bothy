from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="ImportEarlierSession")


@_attrs_define
class ImportEarlierSession:
    """The agent session the thread had before the migration (22, 9 step 1; `--sessions` of the tool). The server writes
    one earlier `agent_sessions` row (`is_current = false`, `started = true`, `ended_at` = the time of the batch,
    `end_reason = import`, the cost baseline) exactly once per thread, agent and reference, also for a thread posted by
    an earlier run, and never after the thread has had a current session; the item member `session` names the outcome.
    `agent_id` must be the operator of the thread (`threads.operator_agent_id`), else `400 validation_failed` at
    `/items/<i>/data/earlier_session/agent_id`. The reference is opaque, at most `agent.session_ref_max_bytes` bytes,
    and is never logged.

        Attributes:
            agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            external_session_ref (str):
            cost_baseline_micros (int | Unset): The cumulative amount of the external session before the binding, in micro
                units of `currency` (22, 9 step 7); absent means 0.
            currency (str | Unset): ISO 4217 code of `cost_baseline_micros`; required when it is above 0.
    """

    agent_id: UUID
    external_session_ref: str
    cost_baseline_micros: int | Unset = UNSET
    currency: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        agent_id = str(self.agent_id)

        external_session_ref = self.external_session_ref

        cost_baseline_micros = self.cost_baseline_micros

        currency = self.currency

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "agent_id": agent_id,
                "external_session_ref": external_session_ref,
            }
        )
        if cost_baseline_micros is not UNSET:
            field_dict["cost_baseline_micros"] = cost_baseline_micros
        if currency is not UNSET:
            field_dict["currency"] = currency

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        agent_id = UUID(d.pop("agent_id"))

        external_session_ref = d.pop("external_session_ref")

        cost_baseline_micros = d.pop("cost_baseline_micros", UNSET)

        currency = d.pop("currency", UNSET)

        import_earlier_session = cls(
            agent_id=agent_id,
            external_session_ref=external_session_ref,
            cost_baseline_micros=cost_baseline_micros,
            currency=currency,
        )

        return import_earlier_session
