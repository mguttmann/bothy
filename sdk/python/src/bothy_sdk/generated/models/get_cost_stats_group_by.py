from enum import StrEnum


class GetCostStatsGroupBy(StrEnum):
    AGENT = "agent"
    NONE = "none"
    SPACE = "space"

    def __str__(self) -> str:
        return str(self.value)
