from enum import StrEnum


class SidebarUpdatedEventType(StrEnum):
    SIDEBAR_UPDATED = "sidebar.updated"

    def __str__(self) -> str:
        return str(self.value)
