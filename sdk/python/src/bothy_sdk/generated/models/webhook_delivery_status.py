from enum import StrEnum


class WebhookDeliveryStatus(StrEnum):
    DEAD = "dead"
    FAILED = "failed"
    PENDING = "pending"
    SUCCEEDED = "succeeded"

    def __str__(self) -> str:
        return str(self.value)
