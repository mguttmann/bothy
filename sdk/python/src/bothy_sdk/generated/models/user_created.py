from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.enrollment_code import EnrollmentCode
    from ..models.user import User


T = TypeVar("T", bound="UserCreated")


@_attrs_define
class UserCreated:
    """
    Attributes:
        user (User): Directory entry of a person.
        enrollment (EnrollmentCode | Unset): One-time code shown once to the owner, handed to the person out of band. A
            replay of the creating request with the same `Idempotency-Key` does not repeat it (`409 conflict`, `reason:
            one_time_secret`).
    """

    user: User
    enrollment: EnrollmentCode | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        user = self.user.to_dict()

        enrollment: dict[str, Any] | Unset = UNSET
        if not isinstance(self.enrollment, Unset):
            enrollment = self.enrollment.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "user": user,
            }
        )
        if enrollment is not UNSET:
            field_dict["enrollment"] = enrollment

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.enrollment_code import EnrollmentCode
        from ..models.user import User

        d = dict(src_dict)
        user = User.from_dict(d.pop("user"))

        _enrollment = d.pop("enrollment", UNSET)
        enrollment: EnrollmentCode | Unset
        if isinstance(_enrollment, Unset):
            enrollment = UNSET
        else:
            enrollment = EnrollmentCode.from_dict(_enrollment)

        user_created = cls(
            user=user,
            enrollment=enrollment,
        )

        user_created.additional_properties = d
        return user_created

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
