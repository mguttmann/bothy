from enum import StrEnum


class TriggerState(StrEnum):
    CANCELLED = "cancelled"
    DONE = "done"
    FAILED = "failed"
    FOLLOWUP_DELIVERED = "followup_delivered"
    QUEUED = "queued"
    RECEIVED = "received"
    WORKING = "working"

    def __str__(self) -> str:
        return str(self.value)
