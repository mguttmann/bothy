// SPDX-License-Identifier: Apache-2.0

import Foundation

/// What a share is sent into (brief M5-06 Design note 1; 01 5.3 M6).
public enum ShareTargetKind: String, Codable, Sendable {
    /// A thread: the message goes into it.
    case thread
    /// The main timeline of a text channel: the message goes into its main thread.
    case channel
    /// A new post in a forum channel: `createThread` with a title and the first message.
    case newPost = "new_post"
}

/// One target of the share sheet, as kept in the App Group (brief M5-06 Design note 7).
///
/// JSON members `kind`, `channel_id`, `thread_id` (the thread; for a channel its main thread;
/// absent for a new post) and `path`, the display path: `"<channel> > <thread title>"` for a
/// thread, `"<channel>"` otherwise. The sheet adds the glyph and the "(new post)" suffix by kind.
public struct ShareTarget: Codable, Equatable, Hashable, Sendable {
    /// The kind of target.
    public let kind: ShareTargetKind
    /// The channel of the target.
    public let channelID: String
    /// The thread the message goes into; nil for a new post.
    public let threadID: String?
    /// The display path, names the person can see.
    public let path: String

    /// Creates a target.
    ///
    /// - Parameters:
    ///   - kind: thread, channel or new post.
    ///   - channelID: the channel.
    ///   - threadID: the thread, the channel's main thread, or nil for a new post.
    ///   - path: the display path.
    public init(kind: ShareTargetKind, channelID: String, threadID: String?, path: String) {
        self.kind = kind
        self.channelID = channelID
        self.threadID = threadID
        self.path = path
    }

    enum CodingKeys: String, CodingKey {
        case kind
        case channelID = "channel_id"
        case threadID = "thread_id"
        case path
    }

    /// Whether `other` names the same place, whatever its display path says (a renamed channel
    /// or thread stays one target).
    func samePlace(as other: ShareTarget) -> Bool {
        kind == other.kind && channelID == other.channelID && threadID == other.threadID
    }
}

/// The recently used targets in the App Group, newest first, at most
/// `client.share.recent_targets` (brief M5-06 Design note 7; 01 14).
///
/// The app writes the list after every successful send of any platform and removes it at the
/// end of the account; the extension adds a target after its own successful send. Without the
/// share settings (the app has not written `client.share.recent_targets` yet) nothing is kept,
/// because the limit is unknown.
///
/// Example:
///
///     let store = TargetStore(state: state)
///     store.recordUse(target)
///     let shown = store.recent()
public final class TargetStore {
    private let state: SharedState
    private let lock = NSLock()

    /// Creates the store over the App Group state.
    ///
    /// - Parameter state: the shared state of the App Group.
    public init(state: SharedState) {
        self.state = state
    }

    /// The recent targets, newest first, at most `client.share.recent_targets`.
    ///
    /// - Returns: the targets; empty without settings or when the stored value is damaged.
    public func recent() -> [ShareTarget] {
        lock.lock()
        defer { lock.unlock() }
        return Array(stored().prefix(limit))
    }

    /// Replaces the whole list (the app, after a send or an `access.changed`).
    ///
    /// - Parameter targets: newest first; duplicates after the first are dropped, the rest is
    ///   cut to `client.share.recent_targets`.
    public func replace(_ targets: [ShareTarget]) {
        lock.lock()
        defer { lock.unlock() }
        write(targets)
    }

    /// Moves `target` to the front (or adds it there) after a successful send.
    ///
    /// - Parameter target: the target just used; its display path replaces the stored one.
    public func recordUse(_ target: ShareTarget) {
        lock.lock()
        defer { lock.unlock() }
        write([target] + stored().filter { !$0.samePlace(as: target) })
    }

    /// Removes every recent target.
    public func clear() {
        lock.lock()
        defer { lock.unlock() }
        state.store.set(nil, forKey: SharedState.shareRecentTargetsKey)
    }

    /// `client.share.recent_targets` as the app wrote it; 0 while unknown.
    private var limit: Int {
        max(0, state.shareSettings?.recentMax ?? 0)
    }

    private func stored() -> [ShareTarget] {
        guard let data = state.store.value(forKey: SharedState.shareRecentTargetsKey) as? Data,
              let targets = try? JSONDecoder().decode([ShareTarget].self, from: data)
        else {
            return []
        }
        return targets
    }

    private func write(_ targets: [ShareTarget]) {
        var kept: [ShareTarget] = []
        for target in targets where !kept.contains(where: { $0.samePlace(as: target) }) {
            kept.append(target)
        }
        kept = Array(kept.prefix(limit))
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.sortedKeys, .withoutEscapingSlashes]
        guard !kept.isEmpty, let data = try? encoder.encode(kept) else {
            state.store.set(nil, forKey: SharedState.shareRecentTargetsKey)
            return
        }
        state.store.set(data, forKey: SharedState.shareRecentTargetsKey)
    }
}
