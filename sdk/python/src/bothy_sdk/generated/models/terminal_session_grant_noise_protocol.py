from enum import StrEnum


class TerminalSessionGrantNoiseProtocol(StrEnum):
    NOISE_NK_25519_CHACHAPOLY_SHA256 = "Noise_NK_25519_ChaChaPoly_SHA256"

    def __str__(self) -> str:
        return str(self.value)
