from enum import StrEnum


class ImportItemSession(StrEnum):
    ALREADY_PRESENT = "already_present"
    DROPPED_LATE = "dropped_late"
    TAKEN_OVER = "taken_over"

    def __str__(self) -> str:
        return str(self.value)
