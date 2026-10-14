from enum import StrEnum


class GdprRequestDueEventType(StrEnum):
    GDPR_REQUEST_DUE = "gdpr.request_due"

    def __str__(self) -> str:
        return str(self.value)
