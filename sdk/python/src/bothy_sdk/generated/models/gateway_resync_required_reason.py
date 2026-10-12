from enum import StrEnum


class GatewayResyncRequiredReason(StrEnum):
    CURSOR_AHEAD = "cursor_ahead"
    EPOCH_CHANGED = "epoch_changed"
    RETENTION = "retention"
    TOO_FAR_BEHIND = "too_far_behind"

    def __str__(self) -> str:
        return str(self.value)
