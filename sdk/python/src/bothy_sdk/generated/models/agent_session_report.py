from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.agent_session_report_bind_failed import AgentSessionReportBindFailed
from ..types import UNSET, Unset

T = TypeVar("T", bound="AgentSessionReport")


@_attrs_define
class AgentSessionReport:
    """
    Attributes:
        external_session_ref (str): The reference the runtime actually uses; a different one replaces the stored
            reference of the session (09, 9.5 item 3). At most `agent.session_ref_max_bytes` bytes of display text.
        model (str | Unset): The pattern of `default_model` (09, 2.2), at most `agent.session_ref_max_bytes` bytes.
        started (bool | Unset): `true` once the runtime confirmed the external session (09, 9.1); `started` only moves
            forward, `false` never resets it.
        bind_failed (AgentSessionReportBindFailed | Unset): The bound session no longer exists in the runtime and the
            adapter started a new one (09, 9.5 item 4); only for a started session, otherwise 409 `conflict` with `reason`
            `session_not_bound`.
    """

    external_session_ref: str
    model: str | Unset = UNSET
    started: bool | Unset = UNSET
    bind_failed: AgentSessionReportBindFailed | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        external_session_ref = self.external_session_ref

        model = self.model

        started = self.started

        bind_failed: str | Unset = UNSET
        if not isinstance(self.bind_failed, Unset):
            bind_failed = self.bind_failed.value

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "external_session_ref": external_session_ref,
            }
        )
        if model is not UNSET:
            field_dict["model"] = model
        if started is not UNSET:
            field_dict["started"] = started
        if bind_failed is not UNSET:
            field_dict["bind_failed"] = bind_failed

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        external_session_ref = d.pop("external_session_ref")

        model = d.pop("model", UNSET)

        started = d.pop("started", UNSET)

        _bind_failed = d.pop("bind_failed", UNSET)
        bind_failed: AgentSessionReportBindFailed | Unset
        if isinstance(_bind_failed, Unset):
            bind_failed = UNSET
        else:
            bind_failed = AgentSessionReportBindFailed(_bind_failed)

        agent_session_report = cls(
            external_session_ref=external_session_ref,
            model=model,
            started=started,
            bind_failed=bind_failed,
        )

        return agent_session_report
