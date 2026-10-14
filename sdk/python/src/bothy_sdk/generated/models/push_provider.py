from enum import StrEnum


class PushProvider(StrEnum):
    APNS = "apns"
    FCM = "fcm"
    UNIFIEDPUSH = "unifiedpush"
    WNS = "wns"

    def __str__(self) -> str:
        return str(self.value)
