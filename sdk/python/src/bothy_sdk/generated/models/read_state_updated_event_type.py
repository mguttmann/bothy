from enum import StrEnum


class ReadStateUpdatedEventType(StrEnum):
    READ_STATE_UPDATED = "read_state.updated"

    def __str__(self) -> str:
        return str(self.value)
