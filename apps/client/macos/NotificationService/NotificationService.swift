// SPDX-License-Identifier: Apache-2.0

import ClientKit

/// The Notification Service Extension of the macOS app (brief M5-04 Design note 4; ADR 0056
/// item 1; docs/spec/13-push-notifications.md 5.1): the same class as on iOS, without the
/// Flutter engine; all logic is `ClientKitNotificationService` of ClientKit.
final class NotificationService: ClientKitNotificationService {}
