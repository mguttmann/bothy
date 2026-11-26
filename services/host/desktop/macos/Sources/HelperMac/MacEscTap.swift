import CoreGraphics
import Foundation
import HelperCore

/// The Esc tap (27 8.4 item 4, 9.1): an active `CGEventTap` on the session that swallows the key
/// down and key up of a real Esc while a session is shown and lets every other event pass.
///
/// An Esc counts as injected when it carries `MacInjector.marker` in `eventSourceUserData` or
/// the state ID of the helper's own event source; such an Esc passes and stops nothing. The tap
/// answers at once on its own thread with `HelperController.escDecision(injected:)` and hands the
/// end of the session to the controller's context, because a slow tap is disabled by macOS.
/// While Secure Input is on, macOS shows the tap no keys (27 9.5); the stop button stays.
public final class MacEscTap: EscTap {
    private let dispatch: (@escaping () -> Void) -> Void
    private let ownSourceStateID: Int64?
    private var thread: Thread?
    private var runLoop: CFRunLoop?
    private var tap: CFMachPort?
    fileprivate var handler: ((Bool) -> EscDecision)?
    /// Whether the key down of the current real Esc was swallowed, so its key up is too.
    fileprivate var swallowingUp = false

    /// Creates the tap.
    /// - Parameters:
    ///   - ownSourceStateID: the state ID of `MacInjector`'s event source.
    ///   - dispatch: runs a closure asynchronously in the controller's context.
    public init(ownSourceStateID: Int64?, dispatch: @escaping (@escaping () -> Void) -> Void) {
        self.ownSourceStateID = ownSourceStateID
        self.dispatch = dispatch
    }

    public func start(handler: @escaping (Bool) -> EscDecision) throws {
        stop()
        self.handler = handler
        let mask =
            (1 << CGEventType.keyDown.rawValue) | (1 << CGEventType.keyUp.rawValue)
        guard
            let port = CGEvent.tapCreate(
                tap: .cgSessionEventTap, place: .headInsertEventTap, options: .defaultTap,
                eventsOfInterest: CGEventMask(mask), callback: escTapCallback,
                userInfo: Unmanaged.passUnretained(self).toOpaque())
        else {
            self.handler = nil
            throw EscTapError.refused
        }
        tap = port
        let ready = DispatchSemaphore(value: 0)
        let thread = Thread { [weak self] in
            guard let self, let source = CFMachPortCreateRunLoopSource(nil, port, 0) else {
                ready.signal()
                return
            }
            self.runLoop = CFRunLoopGetCurrent()
            CFRunLoopAddSource(CFRunLoopGetCurrent(), source, .commonModes)
            CGEvent.tapEnable(tap: port, enable: true)
            ready.signal()
            CFRunLoopRun()
        }
        thread.name = "esc-tap"
        self.thread = thread
        thread.start()
        ready.wait()
    }

    public func stop() {
        if let tap {
            CGEvent.tapEnable(tap: tap, enable: false)
            CFMachPortInvalidate(tap)
        }
        if let runLoop { CFRunLoopStop(runLoop) }
        tap = nil
        runLoop = nil
        thread = nil
        handler = nil
        swallowingUp = false
    }

    /// Whether an event comes from the helper itself.
    fileprivate func isInjected(_ event: CGEvent) -> Bool {
        if event.getIntegerValueField(.eventSourceUserData) == MacInjector.marker { return true }
        if let ownSourceStateID,
            event.getIntegerValueField(.eventSourceStateID) == ownSourceStateID
        {
            return true
        }
        return false
    }

    fileprivate func reenable() {
        if let tap { CGEvent.tapEnable(tap: tap, enable: true) }
    }

    fileprivate func escapeDown(injected: Bool) -> EscDecision {
        let decision = HelperController.escDecision(injected: injected)
        if decision == .swallow, let handler {
            dispatch { _ = handler(false) }
        }
        return decision
    }
}

/// Errors of the tap.
public enum EscTapError: Error {
    /// macOS refused the tap; it needs the accessibility grant.
    case refused
}

private let escapeKeyCode: Int64 = 53  // kVK_Escape

private func escTapCallback(
    proxy: CGEventTapProxy, type: CGEventType, event: CGEvent, userInfo: UnsafeMutableRawPointer?
) -> Unmanaged<CGEvent>? {
    guard let userInfo else { return Unmanaged.passUnretained(event) }
    let tap = Unmanaged<MacEscTap>.fromOpaque(userInfo).takeUnretainedValue()
    switch type {
    case .tapDisabledByTimeout, .tapDisabledByUserInput:
        tap.reenable()
        return Unmanaged.passUnretained(event)
    case .keyDown, .keyUp:
        guard event.getIntegerValueField(.keyboardEventKeycode) == escapeKeyCode,
            !tap.isInjected(event)
        else { return Unmanaged.passUnretained(event) }
        if type == .keyDown {
            if event.getIntegerValueField(.keyboardEventAutorepeat) != 0 { return nil }
            tap.swallowingUp = true
            return tap.escapeDown(injected: false) == .swallow ? nil : Unmanaged.passUnretained(event)
        }
        // The key up of a swallowed Esc must not reach the app alone.
        if tap.swallowingUp {
            tap.swallowingUp = false
            return nil
        }
        return Unmanaged.passUnretained(event)
    default:
        return Unmanaged.passUnretained(event)
    }
}
