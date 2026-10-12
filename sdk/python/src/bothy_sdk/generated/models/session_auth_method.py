from enum import StrEnum


class SessionAuthMethod(StrEnum):
    DEVICE_PAIRING = "device_pairing"
    ENROLLMENT_CODE = "enrollment_code"
    OIDC = "oidc"
    PASSKEY = "passkey"
    PASSWORD_TOTP = "password_totp"
    TRUSTED_HEADER = "trusted_header"

    def __str__(self) -> str:
        return str(self.value)
