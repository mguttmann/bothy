// SPDX-License-Identifier: Apache-2.0
//
// Test doubles of the ClientKit tests: a software signer, a recording HTTP stub (the fake
// server), and a timer the test fires itself, so no test depends on the wall clock.

import CryptoKit
import Foundation
import UserNotifications

@testable import ClientKit

/// A software P-256 key as the `dpop` device key.
final class TestSigner: ProofSigner {
    let signing: P256.Signing.PrivateKey
    var x: Data { signing.publicKey.x963Representation.subdata(in: 1..<33) }
    var y: Data { signing.publicKey.x963Representation.subdata(in: 33..<65) }

    init(signing: P256.Signing.PrivateKey = P256.Signing.PrivateKey()) {
        self.signing = signing
    }

    func sign(_ data: Data) throws -> Data {
        try signing.signature(for: data).rawRepresentation
    }
}

/// The fake server: records every request and answers with the handler's response, or waits
/// until the task is cancelled when the handler returns nil (a server that never answers).
final class StubTransport: HTTPTransport {
    struct Reply {
        var status: Int
        var headers: [String: String] = [:]
        var body: Data = Data()
    }

    enum Failure: Error { case offline }

    private let lock = NSLock()
    private var recorded: [URLRequest] = []
    private let handler: (URLRequest) throws -> Reply?
    /// The requests that never answer, released by the test that started them.
    let hang = Hang()

    init(handler: @escaping (URLRequest) throws -> Reply?) {
        self.handler = handler
    }

    var requests: [URLRequest] {
        lock.lock()
        defer { lock.unlock() }
        return recorded
    }

    func send(_ request: URLRequest) async throws -> (Data, HTTPURLResponse) {
        lock.lock()
        recorded.append(request)
        lock.unlock()
        guard let reply = try handler(request) else {
            // Never answers: wait until cancelled, like a server behind a dead network.
            await withCheckedContinuation { (continuation: CheckedContinuation<Void, Never>) in
                hang.park(continuation)
            }
            throw URLError(.timedOut)
        }
        let response = HTTPURLResponse(
            url: request.url!, statusCode: reply.status, httpVersion: "HTTP/1.1", headerFields: reply.headers)!
        return (reply.body, response)
    }
}

/// Keeps the continuations of hanging requests so they can be released at the end of a test.
final class Hang {
    private let lock = NSLock()
    private var parked: [CheckedContinuation<Void, Never>] = []

    func park(_ continuation: CheckedContinuation<Void, Never>) {
        lock.lock()
        parked.append(continuation)
        lock.unlock()
    }

    func releaseAll() {
        lock.lock()
        let all = parked
        parked = []
        lock.unlock()
        all.forEach { $0.resume() }
    }
}

/// A timer that ends only when the test fires it.
final class ManualSleeper: Sleeper {
    private let lock = NSLock()
    private var waiting: [CheckedContinuation<Void, Never>] = []
    private var started: [CheckedContinuation<Void, Never>] = []
    private(set) var requested: [TimeInterval] = []

    func sleep(seconds: TimeInterval) async {
        await withCheckedContinuation { (continuation: CheckedContinuation<Void, Never>) in
            lock.lock()
            requested.append(seconds)
            waiting.append(continuation)
            let observers = started
            started = []
            lock.unlock()
            observers.forEach { $0.resume() }
        }
    }

    /// Waits until a sleep has begun (an event, not a time).
    func untilSleeping() async {
        await withCheckedContinuation { (continuation: CheckedContinuation<Void, Never>) in
            lock.lock()
            if !waiting.isEmpty {
                lock.unlock()
                continuation.resume()
                return
            }
            started.append(continuation)
            lock.unlock()
        }
    }

    /// Ends every running sleep.
    func fire() {
        lock.lock()
        let all = waiting
        waiting = []
        lock.unlock()
        all.forEach { $0.resume() }
    }
}

/// A clock that moves only when the test moves it.
final class TestClock {
    private let lock = NSLock()
    private var current = Date(timeIntervalSince1970: 1_760_000_000)

    func now() -> Date {
        lock.lock()
        defer { lock.unlock() }
        return current
    }

    func advance(_ seconds: TimeInterval) {
        lock.lock()
        current += seconds
        lock.unlock()
    }
}

let testStrings = PushStrings(notAvailable: "Message not available", titleFormat: "%1$@ · %2$@", textFormat: "%1$@: %2$@")
let instanceID = "0192a000-0000-7000-8000-00000000c0de"
let server = URL(string: "https://chat.example.com")!
let previewToken = "bothy_pvw_EXAMPLEpushpreviewtoken00000000000000000000_4Z1L0A"

/// A push of 13 6.1 with the placeholder already localized.
func pushContent(kind: String = "mention", version: Int = 1, message: String? = "0192f1d3-0a44-7f00-b1c2-d3e4f5a6b7c8") -> UNMutableNotificationContent {
    let content = UNMutableNotificationContent()
    content.title = "New notification"
    content.body = "You were mentioned"
    content.threadIdentifier = "0192f1d2-7c4e-7a10-9a3b-2f6c1e0d4a11"
    content.badge = 3
    content.sound = .default
    var b: [String: Any] = [
        "v": version, "nid": "0192f1d3-0a55-7b21-8c4d-5e6f7a8b9c0d", "k": kind, "i": instanceID,
        "t": "0192f1d2-7c4e-7a10-9a3b-2f6c1e0d4a11",
    ]
    if let message { b["m"] = message }
    content.userInfo = ["b": b]
    return content
}

/// The pieces of one extension run.
struct World {
    let state = SharedState(store: MemoryStore())
    let secrets = MemorySecretStore()
    let signer = TestSigner()
    let clock = TestClock()
    let sleeper = ManualSleeper()
    let transport: StubTransport

    init(mode: PreviewMode? = .full, credentials: Bool = true, handler: @escaping (URLRequest) throws -> StubTransport.Reply?) {
        transport = StubTransport(handler: handler)
        if let mode {
            state.writeSettings(instanceID: instanceID, previewMode: mode, fetchTimeoutMillis: 20_000)
        }
        if credentials {
            try! SharedKeychain(store: secrets).save(
                PushCredentials(previewToken: previewToken, registrationID: "0192f4a0-7b1c-72f6-893c-a890548910a3", serverURL: server),
                instanceID: instanceID)
        }
    }

    var environment: NotificationEnvironment {
        let signer = self.signer
        let clock = self.clock
        return NotificationEnvironment(
            state: state, keychain: SharedKeychain(store: secrets),
            counters: DisplayCounters(store: state.store), nonces: NonceCache(store: state.store),
            transport: transport, signer: { $0 == instanceID ? signer : nil }, strings: testStrings,
            now: clock.now, sleeper: sleeper)
    }
}

/// A JSON body.
func json(_ object: [String: Any]) -> Data {
    try! JSONSerialization.data(withJSONObject: object)
}
