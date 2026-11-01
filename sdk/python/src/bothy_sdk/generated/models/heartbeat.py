from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar, cast

from attrs import define as _attrs_define

from ..models.billing_mode import BillingMode
from ..models.heartbeat_state import HeartbeatState
from ..models.tool_class import ToolClass
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.agent_rate_limit_state import AgentRateLimitState
    from ..models.billing_evidence import BillingEvidence
    from ..models.json_object import JsonObject
    from ..models.project_readiness import ProjectReadiness
    from ..models.project_trash_result import ProjectTrashResult


T = TypeVar("T", bound="Heartbeat")


@_attrs_define
class Heartbeat:
    """
    Attributes:
        state (HeartbeatState):
        runtime_version (str | Unset): Version of the agent runtime (column `reported_runtime_version`); the adapter's
            own version is `details.adapter.version`.
        reported_billing_mode (BillingMode | Unset): Backend policy of an agent. `personal` runs on the subscription of
            exactly one person.
        billing_modes_supported (list[BillingMode] | Unset):
        billing_evidence (BillingEvidence | Unset): How the runtime proves its actual billing mode. Never a key value or
            prefix (schema `billing_evidence.v1`).
        reported_tool_class (ToolClass | Unset): Tool class of an agent runtime, ordered `chat` < `scoped` < `host` (07,
            2.11).
        rate_limit (AgentRateLimitState | Unset): Rate-limit state of the agent runtime at its provider as reported by
            the adapter (`agent_heartbeat.v1`). Never quotas of an account, keys or subscription names.
        details (JsonObject | Unset): Free-form JSON object whose schema is named in the field description.
        project_readiness (list[ProjectReadiness] | Unset): Readiness per bound forum of a runner (M2-03); the channels
            must be distinct. Applied only when present and different from the last list this server process applied; absent
            means "unchanged".
        project_folders (list[str] | Unset): Runners only, the folder names directly under the projects root that are
            Git work trees, sorted, at most `runner.project_folders_max` (09 18.13, M4-44). Applied only when present and
            different from the last list this server process applied; absent means "unchanged". Names only, never a path.
        project_trash_results (list[ProjectTrashResult] | Unset): Runners only, the outcome of each folder move of
            `HeartbeatAck.project_trash` the runner finished since its last heartbeat (09 18.15); the server stores it once
            and audits it as `channel.project.trash`.
    """

    state: HeartbeatState
    runtime_version: str | Unset = UNSET
    reported_billing_mode: BillingMode | Unset = UNSET
    billing_modes_supported: list[BillingMode] | Unset = UNSET
    billing_evidence: BillingEvidence | Unset = UNSET
    reported_tool_class: ToolClass | Unset = UNSET
    rate_limit: AgentRateLimitState | Unset = UNSET
    details: JsonObject | Unset = UNSET
    project_readiness: list[ProjectReadiness] | Unset = UNSET
    project_folders: list[str] | Unset = UNSET
    project_trash_results: list[ProjectTrashResult] | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        state = self.state.value

        runtime_version = self.runtime_version

        reported_billing_mode: str | Unset = UNSET
        if not isinstance(self.reported_billing_mode, Unset):
            reported_billing_mode = self.reported_billing_mode.value

        billing_modes_supported: list[str] | Unset = UNSET
        if not isinstance(self.billing_modes_supported, Unset):
            billing_modes_supported = []
            for billing_modes_supported_item_data in self.billing_modes_supported:
                billing_modes_supported_item = billing_modes_supported_item_data.value
                billing_modes_supported.append(billing_modes_supported_item)

        billing_evidence: dict[str, Any] | Unset = UNSET
        if not isinstance(self.billing_evidence, Unset):
            billing_evidence = self.billing_evidence.to_dict()

        reported_tool_class: str | Unset = UNSET
        if not isinstance(self.reported_tool_class, Unset):
            reported_tool_class = self.reported_tool_class.value

        rate_limit: dict[str, Any] | Unset = UNSET
        if not isinstance(self.rate_limit, Unset):
            rate_limit = self.rate_limit.to_dict()

        details: dict[str, Any] | Unset = UNSET
        if not isinstance(self.details, Unset):
            details = self.details.to_dict()

        project_readiness: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.project_readiness, Unset):
            project_readiness = []
            for project_readiness_item_data in self.project_readiness:
                project_readiness_item = project_readiness_item_data.to_dict()
                project_readiness.append(project_readiness_item)

        project_folders: list[str] | Unset = UNSET
        if not isinstance(self.project_folders, Unset):
            project_folders = self.project_folders

        project_trash_results: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.project_trash_results, Unset):
            project_trash_results = []
            for project_trash_results_item_data in self.project_trash_results:
                project_trash_results_item = project_trash_results_item_data.to_dict()
                project_trash_results.append(project_trash_results_item)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "state": state,
            }
        )
        if runtime_version is not UNSET:
            field_dict["runtime_version"] = runtime_version
        if reported_billing_mode is not UNSET:
            field_dict["reported_billing_mode"] = reported_billing_mode
        if billing_modes_supported is not UNSET:
            field_dict["billing_modes_supported"] = billing_modes_supported
        if billing_evidence is not UNSET:
            field_dict["billing_evidence"] = billing_evidence
        if reported_tool_class is not UNSET:
            field_dict["reported_tool_class"] = reported_tool_class
        if rate_limit is not UNSET:
            field_dict["rate_limit"] = rate_limit
        if details is not UNSET:
            field_dict["details"] = details
        if project_readiness is not UNSET:
            field_dict["project_readiness"] = project_readiness
        if project_folders is not UNSET:
            field_dict["project_folders"] = project_folders
        if project_trash_results is not UNSET:
            field_dict["project_trash_results"] = project_trash_results

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.agent_rate_limit_state import AgentRateLimitState
        from ..models.billing_evidence import BillingEvidence
        from ..models.json_object import JsonObject
        from ..models.project_readiness import ProjectReadiness
        from ..models.project_trash_result import ProjectTrashResult

        d = dict(src_dict)
        state = HeartbeatState(d.pop("state"))

        runtime_version = d.pop("runtime_version", UNSET)

        _reported_billing_mode = d.pop("reported_billing_mode", UNSET)
        reported_billing_mode: BillingMode | Unset
        if isinstance(_reported_billing_mode, Unset):
            reported_billing_mode = UNSET
        else:
            reported_billing_mode = BillingMode(_reported_billing_mode)

        _billing_modes_supported = d.pop("billing_modes_supported", UNSET)
        billing_modes_supported: list[BillingMode] | Unset = UNSET
        if _billing_modes_supported is not UNSET:
            billing_modes_supported = []
            for billing_modes_supported_item_data in _billing_modes_supported:
                billing_modes_supported_item = BillingMode(
                    billing_modes_supported_item_data
                )

                billing_modes_supported.append(billing_modes_supported_item)

        _billing_evidence = d.pop("billing_evidence", UNSET)
        billing_evidence: BillingEvidence | Unset
        if isinstance(_billing_evidence, Unset):
            billing_evidence = UNSET
        else:
            billing_evidence = BillingEvidence.from_dict(_billing_evidence)

        _reported_tool_class = d.pop("reported_tool_class", UNSET)
        reported_tool_class: ToolClass | Unset
        if isinstance(_reported_tool_class, Unset):
            reported_tool_class = UNSET
        else:
            reported_tool_class = ToolClass(_reported_tool_class)

        _rate_limit = d.pop("rate_limit", UNSET)
        rate_limit: AgentRateLimitState | Unset
        if isinstance(_rate_limit, Unset):
            rate_limit = UNSET
        else:
            rate_limit = AgentRateLimitState.from_dict(_rate_limit)

        _details = d.pop("details", UNSET)
        details: JsonObject | Unset
        if isinstance(_details, Unset):
            details = UNSET
        else:
            details = JsonObject.from_dict(_details)

        _project_readiness = d.pop("project_readiness", UNSET)
        project_readiness: list[ProjectReadiness] | Unset = UNSET
        if _project_readiness is not UNSET:
            project_readiness = []
            for project_readiness_item_data in _project_readiness:
                project_readiness_item = ProjectReadiness.from_dict(
                    project_readiness_item_data
                )

                project_readiness.append(project_readiness_item)

        project_folders = cast(list[str], d.pop("project_folders", UNSET))

        _project_trash_results = d.pop("project_trash_results", UNSET)
        project_trash_results: list[ProjectTrashResult] | Unset = UNSET
        if _project_trash_results is not UNSET:
            project_trash_results = []
            for project_trash_results_item_data in _project_trash_results:
                project_trash_results_item = ProjectTrashResult.from_dict(
                    project_trash_results_item_data
                )

                project_trash_results.append(project_trash_results_item)

        heartbeat = cls(
            state=state,
            runtime_version=runtime_version,
            reported_billing_mode=reported_billing_mode,
            billing_modes_supported=billing_modes_supported,
            billing_evidence=billing_evidence,
            reported_tool_class=reported_tool_class,
            rate_limit=rate_limit,
            details=details,
            project_readiness=project_readiness,
            project_folders=project_folders,
            project_trash_results=project_trash_results,
        )

        return heartbeat
