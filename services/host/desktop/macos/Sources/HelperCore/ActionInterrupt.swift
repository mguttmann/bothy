import Foundation

/// Bounds the helper applies itself, so a request can never make it inject for long without a
/// check (coordinator security finding 1 on CU-02).
public enum Limits {
    /// The highest `repeat` of `do key`: the default of `computer.key_repeat_max`
    /// (docs/spec/27-computer-use.md section 18). The host service enforces the configured
    /// value; the helper never sees the server's configuration and holds the default as an
    /// outer bound.
    public static let keyRepeatMax = 50
}

/// A flag that a running action checks between its steps, set from any thread.
///
/// The controller's queue is busy while an action runs, so a stop cannot wait for it: the Esc
/// tap and the stop button set the flag on their own threads, the lock monitor sets it on a
/// lock, and the channel reader sets it for `hide`, `disable` and a broken channel before it
/// queues them. The controller then releases every key and button and ends the action.
///
/// Example:
/// ```swift
/// controller.interrupt.requestStop(.localEscape)  // from the tap thread
/// ```
public final class ActionInterrupt: @unchecked Sendable {
    /// Why an action must stop.
    public enum Reason: Equatable, Sendable {
        /// The person stopped the session (Esc or stop button); the session ends with `stop`.
        case person(StopDetail)
        /// The session ends by the host or the channel (`hide`, `disable`, broken channel).
        case ended
        /// The screen locked; the session pauses (27 8.2).
        case locked
    }

    private let lock = NSLock()
    private var stop: Reason?
    private var screenLocked = false
    private var sessionID: String?

    /// Creates a clear flag.
    public init() {}

    /// The reason to stop now, or nil to go on. A stop or end wins over a lock.
    public var reason: Reason? {
        lock.withLock { stop ?? (screenLocked ? .locked : nil) }
    }

    /// The person stopped the session. Any thread.
    public func requestStop(_ detail: StopDetail) {
        lock.withLock { if stop == nil { stop = .person(detail) } }
    }

    /// The session ends by the host or the channel. Any thread.
    public func requestEnd() {
        lock.withLock { if stop == nil { stop = .ended } }
    }

    /// Sets the lock state. Any thread.
    public func setLocked(_ locked: Bool) {
        lock.withLock { screenLocked = locked }
    }

    /// Looks at a host message before it is queued: a `hide` of the shown session and every
    /// `disable` interrupt a running action. Called by the channel reader.
    /// - Parameter message: the message as received.
    public func note(_ message: JSONValue) {
        switch message["op"]?.stringValue {
        case "disable":
            requestEnd()
        case "hide":
            let matches = lock.withLock { sessionID != nil && sessionID == message["session_id"]?.stringValue }
            if matches { requestEnd() }
        default:
            break
        }
    }

    /// A new session starts: earlier stop requests no longer apply. Called by the controller.
    func begin(sessionID: String) {
        lock.withLock {
            stop = nil
            self.sessionID = sessionID
        }
    }

    /// The session ended. Called by the controller.
    func finish() {
        lock.withLock { sessionID = nil }
    }
}
