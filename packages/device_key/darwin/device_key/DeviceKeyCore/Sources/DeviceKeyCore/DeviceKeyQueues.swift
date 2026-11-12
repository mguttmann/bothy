// SPDX-License-Identifier: Apache-2.0

import Foundation

/// Runs the calls of the plugin channel off the calling thread, each kind on a queue of its own
/// (docs/decisions.d/AUD-pkg-device-key-01.md).
///
/// A step-up signature blocks until the person answered Touch ID, Face ID or the password, and
/// creating a key talks to the Secure Enclave and the keychain. Neither may hold the platform
/// thread, and a waiting step-up prompt must not hold the DPoP signatures of every request, of
/// the refresh and of the gateway behind it:
///
/// | Calls | Queue |
/// |---|---|
/// | `sign` with the `dpop` key | concurrent: the key never asks the person |
/// | `sign` with the `step_up` key | serial: one system prompt at a time |
/// | `ensureKey`, `keyInfo`, `attestation`, `deleteKeys` | serial: they change or read the stored keys in the order of the calls |
///
/// Example:
///
///     let queues = DeviceKeyQueues(manager: DeviceKeyManager(store: KeychainKeyStore()))
///     let signature = try await queues.sign(scope: "https://chat.example.com", purpose: .dpop, data: proof)
///
/// The manager's fields never change and the keychain calls behind it are thread-safe, so using
/// it from these queues is sound.
public final class DeviceKeyQueues: @unchecked Sendable {
    private let manager: DeviceKeyManager
    private let dpopSignatures = DispatchQueue(
        label: "device_key.sign.dpop", qos: .userInitiated, attributes: .concurrent)
    private let stepUpSignatures = DispatchQueue(label: "device_key.sign.step_up", qos: .userInitiated)
    private let storedKeys = DispatchQueue(label: "device_key.keys", qos: .userInitiated)

    /// Creates the queues over `manager`.
    ///
    /// - Parameter manager: the device keys of the app.
    public init(manager: DeviceKeyManager) {
        self.manager = manager
    }

    /// See `DeviceKeyManager.ensureKey(scope:purpose:)`.
    public func ensureKey(scope: String, purpose: KeyPurpose) async throws -> DeviceKeyInfo {
        try await run(on: storedKeys) { try $0.ensureKey(scope: scope, purpose: purpose) }
    }

    /// See `DeviceKeyManager.keyInfo(scope:purpose:)`.
    public func keyInfo(scope: String, purpose: KeyPurpose) async throws -> DeviceKeyInfo? {
        try await run(on: storedKeys) { try $0.keyInfo(scope: scope, purpose: purpose) }
    }

    /// See `DeviceKeyManager.sign(scope:purpose:data:)`; the queue follows from `purpose`.
    public func sign(scope: String, purpose: KeyPurpose, data: Data) async throws -> Data {
        try await run(on: purpose == .dpop ? dpopSignatures : stepUpSignatures) {
            try $0.sign(scope: scope, purpose: purpose, data: data)
        }
    }

    /// See `DeviceKeyManager.attestation(scope:purpose:challenge:)`.
    public func attestation(scope: String, purpose: KeyPurpose, challenge: Data) async throws -> Data? {
        try await run(on: storedKeys) { try $0.attestation(scope: scope, purpose: purpose, challenge: challenge) }
    }

    /// See `DeviceKeyManager.deleteKeys(scope:)`.
    public func deleteKeys(scope: String) async throws {
        try await run(on: storedKeys) { try $0.deleteKeys(scope: scope) }
    }

    private func run<T>(on queue: DispatchQueue, _ body: @escaping (DeviceKeyManager) throws -> T) async throws -> T {
        try await withCheckedThrowingContinuation { continuation in
            queue.async {
                continuation.resume(with: Result { try body(self.manager) })
            }
        }
    }
}
