import CoreGraphics
import Foundation
import HelperCore

/// The Esc tap (27 8.4 item 4, 9.1, 9.6 item 4): an active `CGEventTap` on the session that
/// swallows a real Esc while a session is shown and lets every other event pass.
///
/// An Esc counts as injected when it carries `MacInjector.marker` in `eventSourceUserData` or
/// the state ID of the helper's own event source; such an Esc passes and stops nothing. Which
/// real Esc events are swallowed and which one stops the session is decided by the shared
/// `EscFilter`: one held Esc stops once, its auto-repeat and key up are swallowed, and the state
/// starts fresh with every start. A stop while an Esc is still held keeps the tap until that
/// key up, so the rest of the held Esc never reaches the app. The tap answers at once on its own
/// thread, sets the interrupt flag and hands the end of the session to the controller's context,
/// because a slow tap is disabled by macOS. While Secure Input is on, macOS shows the tap no
/// keys (27 9.5); the stop button stays.
public final class MacEscTap: EscTap {
    private let dispatch: (@escaping () -> Void) -> Void
    private let ownSourceStateID: Int64?
    private let interrupt: ActionInterrupt
    private let filter = EscFilter()
    private let lock = NSLock()
    private var runLoop: CFRunLoop?
    private var tap: CFMachPort?
    private var handler: ((Bool) -> EscDecision)?

    /// Creates the tap.
    /// - Parameters:
    ///   - ownSourceStateID: the state ID of `MacInjector`'s event source.
    ///   - interrupt: set at once on a real Esc, so a running action stops between two units.
    ///   - dispatch: runs a closure asynchronously in the controller's context.
    public init(
        ownSourceStateID: Int64?, interrupt: ActionInterrupt,
        dispatch: @escaping (@escaping () -> Void) -> Void
    ) {
        self.ownSourceStateID = ownSourceStateID
        self.interrupt = interrupt
        self.dispatch = dispatch
    }

    public func start(handler: @escaping (Bool) -> EscDecision) throws {
        tearDown()
        filter.start()
        let mask = (1 << CGEventType.keyDown.rawValue) | (1 << CGEventType.keyUp.rawValue)
        guard
            let port = CGEvent.tapCreate(
                tap: .cgSessionEventTap, place: .headInsertEventTap, options: .defaultTap,
                eventsOfInterest: CGEventMask(mask), callback: escTapCallback,
                userInfo: Unmanaged.passUnretained(self).toOpaque())
        else {
            filter.stop()
            throw EscTapError.refused
        }
        let ready = DispatchSemaphore(value: 0)
        let thread = Thread { [weak self] in
            guard let self, let source = CFMachPortCreateRunLoopSource(nil, port, 0) else {
                ready.signal()
                return
            }
            self.lock.withLock {
                self.runLoop = CFRunLoopGetCurrent()
                self.tap = port
                self.handler = handler
            }
            CFRunLoopAddSource(CFRunLoopGetCurrent(), source, .commonModes)
            CGEvent.tapEnable(tap: port, enable: true)
            ready.signal()
            CFRunLoopRun()
        }
        thread.name = "esc-tap"
        thread.start()
        ready.wait()
    }

    public func stop() {
        filter.stop()
        lock.withLock { handler = nil }
        // A held Esc keeps the tap until its key up (EscFilter.draining).
        if !filter.draining { tearDown() }
    }

    private func tearDown() {
        let (port, loop) = lock.withLock { () -> (CFMachPort?, CFRunLoop?) in
            defer {
                tap = nil
                runLoop = nil
                handler = nil
            }
            return (tap, runLoop)
        }
        if let port {
            CGEvent.tapEnable(tap: port, enable: false)
            CFMachPortInvalidate(port)
        }
        if let loop { CFRunLoopStop(loop) }
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
        if let port = lock.withLock({ tap }) { CGEvent.tapEnable(tap: port, enable: true) }
    }

    /// Judges one Esc event on the tap's thread.
    fileprivate func escape(down: Bool, injected: Bool) -> EscDecision {
        let verdict = filter.handle(down: down, injected: injected)
        if verdict.stopsSession, let handler = lock.withLock({ handler }) {
            // The controller's queue may be busy with a long action; the flag stops it now.
            interrupt.requestStop(.localEscape)
            dispatch { _ = handler(false) }
        }
        // The key up that ends a held Esc after a stop lets the tap go.
        if !down, lock.withLock({ handler == nil }), !filter.draining { tearDown() }
        return verdict.decision
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
        guard event.getIntegerValueField(.keyboardEventKeycode) == escapeKeyCode else {
            return Unmanaged.passUnretained(event)
        }
        let decision = tap.escape(down: type == .keyDown, injected: tap.isInjected(event))
        return decision == .swallow ? nil : Unmanaged.passUnretained(event)
    default:
        return Unmanaged.passUnretained(event)
    }
}
