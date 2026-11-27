// SPDX-License-Identifier: Apache-2.0

import CoreGraphics
import Foundation
import HelperCore

/// The Esc tap (27 8.4 item 4, 9.1, 9.6 items 4 and 5): an active `CGEventTap` on the session
/// that swallows a real Esc while a session is shown and lets every other event pass.
///
/// An Esc counts as injected when it carries `MacInjector.marker` in `eventSourceUserData` or
/// the state ID of the helper's own event source; such an Esc passes and stops nothing. Which
/// real Esc events are swallowed and which one stops the session is decided by the shared
/// `EscFilter`: one held Esc stops once, its auto-repeat and key up are swallowed, and the state
/// starts fresh with every start. A stop while an Esc is still held keeps the tap until that
/// key up, so the rest of the held Esc never reaches the app.
///
/// Every start is a new generation (`TapGeneration`); the callback carries its generation and
/// finds its handler, and a teardown removes its tap, only while that generation is current, so
/// a late callback or teardown of an older tap never acts on a newer one. The tap answers at
/// once on its own thread, sets the interrupt flag and hands the end of the session to the
/// controller's context, because a slow tap is disabled by macOS. While Secure Input is on,
/// macOS shows the tap no keys (27 9.5); the stop button stays.
public final class MacEscTap: EscTap {
    /// What one tap generation holds.
    struct Installed {
        let port: CFMachPort
        let runLoop: CFRunLoop
        let handler: (Bool) -> EscDecision
    }

    private let dispatch: (@escaping () -> Void) -> Void
    private let ownSourceStateID: Int64?
    private let interrupt: ActionInterrupt
    private let filter = EscFilter()
    private let generations = TapGeneration<Installed>()

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
        // The C callback finds the tap through this reference and its generation through the
        // user info, so no per-tap object has to outlive a teardown.
        activeEscTap = self
    }

    public func start(handler: @escaping (Bool) -> EscDecision) throws {
        Self.release(generations.endCurrent())
        filter.start()
        let mask = (1 << CGEventType.keyDown.rawValue) | (1 << CGEventType.keyUp.rawValue)
        // The generation this start will get; begin() below hands out exactly this number,
        // because only the controller's context starts taps.
        let generation = generations.generation + 1
        guard
            let port = CGEvent.tapCreate(
                tap: .cgSessionEventTap, place: .headInsertEventTap, options: .defaultTap,
                eventsOfInterest: CGEventMask(mask), callback: escTapCallback,
                userInfo: UnsafeMutableRawPointer(bitPattern: generation))
        else {
            filter.stop()
            throw EscTapError.refused
        }
        let ready = DispatchSemaphore(value: 0)
        let thread = Thread { [generations] in
            guard let source = CFMachPortCreateRunLoopSource(nil, port, 0) else {
                CFMachPortInvalidate(port)
                ready.signal()
                return
            }
            _ = generations.begin(Installed(port: port, runLoop: CFRunLoopGetCurrent(), handler: handler))
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
        // A held Esc keeps the tap until its key up (EscFilter.draining).
        if !filter.draining { Self.release(generations.endCurrent()) }
    }

    private static func release(_ installed: Installed?) {
        guard let installed else { return }
        CGEvent.tapEnable(tap: installed.port, enable: false)
        CFMachPortInvalidate(installed.port)
        CFRunLoopStop(installed.runLoop)
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

    fileprivate func reenable(generation: Int) {
        if let installed = generations.value(for: generation) {
            CGEvent.tapEnable(tap: installed.port, enable: true)
        }
    }

    /// Judges one Esc event on the thread of the tap of `generation`.
    fileprivate func escape(down: Bool, injected: Bool, generation: Int) -> EscDecision {
        // A late callback of an older tap finds no handler and stops nothing.
        guard let installed = generations.value(for: generation) else { return .pass }
        let verdict = filter.handle(down: down, injected: injected)
        if verdict.stopsSession {
            // The controller's queue may be busy with a long action; the flag stops it now.
            interrupt.requestStop(.localEscape)
            let handler = installed.handler
            dispatch { _ = handler(false) }
        }
        // The key up that ends a held Esc after a stop lets this tap go, and only this one.
        if !down, !filter.isActive, !filter.draining {
            Self.release(generations.end(generation: generation))
        }
        return verdict.decision
    }
}

/// Errors of the tap.
public enum EscTapError: Error {
    /// macOS refused the tap; it needs the accessibility grant.
    case refused
}

/// The one Esc tap of the process, for the C callback.
private weak var activeEscTap: MacEscTap?

private let escapeKeyCode: Int64 = 53  // kVK_Escape

private func escTapCallback(
    proxy: CGEventTapProxy, type: CGEventType, event: CGEvent, userInfo: UnsafeMutableRawPointer?
) -> Unmanaged<CGEvent>? {
    guard let tap = activeEscTap, let userInfo else { return Unmanaged.passUnretained(event) }
    let generation = Int(bitPattern: userInfo)
    switch type {
    case .tapDisabledByTimeout, .tapDisabledByUserInput:
        tap.reenable(generation: generation)
        return Unmanaged.passUnretained(event)
    case .keyDown, .keyUp:
        guard event.getIntegerValueField(.keyboardEventKeycode) == escapeKeyCode else {
            return Unmanaged.passUnretained(event)
        }
        let decision = tap.escape(
            down: type == .keyDown, injected: tap.isInjected(event), generation: generation)
        return decision == .swallow ? nil : Unmanaged.passUnretained(event)
    default:
        return Unmanaged.passUnretained(event)
    }
}
