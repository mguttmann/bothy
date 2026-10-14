from enum import StrEnum


class PinRemovedEventType(StrEnum):
    PIN_REMOVED = "pin.removed"

    def __str__(self) -> str:
        return str(self.value)
