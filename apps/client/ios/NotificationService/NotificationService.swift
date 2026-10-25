// SPDX-License-Identifier: Apache-2.0

import ClientKit

/// The Notification Service Extension of the iOS app (brief M5-04 Design note 4; ADR 0056
/// item 1): Swift without the Flutter engine. All logic is `ClientKitNotificationService` of
/// ClientKit, which the tests run as it is: it fetches the preview of each push with the
/// preview token and the `dpop` device key within `client.push.fetch_timeout`, and otherwise
/// keeps the placeholder.
final class NotificationService: ClientKitNotificationService {}
