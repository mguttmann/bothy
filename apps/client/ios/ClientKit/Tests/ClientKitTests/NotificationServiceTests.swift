// SPDX-License-Identifier: Apache-2.0
//
// The preview fetch and the Notification Service Extension (brief M5-04 Design notes 3 and 4;
// docs/spec/13-push-notifications.md 5.1, 6.4; 15 10.1).
//
// The extension tests run the class every NSE target subclasses (`ClientKitNotificationService`)
// in this XCTest host and call `didReceive` with a payload of 13 6.1; the fake server is the
// recording `StubTransport` (the path of the brief's fallback, recorded in the report). Time is
// a `ManualSleeper` the test fires, never the wall clock.

import Foundation
import Testing
import UserNotifications

@testable import ClientKit

/// The content the extension delivered; waits for the delivery as an event.
final class Delivery {
    private let lock = NSLock()
    private var content: UNNotificationContent?
    private var count = 0
    private var waiters: [CheckedContinuation<UNNotificationContent, Never>] = []

    func handler(_ delivered: UNNotificationContent) {
        lock.lock()
        count += 1
        if content == nil { content = delivered }
        let all = waiters
        waiters = []
        lock.unlock()
        all.forEach { $0.resume(returning: delivered) }
    }

    var deliveries: Int {
        lock.lock()
        defer { lock.unlock() }
        return count
    }

    func value() async -> UNNotificationContent {
        await withCheckedContinuation { continuation in
            lock.lock()
            if let content {
                lock.unlock()
                continuation.resume(returning: content)
                return
            }
            waiters.append(continuation)
            lock.unlock()
        }
    }
}

/// The extension class under test with the world's environment.
final class TestNotificationService: ClientKitNotificationService {
    let world: World
    init(world: World) {
        self.world = world
        super.init()
    }

    override func makeEnvironment() -> NotificationEnvironment { world.environment }
}

private let fullPreview: [String: Any] = [
    "kind": "mention", "thread_id": "0192f1d2-7c4e-7a10-9a3b-2f6c1e0d4a11", "message_id": "0192f1d3-0a44-7f00-b1c2-d3e4f5a6b7c8",
    "badge": 4, "space_name": "Support", "channel_name": "tickets", "thread_title": "Invoice export fails",
    "sender_display_name": "Alex", "excerpt": "Sam can you check the export?", "attachment_count": 1,
]

/// Paths the extension must never call: refresh, login and session endpoints (R-CL-07).
private func touchesSession(_ request: URLRequest) -> Bool {
    let path = request.url?.path ?? ""
    return !path.hasSuffix("/api/v1/push/preview") || ["refresh", "token", "login", "session"].contains { path.contains($0) }
}

private func run(_ world: World, _ content: UNNotificationContent) async -> UNNotificationContent {
    let delivery = Delivery()
    let service = TestNotificationService(world: world)
    service.didReceive(UNNotificationRequest(identifier: "n", content: content, trigger: nil), withContentHandler: delivery.handler)
    return await delivery.value()
}

@Test func testServerReachableShowsText() async throws {
    let world = World { _ in .init(status: 200, headers: ["DPoP-Nonce": "n2"], body: json(fullPreview)) }
    world.clock.advance(0)
    let content = await run(world, pushContent())
    #expect(content.title == "tickets · Invoice export fails")
    #expect(content.body == "Alex: Sam can you check the export?")
    // Grouping, badge and sound stay those of the push.
    #expect(content.threadIdentifier == "0192f1d2-7c4e-7a10-9a3b-2f6c1e0d4a11")
    #expect(content.badge == 3)
    #expect(content.sound != nil)
    let request = try #require(world.transport.requests.first)
    #expect(request.value(forHTTPHeaderField: "Authorization") == "DPoP \(previewToken)")
    #expect(request.value(forHTTPHeaderField: "DPoP")?.split(separator: ".").count == 3)
    let query = URLComponents(url: request.url!, resolvingAgainstBaseURL: false)!.queryItems!
    #expect(Set(query.map(\.name)) == ["nid", "message_id", "kind"])
    #expect(query.first { $0.name == "kind" }?.value == "mention")
    #expect(request.timeoutInterval == 20)
    #expect(!world.transport.requests.contains(where: touchesSession))
    #expect(NonceCache(store: world.state.store).nonce(for: server) == "n2")
    let counters = DisplayCounters(store: world.state.store).take()
    #expect(counters.placeholders == 0 && counters.buckets.reduce(0, +) == 1)
}

@Test func testServerUnreachableShowsPlaceholder() async {
    let world = World { _ in throw StubTransport.Failure.offline }
    let content = await run(world, pushContent())
    #expect(content.title == "New notification")
    #expect(content.body == "You were mentioned")
    #expect(!world.transport.requests.contains(where: touchesSession))
    #expect(DisplayCounters(store: world.state.store).take().placeholders == 1)
}

@Test func testPreviewModeNoneFetchesNothing() async {
    let world = World(mode: PreviewMode.none) { _ in .init(status: 200, body: json(fullPreview)) }
    #expect(world.state.previewMode == PreviewMode.none)
    let content = await run(world, pushContent())
    #expect(world.transport.requests.isEmpty)
    #expect(content.body == "You were mentioned")
    #expect(DisplayCounters(store: world.state.store).take().placeholders == 1)
}

@Test func testNseNeverRefreshes() async {
    // Every outcome: text, 404, 401, offline and a mode the app never wrote.
    let replies: [StubTransport.Reply] = [
        .init(status: 200, body: json(fullPreview)), .init(status: 404), .init(status: 401), .init(status: 500),
    ]
    for reply in replies {
        let world = World { _ in reply }
        _ = await run(world, pushContent())
        #expect(world.transport.requests.count == 1)
        #expect(!world.transport.requests.contains(where: touchesSession))
    }
    let unset = World(mode: nil) { _ in .init(status: 200, body: json(fullPreview)) }
    _ = await run(unset, pushContent())
    #expect(unset.transport.requests.isEmpty)
}

@Test func testPreviewFullNamesNone() async {
    var names = fullPreview
    names["excerpt"] = nil
    names["attachment_count"] = nil
    let namesWorld = World(mode: .names) { _ in .init(status: 200, body: json(names)) }
    let namesContent = await run(namesWorld, pushContent())
    #expect(namesContent.title == "tickets · Invoice export fails")
    #expect(namesContent.body == "Alex: You were mentioned")

    // A DM without channel and title, and a `none` answer without names: the placeholder title.
    let dm: [String: Any] = ["kind": "dm", "badge": 1, "sender_display_name": "Robin", "excerpt": "hi"]
    let dmWorld = World { _ in .init(status: 200, body: json(dm)) }
    let dmContent = await run(dmWorld, pushContent(kind: "dm"))
    #expect(dmContent.title == "New notification")
    #expect(dmContent.body == "Robin: hi")

    let bare: [String: Any] = ["kind": "mention", "badge": 1]
    let bareContent = await run(World { _ in .init(status: 200, body: json(bare)) }, pushContent())
    #expect(bareContent.title == "New notification" && bareContent.body == "You were mentioned")

    let noneWorld = World(mode: PreviewMode.none) { _ in .init(status: 200, body: json(fullPreview)) }
    _ = await run(noneWorld, pushContent())
    #expect(noneWorld.transport.requests.isEmpty)
}

@Test func testNotAvailableOn404() async {
    let world = World { _ in .init(status: 404, body: json(["code": "not_found"])) }
    let content = await run(world, pushContent())
    #expect(content.body == "Message not available")
    #expect(content.title == "New notification")
    #expect(DisplayCounters(store: world.state.store).take().placeholders == 0)
}

@Test func testPlaceholderOnTimeoutAndNetworkError() async {
    // The server never answers; the end of `client.push.fetch_timeout` delivers the placeholder.
    let world = World { _ in nil }
    let delivery = Delivery()
    let service = TestNotificationService(world: world)
    service.didReceive(UNNotificationRequest(identifier: "n", content: pushContent(), trigger: nil), withContentHandler: delivery.handler)
    await world.sleeper.untilSleeping()
    #expect(world.sleeper.requested == [20])
    world.clock.advance(20)
    world.sleeper.fire()
    let content = await delivery.value()
    #expect(content.body == "You were mentioned")
    world.transport.hang.releaseAll()
    let counters = DisplayCounters(store: world.state.store).take()
    #expect(counters.placeholders == 1)
    #expect(counters.buckets[DisplayCounters.bounds.firstIndex(of: 2.5)!] == 0)
    #expect(counters.buckets.last == 1)
    #expect(delivery.deliveries == 1)

    // serviceExtensionTimeWillExpire delivers the placeholder as well, once.
    let expiring = World { _ in nil }
    let second = Delivery()
    let other = TestNotificationService(world: expiring)
    other.didReceive(UNNotificationRequest(identifier: "n", content: pushContent(), trigger: nil), withContentHandler: second.handler)
    await expiring.sleeper.untilSleeping()
    other.serviceExtensionTimeWillExpire()
    #expect(await second.value().body == "You were mentioned")
    expiring.transport.hang.releaseAll()
    expiring.sleeper.fire()
    #expect(second.deliveries == 1)

    // A network error keeps the placeholder too.
    let offline = World { _ in throw URLError(.notConnectedToInternet) }
    #expect(await run(offline, pushContent()).body == "You were mentioned")
}

@Test func testUnknownMembersIgnoredMissingRequiredFails() async {
    var extra = fullPreview
    extra["future_member"] = ["nested": true]
    #expect(PushPreview.decode(json(extra))?.channelName == "tickets")
    var noBadge = fullPreview
    noBadge["badge"] = nil
    #expect(PushPreview.decode(json(noBadge)) == nil)
    var badKind = fullPreview
    badKind["kind"] = "sync"
    #expect(PushPreview.decode(json(badKind)) == nil)
    var wrongType = fullPreview
    wrongType["channel_name"] = 7
    #expect(PushPreview.decode(json(wrongType)) == nil)
    #expect(PushPreview.decode(Data("[]".utf8)) == nil)

    // A missing required member keeps the placeholder in the extension.
    let world = World { _ in .init(status: 200, body: json(noBadge)) }
    #expect(await run(world, pushContent()).body == "You were mentioned")
}

@Test func testNonceRetryOnce() async {
    let lock = NSLock()
    var seen: [String?] = []
    func nonceOf(_ request: URLRequest) -> String? {
        let proof = request.value(forHTTPHeaderField: "DPoP")!
        var base = String(proof.split(separator: ".")[1]).replacingOccurrences(of: "-", with: "+").replacingOccurrences(of: "_", with: "/")
        base += String(repeating: "=", count: (4 - base.count % 4) % 4)
        let object = try! JSONSerialization.jsonObject(with: Data(base64Encoded: base)!) as! [String: Any]
        return object["nonce"] as? String
    }
    let world = World { request in
        lock.lock()
        seen.append(nonceOf(request))
        let first = seen.count == 1
        lock.unlock()
        return first
            ? .init(status: 401, headers: ["DPoP-Nonce": "fresh", "WWW-Authenticate": "DPoP error=\"use_dpop_nonce\""], body: json(["code": "use_dpop_nonce"]))
            : .init(status: 200, body: json(fullPreview))
    }
    #expect(await run(world, pushContent()).title == "tickets · Invoice export fails")
    #expect(seen == [nil, "fresh"])

    // A second `use_dpop_nonce` is not repeated again.
    let always = World { _ in
        .init(status: 401, headers: ["DPoP-Nonce": UUID().uuidString], body: json(["code": "use_dpop_nonce"]))
    }
    #expect(await run(always, pushContent()).body == "You were mentioned")
    #expect(always.transport.requests.count == 2)

    // The cached nonce starts the next fetch.
    let cached = World { _ in .init(status: 200, body: json(fullPreview)) }
    NonceCache(store: cached.state.store).remember("kept", for: server)
    _ = await run(cached, pushContent())
    #expect(nonceOf(cached.transport.requests[0]) == "kept")
}

@Test func testNoContentInSharedState() async {
    let world = World { _ in .init(status: 200, headers: ["DPoP-Nonce": "n2"], body: json(fullPreview)) }
    _ = await run(world, pushContent())
    let allowed: Set<String> = [
        "clientkit.preview_mode", "clientkit.instance_id", "clientkit.fetch_timeout_ms",
        "clientkit.counters.display_buckets", "clientkit.counters.placeholder",
    ]
    for key in world.state.store.keys {
        #expect(allowed.contains(key) || key.hasPrefix("clientkit.nonce."), "unexpected key \(key)")
    }
    let dump = "\((world.state.store as! MemoryStore).values)"
    for content in ["tickets", "Alex", "Invoice", "Sam can", "Support", "0192f1d2", previewToken] {
        #expect(!dump.contains(content))
    }
}

@Test func testPushesWithoutPreviewStayUnchanged() async {
    let world = World { _ in .init(status: 200, body: json(fullPreview)) }
    for content in [pushContent(kind: "security", message: nil), pushContent(version: 2), pushContent(message: nil)] {
        _ = await run(world, content)
    }
    #expect(world.transport.requests.isEmpty)
    // Without credentials for the instance nothing is fetched either.
    let missing = World(credentials: false) { _ in .init(status: 200, body: json(fullPreview)) }
    _ = await run(missing, pushContent())
    #expect(missing.transport.requests.isEmpty)
}

@Test func testPreviewURLKeepsBasePathAndNeedsHttps() {
    let url = PreviewClient.previewURL(server: URL(string: "https://chat.example.com/base/")!, nid: "a", messageID: "b", kind: "dm")
    #expect(url?.absoluteString == "https://chat.example.com/base/api/v1/push/preview?nid=a&message_id=b&kind=dm")
    #expect(PreviewClient.previewURL(server: URL(string: "http://chat.example.com")!, nid: "a", messageID: "b", kind: "dm") == nil)
}

@Test func testFormatTakesArgumentsLiterally() {
    #expect(PreviewRenderer.format("%1$@ · %2$@", "50%2$@", "%1$@") == "50%2$@ · %1$@")
    #expect(PreviewRenderer.format("broken", "a", "b") == "a b")
    #expect(PreviewRenderer.format("%2$@ %% %1$@", "a", "b") == "b % a")
}
