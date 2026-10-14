from enum import StrEnum


class SettingsUpdatedEventType(StrEnum):
    SETTINGS_UPDATED = "settings.updated"

    def __str__(self) -> str:
        return str(self.value)
