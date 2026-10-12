from enum import StrEnum


class HostDisabledReason(StrEnum):
    DUPLICATE_CONNECTION = "duplicate_connection"
    OWNER = "owner"

    def __str__(self) -> str:
        return str(self.value)
