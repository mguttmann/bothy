from enum import StrEnum


class DeletionInfoState(StrEnum):
    FINAL = "final"
    LIVE = "live"
    PENDING = "pending"

    def __str__(self) -> str:
        return str(self.value)
