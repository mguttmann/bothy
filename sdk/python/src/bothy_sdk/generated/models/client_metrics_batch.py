from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.client_metrics_batch_density import ClientMetricsBatchDensity
from ..models.client_metrics_batch_platform import ClientMetricsBatchPlatform
from ..models.client_metrics_batch_theme import ClientMetricsBatchTheme
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.client_metric import ClientMetric


T = TypeVar("T", bound="ClientMetricsBatch")


@_attrs_define
class ClientMetricsBatch:
    """Anonymous client measurements (15, section 17). No contents, no IDs of threads, channels or people.

    Attributes:
        platform (ClientMetricsBatchPlatform):
        app_version (str):
        metrics (list[ClientMetric]):
        density (ClientMetricsBatchDensity | Unset): Mobile apps always send `touch` (01, section 11).
        theme (ClientMetricsBatchTheme | Unset):
    """

    platform: ClientMetricsBatchPlatform
    app_version: str
    metrics: list[ClientMetric]
    density: ClientMetricsBatchDensity | Unset = UNSET
    theme: ClientMetricsBatchTheme | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        platform = self.platform.value

        app_version = self.app_version

        metrics = []
        for metrics_item_data in self.metrics:
            metrics_item = metrics_item_data.to_dict()
            metrics.append(metrics_item)

        density: str | Unset = UNSET
        if not isinstance(self.density, Unset):
            density = self.density.value

        theme: str | Unset = UNSET
        if not isinstance(self.theme, Unset):
            theme = self.theme.value

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "platform": platform,
                "app_version": app_version,
                "metrics": metrics,
            }
        )
        if density is not UNSET:
            field_dict["density"] = density
        if theme is not UNSET:
            field_dict["theme"] = theme

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.client_metric import ClientMetric

        d = dict(src_dict)
        platform = ClientMetricsBatchPlatform(d.pop("platform"))

        app_version = d.pop("app_version")

        metrics = []
        _metrics = d.pop("metrics")
        for metrics_item_data in _metrics:
            metrics_item = ClientMetric.from_dict(metrics_item_data)

            metrics.append(metrics_item)

        _density = d.pop("density", UNSET)
        density: ClientMetricsBatchDensity | Unset
        if isinstance(_density, Unset):
            density = UNSET
        else:
            density = ClientMetricsBatchDensity(_density)

        _theme = d.pop("theme", UNSET)
        theme: ClientMetricsBatchTheme | Unset
        if isinstance(_theme, Unset):
            theme = UNSET
        else:
            theme = ClientMetricsBatchTheme(_theme)

        client_metrics_batch = cls(
            platform=platform,
            app_version=app_version,
            metrics=metrics,
            density=density,
            theme=theme,
        )

        return client_metrics_batch
