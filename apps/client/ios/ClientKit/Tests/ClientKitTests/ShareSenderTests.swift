// SPDX-License-Identifier: Apache-2.0
//
// The send of the Share Extension and the share inbox (brief M5-06 Design notes 4 and 5; 15 12;
// 04 13.2): upload, then the message with the extension's own `client_msg_id`; every failure
// class either goes to the inbox or is shown and keeps nothing.

import Foundation
import Testing

@testable import ClientKit

/// The Unix milliseconds in the first 48 bits of a UUIDv7.
private func millis(_ uuid: String) -> Int64? {
    let hex = uuid.replacingOccurrences(of: "-", with: "")
    guard hex.count == 32, Array(hex)[12] == "7" else { return nil }
    return Int64(hex.prefix(12), radix: 16)
}

@Test func testSendCreatesMessageWithClientMsgId() async throws {
    let world = ShareWorld()
    world.chat.partSize = 1024
    let sender = world.sender()
    let photo = try world.stage("photo.jpg", bytes: pattern(1500), mime: "image/jpeg")
    let note = try world.stage("note.txt", bytes: Data("hello".utf8), mime: "text/plain")
    // The ID is made when Send is tapped, not when the sender was created.
    world.clock.advance(42)

    let outcome = await sender.send(ShareDraft(target: threadTarget, comment: "Alex, the export", title: nil, items: [photo, note]))

    #expect(outcome == .sent)
    let create = try #require(world.chat.requests("POST", "/api/v1/threads/\(threadID)/messages").first)
    let message = body(create)
    let clientMsgID = try #require(message["client_msg_id"] as? String)
    #expect(millis(clientMsgID) == Int64(world.clock.now().timeIntervalSince1970 * 1000))
    #expect(message["content_md"] as? String == "Alex, the export")
    let completed = world.chat.requests.filter { $0.url?.path.hasSuffix("/complete") == true }.map { $0.url!.pathComponents[4] }
    #expect(message["attachment_file_ids"] as? [String] == completed)
    #expect(completed.count == 2)
    // Uploads come before the message.
    let order = world.chat.requests.map { $0.url!.path }
    #expect(order.lastIndex { $0.hasSuffix("/complete") }! < order.firstIndex { $0.hasSuffix("/messages") }!)
    // The target moves to the front of the recent targets; the staged copies are gone.
    #expect(world.targets.recent().first == threadTarget)
    #expect(world.stagedFiles.isEmpty)
    #expect(world.inbox.entries().isEmpty)

    // A channel target posts into its main thread; `200` (a replay of the original) is success.
    world.chat.override = { request in
        request.url?.path.hasSuffix("/messages") == true ? .init(status: 200, headers: ["Idempotent-Replay": "true"], body: json(["id": "x"])) : nil
    }
    let replay = await world.sender().send(ShareDraft(target: channelTarget, comment: "only text", title: nil, items: []))
    #expect(replay == .sent)
    #expect(world.chat.requests("POST", "/api/v1/threads/\(mainThreadID)/messages").count == 1)
    #expect(body(world.chat.requests("POST", "/api/v1/threads/\(mainThreadID)/messages")[0])["attachment_file_ids"] == nil)
    #expect(world.targets.recent() == [channelTarget, threadTarget])

    // A new forum post goes through `createThread` with `first_message`.
    let post = await world.sender().send(ShareDraft(target: forumTarget, comment: "see attached", title: "Idea", items: [
        try world.stage("sketch.png", bytes: pattern(100), mime: "image/png"),
    ]))
    #expect(post == .sent)
    let thread = try #require(world.chat.requests("POST", "/api/v1/channels/\(forumTarget.channelID)/threads").first)
    let first = try #require(body(thread)["first_message"] as? [String: Any])
    #expect(body(thread)["title"] as? String == "Idea")
    #expect(first["content_md"] as? String == "see attached")
    #expect((first["attachment_file_ids"] as? [String])?.count == 1)
    #expect(millis(first["client_msg_id"] as? String ?? "") != nil)
    #expect(thread.value(forHTTPHeaderField: "Idempotency-Key") == nil)
}

@Test func testFailuresGoToInbox() async throws {
    struct Case {
        let name: String
        let session: Bool
        let failing: (URLRequest) throws -> StubTransport.Reply?
        var revoked = false
    }
    let messages = { (request: URLRequest) in request.url?.path.hasSuffix("/messages") == true }
    let cases: [Case] = [
        Case(name: "network error", session: true) { request in
            if messages(request) { throw URLError(.notConnectedToInternet) }
            return nil
        },
        Case(name: "timeout", session: true) { request in
            if messages(request) { throw URLError(.timedOut) }
            return nil
        },
        Case(name: "5xx", session: true) { request in messages(request) ? .init(status: 503) : nil },
        Case(name: "429", session: true) { request in messages(request) ? .init(status: 429, headers: ["Retry-After": "3"]) : nil },
        Case(name: "409 request_in_progress", session: true) { request in messages(request) ? problem(409, "request_in_progress") : nil },
        Case(name: "missing session", session: false) { _ in nil },
        Case(name: "revoked session", session: true, failing: { request in messages(request) ? problem(401, "session_revoked") : nil }, revoked: true),
    ]
    for item in cases {
        let world = ShareWorld(session: item.session)
        let failing = item.failing
        world.chat.override = { request in try failing(request) }
        let staged = try world.stage("photo.jpg", bytes: pattern(700), mime: "image/jpeg")

        let outcome = await world.sender().send(ShareDraft(target: threadTarget, comment: "for Sam", title: nil, items: [staged]))

        #expect(outcome == .queued, "\(item.name)")
        let entry = try #require(world.inbox.entries().first, "\(item.name)")
        #expect(world.inbox.entries().count == 1)
        #expect(millis(entry.clientMsgID) != nil, "\(item.name)")
        #expect(entry.instanceID == instanceID && entry.target == threadTarget && entry.comment == "for Sam" && entry.title == nil)
        #expect(entry.createdAt == world.clock.now())
        let file = try #require(entry.files.first)
        #expect(file.name == "photo.jpg" && file.mimeType == "image/jpeg" && file.sizeBytes == 700)
        #expect(try Data(contentsOf: file.url) == pattern(700), "\(item.name)")
        if item.session {
            // The upload finished before the message failed: its ID travels with the entry.
            #expect(file.completedFileID != nil, "\(item.name)")
            let sent = world.chat.requests("POST", "/messages").first.map { body($0)["client_msg_id"] as? String }
            #expect(sent == entry.clientMsgID, "\(item.name)")
        } else {
            #expect(file.completedFileID == nil && world.chat.requests.isEmpty)
        }
        #expect(world.stagedFiles.isEmpty, "\(item.name)")
        #expect((try world.keychain.load(instanceID: instanceID) == nil) == (item.revoked || !item.session), "\(item.name)")
        #expect(world.targets.recent().isEmpty)
    }

    // A failure during the upload keeps the file without an upload ID.
    let world = ShareWorld()
    world.chat.override = { request in request.url?.path.hasSuffix("/uploads") == true ? .init(status: 500) : nil }
    let staged = try world.stage("a.pdf", bytes: pattern(50), mime: "application/pdf")
    #expect(await world.sender().send(ShareDraft(target: threadTarget, comment: "", title: nil, items: [staged])) == .queued)
    #expect(world.inbox.entries().first?.files.first?.completedFileID == nil)

    // Without a session object at all (no keychain item when the sheet opened) it queues too.
    let none = ShareWorld(session: false)
    let other = try none.stage("b.txt", bytes: Data("x".utf8), mime: "text/plain")
    #expect(await none.sender(withSession: false).send(ShareDraft(target: forumTarget, comment: "c", title: "T", items: [other])) == .queued)
    #expect(none.inbox.entries().first?.title == "T")
    #expect(none.chat.requests.isEmpty)
}

@Test func test4xxShowsReasonAndKeepsNothing() async throws {
    let refusals: [(String, Int, String)] = [("/uploads", 403, "forbidden"), ("/uploads", 413, "payload_too_large"), ("/messages", 403, "forbidden")]
    for (suffix, status, code) in refusals {
        let world = ShareWorld()
        world.chat.override = { request in request.url?.path.hasSuffix(suffix) == true ? problem(status, code) : nil }
        let staged = try world.stage("photo.jpg", bytes: pattern(300), mime: "image/jpeg")

        let outcome = await world.sender().send(ShareDraft(target: threadTarget, comment: "c", title: nil, items: [staged]))

        #expect(outcome == .refused(status: status, code: code))
        #expect(world.inbox.entries().isEmpty)
        #expect(world.stagedFiles.isEmpty)
        #expect(!FileManager.default.fileExists(atPath: staged.fileURL.deletingLastPathComponent().path))
        #expect(world.targets.recent().isEmpty)
        // The session stays: a refusal is about the target, not the person.
        #expect(try world.keychain.load(instanceID: instanceID) != nil)
    }
}

@Test func testInboxManifestHasNoTokens() async throws {
    let world = ShareWorld()
    world.chat.override = { request in request.url?.path.hasSuffix("/messages") == true ? .init(status: 503) : nil }
    let staged = try world.stage("photo.jpg", bytes: pattern(64), mime: "image/jpeg")
    #expect(await world.sender().send(ShareDraft(target: threadTarget, comment: "c", title: nil, items: [staged])) == .queued)

    let entry = try #require(world.inbox.entries().first)
    let folder = world.root.appendingPathComponent("ShareInbox").appendingPathComponent(entry.clientMsgID)
    let manifest = try Data(contentsOf: folder.appendingPathComponent("manifest.json"))
    let text = String(decoding: manifest, as: UTF8.self)
    for secret in [childAccess, childRefresh, "child_access", "child_refresh", "access_token", "refresh_token"] {
        #expect(!text.contains(secret))
    }
    // The members of the contract, and nothing else.
    let object = try #require(try JSONSerialization.jsonObject(with: manifest) as? [String: Any])
    #expect(Set(object.keys) == ["version", "client_msg_id", "instance_id", "target", "comment", "created_at_ms", "files"])
    #expect(object["version"] as? Int == 1)
    let target = try #require(object["target"] as? [String: Any])
    #expect(target["kind"] as? String == "thread" && target["thread_id"] as? String == threadID && target["path"] as? String == threadTarget.path)
    let file = try #require((object["files"] as? [[String: Any]])?.first)
    #expect(file["file"] as? String == "files/1-photo.jpg")
    #expect(Set(file.keys) == ["file", "name", "mime_type", "size_bytes", "completed_file_id"])
}

@Test func testInboxWritesAtomicallyAndReadsOnlyFinishedEntries() throws {
    let world = ShareWorld()
    let inbox = world.inbox
    let staged = try world.stage("../odd name?.txt", bytes: Data("x".utf8), mime: "text/plain")
    let id = UUIDv7.make(now: world.clock.now())
    try inbox.write(clientMsgID: id, instanceID: instanceID, target: forumTarget, comment: "c", title: "T",
                    createdAt: world.clock.now(), files: [ShareInboxItem(item: staged, completedFileID: "f1")])
    let entries = inbox.entries()
    #expect(entries.count == 1)
    let file = try #require(entries.first?.files.first)
    #expect(file.completedFileID == "f1" && file.name == "../odd name?.txt")
    // The stored name is safe: it stays inside the entry's `files` folder.
    #expect(file.url.deletingLastPathComponent().lastPathComponent == "files")
    #expect(file.url.lastPathComponent.hasPrefix("1-") && !file.url.lastPathComponent.contains("/"))
    #expect(!FileManager.default.fileExists(atPath: staged.fileURL.path))

    // A hidden temp folder and a folder without a manifest are not entries.
    let base = world.root.appendingPathComponent("ShareInbox")
    try FileManager.default.createDirectory(at: base.appendingPathComponent(".tmp-x/files"), withIntermediateDirectories: true)
    try Data("{}".utf8).write(to: base.appendingPathComponent(".tmp-x/manifest.json"))
    try FileManager.default.createDirectory(at: base.appendingPathComponent(UUIDv7.make()), withIntermediateDirectories: true)
    #expect(inbox.entries().map(\.clientMsgID) == [id])

    try inbox.delete(clientMsgID: id)
    #expect(inbox.entries().isEmpty)
    try inbox.delete(clientMsgID: id)
    #expect(throws: (any Error).self) { try inbox.delete(clientMsgID: "../ShareStaging") }
}

@Test func testEnvironmentWithoutSessionStillQueues() async throws {
    // No share settings: nobody is signed in, there is nothing to build.
    let empty = ShareWorld(session: false)
    empty.state.clearShareData()
    #expect(ShareEnvironment.assemble(state: empty.state, root: empty.root, secrets: empty.secrets, signer: { _ in nil }, transport: empty.chat, now: empty.clock.now) == nil)

    // Settings but no child session: no search, and a send goes to the inbox.
    let world = ShareWorld(session: false)
    let signer = world.signer
    let live = try #require(ShareEnvironment.assemble(
        state: world.state, root: world.root, secrets: world.secrets, signer: { _ in signer }, transport: world.chat, now: world.clock.now))
    #expect(live.session == nil && live.search == nil)
    #expect(live.settings.recentMax == 3)
    let staged = try live.inbox.stagingFolder().appendingPathComponent("a.txt")
    try Data("a".utf8).write(to: staged)
    let outcome = await live.sender.send(ShareDraft(target: threadTarget, comment: "c", title: nil, items: [
        ShareItem(fileURL: staged, name: "a.txt", mimeType: "text/plain", sizeBytes: 1),
    ]))
    #expect(outcome == .queued)
    #expect(live.inbox.entries().count == 1)
    #expect(world.chat.requests.isEmpty)
    // Only the child item was looked at.
    #expect(world.secrets.all.allSatisfy { $0 == ShareSessionKeychain.account(instanceID: instanceID) })

    // With the child session and the device key everything is wired.
    let signed = ShareWorld()
    let key = signed.signer
    let full = try #require(ShareEnvironment.assemble(
        state: signed.state, root: signed.root, secrets: signed.secrets, signer: { $0 == instanceID ? key : nil }, transport: signed.chat, now: signed.clock.now))
    #expect(full.session != nil && full.search != nil)
    // Without the device key there is no session either.
    let keyless = try #require(ShareEnvironment.assemble(
        state: signed.state, root: signed.root, secrets: signed.secrets, signer: { _ in nil }, transport: signed.chat, now: signed.clock.now))
    #expect(keyless.session == nil)
}

/// At the end of an account (R-SY-14) the inbox and the staging folders go with everything in
/// them: entries, half-written entries and the copies of a sheet whose process was killed.
@Test func testRemoveAllDeletesInboxAndStaging() throws {
    let world = ShareWorld()
    let inbox = world.inbox
    let staged = try world.stage("photo.jpg", bytes: Data("x".utf8), mime: "image/jpeg")
    let id = UUIDv7.make(now: world.clock.now())
    try inbox.write(clientMsgID: id, instanceID: instanceID, target: forumTarget, comment: "c", title: "T",
                    createdAt: world.clock.now(), files: [ShareInboxItem(item: staged, completedFileID: nil)])
    let left = try world.stage("left.jpg", bytes: Data("y".utf8), mime: "image/jpeg")
    let half = world.root.appendingPathComponent("ShareInbox/.tmp-x")
    try FileManager.default.createDirectory(at: half, withIntermediateDirectories: true)

    try inbox.removeAll()

    #expect(inbox.entries().isEmpty)
    #expect(!FileManager.default.fileExists(atPath: world.root.appendingPathComponent("ShareInbox").path))
    #expect(!FileManager.default.fileExists(atPath: world.root.appendingPathComponent("ShareStaging").path))
    #expect(!FileManager.default.fileExists(atPath: left.fileURL.path))
    try inbox.removeAll()
}
