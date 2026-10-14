from enum import StrEnum


class PriceUnit(StrEnum):
    CACHE_READ_MTOK = "cache_read_mtok"
    CACHE_WRITE_1H_MTOK = "cache_write_1h_mtok"
    CACHE_WRITE_MTOK = "cache_write_mtok"
    INPUT_MTOK = "input_mtok"
    OUTPUT_MTOK = "output_mtok"
    REQUEST = "request"

    def __str__(self) -> str:
        return str(self.value)
