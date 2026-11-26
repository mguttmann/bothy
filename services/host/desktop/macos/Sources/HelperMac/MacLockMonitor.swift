import AppKit
import CoreGraphics
import Foundation
import HelperCore

/// Lock, screen saver, user switch and display sleep (27 8.2, 9.1).
///
/// Two sources together: the notifications (`com.apple.screenIsLocked` and `...Unlocked`, the
/// screen saver's `didstart` and `didstop`, `NSWorkspace` session resign and become active,
/// screens did sleep and wake) and the session dictionary of `CGSessionCopyCurrentDictionary`,
/// read on every check. Which of the notifications are reliable on current macOS is a probe of
/// 27 section 16; reading the dictionary on every request keeps the gate correct even when a
/// notification is missed.
public final class MacLockMonitor: LockMonitor {
    private let dispatch: (@escaping () -> Void) -> Void
    private let interrupt: ActionInterrupt
    private var observers: [(NotificationCenter, NSObjectProtocol)] = []
    private var screenLocked = false
    private var screenSaver = false
    private var sessionInactive = false
    private var displayAsleep = false
    private let lock = NSLock()

    /// Creates the monitor.
    /// - Parameters:
    ///   - interrupt: told the lock state at once, so a running action stops between two steps.
    ///   - dispatch: runs a closure asynchronously in the controller's context.
    public init(interrupt: ActionInterrupt, dispatch: @escaping (@escaping () -> Void) -> Void) {
        self.interrupt = interrupt
        self.dispatch = dispatch
    }

    public func start(onChange: @escaping () -> Void) {
        let distributed = DistributedNotificationCenter.default()
        let workspace = NSWorkspace.shared.notificationCenter
        let watch: [(NotificationCenter, Notification.Name, (MacLockMonitor) -> Void)] = [
            (distributed, .init("com.apple.screenIsLocked"), { $0.screenLocked = true }),
            (distributed, .init("com.apple.screenIsUnlocked"), { $0.screenLocked = false }),
            (distributed, .init("com.apple.screensaver.didstart"), { $0.screenSaver = true }),
            (distributed, .init("com.apple.screensaver.didstop"), { $0.screenSaver = false }),
            (workspace, NSWorkspace.sessionDidResignActiveNotification, { $0.sessionInactive = true }),
            (workspace, NSWorkspace.sessionDidBecomeActiveNotification, { $0.sessionInactive = false }),
            (workspace, NSWorkspace.screensDidSleepNotification, { $0.displayAsleep = true }),
            (workspace, NSWorkspace.screensDidWakeNotification, { $0.displayAsleep = false }),
        ]
        // Distributed notifications arrive through the run loop of the registering thread; the
        // controller's queue has none, so the observers live on the main thread.
        MainThread.sync {
            for (center, name, update) in watch {
                let token = center.addObserver(forName: name, object: nil, queue: nil) {
                    [weak self] _ in
                    guard let self else { return }
                    self.lock.withLock { update(self) }
                    self.interrupt.setLocked(self.isLocked())
                    self.dispatch(onChange)
                }
                observers.append((center, token))
            }
        }
    }

    public func stop() {
        MainThread.sync {
            for (center, token) in observers { center.removeObserver(token) }
            observers.removeAll()
        }
    }

    public func isLocked() -> Bool {
        let flagged = lock.withLock { screenLocked || screenSaver || sessionInactive || displayAsleep }
        return flagged || Self.sessionDictionarySaysLocked()
    }

    /// Reads the console session: locked when the screen lock flag is set or the session is not
    /// on the console (another user is switched in). `CGSSessionScreenIsLocked` is not a
    /// documented constant; its absence reads as unlocked.
    static func sessionDictionarySaysLocked() -> Bool {
        guard let dictionary = CGSessionCopyCurrentDictionary() as? [String: Any] else {
            return false
        }
        if (dictionary["CGSSessionScreenIsLocked"] as? Bool) == true { return true }
        if let onConsole = dictionary[kCGSessionOnConsoleKey as String] as? Bool, !onConsole {
            return true
        }
        return false
    }
}
