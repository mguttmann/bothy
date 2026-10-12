from enum import StrEnum


class ClientMetricsBatchTheme(StrEnum):
    DARK = "dark"
    LIGHT = "light"

    def __str__(self) -> str:
        return str(self.value)
