from enum import StrEnum


class GdprRequestState(StrEnum):
    APPROVED = "approved"
    CANCELLED = "cancelled"
    COMPLETED = "completed"
    FAILED = "failed"
    REJECTED = "rejected"
    REQUESTED = "requested"
    RUNNING = "running"

    def __str__(self) -> str:
        return str(self.value)
