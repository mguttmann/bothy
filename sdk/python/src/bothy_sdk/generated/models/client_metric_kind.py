from enum import StrEnum


class ClientMetricKind(StrEnum):
    COUNTER = "counter"
    GAUGE = "gauge"
    HISTOGRAM = "histogram"

    def __str__(self) -> str:
        return str(self.value)
