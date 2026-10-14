from enum import StrEnum


class GroupSource(StrEnum):
    LOCAL = "local"
    OIDC_CLAIM = "oidc_claim"

    def __str__(self) -> str:
        return str(self.value)
