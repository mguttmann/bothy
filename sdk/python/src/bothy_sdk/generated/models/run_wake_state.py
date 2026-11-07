from enum import StrEnum


class RunWakeState(StrEnum):
    CANCELLED = "cancelled"
    EXPIRED = "expired"
    FAILED = "failed"
    FIRED = "fired"
    PENDING = "pending"
    REFUSED = "refused"

    def __str__(self) -> str:
        return str(self.value)
