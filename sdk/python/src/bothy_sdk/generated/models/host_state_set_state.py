from enum import StrEnum


class HostStateSetState(StrEnum):
    ACTIVE = "active"
    DISABLED = "disabled"
    RETIRED = "retired"

    def __str__(self) -> str:
        return str(self.value)
