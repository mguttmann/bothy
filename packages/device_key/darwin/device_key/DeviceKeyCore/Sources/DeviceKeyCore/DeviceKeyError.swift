// SPDX-License-Identifier: Apache-2.0

import CryptoTokenKit
import Foundation
import LocalAuthentication
import Security

/// The errors of the device key, with the stable codes the Dart side maps (brief M4-10, note 1).
///
/// Messages are fixed texts: they never contain key material, a scope or a platform message.
public enum DeviceKeyError: Error, Equatable, CaseIterable, CustomStringConvertible, CustomDebugStringConvertible {
    /// The user cancelled the biometrics or password prompt.
    case userCancelled
    /// The key can no longer be used (for example new biometrics); the app registers anew (08 5.4).
    case keyInvalidated
    /// No key exists, or the device cannot hold or use one right now.
    case notAvailable
    /// Any other failure.
    case `internal`

    /// The stable code sent over the plugin channel.
    public var code: String {
        switch self {
        case .userCancelled: return "user_cancelled"
        case .keyInvalidated: return "key_invalidated"
        case .notAvailable: return "not_available"
        case .internal: return "internal"
        }
    }

    /// A fixed English text for logs; without key material.
    public var message: String {
        switch self {
        case .userCancelled: return "the user cancelled the confirmation"
        case .keyInvalidated: return "the device key was invalidated"
        case .notAvailable: return "the device key is not available"
        case .internal: return "the device key operation failed"
        }
    }

    /// The code and the fixed text.
    public var description: String { "\(code): \(message)" }

    /// The same as `description`, so `String(reflecting:)` shows nothing more.
    public var debugDescription: String { description }

    /// Maps a platform error to a stable code without keeping the platform's message.
    ///
    /// - Parameter error: an error of Security, LocalAuthentication, CryptoKit or this package.
    /// - Returns: the matching case; `internal` for anything unknown.
    public static func classify(_ error: Error) -> DeviceKeyError {
        if let own = error as? DeviceKeyError {
            return own
        }
        let ns = error as NSError
        if ns.domain == LAError.errorDomain {
            switch LAError.Code(rawValue: ns.code) {
            case .userCancel, .systemCancel, .appCancel, .userFallback:
                return .userCancelled
            case .biometryNotEnrolled:
                return .keyInvalidated
            case .passcodeNotSet, .biometryNotAvailable, .biometryLockout, .notInteractive:
                return .notAvailable
            default:
                return .internal
            }
        }
        if ns.domain == TKError.errorDomain {
            // Secure Enclave operations fail in CryptoTokenKit's domain. The enclave loads any blob
            // and rejects a key it no longer accepts only when it signs.
            switch TKError.Code(rawValue: ns.code) {
            case .corruptedData, .objectNotFound:
                return .keyInvalidated
            case .canceledByUser:
                return .userCancelled
            default:
                return .internal
            }
        }
        if ns.domain == NSOSStatusErrorDomain {
            switch OSStatus(ns.code) {
            case errSecUserCanceled:
                return .userCancelled
            case errSecMissingEntitlement, errSecInteractionNotAllowed, errSecNotAvailable:
                return .notAvailable
            default:
                return .internal
            }
        }
        return .internal
    }
}
