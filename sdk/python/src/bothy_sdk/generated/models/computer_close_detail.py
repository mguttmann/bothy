from enum import StrEnum


class ComputerCloseDetail(StrEnum):
    BUSY = "busy"
    CLIENT_STOP = "client_stop"
    HANDSHAKE_FAILED = "handshake_failed"
    HELPER_ABSENT = "helper_absent"
    HELPER_CRASHED = "helper_crashed"
    HELPER_UNTRUSTED = "helper_untrusted"
    INDICATOR_STOP = "indicator_stop"
    LOCAL_ESCAPE = "local_escape"
    NOT_ENABLED = "not_enabled"
    OPEN_TIMEOUT = "open_timeout"
    STREAM_FAULT = "stream_fault"
    TOKEN_EXPIRED = "token_expired"
    TOKEN_INVALID = "token_invalid"
    TOKEN_REPLAYED = "token_replayed"

    def __str__(self) -> str:
        return str(self.value)
