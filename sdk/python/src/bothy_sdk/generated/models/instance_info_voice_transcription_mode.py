from enum import StrEnum


class InstanceInfoVoiceTranscriptionMode(StrEnum):
    AGENT = "agent"
    OFF = "off"
    SERVICE = "service"

    def __str__(self) -> str:
        return str(self.value)
