from enum import StrEnum


class StatsSloState(StrEnum):
    AMBER = "amber"
    GREEN = "green"
    INSUFFICIENT_DATA = "insufficient_data"
    RED = "red"

    def __str__(self) -> str:
        return str(self.value)
