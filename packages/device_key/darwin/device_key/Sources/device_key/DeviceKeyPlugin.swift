// SPDX-License-Identifier: Apache-2.0

import DeviceKeyCore
import Foundation

#if os(iOS)
    import Flutter
#elseif os(macOS)
    import FlutterMacOS
#endif

/// The Flutter plugin `device_key` for iOS and macOS: connects the Pigeon channel to
/// `DeviceKeyCore` (brief M4-10).
///
/// The keychain access group the DPoP key shares with the app extensions comes from the build
/// configuration: the app sets the Info.plist key `DeviceKeyAccessGroup` (for example to
/// `$(AppIdentifierPrefix)com.example.shared`), and the extensions read the same key, so both
/// open the same item. Without the key, or with an empty value, the item stays in the app's
/// default group.
public final class DeviceKeyPlugin: NSObject, FlutterPlugin {
    /// The Info.plist key that names the shared keychain access group.
    public static let accessGroupInfoKey = "DeviceKeyAccessGroup"

    /// Registers the host API on the plugin registrar's messenger.
    ///
    /// - Parameter registrar: the registrar Flutter passes when the app starts.
    public static func register(with registrar: FlutterPluginRegistrar) {
        #if os(iOS)
            let messenger = registrar.messenger()
        #else
            let messenger = registrar.messenger
        #endif
        // An empty value is a build without a shared group (no team signature, M5-04): the key
        // then stays in the app's default group, as without the Info.plist key.
        let group = (Bundle.main.object(forInfoDictionaryKey: accessGroupInfoKey) as? String)
            .flatMap { $0.isEmpty ? nil : $0 }
        let manager = DeviceKeyManager(store: KeychainKeyStore(), accessGroup: group)
        DeviceKeyHostApiSetup.setUp(binaryMessenger: messenger, api: DeviceKeyHostApiImpl(manager: manager))
    }
}

/// The host side of the Pigeon interface; every error leaves as a `PigeonError` with a stable code.
final class DeviceKeyHostApiImpl: DeviceKeyHostApi {
    private let manager: DeviceKeyManager
    /// Signatures wait for the user's confirmation, so they run off the platform thread.
    private let queue = DispatchQueue(label: "device_key.sign")

    init(manager: DeviceKeyManager) {
        self.manager = manager
    }

    func ensureKey(scope: String, purpose: KeyPurposeMessage) throws -> DeviceKeyInfoMessage {
        try Self.bridged { Self.message(try manager.ensureKey(scope: scope, purpose: purpose.core)) }
    }

    func keyInfo(scope: String, purpose: KeyPurposeMessage) throws -> DeviceKeyInfoMessage? {
        try Self.bridged { try manager.keyInfo(scope: scope, purpose: purpose.core).map(Self.message) }
    }

    /// Signs `data`; `prompt` is ignored here, because iOS and macOS show the system's own text
    /// (it carries the Android prompt texts, brief M5-01 Design note 1).
    func sign(
        scope: String, purpose: KeyPurposeMessage, data: FlutterStandardTypedData, prompt: PromptTextsMessage?
    ) async throws -> FlutterStandardTypedData {
        let box = ManagerBox(manager: manager)
        let bytes = data.data
        let core = purpose.core
        return try await withCheckedThrowingContinuation { continuation in
            queue.async {
                do {
                    let signature = try Self.bridged { try box.manager.sign(scope: scope, purpose: core, data: bytes) }
                    continuation.resume(returning: FlutterStandardTypedData(bytes: signature))
                } catch {
                    continuation.resume(throwing: error)
                }
            }
        }
    }

    func attestation(scope: String, purpose: KeyPurposeMessage, challenge: FlutterStandardTypedData) throws
        -> FlutterStandardTypedData?
    {
        try Self.bridged {
            try manager.attestation(scope: scope, purpose: purpose.core, challenge: challenge.data)
                .map { FlutterStandardTypedData(bytes: $0) }
        }
    }

    func deleteKeys(scope: String) throws {
        try Self.bridged { try manager.deleteKeys(scope: scope) }
    }

    private static func message(_ info: DeviceKeyInfo) -> DeviceKeyInfoMessage {
        DeviceKeyInfoMessage(
            x: FlutterStandardTypedData(bytes: info.x),
            y: FlutterStandardTypedData(bytes: info.y),
            hardwareBound: info.hardwareBound,
            requiresUserPresence: info.requiresUserPresence)
    }

    /// Runs `body` and turns its error into a `PigeonError` with the fixed code and text only.
    private static func bridged<T>(_ body: () throws -> T) throws -> T {
        do {
            return try body()
        } catch {
            let mapped = DeviceKeyError.classify(error)
            throw PigeonError(code: mapped.code, message: mapped.message, details: nil)
        }
    }
}

/// Carries the manager onto the signing queue. The manager's fields never change and the
/// keychain calls behind it are thread-safe, so handing it to another thread is sound.
private struct ManagerBox: @unchecked Sendable {
    let manager: DeviceKeyManager
}

extension KeyPurposeMessage {
    /// The purpose in the core's terms.
    var core: KeyPurpose {
        switch self {
        case .dpop: return .dpop
        case .stepUp: return .stepUp
        }
    }
}
