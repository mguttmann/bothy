from enum import StrEnum


class CommandChangedPayloadChange(StrEnum):
    REGISTERED = "registered"
    REMOVED = "removed"
    UPDATED = "updated"

    def __str__(self) -> str:
        return str(self.value)
