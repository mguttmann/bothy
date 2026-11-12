// SPDX-License-Identifier: Apache-2.0
//
// The queues of the plugin channel (docs/decisions.d/AUD-pkg-device-key-01.md): a step-up
// signature waits for the person, and nothing else may wait behind it. The store of these tests
// holds the step-up key's secret back the way the system holds the signature back while its
// prompt is open.

import Foundation
import Testing

@testable import DeviceKeyCore

/// A store whose step-up secrets wait until `open()`; everything else answers at once.
private final class GatedStore: KeyStore, @unchecked Sendable {
    private let inner = InMemoryKeyStore()
    private let lock = NSLock()
    private let gate = DispatchSemaphore(value: 0)
    private let reached = DispatchSemaphore(value: 0)

    /// Lets one waiting step-up read go on.
    func open() { gate.signal() }

    /// Waits until a step-up read is held back; false after `seconds`.
    func waitUntilHeld(seconds: Double) -> Bool {
        reached.wait(timeout: .now() + seconds) == .success
    }

    func add(_ item: KeyItem) throws {
        try lock.withLock { try inner.add(item) }
    }

    func attributes(service: String, account: String) throws -> KeyItemAttributes? {
        try lock.withLock { try inner.attributes(service: service, account: account) }
    }

    func secret(service: String, account: String) throws -> Data? {
        if account.hasSuffix(".\(KeyPurpose.stepUp.rawValue)") {
            reached.signal()
            gate.wait()
        }
        return try lock.withLock { try inner.secret(service: service, account: account) }
    }

    func delete(service: String, account: String) throws {
        try lock.withLock { try inner.delete(service: service, account: account) }
    }
}

/// Hands a value to a continuation once; later values are dropped.
private final class Once<T: Sendable>: @unchecked Sendable {
    private let lock = NSLock()
    private var continuation: CheckedContinuation<T, Never>?

    init(_ continuation: CheckedContinuation<T, Never>) {
        self.continuation = continuation
    }

    func resume(_ value: T) {
        let waiting = lock.withLock { () -> CheckedContinuation<T, Never>? in
            defer { continuation = nil }
            return continuation
        }
        waiting?.resume(returning: value)
    }
}

/// The result of `body`, or nil when it has not answered after `seconds`; `body` then runs on
/// by itself, so a call that waits behind a held queue does not hold the test.
private func within<T: Sendable>(seconds: Double, _ body: @escaping @Sendable () async throws -> T) async -> T? {
    await withCheckedContinuation { (continuation: CheckedContinuation<T?, Never>) in
        let once = Once(continuation)
        Task { once.resume(try? await body()) }
        Task {
            try? await Task.sleep(nanoseconds: UInt64(seconds * 1_000_000_000))
            once.resume(nil)
        }
    }
}

private let scope = "https://chat.example.com"

private func makeQueues(_ store: GatedStore) -> DeviceKeyQueues {
    DeviceKeyQueues(
        manager: DeviceKeyManager(
            store: store,
            capabilities: PlatformCapabilities(enclaveAvailable: false, biometryEnrolled: false),
            labelPrefix: "test.device_key.\(UUID().uuidString)"))
}

@Test func testDpopSignatureAnswersWhileAStepUpPromptIsOpen() async throws {
    let store = GatedStore()
    let queues = makeQueues(store)
    _ = try await queues.ensureKey(scope: scope, purpose: .dpop)
    _ = try await queues.ensureKey(scope: scope, purpose: .stepUp)

    // The step-up signature starts and waits, as it does while the system prompt is open.
    let confirmation = Task { try await queues.sign(scope: scope, purpose: .stepUp, data: Data("challenge".utf8)) }
    #expect(store.waitUntilHeld(seconds: 5))

    // A DPoP proof of a request, a refresh or a gateway reauth must not wait behind it.
    let proof = await within(seconds: 5) {
        try await queues.sign(scope: scope, purpose: .dpop, data: Data("header.payload".utf8))
    }
    #expect(proof?.count == 64, "a DPoP signature waits behind the open step-up prompt")

    // Neither do the calls that read, create or delete keys.
    let info = await within(seconds: 5) { try await queues.keyInfo(scope: scope, purpose: .dpop) }
    #expect(info != .none, "a key lookup waits behind the open step-up prompt")

    store.open()
    #expect(try await confirmation.value.count == 64)
}

@Test func testDpopSignaturesRunSideBySide() async throws {
    let store = GatedStore()
    let queues = makeQueues(store)
    _ = try await queues.ensureKey(scope: scope, purpose: .dpop)
    let signatures = try await withThrowingTaskGroup(of: Data.self) { group in
        for index in 0..<16 {
            group.addTask { try await queues.sign(scope: scope, purpose: .dpop, data: Data("proof \(index)".utf8)) }
        }
        var all: [Data] = []
        for try await signature in group { all.append(signature) }
        return all
    }
    #expect(signatures.count == 16)
    #expect(signatures.allSatisfy { $0.count == 64 })
}

@Test func testQueuesReportTheManagersErrorsAndResults() async throws {
    let store = GatedStore()
    let queues = makeQueues(store)
    #expect(try await queues.keyInfo(scope: scope, purpose: .dpop) == nil)
    await #expect(throws: DeviceKeyError.notAvailable) {
        try await queues.sign(scope: scope, purpose: .dpop, data: Data([1]))
    }
    await #expect(throws: DeviceKeyError.notAvailable) {
        try await queues.attestation(scope: scope, purpose: .dpop, challenge: Data([1]))
    }
    let created = try await queues.ensureKey(scope: scope, purpose: .dpop)
    #expect(try await queues.keyInfo(scope: scope, purpose: .dpop) == created)
    #expect(try await queues.attestation(scope: scope, purpose: .dpop, challenge: Data([1])) == nil)
    try await queues.deleteKeys(scope: scope)
    #expect(try await queues.keyInfo(scope: scope, purpose: .dpop) == nil)
}
