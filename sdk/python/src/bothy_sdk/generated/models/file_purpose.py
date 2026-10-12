from enum import StrEnum


class FilePurpose(StrEnum):
    ATTACHMENT = "attachment"
    AVATAR = "avatar"
    BRANDING = "branding"
    GDPR_EXPORT = "gdpr_export"

    def __str__(self) -> str:
        return str(self.value)
