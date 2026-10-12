from enum import StrEnum


class TokenResponseTokenType(StrEnum):
    DPOP = "DPoP"

    def __str__(self) -> str:
        return str(self.value)
