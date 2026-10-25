// SPDX-License-Identifier: Apache-2.0

import Foundation
import UserNotifications

/// Something that holds the delegate of the notification center; `UNUserNotificationCenter` in
/// the app, a plain object in the tests (the real center needs an app bundle).
public protocol NotificationDelegateHolder: AnyObject {
    /// The current delegate.
    var delegate: UNUserNotificationCenterDelegate? { get set }
}

extension UNUserNotificationCenter: NotificationDelegateHolder {}

/// The app's delegate of the notification center (brief M5-04 Design note 5;
/// docs/decisions.d/M5-04.md D-M5-04-9).
///
/// The center has one delegate, and other parts of the app need it too: on macOS
/// `flutter_local_notifications` makes itself the delegate when it registers, and its clicks open
/// the thread of a local notification (M4-30); on iOS the Flutter app delegate hands the
/// callbacks to its plugins. So the router keeps the delegate it replaces and handles only
/// pushes, the notifications whose `userInfo` carries the object `b` of
/// docs/spec/13-push-notifications.md 6.1; every other notification goes to that delegate
/// unchanged.
public final class NotificationCenterRouter: NSObject, UNUserNotificationCenterDelegate {
    /// The delegate the router replaced, which receives every notification that is no push.
    public private(set) var previous: UNUserNotificationCenterDelegate?
    private let onTap: (PushPayload) -> Void

    /// Creates the router.
    ///
    /// - Parameter onTap: receives the payload of a push the person opened.
    public init(onTap: @escaping (PushPayload) -> Void) {
        self.onTap = onTap
    }

    /// Becomes the delegate of `center`, keeping the one it replaces; calling it again after
    /// another component took the center over keeps that component as well, and a second call
    /// while the router is the delegate changes nothing.
    ///
    /// - Parameters:
    ///   - center: the notification center.
    ///   - fallback: the delegate to forward to when the center has none yet (the iOS app
    ///     delegate, which hands the callbacks to the plugins).
    public func install(on center: NotificationDelegateHolder, fallback: UNUserNotificationCenterDelegate? = nil) {
        if center.delegate === self { return }
        if let current = center.delegate {
            previous = current
        } else if previous == nil {
            previous = fallback
        }
        center.delegate = self
    }

    /// Whether a notification with `userInfo` is a push of this app.
    ///
    /// - Parameter userInfo: the notification's `userInfo`.
    /// - Returns: the payload of a push, nil for anything else.
    public static func push(_ userInfo: [AnyHashable: Any]) -> PushPayload? {
        PushPayload.parse(userInfo)
    }

    /// A push the person opened goes to `onTap`; anything else to the previous delegate.
    public func userNotificationCenter(
        _ center: UNUserNotificationCenter, didReceive response: UNNotificationResponse,
        withCompletionHandler completionHandler: @escaping () -> Void
    ) {
        guard let payload = Self.push(response.notification.request.content.userInfo) else {
            let selector = #selector(UNUserNotificationCenterDelegate.userNotificationCenter(_:didReceive:withCompletionHandler:))
            if let previous, previous.responds(to: selector) {
                previous.userNotificationCenter?(center, didReceive: response, withCompletionHandler: completionHandler)
            } else {
                completionHandler()
            }
            return
        }
        if response.actionIdentifier == UNNotificationDefaultActionIdentifier {
            onTap(payload)
        }
        completionHandler()
    }

    /// A push while the app is in front shows as in the background (the server sends none for
    /// the thread the person is active in, 13 4.4); anything else is the previous delegate's.
    public func userNotificationCenter(
        _ center: UNUserNotificationCenter, willPresent notification: UNNotification,
        withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void
    ) {
        guard Self.push(notification.request.content.userInfo) != nil else {
            let selector = #selector(UNUserNotificationCenterDelegate.userNotificationCenter(_:willPresent:withCompletionHandler:))
            if let previous, previous.responds(to: selector) {
                previous.userNotificationCenter?(center, willPresent: notification, withCompletionHandler: completionHandler)
            } else {
                completionHandler([])
            }
            return
        }
        completionHandler([.banner, .list, .sound, .badge])
    }
}
