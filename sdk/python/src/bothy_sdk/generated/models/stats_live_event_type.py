from enum import StrEnum


class StatsLiveEventType(StrEnum):
    STATS_LIVE = "stats.live"

    def __str__(self) -> str:
        return str(self.value)
