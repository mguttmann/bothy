from enum import StrEnum


class RunFollowupState(StrEnum):
    ABSORBED = "absorbed"
    DEFERRED = "deferred"
    DELIVERED = "delivered"
    DISCARDED = "discarded"
    QUEUED = "queued"

    def __str__(self) -> str:
        return str(self.value)
