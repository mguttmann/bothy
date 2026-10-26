// SPDX-License-Identifier: Apache-2.0
//
// The share settings and recent targets in the App Group and the target search (brief M5-06
// Design notes 1 and 7; 01 14 `client.share.recent_targets`; 15 1 principle 1).

import Foundation
import Testing

@testable import ClientKit

@Test func testShareSettingsInSharedState() throws {
    let state = SharedState(store: MemoryStore())
    #expect(state.shareSettings == nil)
    let settings = ShareSettings(instanceID: instanceID, recentMax: 5, maxUploadBytes: 3_000_000_000, maxAttachments: 10)
    state.writeShareSettings(settings)
    #expect(state.shareSettings == settings)
    #expect(state.instanceID == instanceID)
    // Unknown limits stay absent.
    state.writeShareSettings(ShareSettings(instanceID: instanceID, recentMax: 5, maxUploadBytes: nil, maxAttachments: nil))
    #expect(state.shareSettings?.maxUploadBytes == nil && state.shareSettings?.maxAttachments == nil)
    #expect(Set(state.store.keys) == ["clientkit.instance_id", "clientkit.share.recent_max"])

    TargetStore(state: state).replace([threadTarget])
    let keys = Set(state.store.keys)
    #expect(keys == ["clientkit.instance_id", "clientkit.share.recent_max", "clientkit.share.recent_targets"])
    // Every end of the account removes the share data with the names in it.
    state.clearAccount()
    #expect(state.store.keys.isEmpty)
    state.writeShareSettings(settings)
    TargetStore(state: state).replace([threadTarget])
    state.clearShareData()
    #expect(Set(state.store.keys) == ["clientkit.instance_id"])
}

@Test func testTargetStoreKeepsRecentNewestFirst() throws {
    let state = SharedState(store: MemoryStore())
    let store = TargetStore(state: state)
    // Without `client.share.recent_targets` written by the app nothing is kept.
    store.recordUse(threadTarget)
    #expect(store.recent().isEmpty)

    state.writeShareSettings(ShareSettings(instanceID: instanceID, recentMax: 3, maxUploadBytes: nil, maxAttachments: nil))
    let a = ShareTarget(kind: .thread, channelID: channelID, threadID: "t-a", path: "tickets > A")
    let b = ShareTarget(kind: .channel, channelID: "c-b", threadID: "m-b", path: "general")
    let c = ShareTarget(kind: .newPost, channelID: "c-c", threadID: nil, path: "ideas")
    let d = ShareTarget(kind: .thread, channelID: channelID, threadID: "t-d", path: "tickets > D")
    store.replace([a, b, c, d])
    #expect(store.recent() == [a, b, c])
    store.recordUse(c)
    #expect(store.recent() == [c, a, b])
    store.recordUse(d)
    #expect(store.recent() == [d, c, a])
    // The same target under a new name moves to the front once, with the new name.
    let renamed = ShareTarget(kind: .thread, channelID: channelID, threadID: "t-a", path: "tickets > A2")
    store.recordUse(renamed)
    #expect(store.recent() == [renamed, d, c])
    // A smaller limit written later applies on read.
    state.writeShareSettings(ShareSettings(instanceID: instanceID, recentMax: 1, maxUploadBytes: nil, maxAttachments: nil))
    #expect(store.recent() == [renamed])

    // The JSON of the contract.
    store.replace([c])
    let data = try #require(state.store.value(forKey: "clientkit.share.recent_targets") as? Data)
    let array = try #require(try JSONSerialization.jsonObject(with: data) as? [[String: Any]])
    #expect(array.count == 1 && array[0]["kind"] as? String == "new_post" && array[0]["channel_id"] as? String == "c-c")
    #expect(array[0]["thread_id"] == nil && array[0]["path"] as? String == "ideas")
    store.clear()
    #expect(store.recent().isEmpty)
    // A damaged value reads as empty.
    state.store.set(Data("[{".utf8), forKey: "clientkit.share.recent_targets")
    #expect(store.recent().isEmpty)
}

private func channelView(_ id: String, _ name: String, kind: String = "text", level: String = "write", readOnly: Bool = false, main: String? = nil) -> [String: Any] {
    var channel: [String: Any] = ["id": id, "kind": kind, "name": name, "read_only": readOnly]
    if let main { channel["main_thread_id"] = main }
    return ["channel": channel, "level": level, "capabilities": []]
}

@Test func testTargetSearchOffersOnlyWritableTargets() async throws {
    let world = ShareWorld()
    world.chat.spaces = [
        ["space": ["id": "s1", "name": "Support"], "level": "write", "capabilities": []],
        ["space": ["id": "s2", "name": "Lab"], "level": "read", "capabilities": []],
    ]
    world.chat.channels = [
        "s1": [
            channelView(channelID, "Tickets", main: mainThreadID),
            channelView("c-forum", "ticket-ideas", kind: "forum", level: "trigger"),
            channelView("c-read", "tickets-archive", level: "read", main: "m-read"),
            channelView("c-ro", "tickets-import", readOnly: true, main: "m-ro"),
            channelView("c-other", "general", main: "m-other"),
        ],
        "s2": [channelView("c-lab", "lab-tickets", main: "m-lab")],
    ]
    world.chat.titleHits = [
        ["thread_id": threadID, "channel_id": channelID, "title": "Invoice export fails", "last_activity_at": "2026-10-10T09:30:00.000Z"],
        ["thread_id": "t-read", "channel_id": "c-read", "title": "Old ticket", "last_activity_at": "2026-10-10T09:30:00.000Z"],
    ]
    let search = TargetSearch(api: world.session())

    let found = try await search.search("TICK")

    #expect(found == [
        ShareTarget(kind: .channel, channelID: channelID, threadID: mainThreadID, path: "Tickets"),
        ShareTarget(kind: .newPost, channelID: "c-forum", threadID: nil, path: "ticket-ideas"),
        ShareTarget(kind: .channel, channelID: "c-lab", threadID: "m-lab", path: "lab-tickets"),
        ShareTarget(kind: .thread, channelID: channelID, threadID: threadID, path: "Tickets > Invoice export fails"),
    ])
    let query = URLComponents(url: try #require(world.chat.requests("GET", "/search/thread-titles").first?.url), resolvingAgainstBaseURL: false)
    #expect(query?.queryItems == [URLQueryItem(name: "q", value: "TICK")])

    // The structure is read once per search object; the titles on every query.
    _ = try await search.search("gen")
    #expect(world.chat.requests("GET", "/api/v1/spaces").count == 1)
    #expect(world.chat.requests("GET", "/channels").count == 2)
    #expect(world.chat.requests("GET", "/search/thread-titles").count == 2)

    // A `+` in the text reaches the server as a plus, not as a space.
    _ = try await search.search("c++ tickets")
    let plus = try #require(world.chat.requests("GET", "/search/thread-titles").last?.url)
    #expect(URLComponents(url: plus, resolvingAgainstBaseURL: false)?.percentEncodedQuery == "q=c%2B%2B%20tickets")
    #expect(world.chat.requests("GET", "/search/thread-titles").count == 3)

    // An empty query lists every writable channel and asks no title search.
    let all = try await search.search("  ")
    #expect(all.map(\.channelID) == [channelID, "c-forum", "c-other", "c-lab"])
    #expect(world.chat.requests("GET", "/search/thread-titles").count == 3)

    // A failure of the server reaches the sheet as an error.
    world.chat.override = { request in request.url?.path.hasSuffix("/thread-titles") == true ? .init(status: 503) : nil }
    await #expect(throws: ShareError.transient) { try await search.search("x") }
}
