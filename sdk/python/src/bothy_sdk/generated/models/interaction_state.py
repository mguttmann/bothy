from enum import StrEnum


class InteractionState(StrEnum):
    ACKNOWLEDGED = "acknowledged"
    DISPATCHED = "dispatched"
    FAILED = "failed"
    RECEIVED = "received"
    REJECTED = "rejected"

    def __str__(self) -> str:
        return str(self.value)
