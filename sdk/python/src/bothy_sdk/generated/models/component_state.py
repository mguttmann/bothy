from enum import StrEnum


class ComponentState(StrEnum):
    ACTIVE = "active"
    DISABLED = "disabled"
    EXPIRED = "expired"
    USED = "used"

    def __str__(self) -> str:
        return str(self.value)
