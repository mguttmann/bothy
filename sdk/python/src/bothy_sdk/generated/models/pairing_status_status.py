from enum import StrEnum


class PairingStatusStatus(StrEnum):
    REQUESTED = "requested"

    def __str__(self) -> str:
        return str(self.value)
