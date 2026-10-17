// SPDX-License-Identifier: Apache-2.0
//
// Tests of brief M4-10 for the Swift core. They use Swift Testing, which the command line tools of
// a Mac without Xcode ship as well as Xcode, so the hardware path can be run on any Mac with an
// Apple chip (docs/DECISIONS.md D-M4-10-6). Every key carries a test-only label prefix and is
// deleted by a `defer`, which also runs after a failed expectation.

import CryptoKit
import CryptoTokenKit
import Foundation
import LocalAuthentication
import Security
import Testing

@testable import DeviceKeyCore

private let enclaveSkipReason: Comment = "secure enclave not available on this host"

/// A manager over an in-memory store with a unique test prefix; software keys unless told otherwise.
private func makeManager(
    enclave: Bool = false,
    biometry: Bool = false,
    store: KeyStore = InMemoryKeyStore(),
    accessGroup: String? = nil
) -> DeviceKeyManager {
    DeviceKeyManager(
        store: store,
        capabilities: PlatformCapabilities(enclaveAvailable: enclave, biometryEnrolled: biometry),
        labelPrefix: "test.device_key.\(UUID().uuidString)",
        accessGroup: accessGroup
    )
}

/// Whether this process may write to the Data Protection keychain; a test binary without the
/// keychain entitlement gets errSecMissingEntitlement and the round-trip test reports a skip.
private func keychainUsable() -> Bool {
    let query: [String: Any] = [
        kSecClass as String: kSecClassGenericPassword,
        kSecAttrService as String: "test.device_key.probe.\(UUID().uuidString)",
        kSecAttrAccount as String: "probe",
        kSecUseDataProtectionKeychain as String: true,
        kSecAttrAccessible as String: kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly,
        kSecValueData as String: Data([0]),
    ]
    let status = SecItemAdd(query as CFDictionary, nil)
    SecItemDelete(query.filter { $0.key != kSecValueData as String && $0.key != kSecAttrAccessible as String } as CFDictionary)
    return status == errSecSuccess
}

private func publicKey(_ info: DeviceKeyInfo) throws -> P256.Signing.PublicKey {
    try P256.Signing.PublicKey(x963Representation: Data([0x04]) + info.x + info.y)
}

@Test func testCreatesTwoDistinctKeysPerScope() throws {
    let manager = makeManager()
    defer { try? manager.deleteKeys(scope: "https://chat.example.com") }
    let dpop = try manager.ensureKey(scope: "https://chat.example.com", purpose: .dpop)
    let stepUp = try manager.ensureKey(scope: "https://chat.example.com", purpose: .stepUp)
    #expect(dpop.x.count == 32 && dpop.y.count == 32)
    #expect(stepUp.x.count == 32 && stepUp.y.count == 32)
    #expect(dpop.x + dpop.y != stepUp.x + stepUp.y)
    let again = try manager.ensureKey(scope: "https://chat.example.com", purpose: .dpop)
    #expect(again == dpop)
    #expect(try manager.keyInfo(scope: "https://chat.example.com", purpose: .dpop) == dpop)
    #expect(dpop.requiresUserPresence == false)
    #expect(stepUp.requiresUserPresence == true)
}

@Test func testScopesAreSeparated() throws {
    let store = InMemoryKeyStore()
    let manager = makeManager(store: store)
    defer {
        try? manager.deleteKeys(scope: "https://a.example.com")
        try? manager.deleteKeys(scope: "https://b.example.com")
    }
    let a = try manager.ensureKey(scope: "https://a.example.com", purpose: .dpop)
    let b = try manager.ensureKey(scope: "https://b.example.com", purpose: .dpop)
    #expect(a.x + a.y != b.x + b.y)
    // The scope is hashed into the label: no item names the server in clear text.
    for item in store.items.values {
        #expect(!item.account.contains("example.com"))
        #expect(!item.label.contains("example.com"))
    }
    try manager.deleteKeys(scope: "https://a.example.com")
    #expect(try manager.keyInfo(scope: "https://a.example.com", purpose: .dpop) == nil)
    #expect(try manager.keyInfo(scope: "https://b.example.com", purpose: .dpop) == b)
}

@Test func testSignatureIsRawAndVerifies() throws {
    let manager = makeManager()
    defer { try? manager.deleteKeys(scope: "https://chat.example.com") }
    let info = try manager.ensureKey(scope: "https://chat.example.com", purpose: .dpop)
    let data = Data("header.payload".utf8)
    let raw = try manager.sign(scope: "https://chat.example.com", purpose: .dpop, data: data)
    #expect(raw.count == 64)
    let signature = try P256.Signing.ECDSASignature(rawRepresentation: raw)
    let key = try publicKey(info)
    #expect(key.isValidSignature(signature, for: data))
    #expect(!key.isValidSignature(signature, for: Data("header.other".utf8)))
}

@Test(.enabled(if: SecureEnclave.isAvailable, enclaveSkipReason))
func testHardwarePathOnAppleChip() throws {
    let store = InMemoryKeyStore()
    let manager = makeManager(enclave: true, store: store)
    defer { try? manager.deleteKeys(scope: "https://chat.example.com") }
    // Only the dpop key: its access control asks for nothing, so no test waits for Touch ID.
    let info = try manager.ensureKey(scope: "https://chat.example.com", purpose: .dpop)
    #expect(info.hardwareBound)
    let item = try #require(store.items.values.first)
    #expect(item.attributes.hardwareBound)
    // The stored secret is the enclave-wrapped blob: no raw private key can be made from it.
    #expect(item.secret.count != 32)
    #expect(throws: (any Error).self) { try P256.Signing.PrivateKey(rawRepresentation: item.secret) }
    let restored = try SecureEnclave.P256.Signing.PrivateKey(dataRepresentation: item.secret)
    #expect(restored.publicKey.x963Representation == Data([0x04]) + info.x + info.y)
    let data = Data("proof".utf8)
    let raw = try manager.sign(scope: "https://chat.example.com", purpose: .dpop, data: data)
    #expect(raw.count == 64)
    #expect(try publicKey(info).isValidSignature(.init(rawRepresentation: raw), for: data))
}

@Test func testFallbackWithoutEnclave() throws {
    let store = InMemoryKeyStore()
    let manager = makeManager(enclave: false, store: store)
    defer { try? manager.deleteKeys(scope: "https://chat.example.com") }
    let info = try manager.ensureKey(scope: "https://chat.example.com", purpose: .dpop)
    #expect(info.hardwareBound == false)
    let data = Data("proof".utf8)
    let raw = try manager.sign(scope: "https://chat.example.com", purpose: .dpop, data: data)
    #expect(try publicKey(info).isValidSignature(.init(rawRepresentation: raw), for: data))
    // The step-up key without an enclave carries user presence on the keychain item.
    _ = try manager.ensureKey(scope: "https://chat.example.com", purpose: .stepUp)
    let stepUp = try #require(store.items.values.first { $0.account.hasSuffix(".step_up") })
    #expect(stepUp.attributes.policy.itemFlags == [.userPresence])
    #expect(stepUp.attributes.hardwareBound == false)
}

@Test(arguments: [false, true])
func testStepUpRequiresUserPresenceFlag(biometry: Bool) throws {
    let confirm: Set<AccessFlag> = [.userPresence, .biometryCurrentSet]
    // Stored items, software path: the step-up item asks the user, the dpop item does not.
    let store = InMemoryKeyStore()
    let manager = makeManager(enclave: false, biometry: biometry, store: store)
    defer { try? manager.deleteKeys(scope: "https://chat.example.com") }
    _ = try manager.ensureKey(scope: "https://chat.example.com", purpose: .dpop)
    _ = try manager.ensureKey(scope: "https://chat.example.com", purpose: .stepUp)
    let dpop = try #require(store.items.values.first { $0.account.hasSuffix(".dpop") })
    let stepUp = try #require(store.items.values.first { $0.account.hasSuffix(".step_up") })
    #expect(dpop.attributes.policy.allFlags.isDisjoint(with: confirm))
    #expect(!stepUp.attributes.policy.allFlags.isDisjoint(with: confirm))
    #expect(dpop.attributes.policy.accessibility == .afterFirstUnlockThisDeviceOnly)
    #expect(stepUp.attributes.policy.accessibility == .whenPasscodeSetThisDeviceOnly)
    // Enclave path: the policy the manager passes to the enclave key. Creating an enclave key
    // with WhenPasscodeSet needs an unlocked user session, which a build host over ssh lacks
    // (errSecInteractionNotAllowed), so the policy is checked where it is decided.
    let enclave = PlatformCapabilities(enclaveAvailable: true, biometryEnrolled: biometry)
    let expected: AccessFlag = biometry ? .biometryCurrentSet : .userPresence
    let enclaveStepUp = KeyPolicy.policy(for: .stepUp, capabilities: enclave)
    let enclaveDpop = KeyPolicy.policy(for: .dpop, capabilities: enclave)
    #expect(enclaveStepUp.keyFlags == [.privateKeyUsage, expected])
    #expect(enclaveStepUp.itemFlags.isEmpty)
    #expect(enclaveStepUp.accessibility == .whenPasscodeSetThisDeviceOnly)
    #expect(enclaveDpop.keyFlags == [.privateKeyUsage])
    #expect(enclaveDpop.allFlags.isDisjoint(with: confirm))
    #expect(enclaveDpop.accessibility == .afterFirstUnlockThisDeviceOnly)
}

@Test func testItemsAreThisDeviceOnlyAndNotSynchronizable() throws {
    let store = InMemoryKeyStore()
    let manager = makeManager(store: store, accessGroup: "TEAMID.com.example.shared")
    defer { try? manager.deleteKeys(scope: "https://chat.example.com") }
    _ = try manager.ensureKey(scope: "https://chat.example.com", purpose: .dpop)
    _ = try manager.ensureKey(scope: "https://chat.example.com", purpose: .stepUp)
    #expect(store.items.count == 2)
    for item in store.items.values {
        #expect(item.attributes.policy.accessibility.isThisDeviceOnly)
        #expect(item.attributes.synchronizable == false)
    }
    // Only the dpop key is shared with the extensions.
    let dpop = try #require(store.items.values.first { $0.account.hasSuffix(".dpop") })
    let stepUp = try #require(store.items.values.first { $0.account.hasSuffix(".step_up") })
    #expect(dpop.attributes.accessGroup == "TEAMID.com.example.shared")
    #expect(stepUp.attributes.accessGroup == nil)
}

@Test(.enabled(if: keychainUsable(), "the process lacks the keychain entitlement (errSecMissingEntitlement)"))
func testKeychainRoundTrip() throws {
    let manager = DeviceKeyManager(
        store: KeychainKeyStore(),
        capabilities: PlatformCapabilities(enclaveAvailable: SecureEnclave.isAvailable, biometryEnrolled: false),
        labelPrefix: "test.device_key.\(UUID().uuidString)",
        accessGroup: nil
    )
    let scope = "https://chat.example.com"
    defer { try? manager.deleteKeys(scope: scope) }
    let info = try manager.ensureKey(scope: scope, purpose: .dpop)
    #expect(try manager.ensureKey(scope: scope, purpose: .dpop) == info)
    let attributes = try #require(try manager.storedAttributes(scope: scope, purpose: .dpop))
    #expect(attributes.policy.accessibility == .afterFirstUnlockThisDeviceOnly)
    #expect(attributes.synchronizable == false)
    let data = Data("proof".utf8)
    let raw = try manager.sign(scope: scope, purpose: .dpop, data: data)
    #expect(try publicKey(info).isValidSignature(.init(rawRepresentation: raw), for: data))
    try manager.deleteKeys(scope: scope)
    #expect(try manager.keyInfo(scope: scope, purpose: .dpop) == nil)
}

@Test func testDeleteRemovesBothKeys() throws {
    let store = InMemoryKeyStore()
    let manager = makeManager(store: store)
    defer { try? manager.deleteKeys(scope: "https://chat.example.com") }
    _ = try manager.ensureKey(scope: "https://chat.example.com", purpose: .dpop)
    _ = try manager.ensureKey(scope: "https://chat.example.com", purpose: .stepUp)
    try manager.deleteKeys(scope: "https://chat.example.com")
    #expect(store.items.isEmpty)
    #expect(try manager.keyInfo(scope: "https://chat.example.com", purpose: .dpop) == nil)
    #expect(try manager.keyInfo(scope: "https://chat.example.com", purpose: .stepUp) == nil)
    // Deleting again is not an error: the app may retry after a crash.
    try manager.deleteKeys(scope: "https://chat.example.com")
    #expect(throws: DeviceKeyError.notAvailable) {
        try manager.sign(scope: "https://chat.example.com", purpose: .dpop, data: Data())
    }
}

@Test func testErrorsCarryNoKeyMaterial() throws {
    let store = InMemoryKeyStore()
    let manager = makeManager(store: store)
    defer { try? manager.deleteKeys(scope: "https://chat.example.com") }
    let info = try manager.ensureKey(scope: "https://chat.example.com", purpose: .dpop)
    let item = try #require(store.items.values.first)
    // A corrupt secret must fail without echoing it or the public key.
    store.items[item.key] = KeyItem(
        service: item.service, account: item.account, label: item.label,
        secret: Data(repeating: 0xAB, count: 7), attributes: item.attributes)
    var thrown: DeviceKeyError?
    do {
        _ = try manager.sign(scope: "https://chat.example.com", purpose: .dpop, data: Data("x".utf8))
    } catch let error as DeviceKeyError {
        thrown = error
    }
    let error = try #require(thrown)
    let texts = [error.code, error.message, String(describing: error), String(reflecting: error)]
    let forbidden = [item.secret, info.x, info.y, item.attributes.publicKey].flatMap { bytes in
        [bytes.base64EncodedString(), bytes.map { String(format: "%02x", $0) }.joined()]
    } + ["chat.example.com"]
    for text in texts {
        for needle in forbidden where !needle.isEmpty {
            #expect(!text.contains(needle))
        }
    }
    #expect(DeviceKeyError.allCases.map(\.code) == ["user_cancelled", "key_invalidated", "not_available", "internal"])
}

@Test func testPlatformErrorsMapToStableCodes() {
    let cases: [(Error, DeviceKeyError)] = [
        (NSError(domain: NSOSStatusErrorDomain, code: Int(errSecUserCanceled)), .userCancelled),
        (NSError(domain: LAError.errorDomain, code: LAError.userCancel.rawValue), .userCancelled),
        (NSError(domain: LAError.errorDomain, code: LAError.systemCancel.rawValue), .userCancelled),
        (NSError(domain: LAError.errorDomain, code: LAError.biometryNotEnrolled.rawValue), .keyInvalidated),
        (NSError(domain: NSOSStatusErrorDomain, code: Int(errSecMissingEntitlement)), .notAvailable),
        (NSError(domain: NSOSStatusErrorDomain, code: Int(errSecInteractionNotAllowed)), .notAvailable),
        (NSError(domain: LAError.errorDomain, code: LAError.passcodeNotSet.rawValue), .notAvailable),
        (NSError(domain: "other", code: 1), .internal),
        (DeviceKeyError.keyInvalidated, .keyInvalidated),
    ]
    for (error, expected) in cases {
        #expect(DeviceKeyError.classify(error) == expected)
    }
}

@Test func testAttestationIsAbsentOnApplePlatforms() throws {
    let manager = makeManager()
    defer { try? manager.deleteKeys(scope: "https://chat.example.com") }
    _ = try manager.ensureKey(scope: "https://chat.example.com", purpose: .dpop)
    #expect(try manager.attestation(scope: "https://chat.example.com", purpose: .dpop, challenge: Data([1])) == nil)
}

@Test func testLabelHashesTheScope() {
    let label = DeviceKeyManager.account(scope: "https://chat.example.com", purpose: .stepUp)
    let digest = SHA256.hash(data: Data("https://chat.example.com".utf8)).map { String(format: "%02x", $0) }.joined()
    #expect(label == "\(digest).step_up")
}

@Test func testBiometryCountsAsEnrolledWhenOnlyUnavailableNow() {
    // Enrolled and usable.
    #expect(PlatformCapabilities.isBiometryEnrolled(canEvaluate: true, biometryType: .touchID, errorCode: nil))
    // Enrolled, but the lid is closed, the sensor is locked out or busy: still enrolled, so the
    // step-up key is bound to the biometrics (08 6.1) and not to the device password.
    for code in [LAError.systemCancel, .biometryLockout, .biometryNotAvailable, .notInteractive] {
        #expect(PlatformCapabilities.isBiometryEnrolled(canEvaluate: false, biometryType: .touchID, errorCode: code.rawValue))
    }
    // Nothing enrolled, or no sensor at all.
    #expect(!PlatformCapabilities.isBiometryEnrolled(
        canEvaluate: false, biometryType: .touchID, errorCode: LAError.biometryNotEnrolled.rawValue))
    #expect(!PlatformCapabilities.isBiometryEnrolled(
        canEvaluate: false, biometryType: .none, errorCode: LAError.biometryNotAvailable.rawValue))
}

@Test(.enabled(if: SecureEnclave.isAvailable, enclaveSkipReason))
func testRejectedEnclaveKeyReportsKeyInvalidated() throws {
    // The enclave accepts any blob when the key is loaded and rejects it only when it signs
    // (CryptoTokenKit corruptedData); that must reach the app as key_invalidated (08 5.4).
    let store = InMemoryKeyStore()
    let manager = makeManager(enclave: true, store: store)
    defer { try? manager.deleteKeys(scope: "https://chat.example.com") }
    _ = try manager.ensureKey(scope: "https://chat.example.com", purpose: .dpop)
    let item = try #require(store.items.values.first)
    var blob = item.secret
    blob[blob.count / 2] ^= 0xFF
    store.items[item.key] = KeyItem(
        service: item.service, account: item.account, label: item.label, secret: blob, attributes: item.attributes)
    #expect(throws: DeviceKeyError.keyInvalidated) {
        try manager.sign(scope: "https://chat.example.com", purpose: .dpop, data: Data("proof".utf8))
    }
}

@Test func testTokenErrorsMapToStableCodes() {
    let cases: [(TKError.Code, DeviceKeyError)] = [
        (.corruptedData, .keyInvalidated),
        (.objectNotFound, .keyInvalidated),
        (.canceledByUser, .userCancelled),
        (.communicationError, .internal),
    ]
    for (code, expected) in cases {
        #expect(DeviceKeyError.classify(NSError(domain: TKError.errorDomain, code: code.rawValue)) == expected)
    }
}
