from enum import StrEnum


class SigningKeyPurpose(StrEnum):
    ACCESS_TOKEN = "access_token"
    AUDIT_CHECKPOINT = "audit_checkpoint"
    INVOCATION = "invocation"
    TERMINAL = "terminal"
    WEBHOOK_OUT = "webhook_out"

    def __str__(self) -> str:
        return str(self.value)
