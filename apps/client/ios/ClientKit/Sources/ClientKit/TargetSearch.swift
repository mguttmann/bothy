// SPDX-License-Identifier: Apache-2.0

import Foundation

/// The search of the share sheet: more targets than the recent ones (brief M5-06 Design note 1).
public protocol TargetSearching {
    /// The targets matching `query`.
    ///
    /// - Parameter query: what the person typed.
    /// - Returns: writable targets, channels first, then threads.
    /// - Throws: `ShareError` when the server cannot be asked.
    func search(_ query: String) async throws -> [ShareTarget]
}

/// The search over the person's structure and the thread titles (brief M5-06 Design note 1;
/// 15 1 principle 1).
///
/// The channels come from `listSpaces` and `listSpaceChannels`, read once per instance of this
/// class (one sheet) and filtered by name, case-insensitive; thread titles come from
/// `searchThreadTitles` on every query. Only what the server reports as writable is offered: the
/// channel's `level` is `write` or `trigger` and it is not `read_only`; a title hit only when its
/// channel is. That is no permission logic of its own: the server's answer decides, and a send
/// into a target that lost the right fails visibly. A text channel becomes a `channel` target
/// with its main thread, a forum a `new_post` target; DM channels are not targets of the sheet.
///
/// Example:
///
///     let search = TargetSearch(api: session)
///     let targets = try await search.search("tick")
public final class TargetSearch: TargetSearching {
    private struct Channel {
        let id: String
        let name: String
        let target: ShareTarget
    }

    private let api: AuthorizedRequester
    private let lock = NSLock()
    private var cached: [Channel]?

    /// Creates the search.
    ///
    /// - Parameter api: the session for the calls to the server.
    public init(api: AuthorizedRequester) {
        self.api = api
    }

    /// See `TargetSearching.search(_:)`. An empty query lists every writable channel and asks
    /// no title search (`searchThreadTitles` needs at least one character).
    public func search(_ query: String) async throws -> [ShareTarget] {
        let channels = try await writableChannels()
        let text = query.trimmingCharacters(in: .whitespacesAndNewlines)
        let matching = channels.filter { text.isEmpty || $0.name.range(of: text, options: .caseInsensitive) != nil }
        var targets = matching.map(\.target)
        if !text.isEmpty {
            let byID = Dictionary(channels.map { ($0.id, $0) }, uniquingKeysWith: { first, _ in first })
            for hit in try await titles(text) {
                guard let channel = byID[hit.channelID] else { continue }
                let target = ShareTarget(kind: .thread, channelID: channel.id, threadID: hit.threadID, path: "\(channel.name) > \(hit.title)")
                if !targets.contains(target) { targets.append(target) }
            }
        }
        return targets
    }

    /// The writable channels of every visible space, in the server's order; loaded once.
    private func writableChannels() async throws -> [Channel] {
        if let cached = locked({ cached }) { return cached }
        let server = try await api.serverURL()
        var channels: [Channel] = []
        for space in try await items(server, "/spaces") {
            guard let id = (space["space"] as? [String: Any])?["id"] as? String else { continue }
            for entry in try await items(server, "/spaces/\(ShareAPI.segment(id))/channels") {
                if let channel = Self.writable(entry) { channels.append(channel) }
            }
        }
        locked { cached = channels }
        return channels
    }

    /// The target of a `ChannelView` the server reports as writable, else nil.
    private static func writable(_ entry: [String: Any]) -> Channel? {
        guard let level = entry["level"] as? String, level == "write" || level == "trigger",
              let channel = entry["channel"] as? [String: Any],
              (channel["read_only"] as? Bool) != true,
              let id = channel["id"] as? String, let name = channel["name"] as? String
        else {
            return nil
        }
        switch channel["kind"] as? String {
        case "text":
            guard let main = channel["main_thread_id"] as? String else { return nil }
            return Channel(id: id, name: name, target: ShareTarget(kind: .channel, channelID: id, threadID: main, path: name))
        case "forum":
            return Channel(id: id, name: name, target: ShareTarget(kind: .newPost, channelID: id, threadID: nil, path: name))
        default:
            return nil
        }
    }

    private func titles(_ query: String) async throws -> [(threadID: String, channelID: String, title: String)] {
        let server = try await api.serverURL()
        let hits = try await items(server, "/search/thread-titles", query: [URLQueryItem(name: "q", value: query)])
        return hits.compactMap { hit in
            guard let thread = hit["thread_id"] as? String, let channel = hit["channel_id"] as? String, let title = hit["title"] as? String else {
                return nil
            }
            return (thread, channel, title)
        }
    }

    /// `items` of a list answer.
    private func items(_ server: URL, _ path: String, query: [URLQueryItem] = []) async throws -> [[String: Any]] {
        guard let url = ShareAPI.url(server: server, path: path, query: query) else { throw ShareError.noSession }
        let (data, response) = try await api.send(ShareAPI.jsonRequest("GET", url: url, body: nil))
        try ShareAPI.check(response, data)
        guard let items = ShareAPI.object(data)?["items"] as? [[String: Any]] else { throw ShareError.transient }
        return items
    }

    private func locked<T>(_ body: () -> T) -> T {
        lock.lock()
        defer { lock.unlock() }
        return body()
    }
}
