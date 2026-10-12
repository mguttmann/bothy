from enum import StrEnum


class PinAddedEventType(StrEnum):
    PIN_ADDED = "pin.added"

    def __str__(self) -> str:
        return str(self.value)
