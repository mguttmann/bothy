from enum import StrEnum


class ClientMetricsBatchPlatform(StrEnum):
    ANDROID = "android"
    IOS = "ios"
    MACOS = "macos"
    WEB = "web"
    WINDOWS = "windows"

    def __str__(self) -> str:
        return str(self.value)
