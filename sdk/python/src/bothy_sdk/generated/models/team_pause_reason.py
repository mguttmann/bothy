from enum import StrEnum


class TeamPauseReason(StrEnum):
    PINGPONG = "pingpong"
    STOPPED = "stopped"
    TURN_BUDGET = "turn_budget"

    def __str__(self) -> str:
        return str(self.value)
