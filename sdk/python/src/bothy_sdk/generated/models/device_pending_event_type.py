from enum import StrEnum


class DevicePendingEventType(StrEnum):
    DEVICE_PENDING = "device.pending"

    def __str__(self) -> str:
        return str(self.value)
