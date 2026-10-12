from enum import StrEnum


class InstanceUpdatedEventType(StrEnum):
    INSTANCE_UPDATED = "instance.updated"

    def __str__(self) -> str:
        return str(self.value)
