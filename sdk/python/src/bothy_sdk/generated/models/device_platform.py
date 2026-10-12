from enum import StrEnum


class DevicePlatform(StrEnum):
    ANDROID = "android"
    CLI = "cli"
    IOS = "ios"
    MACOS = "macos"
    WEB = "web"
    WINDOWS = "windows"

    def __str__(self) -> str:
        return str(self.value)
