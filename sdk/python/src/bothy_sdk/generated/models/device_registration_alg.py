from enum import StrEnum


class DeviceRegistrationAlg(StrEnum):
    EDDSA = "EdDSA"
    ES256 = "ES256"

    def __str__(self) -> str:
        return str(self.value)
