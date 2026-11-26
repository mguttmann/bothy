import Foundation

/// The generation of the Esc tap (27 9.6 item 5): every start raises it, and a tap's callback or
/// teardown acts only while its generation is the current one, checked and done under one lock.
/// So the late end of an older tap (a callback that fires after the next start) never removes
/// or drives a newer one.
///
/// `Value` is what one tap holds (the macOS tap: its port, run loop and handler; the test
/// fake: its handler).
///
/// Example:
/// ```swift
/// let generation = slot.begin(tap)          // start
/// if let tap = slot.end(generation: generation) { release(tap) }  // its own teardown only
/// ```
public final class TapGeneration<Value>: @unchecked Sendable {
    private let lock = NSLock()
    private var current: Value?
    private var number = 0

    /// Creates an empty slot; the first generation is 1, so 0 never names a tap.
    public init() {}

    /// The number of the latest generation.
    public var generation: Int { lock.withLock { number } }

    /// The value of the current tap, if one is installed.
    public var currentValue: Value? { lock.withLock { current } }

    /// Installs a new tap.
    /// - Parameter value: what the tap holds.
    /// - Returns: its generation.
    public func begin(_ value: Value) -> Int {
        lock.withLock {
            number += 1
            current = value
            return number
        }
    }

    /// The value of the tap of `generation`, nil when that tap is no longer the current one.
    /// A callback uses it to find its handler.
    public func value(for generation: Int) -> Value? {
        lock.withLock { generation == number ? current : nil }
    }

    /// Removes the tap of `generation` when it is still the current one.
    /// - Returns: its value, for the caller to release; nil when a newer tap is current or the
    ///   tap was already removed.
    public func end(generation: Int) -> Value? {
        lock.withLock {
            guard generation == number, let value = current else { return nil }
            current = nil
            return value
        }
    }

    /// Removes the current tap, whatever its generation.
    /// - Returns: its value, nil when none is installed.
    public func endCurrent() -> Value? {
        lock.withLock {
            defer { current = nil }
            return current
        }
    }
}
