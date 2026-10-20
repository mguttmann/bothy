from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..models.incoming_webhook_post_status import IncomingWebhookPostStatus
from ..models.incoming_webhook_post_version import IncomingWebhookPostVersion
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.alertmanager_alert import AlertmanagerAlert
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="IncomingWebhookPost")


@_attrs_define
class IncomingWebhookPost:
    """Body of an incoming webhook call in one of two shapes, recognised per request (11 8.2,
    docs/DECISIONS.md D-M2-27-5). A body with at least one Alertmanager member (`version`,
    `groupKey`, `truncatedAlerts`, `status`, `receiver`, `groupLabels`, `commonLabels`,
    `commonAnnotations`, `externalURL`, `alerts`) is the unchanged webhook payload of Prometheus
    Alertmanager (format `alertmanager`) and must contain no native member; every other body is
    `native` and needs `content_md`. Unknown members are refused.

        Attributes:
            content_md (str | Unset): Native. Markdown content, at most `message.max_content_bytes` UTF-8 bytes.
            client_msg_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            title (str | Unset): Native. Title of a new thread when the webhook has no fixed thread; required in a forum.
            thread_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            version (IncomingWebhookPostVersion | Unset): Alertmanager. Version of the webhook payload.
            group_key (str | Unset): Alertmanager. Key of the alert group.
            truncated_alerts (int | Unset): Alertmanager. Number of alerts left out of `alerts`.
            status (IncomingWebhookPostStatus | Unset): Alertmanager. State of the group.
            receiver (str | Unset): Alertmanager. Name of the receiver.
            group_labels (JsonObject | Unset): Free-form JSON object whose schema is named in the field description.
            common_labels (JsonObject | Unset): Free-form JSON object whose schema is named in the field description.
            common_annotations (JsonObject | Unset): Free-form JSON object whose schema is named in the field description.
            external_url (str | Unset): Alertmanager. Address of the sending Alertmanager; written as text, never fetched.
            alerts (list[AlertmanagerAlert] | Unset): Alertmanager. The alerts of the notification.
    """

    content_md: str | Unset = UNSET
    client_msg_id: UUID | Unset = UNSET
    title: str | Unset = UNSET
    thread_id: UUID | Unset = UNSET
    version: IncomingWebhookPostVersion | Unset = UNSET
    group_key: str | Unset = UNSET
    truncated_alerts: int | Unset = UNSET
    status: IncomingWebhookPostStatus | Unset = UNSET
    receiver: str | Unset = UNSET
    group_labels: JsonObject | Unset = UNSET
    common_labels: JsonObject | Unset = UNSET
    common_annotations: JsonObject | Unset = UNSET
    external_url: str | Unset = UNSET
    alerts: list[AlertmanagerAlert] | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        content_md = self.content_md

        client_msg_id: str | Unset = UNSET
        if not isinstance(self.client_msg_id, Unset):
            client_msg_id = str(self.client_msg_id)

        title = self.title

        thread_id: str | Unset = UNSET
        if not isinstance(self.thread_id, Unset):
            thread_id = str(self.thread_id)

        version: str | Unset = UNSET
        if not isinstance(self.version, Unset):
            version = self.version.value

        group_key = self.group_key

        truncated_alerts = self.truncated_alerts

        status: str | Unset = UNSET
        if not isinstance(self.status, Unset):
            status = self.status.value

        receiver = self.receiver

        group_labels: dict[str, Any] | Unset = UNSET
        if not isinstance(self.group_labels, Unset):
            group_labels = self.group_labels.to_dict()

        common_labels: dict[str, Any] | Unset = UNSET
        if not isinstance(self.common_labels, Unset):
            common_labels = self.common_labels.to_dict()

        common_annotations: dict[str, Any] | Unset = UNSET
        if not isinstance(self.common_annotations, Unset):
            common_annotations = self.common_annotations.to_dict()

        external_url = self.external_url

        alerts: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.alerts, Unset):
            alerts = []
            for alerts_item_data in self.alerts:
                alerts_item = alerts_item_data.to_dict()
                alerts.append(alerts_item)

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if content_md is not UNSET:
            field_dict["content_md"] = content_md
        if client_msg_id is not UNSET:
            field_dict["client_msg_id"] = client_msg_id
        if title is not UNSET:
            field_dict["title"] = title
        if thread_id is not UNSET:
            field_dict["thread_id"] = thread_id
        if version is not UNSET:
            field_dict["version"] = version
        if group_key is not UNSET:
            field_dict["groupKey"] = group_key
        if truncated_alerts is not UNSET:
            field_dict["truncatedAlerts"] = truncated_alerts
        if status is not UNSET:
            field_dict["status"] = status
        if receiver is not UNSET:
            field_dict["receiver"] = receiver
        if group_labels is not UNSET:
            field_dict["groupLabels"] = group_labels
        if common_labels is not UNSET:
            field_dict["commonLabels"] = common_labels
        if common_annotations is not UNSET:
            field_dict["commonAnnotations"] = common_annotations
        if external_url is not UNSET:
            field_dict["externalURL"] = external_url
        if alerts is not UNSET:
            field_dict["alerts"] = alerts

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.alertmanager_alert import AlertmanagerAlert
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        content_md = d.pop("content_md", UNSET)

        _client_msg_id = d.pop("client_msg_id", UNSET)
        client_msg_id: UUID | Unset
        if isinstance(_client_msg_id, Unset):
            client_msg_id = UNSET
        else:
            client_msg_id = UUID(_client_msg_id)

        title = d.pop("title", UNSET)

        _thread_id = d.pop("thread_id", UNSET)
        thread_id: UUID | Unset
        if isinstance(_thread_id, Unset):
            thread_id = UNSET
        else:
            thread_id = UUID(_thread_id)

        _version = d.pop("version", UNSET)
        version: IncomingWebhookPostVersion | Unset
        if isinstance(_version, Unset):
            version = UNSET
        else:
            version = IncomingWebhookPostVersion(_version)

        group_key = d.pop("groupKey", UNSET)

        truncated_alerts = d.pop("truncatedAlerts", UNSET)

        _status = d.pop("status", UNSET)
        status: IncomingWebhookPostStatus | Unset
        if isinstance(_status, Unset):
            status = UNSET
        else:
            status = IncomingWebhookPostStatus(_status)

        receiver = d.pop("receiver", UNSET)

        _group_labels = d.pop("groupLabels", UNSET)
        group_labels: JsonObject | Unset
        if isinstance(_group_labels, Unset):
            group_labels = UNSET
        else:
            group_labels = JsonObject.from_dict(_group_labels)

        _common_labels = d.pop("commonLabels", UNSET)
        common_labels: JsonObject | Unset
        if isinstance(_common_labels, Unset):
            common_labels = UNSET
        else:
            common_labels = JsonObject.from_dict(_common_labels)

        _common_annotations = d.pop("commonAnnotations", UNSET)
        common_annotations: JsonObject | Unset
        if isinstance(_common_annotations, Unset):
            common_annotations = UNSET
        else:
            common_annotations = JsonObject.from_dict(_common_annotations)

        external_url = d.pop("externalURL", UNSET)

        _alerts = d.pop("alerts", UNSET)
        alerts: list[AlertmanagerAlert] | Unset = UNSET
        if _alerts is not UNSET:
            alerts = []
            for alerts_item_data in _alerts:
                alerts_item = AlertmanagerAlert.from_dict(alerts_item_data)

                alerts.append(alerts_item)

        incoming_webhook_post = cls(
            content_md=content_md,
            client_msg_id=client_msg_id,
            title=title,
            thread_id=thread_id,
            version=version,
            group_key=group_key,
            truncated_alerts=truncated_alerts,
            status=status,
            receiver=receiver,
            group_labels=group_labels,
            common_labels=common_labels,
            common_annotations=common_annotations,
            external_url=external_url,
            alerts=alerts,
        )

        return incoming_webhook_post
