from enum import StrEnum


class EnrollmentCodePurpose(StrEnum):
    FIRST_LOGIN = "first_login"
    IDENTITY_LINK = "identity_link"
    RECOVERY = "recovery"

    def __str__(self) -> str:
        return str(self.value)
