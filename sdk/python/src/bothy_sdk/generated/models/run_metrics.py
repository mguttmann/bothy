from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.run_api_retry import RunApiRetry


T = TypeVar("T", bound="RunMetrics")


@_attrs_define
class RunMetrics:
    """Schema `run_metrics.v1`.

    Attributes:
        context_pct (float | Unset):
        tokens_in (int | Unset):
        tokens_out (int | Unset):
        tokens_cache_read (int | Unset):
        tokens_cache_write (int | Unset): Cache writes with 5 minutes (or without a stated duration).
        tokens_cache_write_1h (int | Unset): Cache writes with 1 hour, counted separately.
        subagents (int | Unset):
        background_jobs (int | Unset):
        api_retries (int | Unset):
        progress_events (int | Unset): Count of progress messages of the runtime in the current turn that changed
            nothing visible (for example a long running tool); a patch of it counts as progress against `run.stall_after`
            (09 5.5, 18.4).
        last_api_retry (RunApiRetry | Unset): Last retry of a model API call, shown in the live box while set.
    """

    context_pct: float | Unset = UNSET
    tokens_in: int | Unset = UNSET
    tokens_out: int | Unset = UNSET
    tokens_cache_read: int | Unset = UNSET
    tokens_cache_write: int | Unset = UNSET
    tokens_cache_write_1h: int | Unset = UNSET
    subagents: int | Unset = UNSET
    background_jobs: int | Unset = UNSET
    api_retries: int | Unset = UNSET
    progress_events: int | Unset = UNSET
    last_api_retry: RunApiRetry | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        context_pct = self.context_pct

        tokens_in = self.tokens_in

        tokens_out = self.tokens_out

        tokens_cache_read = self.tokens_cache_read

        tokens_cache_write = self.tokens_cache_write

        tokens_cache_write_1h = self.tokens_cache_write_1h

        subagents = self.subagents

        background_jobs = self.background_jobs

        api_retries = self.api_retries

        progress_events = self.progress_events

        last_api_retry: dict[str, Any] | Unset = UNSET
        if not isinstance(self.last_api_retry, Unset):
            last_api_retry = self.last_api_retry.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if context_pct is not UNSET:
            field_dict["context_pct"] = context_pct
        if tokens_in is not UNSET:
            field_dict["tokens_in"] = tokens_in
        if tokens_out is not UNSET:
            field_dict["tokens_out"] = tokens_out
        if tokens_cache_read is not UNSET:
            field_dict["tokens_cache_read"] = tokens_cache_read
        if tokens_cache_write is not UNSET:
            field_dict["tokens_cache_write"] = tokens_cache_write
        if tokens_cache_write_1h is not UNSET:
            field_dict["tokens_cache_write_1h"] = tokens_cache_write_1h
        if subagents is not UNSET:
            field_dict["subagents"] = subagents
        if background_jobs is not UNSET:
            field_dict["background_jobs"] = background_jobs
        if api_retries is not UNSET:
            field_dict["api_retries"] = api_retries
        if progress_events is not UNSET:
            field_dict["progress_events"] = progress_events
        if last_api_retry is not UNSET:
            field_dict["last_api_retry"] = last_api_retry

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.run_api_retry import RunApiRetry

        d = dict(src_dict)
        context_pct = d.pop("context_pct", UNSET)

        tokens_in = d.pop("tokens_in", UNSET)

        tokens_out = d.pop("tokens_out", UNSET)

        tokens_cache_read = d.pop("tokens_cache_read", UNSET)

        tokens_cache_write = d.pop("tokens_cache_write", UNSET)

        tokens_cache_write_1h = d.pop("tokens_cache_write_1h", UNSET)

        subagents = d.pop("subagents", UNSET)

        background_jobs = d.pop("background_jobs", UNSET)

        api_retries = d.pop("api_retries", UNSET)

        progress_events = d.pop("progress_events", UNSET)

        _last_api_retry = d.pop("last_api_retry", UNSET)
        last_api_retry: RunApiRetry | Unset
        if isinstance(_last_api_retry, Unset):
            last_api_retry = UNSET
        else:
            last_api_retry = RunApiRetry.from_dict(_last_api_retry)

        run_metrics = cls(
            context_pct=context_pct,
            tokens_in=tokens_in,
            tokens_out=tokens_out,
            tokens_cache_read=tokens_cache_read,
            tokens_cache_write=tokens_cache_write,
            tokens_cache_write_1h=tokens_cache_write_1h,
            subagents=subagents,
            background_jobs=background_jobs,
            api_retries=api_retries,
            progress_events=progress_events,
            last_api_retry=last_api_retry,
        )

        return run_metrics
