from enum import StrEnum


class ChannelSettingsChangedEventType(StrEnum):
    CHANNEL_SETTINGS_CHANGED = "channel.settings_changed"

    def __str__(self) -> str:
        return str(self.value)
