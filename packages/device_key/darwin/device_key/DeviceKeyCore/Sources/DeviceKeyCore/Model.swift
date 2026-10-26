// SPDX-License-Identifier: Apache-2.0

import CryptoKit
import Foundation
import LocalAuthentication

/// The purpose of a device key (docs/spec/08-identity.md 6.1, D-L10, R-CL-05).
///
/// Every server a device is registered with has one key of each purpose.
public enum KeyPurpose: String, CaseIterable, Sendable {
    /// Signs every DPoP proof; never asks the user, so the extensions can use it too.
    case dpop
    /// Signs step-up confirmations; every signature asks for biometrics or the device password.
    case stepUp = "step_up"
}

/// The public part of a device key and how it is stored.
public struct DeviceKeyInfo: Equatable, Sendable {
    /// The x coordinate of the P-256 public key, 32 bytes big-endian.
    public let x: Data
    /// The y coordinate of the P-256 public key, 32 bytes big-endian.
    public let y: Data
    /// Whether the private key lives in the Secure Enclave; false for the keychain fallback.
    public let hardwareBound: Bool
    /// Whether every signature asks the user (the step-up key).
    public let requiresUserPresence: Bool

    /// Creates the description of a key.
    ///
    /// - Parameters:
    ///   - x: the x coordinate, 32 bytes big-endian.
    ///   - y: the y coordinate, 32 bytes big-endian.
    ///   - hardwareBound: whether the key lives in the Secure Enclave.
    ///   - requiresUserPresence: whether every signature asks the user.
    public init(x: Data, y: Data, hardwareBound: Bool, requiresUserPresence: Bool) {
        self.x = x
        self.y = y
        self.hardwareBound = hardwareBound
        self.requiresUserPresence = requiresUserPresence
    }
}

/// What the device offers for keys; injected so the tests can force either path.
public struct PlatformCapabilities: Sendable {
    /// Whether a Secure Enclave can hold P-256 keys (`SecureEnclave.isAvailable`).
    public let enclaveAvailable: Bool
    /// Whether biometrics are enrolled; then the step-up key is bound to them.
    public let biometryEnrolled: Bool
    /// Whether the platform can confirm the user (biometrics or a passcode) at all; false only
    /// in the iOS simulator, which has no passcode and answers LocalAuthentication with -1020.
    public let userConfirmationAvailable: Bool

    /// Creates a description of the platform.
    ///
    /// - Parameters:
    ///   - enclaveAvailable: whether the Secure Enclave can be used.
    ///   - biometryEnrolled: whether Touch ID or Face ID has enrolled biometrics.
    ///   - userConfirmationAvailable: whether the user can be confirmed at all (false only in
    ///     the iOS simulator).
    public init(enclaveAvailable: Bool, biometryEnrolled: Bool, userConfirmationAvailable: Bool = true) {
        self.enclaveAvailable = enclaveAvailable
        self.biometryEnrolled = biometryEnrolled
        self.userConfirmationAvailable = userConfirmationAvailable
    }

    /// The capabilities of the running device.
    public static var current: PlatformCapabilities {
        let context = LAContext()
        var error: NSError?
        let canEvaluate = context.canEvaluatePolicy(.deviceOwnerAuthenticationWithBiometrics, error: &error)
        let enrolled = isBiometryEnrolled(
            canEvaluate: canEvaluate, biometryType: context.biometryType, errorCode: error?.code)
        #if targetEnvironment(simulator)
        let confirmation = false
        #else
        let confirmation = true
        #endif
        return PlatformCapabilities(
            enclaveAvailable: SecureEnclave.isAvailable, biometryEnrolled: enrolled,
            userConfirmationAvailable: confirmation)
    }

    /// Whether biometrics are enrolled, from the answer of `LAContext.canEvaluatePolicy`.
    ///
    /// The check also fails while enrolled biometrics are only unusable for now (closed lid,
    /// lockout); those still count as enrolled, so a key created in that moment is bound to the
    /// biometrics (08 6.1) and not to the device password for the life of the registration.
    ///
    /// - Parameters:
    ///   - canEvaluate: the result of `canEvaluatePolicy(.deviceOwnerAuthenticationWithBiometrics)`.
    ///   - biometryType: `LAContext.biometryType` after that call; `.none` means no sensor.
    ///   - errorCode: the `LAError` code the call reported, if any.
    /// - Returns: true unless there is no sensor or nothing is enrolled.
    public static func isBiometryEnrolled(canEvaluate: Bool, biometryType: LABiometryType, errorCode: Int?) -> Bool {
        if canEvaluate {
            return true
        }
        if biometryType == .none {
            return false
        }
        return errorCode != LAError.biometryNotEnrolled.rawValue
    }
}

/// An access control flag of a key or keychain item (a subset of `SecAccessControlCreateFlags`).
public enum AccessFlag: String, CaseIterable, Sendable {
    /// The Secure Enclave may use the key for signing.
    case privateKeyUsage
    /// Biometrics as enrolled now; enrolling new ones invalidates the key (08 6.1).
    case biometryCurrentSet
    /// Biometrics or the device password.
    case userPresence
}

/// When a key or item is readable; every value is bound to this device (no backup, no iCloud).
public enum Accessibility: String, Sendable {
    /// `kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly`: the extensions can sign while locked.
    case afterFirstUnlockThisDeviceOnly
    /// `kSecAttrAccessibleWhenPasscodeSetThisDeviceOnly`: gone when the passcode is removed.
    case whenPasscodeSetThisDeviceOnly

    /// Whether the value keeps the item out of backups and the iCloud keychain.
    public var isThisDeviceOnly: Bool { rawValue.hasSuffix("ThisDeviceOnly") }
}

/// The protection of one stored key: on the enclave key itself and on its keychain item.
public struct KeyPolicy: Equatable, Sendable {
    /// When the key and its item are readable.
    public let accessibility: Accessibility
    /// The access control of the Secure Enclave key; empty for a software key.
    public let keyFlags: Set<AccessFlag>
    /// The access control of the keychain item; set only for the software step-up key.
    public let itemFlags: Set<AccessFlag>

    /// Creates a policy.
    ///
    /// - Parameters:
    ///   - accessibility: when the key and its item are readable.
    ///   - keyFlags: the access control of an enclave key.
    ///   - itemFlags: the access control of the keychain item.
    public init(accessibility: Accessibility, keyFlags: Set<AccessFlag>, itemFlags: Set<AccessFlag>) {
        self.accessibility = accessibility
        self.keyFlags = keyFlags
        self.itemFlags = itemFlags
    }

    /// The flags of the key and of the item together.
    public var allFlags: Set<AccessFlag> { keyFlags.union(itemFlags) }

    /// The policy of Design note 3 of brief M4-10 (docs/DECISIONS.md D-M4-10-3).
    ///
    /// The step-up key is bound to the enrolled biometrics where they exist (08 6.1); a Mac without
    /// Touch ID still confirms with the device password (`.userPresence`). Without an enclave the
    /// item carries `.userPresence`, so reading the key for a signature asks the user.
    ///
    /// - Parameters:
    ///   - purpose: the key's purpose.
    ///   - capabilities: what the device offers.
    /// - Returns: the policy for a new key.
    public static func policy(for purpose: KeyPurpose, capabilities: PlatformCapabilities) -> KeyPolicy {
        switch purpose {
        case .dpop:
            return KeyPolicy(
                accessibility: .afterFirstUnlockThisDeviceOnly,
                keyFlags: capabilities.enclaveAvailable ? [.privateKeyUsage] : [],
                itemFlags: [])
        case .stepUp:
            if !capabilities.userConfirmationAvailable {
                // The iOS simulator can confirm no user and has no passcode, so no key that
                // needs either could be made and no login would work there; this lets the
                // device tests of sharing (brief M5-06) use the platform's keys, which the Share
                // Extension signs with. A device always has `userConfirmationAvailable`.
                return KeyPolicy(
                    accessibility: .afterFirstUnlockThisDeviceOnly,
                    keyFlags: capabilities.enclaveAvailable ? [.privateKeyUsage] : [],
                    itemFlags: [])
            }
            if capabilities.enclaveAvailable {
                let confirm: AccessFlag = capabilities.biometryEnrolled ? .biometryCurrentSet : .userPresence
                return KeyPolicy(
                    accessibility: .whenPasscodeSetThisDeviceOnly,
                    keyFlags: [.privateKeyUsage, confirm],
                    itemFlags: [])
            }
            return KeyPolicy(accessibility: .whenPasscodeSetThisDeviceOnly, keyFlags: [], itemFlags: [.userPresence])
        }
    }
}
