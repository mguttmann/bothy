// SPDX-License-Identifier: Apache-2.0
//
// The shared keychain, the App Group state and the display counters (brief M5-04 Design note 1).

import Foundation
import Security
import Testing

@testable import ClientKit

private let testGroup = "clientkit.tests.shared"

/// Whether this process may write to the shared access group; a test binary without the keychain
/// entitlement gets errSecMissingEntitlement and the round trip reports a skip.
private func sharedGroupUsable() -> Bool {
    let attributes = KeychainSecretStore.addAttributes(
        value: Data([0]), service: "clientkit.probe.\(UUID().uuidString)", account: "probe", accessGroup: testGroup)
    let status = SecItemAdd(attributes as CFDictionary, nil)
    SecItemDelete(attributes.filter { $0.key != kSecValueData as String && $0.key != kSecAttrAccessible as String && $0.key != kSecAttrSynchronizable as String } as CFDictionary)
    return status == errSecSuccess
}

@Test func testSharedKeychainThisDeviceOnly() throws {
    let attributes = KeychainSecretStore.addAttributes(
        value: Data("x".utf8), service: SharedKeychain.service, account: "a", accessGroup: testGroup)
    #expect(attributes[kSecAttrAccessible as String] as? String == kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly as String)
    #expect((attributes[kSecAttrSynchronizable as String] as? Bool) == false)
    #expect(attributes[kSecAttrAccessGroup as String] as? String == testGroup)
    #expect(attributes[kSecUseDataProtectionKeychain as String] as? Bool == true)

    // The in-memory view: save, load, replace, delete per instance.
    let keychain = SharedKeychain(store: MemorySecretStore())
    let first = PushCredentials(previewToken: "bothy_pvw_a", registrationID: "r1", serverURL: server)
    try keychain.save(first, instanceID: instanceID)
    #expect(try keychain.load(instanceID: instanceID) == first)
    #expect(try keychain.load(instanceID: "other") == nil)
    let second = PushCredentials(previewToken: "bothy_pvw_b", registrationID: "r2", serverURL: server)
    try keychain.save(second, instanceID: instanceID)
    #expect(try keychain.load(instanceID: instanceID) == second)
    try keychain.delete(instanceID: instanceID)
    #expect(try keychain.load(instanceID: instanceID) == nil)
    #expect(!"\(second)".contains("bothy_pvw"))
    // The instance ID is hashed in the account, as the device keys do.
    #expect(!SharedKeychain.account(instanceID: instanceID, name: "preview_token").contains(instanceID))
}

@Test(.enabled(if: sharedGroupUsable(), "the test binary has no keychain access group entitlement"))
func testSharedKeychainRealRoundTrip() throws {
    let keychain = SharedKeychain(store: KeychainSecretStore(accessGroup: testGroup))
    let scope = "test.\(UUID().uuidString)"
    defer { try? keychain.delete(instanceID: scope) }
    let credentials = PushCredentials(previewToken: "bothy_pvw_a", registrationID: "r1", serverURL: server)
    try keychain.save(credentials, instanceID: scope)
    #expect(try keychain.load(instanceID: scope) == credentials)
}

@Test func testConfigurationNeedsBothGroups() {
    #expect(ClientKitConfiguration.from(infoValue: { _ in nil }) == nil)
    #expect(ClientKitConfiguration.from(infoValue: { _ in "" }) == nil)
    let values = ["ClientKitAppGroup": "group.com.example.chat", "DeviceKeyAccessGroup": "TEAMID1234.com.example.chat.shared"]
    #expect(ClientKitConfiguration.from(infoValue: { values[$0] }) ==
        ClientKitConfiguration(appGroup: "group.com.example.chat", keychainGroup: "TEAMID1234.com.example.chat.shared"))
}

@Test func testSharedStateValues() {
    let state = SharedState(store: MemoryStore())
    #expect(state.previewMode == nil)
    state.writeSettings(instanceID: instanceID, previewMode: .names, fetchTimeoutMillis: 20_000)
    #expect(state.previewMode == .names)
    #expect(state.instanceID == instanceID)
    #expect(state.fetchTimeoutMillis == 20_000)
    state.clearAccount()
    #expect(state.previewMode == nil && state.instanceID == nil)
}

@Test func testDisplayCountersBucketsAndReset() {
    let counters = DisplayCounters(store: MemoryStore())
    counters.record(duration: 0.3, placeholder: false)
    counters.record(duration: 0.3, placeholder: true)
    counters.record(duration: 99, placeholder: true)
    counters.record(duration: -1, placeholder: false)
    let snapshot = counters.take()
    #expect(snapshot.placeholders == 2)
    #expect(snapshot.buckets[DisplayCounters.bounds.firstIndex(of: 0.5)!] == 2)
    #expect(snapshot.buckets.last == 1)
    #expect(snapshot.buckets.reduce(0, +) == 3)
    #expect(counters.take().isEmpty)
}
