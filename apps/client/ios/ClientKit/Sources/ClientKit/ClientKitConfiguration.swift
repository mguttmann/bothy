// SPDX-License-Identifier: Apache-2.0

import Foundation

/// The shared places of one build: the App Group and the keychain access group the app shares
/// with its extensions (docs/spec/15-client-architecture.md 9; brief M5-04 Design note 1).
///
/// Both names come from the build configuration through Info.plist keys that the app and every
/// extension carry with the same value. A build without a signing team has neither, because
/// App Groups and keychain sharing need a team signature; then push is not available on this
/// build and nothing is shared (docs/decisions.d/M5-04.md D-M5-04-1).
///
/// Example:
///
///     guard let config = ClientKitConfiguration.fromBundle() else { return }
///     let keychain = SharedKeychain(store: KeychainSecretStore(accessGroup: config.keychainGroup))
public struct ClientKitConfiguration: Equatable, Sendable {
    /// The Info.plist key of the App Group.
    public static let appGroupInfoKey = "ClientKitAppGroup"
    /// The Info.plist key of the shared keychain access group; the device key plugin reads the
    /// same key, so the DPoP key and the push items sit in one group.
    public static let keychainGroupInfoKey = "DeviceKeyAccessGroup"

    /// The App Group of the shared `UserDefaults` suite.
    public let appGroup: String
    /// The keychain access group of the shared items and of the DPoP key.
    public let keychainGroup: String

    /// Creates a configuration.
    ///
    /// - Parameters:
    ///   - appGroup: the App Group identifier.
    ///   - keychainGroup: the keychain access group, with the team prefix.
    public init(appGroup: String, keychainGroup: String) {
        self.appGroup = appGroup
        self.keychainGroup = keychainGroup
    }

    /// Reads the configuration from the Info.plist of `bundle`.
    ///
    /// - Parameter bundle: the bundle of the running app or extension.
    /// - Returns: the configuration, or nil when either key is missing or empty (a build
    ///   without a signing team, where push is not available).
    public static func fromBundle(_ bundle: Bundle = .main) -> ClientKitConfiguration? {
        from(infoValue: { bundle.object(forInfoDictionaryKey: $0) })
    }

    /// Reads the configuration from Info.plist values; separated from `fromBundle` for tests.
    ///
    /// - Parameter infoValue: returns the value of an Info.plist key.
    /// - Returns: the configuration, or nil when either value is missing, not a text or empty.
    static func from(infoValue: (String) -> Any?) -> ClientKitConfiguration? {
        guard let group = infoValue(appGroupInfoKey) as? String,
              let keychain = infoValue(keychainGroupInfoKey) as? String,
              !group.isEmpty, !keychain.isEmpty
        else {
            return nil
        }
        return ClientKitConfiguration(appGroup: group, keychainGroup: keychain)
    }
}
