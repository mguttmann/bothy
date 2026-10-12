from enum import StrEnum


class InteractionRespondState(StrEnum):
    ACKNOWLEDGED = "acknowledged"
    FAILED = "failed"
    REJECTED = "rejected"

    def __str__(self) -> str:
        return str(self.value)
