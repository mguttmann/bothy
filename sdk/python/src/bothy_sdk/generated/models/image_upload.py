from __future__ import annotations

from collections.abc import Mapping
from io import BytesIO
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from .. import types
from ..types import File

T = TypeVar("T", bound="ImageUpload")


@_attrs_define
class ImageUpload:
    """Image as the one multipart part `file` (a further part is rejected with 400 `validation_failed`). The server detects
    the type from the bytes; it accepts only JPEG and PNG, whose metadata it removes losslessly, and still WebP, which
    it converts to PNG or JPEG without metadata. Thumbnails are re-encoded. Any other detected type ends the file
    `failed` with `file_type_denied`.

        Attributes:
            file (File):
    """

    file: File

    def to_dict(self) -> dict[str, Any]:
        file = self.file.to_tuple()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "file": file,
            }
        )

        return field_dict

    def to_multipart(self) -> types.RequestFiles:
        files: types.RequestFiles = []

        files.append(("file", self.file.to_tuple()))

        return files

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        file = File(payload=BytesIO(d.pop("file")))

        image_upload = cls(
            file=file,
        )

        return image_upload
