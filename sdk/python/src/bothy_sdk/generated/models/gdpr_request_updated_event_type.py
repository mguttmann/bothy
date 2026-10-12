from enum import StrEnum


class GdprRequestUpdatedEventType(StrEnum):
    GDPR_REQUEST_UPDATED = "gdpr.request_updated"

    def __str__(self) -> str:
        return str(self.value)
