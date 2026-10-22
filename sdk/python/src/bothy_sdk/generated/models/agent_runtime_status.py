from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.agent_runtime_state import AgentRuntimeState
from ..models.billing_mode import BillingMode
from ..models.tool_class import ToolClass
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.agent_rate_limit_state import AgentRateLimitState
    from ..models.billing_evidence import BillingEvidence


T = TypeVar("T", bound="AgentRuntimeStatus")


@_attrs_define
class AgentRuntimeStatus:
    """Reported state of the agent runtime (heartbeat), not the configured state.

    Attributes:
        state (AgentRuntimeState): State derived from heartbeats.
        reported_billing_mode (BillingMode | Unset): Backend policy of an agent. `personal` runs on the subscription of
            exactly one person.
        runtime_version (str | Unset):
        last_heartbeat_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix
            `Z`.
        billing_modes_supported (list[BillingMode] | Unset):
        reported_tool_class (ToolClass | Unset): Tool class of an agent runtime, ordered `chat` < `scoped` < `host` (07,
            2.11).
        billing_mode_matches (bool | Unset): False when the reported mode differs from the configured `billing_mode`.
        rate_limit (AgentRateLimitState | Unset): Rate-limit state of the agent runtime at its provider as reported by
            the adapter (`agent_heartbeat.v1`). Never quotas of an account, keys or subscription names.
        billing_evidence (BillingEvidence | Unset): How the runtime proves its actual billing mode. Never a key value or
            prefix (schema `billing_evidence.v1`).
        warnings (list[str] | Unset): Known values `billing_mode_mismatch` (the reported mode has differed from the
            configured one for longer than `billing.mismatch_alert_after`), `billing_mode_unsupported` (no reported mode and
            the configured one is not in `billing_modes_supported`), `billing_evidence_unknown` (`billing_evidence.key_type`
            is `unknown`) and `tool_class_mismatch` (the reported tool class differs from the configured one). Only while
            the state is `online` or `degraded`.
    """

    state: AgentRuntimeState
    reported_billing_mode: BillingMode | Unset = UNSET
    runtime_version: str | Unset = UNSET
    last_heartbeat_at: datetime.datetime | Unset = UNSET
    billing_modes_supported: list[BillingMode] | Unset = UNSET
    reported_tool_class: ToolClass | Unset = UNSET
    billing_mode_matches: bool | Unset = UNSET
    rate_limit: AgentRateLimitState | Unset = UNSET
    billing_evidence: BillingEvidence | Unset = UNSET
    warnings: list[str] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        state = self.state.value

        reported_billing_mode: str | Unset = UNSET
        if not isinstance(self.reported_billing_mode, Unset):
            reported_billing_mode = self.reported_billing_mode.value

        runtime_version = self.runtime_version

        last_heartbeat_at: str | Unset = UNSET
        if not isinstance(self.last_heartbeat_at, Unset):
            last_heartbeat_at = self.last_heartbeat_at.isoformat()

        billing_modes_supported: list[str] | Unset = UNSET
        if not isinstance(self.billing_modes_supported, Unset):
            billing_modes_supported = []
            for billing_modes_supported_item_data in self.billing_modes_supported:
                billing_modes_supported_item = billing_modes_supported_item_data.value
                billing_modes_supported.append(billing_modes_supported_item)

        reported_tool_class: str | Unset = UNSET
        if not isinstance(self.reported_tool_class, Unset):
            reported_tool_class = self.reported_tool_class.value

        billing_mode_matches = self.billing_mode_matches

        rate_limit: dict[str, Any] | Unset = UNSET
        if not isinstance(self.rate_limit, Unset):
            rate_limit = self.rate_limit.to_dict()

        billing_evidence: dict[str, Any] | Unset = UNSET
        if not isinstance(self.billing_evidence, Unset):
            billing_evidence = self.billing_evidence.to_dict()

        warnings: list[str] | Unset = UNSET
        if not isinstance(self.warnings, Unset):
            warnings = self.warnings

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "state": state,
            }
        )
        if reported_billing_mode is not UNSET:
            field_dict["reported_billing_mode"] = reported_billing_mode
        if runtime_version is not UNSET:
            field_dict["runtime_version"] = runtime_version
        if last_heartbeat_at is not UNSET:
            field_dict["last_heartbeat_at"] = last_heartbeat_at
        if billing_modes_supported is not UNSET:
            field_dict["billing_modes_supported"] = billing_modes_supported
        if reported_tool_class is not UNSET:
            field_dict["reported_tool_class"] = reported_tool_class
        if billing_mode_matches is not UNSET:
            field_dict["billing_mode_matches"] = billing_mode_matches
        if rate_limit is not UNSET:
            field_dict["rate_limit"] = rate_limit
        if billing_evidence is not UNSET:
            field_dict["billing_evidence"] = billing_evidence
        if warnings is not UNSET:
            field_dict["warnings"] = warnings

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.agent_rate_limit_state import AgentRateLimitState
        from ..models.billing_evidence import BillingEvidence

        d = dict(src_dict)
        state = AgentRuntimeState(d.pop("state"))

        _reported_billing_mode = d.pop("reported_billing_mode", UNSET)
        reported_billing_mode: BillingMode | Unset
        if isinstance(_reported_billing_mode, Unset):
            reported_billing_mode = UNSET
        else:
            reported_billing_mode = BillingMode(_reported_billing_mode)

        runtime_version = d.pop("runtime_version", UNSET)

        _last_heartbeat_at = d.pop("last_heartbeat_at", UNSET)
        last_heartbeat_at: datetime.datetime | Unset
        if isinstance(_last_heartbeat_at, Unset):
            last_heartbeat_at = UNSET
        else:
            last_heartbeat_at = datetime.datetime.fromisoformat(_last_heartbeat_at)

        _billing_modes_supported = d.pop("billing_modes_supported", UNSET)
        billing_modes_supported: list[BillingMode] | Unset = UNSET
        if _billing_modes_supported is not UNSET:
            billing_modes_supported = []
            for billing_modes_supported_item_data in _billing_modes_supported:
                billing_modes_supported_item = BillingMode(
                    billing_modes_supported_item_data
                )

                billing_modes_supported.append(billing_modes_supported_item)

        _reported_tool_class = d.pop("reported_tool_class", UNSET)
        reported_tool_class: ToolClass | Unset
        if isinstance(_reported_tool_class, Unset):
            reported_tool_class = UNSET
        else:
            reported_tool_class = ToolClass(_reported_tool_class)

        billing_mode_matches = d.pop("billing_mode_matches", UNSET)

        _rate_limit = d.pop("rate_limit", UNSET)
        rate_limit: AgentRateLimitState | Unset
        if isinstance(_rate_limit, Unset):
            rate_limit = UNSET
        else:
            rate_limit = AgentRateLimitState.from_dict(_rate_limit)

        _billing_evidence = d.pop("billing_evidence", UNSET)
        billing_evidence: BillingEvidence | Unset
        if isinstance(_billing_evidence, Unset):
            billing_evidence = UNSET
        else:
            billing_evidence = BillingEvidence.from_dict(_billing_evidence)

        warnings = cast(list[str], d.pop("warnings", UNSET))

        agent_runtime_status = cls(
            state=state,
            reported_billing_mode=reported_billing_mode,
            runtime_version=runtime_version,
            last_heartbeat_at=last_heartbeat_at,
            billing_modes_supported=billing_modes_supported,
            reported_tool_class=reported_tool_class,
            billing_mode_matches=billing_mode_matches,
            rate_limit=rate_limit,
            billing_evidence=billing_evidence,
            warnings=warnings,
        )

        agent_runtime_status.additional_properties = d
        return agent_runtime_status

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
