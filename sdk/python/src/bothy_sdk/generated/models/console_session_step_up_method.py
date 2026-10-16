from enum import StrEnum


class ConsoleSessionStepUpMethod(StrEnum):
    OIDC_REAUTH = "oidc_reauth"
    PASSKEY = "passkey"
    PASSWORD_TOTP = "password_totp"

    def __str__(self) -> str:
        return str(self.value)
