import Foundation

/// The state of an Esc tap (27 9.6 item 4): which Esc events to swallow and which one stops the
/// session. Shared by the macOS tap and the test fakes, so both follow one rule.
///
/// - An injected Esc always passes and changes nothing.
/// - A real Esc key down while the tap runs stops the session once and is swallowed; the key
///   stays "held", so its auto-repeat and its key up are swallowed without a second stop, even
///   after the session ended and the tap was told to stop.
/// - `start()` begins fresh (a new session never inherits a held Esc); `stop()` forgets
///   everything except the rest of a held Esc, which it still swallows up to its key up.
///
/// Thread-safe: the platform tap calls it on its own thread, the controller starts and stops it.
///
/// Example:
/// ```swift
/// let verdict = filter.handle(down: true, injected: false)  // (.swallow, stopsSession: true)
/// ```
public final class EscFilter: @unchecked Sendable {
    private let lock = NSLock()
    private var active = false
    private var held = false

    /// Creates a stopped filter.
    public init() {}

    /// The tap starts for a session: begin fresh.
    public func start() {
        lock.withLock {
            active = true
            held = false
        }
    }

    /// The tap stops: no new press stops anything; the rest of a held Esc is still swallowed.
    public func stop() {
        lock.withLock { active = false }
    }

    /// Whether the tap must keep running because a held Esc still has events to swallow.
    public var draining: Bool {
        lock.withLock { !active && held }
    }

    /// Judges one Esc event.
    /// - Parameters:
    ///   - down: key down (an auto-repeat is a key down while held) or key up.
    ///   - injected: whether the helper itself sent it.
    /// - Returns: what the tap does with the event, and whether it stops the session.
    public func handle(down: Bool, injected: Bool) -> (decision: EscDecision, stopsSession: Bool) {
        if injected { return (.pass, false) }
        return lock.withLock {
            if down {
                if held { return (.swallow, false) }
                guard active else { return (.pass, false) }
                held = true
                return (.swallow, true)
            }
            guard held else { return (.pass, false) }
            held = false
            return (.swallow, false)
        }
    }
}
