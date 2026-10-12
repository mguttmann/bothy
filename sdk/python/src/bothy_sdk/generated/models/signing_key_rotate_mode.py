from enum import StrEnum


class SigningKeyRotateMode(StrEnum):
    EMERGENCY = "emergency"
    SCHEDULED = "scheduled"

    def __str__(self) -> str:
        return str(self.value)
