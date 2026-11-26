from enum import StrEnum


class HostComputerSignature(StrEnum):
    DEVELOPMENT = "development"
    VERIFIED = "verified"

    def __str__(self) -> str:
        return str(self.value)
