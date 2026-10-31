from enum import StrEnum


class SidebarChannelUpdatedEventType(StrEnum):
    SIDEBAR_CHANNEL_UPDATED = "sidebar_channel.updated"

    def __str__(self) -> str:
        return str(self.value)
