from enum import StrEnum


class HostState(StrEnum):
    ACTIVE = "active"
    CONFIRMING = "confirming"
    DISABLED = "disabled"
    PAIRING = "pairing"
    RETIRED = "retired"

    def __str__(self) -> str:
        return str(self.value)
