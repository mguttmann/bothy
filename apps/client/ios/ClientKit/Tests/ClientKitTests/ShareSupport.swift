// SPDX-License-Identifier: Apache-2.0
//
// Test doubles of the share tests (brief M5-06 Design notes 3 to 5): a fake chat server with the
// upload protocol of docs/spec/12-files.md 3, the refresh of the child family and the message and
// search operations; a secret store that records which accounts were read; a latch the test opens
// itself; the staged files of one share. No test depends on the wall clock: every time comes from
// the `TestClock`.

import CryptoKit
import Foundation

@testable import ClientKit

/// The tokens of the child session in the tests; recognizable so a test can search bytes for them.
let childAccess = "child_access_EXAMPLE_0001"
let childRefresh = "child_refresh_EXAMPLE_0001"
let childSessionID = "0192f4a0-7b1c-75c3-8a09-bf22e1241000"
let channelID = "0192f4a0-7b1c-72d4-8bae-7661d2b2f203"
let mainThreadID = "0192f4a0-7b1c-78f9-8074-fa94e84ba000"
let threadID = "0192f4a0-7b1c-78f9-8074-fa94e84bab7d"

/// The child session of `clock`'s time: access valid for `accessIn` seconds, refresh for a day.
func childTokens(_ clock: TestClock, accessIn: TimeInterval = 900, access: String = childAccess, refresh: String = childRefresh) -> ExtensionSessionTokens {
    ExtensionSessionTokens(
        serverURL: server, sessionID: childSessionID,
        accessToken: access, accessExpiresAt: clock.now().addingTimeInterval(accessIn),
        refreshToken: refresh, refreshExpiresAt: clock.now().addingTimeInterval(86_400))
}

/// A `SecretStore` that records every account it is asked for, over a memory store.
final class RecordingSecretStore: SecretStore {
    let inner = MemorySecretStore()
    private let lock = NSLock()
    private var log: [(op: String, account: String)] = []

    var reads: [String] { entries("read") }
    var writes: [String] { entries("write") }
    var deletes: [String] { entries("delete") }
    var all: [String] {
        lock.lock()
        defer { lock.unlock() }
        return log.map(\.account)
    }

    private func entries(_ op: String) -> [String] {
        lock.lock()
        defer { lock.unlock() }
        return log.filter { $0.op == op }.map(\.account)
    }

    private func note(_ op: String, _ account: String) {
        lock.lock()
        log.append((op, account))
        lock.unlock()
    }

    func write(_ value: Data, service: String, account: String) throws {
        note("write", account)
        try inner.write(value, service: service, account: account)
    }

    func read(service: String, account: String) throws -> Data? {
        note("read", account)
        return try inner.read(service: service, account: account)
    }

    func delete(service: String, account: String) throws {
        note("delete", account)
        try inner.delete(service: service, account: account)
    }
}

/// Opens once; every `wait` before and after returns when it is open. An event, not a time.
final class Latch {
    private let lock = NSLock()
    private var isOpen = false
    private var waiters: [CheckedContinuation<Void, Never>] = []

    func wait() async {
        await withCheckedContinuation { (continuation: CheckedContinuation<Void, Never>) in
            lock.lock()
            if isOpen {
                lock.unlock()
                continuation.resume()
                return
            }
            waiters.append(continuation)
            lock.unlock()
        }
    }

    func open() {
        lock.lock()
        isOpen = true
        let all = waiters
        waiters = []
        lock.unlock()
        all.forEach { $0.resume() }
    }
}

/// A `FileReading` that records the size of every read, so a test can prove that no read is
/// larger than a part.
final class RecordingReader: FileReading {
    private let lock = NSLock()
    private(set) var sizes: [Int] = []
    let inner = FileHandleReader()

    var largest: Int {
        lock.lock()
        defer { lock.unlock() }
        return sizes.max() ?? 0
    }

    func open(_ url: URL) throws -> ReadableFile {
        RecordingFile(file: try inner.open(url), reader: self)
    }

    fileprivate func note(_ size: Int) {
        lock.lock()
        sizes.append(size)
        lock.unlock()
    }
}

private final class RecordingFile: ReadableFile {
    let file: ReadableFile
    let reader: RecordingReader

    init(file: ReadableFile, reader: RecordingReader) {
        self.file = file
        self.reader = reader
    }

    func read(at offset: Int64, count: Int) throws -> Data {
        let data = try file.read(at: offset, count: count)
        reader.note(data.count)
        return data
    }

    func close() {
        file.close()
    }
}

/// The body of a request as JSON.
func body(_ request: URLRequest) -> [String: Any] {
    guard let data = request.httpBody, let object = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else { return [:] }
    return object
}

/// The payload of a DPoP proof.
func proofPayload(_ request: URLRequest) -> [String: Any] {
    guard let proof = request.value(forHTTPHeaderField: "DPoP") else { return [:] }
    let parts = proof.split(separator: ".")
    guard parts.count == 3 else { return [:] }
    var text = String(parts[1]).replacingOccurrences(of: "-", with: "+").replacingOccurrences(of: "_", with: "/")
    while text.count % 4 != 0 { text += "=" }
    guard let data = Data(base64Encoded: text) else { return [:] }
    return (try? JSONSerialization.jsonObject(with: data) as? [String: Any]) ?? [:]
}

func hex(_ data: Data) -> String {
    SHA256.hash(data: data).map { String(format: "%02x", $0) }.joined()
}

/// A problem body (RFC 9457) with `code`.
func problem(_ status: Int, _ code: String) -> StubTransport.Reply {
    .init(status: status, headers: ["Content-Type": "application/problem+json"], body: json(["status": status, "code": code]))
}

/// The fake chat server: the operations the share extension calls, with the state they need.
///
/// `override` answers a request before the normal routes (failure injection); returning nil
/// lets the route answer. Every request is recorded with its body; `bodies` holds the bytes of
/// every request body for the memory check.
final class FakeChat: HTTPTransport {
    typealias Reply = StubTransport.Reply

    private let lock = NSLock()
    private var recorded: [URLRequest] = []
    private var accessTokens: Set<String> = [childAccess]
    private var refreshToken = childRefresh
    private var generation = 1
    private var uploads: [String: (size: Int, parts: [Int: String])] = [:]
    private var nextFile = 1

    var partSize = 1024
    var mode = "direct"
    /// Answers before the routes; nil lets the routes answer.
    var override: ((URLRequest) async throws -> Reply?)?
    /// Called for every refresh request before it is answered.
    var onRefresh: ((URLRequest) async -> Void)?
    var spaces: [[String: Any]] = []
    var channels: [String: [[String: Any]]] = [:]
    var titleHits: [[String: Any]] = []

    var requests: [URLRequest] {
        lock.lock()
        defer { lock.unlock() }
        return recorded
    }

    func requests(_ method: String, _ suffix: String) -> [URLRequest] {
        requests.filter { $0.httpMethod == method && ($0.url?.path ?? "").hasSuffix(suffix) }
    }

    func requests(matching method: String, _ fragment: String) -> [URLRequest] {
        requests.filter { $0.httpMethod == method && ($0.url?.absoluteString ?? "").contains(fragment) }
    }

    var refreshes: [URLRequest] { requests("POST", "/api/v1/auth/token/refresh") }

    /// Makes the current access token invalid, as when it expired on the server.
    func expireAccess() {
        lock.lock()
        accessTokens = []
        lock.unlock()
    }

    func confirmed(_ fileID: String) -> [Int: String] {
        lock.lock()
        defer { lock.unlock() }
        return uploads[fileID]?.parts ?? [:]
    }

    private func locked<T>(_ body: () -> T) -> T {
        lock.lock()
        defer { lock.unlock() }
        return body()
    }

    func send(_ request: URLRequest) async throws -> (Data, HTTPURLResponse) {
        locked { recorded.append(request) }
        let reply: Reply
        if let override, let early = try await override(request) {
            reply = early
        } else {
            reply = await answer(request)
        }
        let response = HTTPURLResponse(url: request.url!, statusCode: reply.status, httpVersion: "HTTP/1.1", headerFields: reply.headers)!
        return (reply.body, response)
    }

    private func authorized(_ request: URLRequest) -> Bool {
        let header = request.value(forHTTPHeaderField: "Authorization") ?? ""
        return locked { accessTokens.contains { header == "DPoP \($0)" } }
    }

    /// The answer of the routes, without recording; a test may store a part and then fail the reply.
    func answer(_ request: URLRequest) async -> Reply {
        let url = request.url!
        let path = url.path
        let method = request.httpMethod ?? "GET"
        if url.host == "objects.example.com" {
            return presignedPart(request)
        }
        if method == "POST", path == "/api/v1/auth/token/refresh" {
            await onRefresh?(request)
            return refresh(request)
        }
        guard authorized(request) else { return problem(401, "token_expired") }
        // The path below `/api/v1`, with `*` matching one segment.
        let segments = Array(path.split(separator: "/").map(String.init).dropFirst(2))
        func at(_ verb: String, _ pattern: String) -> Bool {
            let parts = pattern.split(separator: "/").map(String.init)
            return verb == method && parts.count == segments.count && zip(parts, segments).allSatisfy { $0 == "*" || $0 == $1 }
        }
        if at("POST", "channels/*/uploads") { return createUpload(request) }
        if at("PUT", "uploads/*/parts/*") { return directPart(request, file: segments[1], number: Int(segments[3]) ?? 0) }
        if at("GET", "uploads/*") {
            let file = segments[1]
            let parts = confirmed(file).sorted { $0.key < $1.key }.map { ["part_number": $0.key, "etag": $0.value] as [String: Any] }
            return .init(status: 200, body: json([
                "file_id": file, "upload_state": parts.isEmpty ? "pending" : "uploading", "part_size_bytes": partSize,
                "completed_parts": parts, "expires_at": "2026-10-10T09:30:00.000Z",
            ]))
        }
        if at("POST", "uploads/*/complete") {
            let file = segments[1]
            return .init(status: 200, body: json(["id": file, "upload_state": "pending", "size_bytes": locked { uploads[file]?.size ?? 0 }]))
        }
        if at("POST", "threads/*/messages") {
            return .init(status: 201, body: json(["id": "0192f4a0-7b1c-7000-8000-00000000a001"]))
        }
        if at("POST", "channels/*/threads") {
            return .init(status: 201, body: json(["thread": ["id": "0192f4a0-7b1c-7000-8000-00000000b001"]]))
        }
        if at("GET", "spaces") { return .init(status: 200, body: json(["items": spaces])) }
        if at("GET", "spaces/*/channels") { return .init(status: 200, body: json(["items": channels[segments[1]] ?? []])) }
        if at("GET", "search/thread-titles") { return .init(status: 200, body: json(["items": titleHits])) }
        return problem(404, "not_found")
    }

    private func refresh(_ request: URLRequest) -> Reply {
        let presented = body(request)["refresh_token"] as? String
        return locked {
            guard presented == refreshToken else { return problem(401, "unauthenticated") }
            generation += 1
            let access = "child_access_EXAMPLE_000\(generation)"
            refreshToken = "child_refresh_EXAMPLE_000\(generation)"
            accessTokens = [access]
            return .init(status: 200, headers: ["DPoP-Nonce": "nonce-\(generation)"], body: json([
                "access_token": access, "token_type": "DPoP", "expires_in": 900,
                "refresh_token": refreshToken, "refresh_expires_at": "2025-10-10T09:30:00.000Z",
                "session": ["id": childSessionID], "device": ["id": "0192f4a0-7b1c-7848-8501-1716705c7dbe"],
            ]))
        }
    }

    private func createUpload(_ request: URLRequest) -> Reply {
        let size = body(request)["size_bytes"] as? Int ?? 0
        let count = max(1, (size + partSize - 1) / partSize)
        return locked {
            let file = String(format: "0192f4a0-7b1c-7000-8000-%012d", nextFile)
            nextFile += 1
            uploads[file] = (size, [:])
            let parts: [[String: Any]] = (1...count).map { n in
                let url = mode == "direct"
                    ? "/api/v1/uploads/\(file)/parts/\(n)"
                    : "https://objects.example.com/bucket/\(file)/\(n)?X-Amz-Signature=sig\(n)"
                return ["part_number": n, "url": url, "headers": [["name": "Content-Type", "value": "application/octet-stream"]]]
            }
            return .init(status: 201, body: json([
                "file_id": file, "mode": mode, "part_size_bytes": partSize, "parts": parts,
                "expires_at": "2026-10-10T09:30:00.000Z",
            ]))
        }
    }

    private func digestMatches(_ request: URLRequest) -> Bool {
        let data = request.httpBody ?? Data()
        let expected = "sha-256=:\(Data(SHA256.hash(data: data)).base64EncodedString()):"
        return request.value(forHTTPHeaderField: "Content-Digest") == expected
    }

    private func directPart(_ request: URLRequest, file: String, number: Int) -> Reply {
        guard digestMatches(request) else { return problem(400, "validation_failed") }
        let etag = "\"\(hex(request.httpBody ?? Data()))\""
        locked { uploads[file]?.parts[number] = etag }
        return .init(status: 200, body: json(["part_number": number, "etag": etag]))
    }

    private func presignedPart(_ request: URLRequest) -> Reply {
        let segments = request.url!.path.split(separator: "/").map(String.init)
        guard segments.count == 3, let number = Int(segments[2]), digestMatches(request) else { return .init(status: 400) }
        let etag = "\"s3-\(number)\""
        locked { uploads[segments[1]]?.parts[number] = etag }
        return .init(status: 200, headers: ["ETag": etag])
    }
}

/// The pieces of one share: the App Group container, the state, the keychain and the fake server.
final class ShareWorld {
    let root: URL
    let state = SharedState(store: MemoryStore())
    let secrets = RecordingSecretStore()
    let signer = TestSigner()
    let clock = TestClock()
    let chat = FakeChat()
    let reader = RecordingReader()

    init(session: Bool = true, accessIn: TimeInterval = 900) {
        root = FileManager.default.temporaryDirectory.appendingPathComponent("share-\(UUID().uuidString)", isDirectory: true)
        try! FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        state.writeShareSettings(ShareSettings(instanceID: instanceID, recentMax: 3, maxUploadBytes: 1_000_000, maxAttachments: 10))
        if session {
            try! keychain.save(childTokens(clock, accessIn: accessIn), instanceID: instanceID)
        }
    }

    deinit {
        try? FileManager.default.removeItem(at: root)
    }

    var keychain: ShareSessionKeychain { ShareSessionKeychain(store: secrets) }
    var nonces: NonceCache { NonceCache(store: state.store) }
    lazy var inbox = ShareInbox(root: root)
    lazy var targets = TargetStore(state: state)

    func session() -> ExtensionSession {
        let clock = self.clock
        return ExtensionSession(
            instanceID: instanceID, keychain: keychain, signer: signer, nonces: nonces, transport: chat, now: { clock.now() })
    }

    func uploader(hashChunkBytes: Int = 256) -> UploadClient {
        UploadClient(api: session(), transport: chat, reader: reader, hashChunkBytes: hashChunkBytes, idempotencyKey: { UUIDv7.make() })
    }

    func sender(withSession: Bool = true) -> ShareSender {
        let clock = self.clock
        return ShareSender(
            instanceID: instanceID, api: withSession ? session() : nil, transport: chat,
            inbox: inbox, targets: targets, reader: reader, now: { clock.now() })
    }

    /// Writes `bytes` as a staged copy of a shared item.
    func stage(_ name: String, bytes: Data, mime: String = "application/octet-stream") throws -> ShareItem {
        let folder = try inbox.stagingFolder()
        let url = folder.appendingPathComponent(name)
        try bytes.write(to: url)
        return ShareItem(fileURL: url, name: name, mimeType: mime, sizeBytes: Int64(bytes.count))
    }

    var stagingRoot: URL { root.appendingPathComponent("ShareStaging", isDirectory: true) }

    /// The files left under the staging folder.
    var stagedFiles: [String] {
        let enumerator = FileManager.default.enumerator(at: stagingRoot, includingPropertiesForKeys: [.isRegularFileKey])
        var names: [String] = []
        while let url = enumerator?.nextObject() as? URL {
            if (try? url.resourceValues(forKeys: [.isRegularFileKey]).isRegularFile) == true { names.append(url.lastPathComponent) }
        }
        return names
    }
}

/// Deterministic bytes of `count` length; each part of a test differs from the others.
func pattern(_ count: Int) -> Data {
    Data((0..<count).map { UInt8(truncatingIfNeeded: $0 &* 31 &+ $0 / 251) })
}

let threadTarget = ShareTarget(kind: .thread, channelID: channelID, threadID: threadID, path: "tickets > Invoice export fails")
let channelTarget = ShareTarget(kind: .channel, channelID: channelID, threadID: mainThreadID, path: "tickets")
let forumTarget = ShareTarget(kind: .newPost, channelID: "0192f4a0-7b1c-72d4-8bae-7661d2b2f999", threadID: nil, path: "ideas")
