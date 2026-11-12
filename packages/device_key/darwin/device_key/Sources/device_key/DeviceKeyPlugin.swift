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
///
/// No call works on the platform thread: each runs on the queue `DeviceKeyQueues` gives its kind,
/// so a step-up signature that waits for the person holds neither the DPoP signatures nor the
/// calls that read or create keys.
final class DeviceKeyHostApiImpl: DeviceKeyHostApi {
    private let queues: DeviceKeyQueues

    init(manager: DeviceKeyManager) {
        queues = DeviceKeyQueues(manager: manager)
    }

    func ensureKey(scope: String, purpose: KeyPurposeMessage) async throws -> DeviceKeyInfoMessage {
        do {
            return Self.message(try await queues.ensureKey(scope: scope, purpose: purpose.core))
        } catch {
            throw Self.bridged(error)
        }
    }

    func keyInfo(scope: String, purpose: KeyPurposeMessage) async throws -> DeviceKeyInfoMessage? {
        do {
            return try await queues.keyInfo(scope: scope, purpose: purpose.core).map(Self.message)
        } catch {
            throw Self.bridged(error)
        }
    }

    /// Signs `data`; `prompt` is ignored here, because iOS and macOS show the system's own text
    /// (it carries the Android prompt texts, brief M5-01 Design note 1).
    func sign(
        scope: String, purpose: KeyPurposeMessage, data: FlutterStandardTypedData, prompt: PromptTextsMessage?
    ) async throws -> FlutterStandardTypedData {
        do {
            let signature = try await queues.sign(scope: scope, purpose: purpose.core, data: data.data)
            return FlutterStandardTypedData(bytes: signature)
        } catch {
            throw Self.bridged(error)
        }
    }

    func attestation(scope: String, purpose: KeyPurposeMessage, challenge: FlutterStandardTypedData) async throws
        -> FlutterStandardTypedData?
    {
        do {
            return try await queues.attestation(scope: scope, purpose: purpose.core, challenge: challenge.data)
                .map { FlutterStandardTypedData(bytes: $0) }
        } catch {
            throw Self.bridged(error)
        }
    }

    func deleteKeys(scope: String) async throws {
        do {
            try await queues.deleteKeys(scope: scope)
        } catch {
            throw Self.bridged(error)
        }
    }

    private static func message(_ info: DeviceKeyInfo) -> DeviceKeyInfoMessage {
        DeviceKeyInfoMessage(
            x: FlutterStandardTypedData(bytes: info.x),
            y: FlutterStandardTypedData(bytes: info.y),
            hardwareBound: info.hardwareBound,
            requiresUserPresence: info.requiresUserPresence)
    }

    /// The `PigeonError` of `error`, with the fixed code and text only.
    private static func bridged(_ error: Error) -> PigeonError {
        let mapped = DeviceKeyError.classify(error)
        return PigeonError(code: mapped.code, message: mapped.message, details: nil)
    }
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
