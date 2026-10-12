from enum import StrEnum


class IdentityProviderKind(StrEnum):
    LOCAL = "local"
    OIDC = "oidc"
    TRUSTED_HEADER = "trusted_header"

    def __str__(self) -> str:
        return str(self.value)
