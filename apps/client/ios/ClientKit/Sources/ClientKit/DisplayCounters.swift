// SPDX-License-Identifier: Apache-2.0

import Foundation

/// The display counters of the extension in the App Group (docs/spec/15-client-architecture.md
/// 17; docs/observability/signals.md 4.6a; brief M5-04 Design notes 1 and 5).
///
/// The extension cannot send metrics itself, so it counts here and the app takes the counts at
/// its next start and submits them as `client.push_display.duration` (histogram) and
/// `client.push.placeholder` (counter). The duration is kept as bucket counts with the upper
/// bounds "durations up to seconds" of signals.md 4.7, the only bounds the server accepts; a
/// value above the top bound counts in the top bucket, as the app's own collector does. Only
/// counts are stored: no time stamps, no IDs.
public final class DisplayCounters {
    /// The upper bounds of the duration buckets in seconds (signals.md 4.7).
    public static let bounds: [Double] = [0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10]

    static let bucketsKey = "clientkit.counters.display_buckets"
    static let placeholderKey = "clientkit.counters.placeholder"

    /// What `take()` returns.
    public struct Snapshot: Equatable, Sendable {
        /// The count per bucket, aligned with `DisplayCounters.bounds`.
        public let buckets: [Int]
        /// The pushes whose placeholder stayed.
        public let placeholders: Int

        /// Whether nothing was counted.
        public var isEmpty: Bool { placeholders == 0 && buckets.allSatisfy { $0 == 0 } }
    }

    private let store: KeyValueStore
    // The extension and the app may run at the same time; the lock keeps one process's
    // read-modify-write whole. Across processes the last writer wins, which can lose one count,
    // never invent one.
    private let lock = NSLock()

    /// Creates the counters over the App Group store.
    ///
    /// - Parameter store: the store of `SharedState`.
    public init(store: KeyValueStore) {
        self.store = store
    }

    /// Counts one delivered push.
    ///
    /// - Parameters:
    ///   - duration: seconds from receipt to delivery; a negative or non-finite value is ignored.
    ///   - placeholder: whether the placeholder stayed instead of the text.
    public func record(duration: TimeInterval, placeholder: Bool) {
        lock.lock()
        defer { lock.unlock() }
        if duration.isFinite, duration >= 0 {
            var buckets = currentBuckets()
            let index = Self.bounds.firstIndex(where: { duration <= $0 }) ?? Self.bounds.count - 1
            buckets[index] += 1
            store.set(buckets.map { NSNumber(value: $0) }, forKey: Self.bucketsKey)
        }
        if placeholder {
            let count = (store.value(forKey: Self.placeholderKey) as? NSNumber)?.intValue ?? 0
            store.set(NSNumber(value: count + 1), forKey: Self.placeholderKey)
        }
    }

    /// Returns the counts since the last call and resets them.
    ///
    /// - Returns: the counts.
    public func take() -> Snapshot {
        lock.lock()
        defer { lock.unlock() }
        let snapshot = Snapshot(
            buckets: currentBuckets(),
            placeholders: (store.value(forKey: Self.placeholderKey) as? NSNumber)?.intValue ?? 0)
        store.set(nil, forKey: Self.bucketsKey)
        store.set(nil, forKey: Self.placeholderKey)
        return snapshot
    }

    private func currentBuckets() -> [Int] {
        let stored = (store.value(forKey: Self.bucketsKey) as? [NSNumber])?.map(\.intValue) ?? []
        // A list of another length (a build with other bounds) starts again rather than shift.
        return stored.count == Self.bounds.count ? stored : Array(repeating: 0, count: Self.bounds.count)
    }
}
