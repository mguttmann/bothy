from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.client_metric_kind import ClientMetricKind
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.client_metric_buckets_item import ClientMetricBucketsItem


T = TypeVar("T", bound="ClientMetric")


@_attrs_define
class ClientMetric:
    """
    Attributes:
        name (str): One name of the closed list in docs/observability/signals.md section 4.6a:
            `client.cold_start.duration`, `client.thread_open.duration`, `client.send_local.duration`, `client.frames`,
            `client.frames.over_budget`, `client.catchup.duration`, `client.catchup.events`, `client.resync`,
            `client.reconnects`, `client.outbox.depth`, `client.push_display.duration`, `client.push.placeholder`. Not an
            enum on purpose: the server drops an entry with an unknown name, or with a `kind` that does not match the type
            of its name, one by one and still accepts the rest of the batch, so a newer app does not lose its batch against
            an older server.
        kind (ClientMetricKind):
        close_code (int | Unset): Only for `client.reconnects`; a close code of 04 section 6.4.
        value (float | Unset): Counter increment or gauge value.
        buckets (list[ClientMetricBucketsItem] | Unset): Histogram buckets (upper bound and count).
    """

    name: str
    kind: ClientMetricKind
    close_code: int | Unset = UNSET
    value: float | Unset = UNSET
    buckets: list[ClientMetricBucketsItem] | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        kind = self.kind.value

        close_code = self.close_code

        value = self.value

        buckets: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.buckets, Unset):
            buckets = []
            for buckets_item_data in self.buckets:
                buckets_item = buckets_item_data.to_dict()
                buckets.append(buckets_item)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "name": name,
                "kind": kind,
            }
        )
        if close_code is not UNSET:
            field_dict["close_code"] = close_code
        if value is not UNSET:
            field_dict["value"] = value
        if buckets is not UNSET:
            field_dict["buckets"] = buckets

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.client_metric_buckets_item import ClientMetricBucketsItem

        d = dict(src_dict)
        name = d.pop("name")

        kind = ClientMetricKind(d.pop("kind"))

        close_code = d.pop("close_code", UNSET)

        value = d.pop("value", UNSET)

        _buckets = d.pop("buckets", UNSET)
        buckets: list[ClientMetricBucketsItem] | Unset = UNSET
        if _buckets is not UNSET:
            buckets = []
            for buckets_item_data in _buckets:
                buckets_item = ClientMetricBucketsItem.from_dict(buckets_item_data)

                buckets.append(buckets_item)

        client_metric = cls(
            name=name,
            kind=kind,
            close_code=close_code,
            value=value,
            buckets=buckets,
        )

        return client_metric
