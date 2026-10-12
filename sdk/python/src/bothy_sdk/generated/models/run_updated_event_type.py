from enum import StrEnum


class RunUpdatedEventType(StrEnum):
    RUN_UPDATED = "run.updated"

    def __str__(self) -> str:
        return str(self.value)
