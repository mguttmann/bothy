from enum import StrEnum


class AppInstallationState(StrEnum):
    ACTIVE = "active"
    SUSPENDED = "suspended"
    UNINSTALLED = "uninstalled"

    def __str__(self) -> str:
        return str(self.value)
