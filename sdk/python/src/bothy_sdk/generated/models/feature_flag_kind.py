from enum import StrEnum


class FeatureFlagKind(StrEnum):
    OPTION = "option"
    RELEASE = "release"

    def __str__(self) -> str:
        return str(self.value)
