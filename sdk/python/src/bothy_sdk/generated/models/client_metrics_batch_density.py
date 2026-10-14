from enum import StrEnum


class ClientMetricsBatchDensity(StrEnum):
    COMFORTABLE = "comfortable"
    COMPACT = "compact"
    TOUCH = "touch"

    def __str__(self) -> str:
        return str(self.value)
