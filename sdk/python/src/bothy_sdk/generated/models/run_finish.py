from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.run_finish_state import RunFinishState
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.cost_report import CostReport
    from ..models.run_metrics import RunMetrics


T = TypeVar("T", bound="RunFinish")


@_attrs_define
class RunFinish:
    """
    Attributes:
        state (RunFinishState):
        reason (str | Unset): Required for `failed` and `cancelled`, e.g. `billing_mode_mismatch`, `adapter_error`,
            `user_cancelled`.
        summary (str | Unset):
        metrics (RunMetrics | Unset): Schema `run_metrics.v1`.
        costs (list[CostReport] | Unset): The consumption of this run, one report per model (a second report of the same
            provider and model is invalid). The server books one cost entry per report at the end of the run (10, 12.1); a
            repeated finish books nothing. Only a run that started (`billing_mode_used` set) may report costs.
    """

    state: RunFinishState
    reason: str | Unset = UNSET
    summary: str | Unset = UNSET
    metrics: RunMetrics | Unset = UNSET
    costs: list[CostReport] | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        state = self.state.value

        reason = self.reason

        summary = self.summary

        metrics: dict[str, Any] | Unset = UNSET
        if not isinstance(self.metrics, Unset):
            metrics = self.metrics.to_dict()

        costs: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.costs, Unset):
            costs = []
            for costs_item_data in self.costs:
                costs_item = costs_item_data.to_dict()
                costs.append(costs_item)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "state": state,
            }
        )
        if reason is not UNSET:
            field_dict["reason"] = reason
        if summary is not UNSET:
            field_dict["summary"] = summary
        if metrics is not UNSET:
            field_dict["metrics"] = metrics
        if costs is not UNSET:
            field_dict["costs"] = costs

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.cost_report import CostReport
        from ..models.run_metrics import RunMetrics

        d = dict(src_dict)
        state = RunFinishState(d.pop("state"))

        reason = d.pop("reason", UNSET)

        summary = d.pop("summary", UNSET)

        _metrics = d.pop("metrics", UNSET)
        metrics: RunMetrics | Unset
        if isinstance(_metrics, Unset):
            metrics = UNSET
        else:
            metrics = RunMetrics.from_dict(_metrics)

        _costs = d.pop("costs", UNSET)
        costs: list[CostReport] | Unset = UNSET
        if _costs is not UNSET:
            costs = []
            for costs_item_data in _costs:
                costs_item = CostReport.from_dict(costs_item_data)

                costs.append(costs_item)

        run_finish = cls(
            state=state,
            reason=reason,
            summary=summary,
            metrics=metrics,
            costs=costs,
        )

        return run_finish
