// SPDX-License-Identifier: Apache-2.0

import Foundation
import HelperCore
import Security

/// The helper's own signature state (D-CU-01-29, brief item 24).
///
/// `verified` when the running code satisfies the Developer ID requirement (an Apple anchor, the
/// Developer ID intermediate and the Developer ID Application leaf); every other build, an ad hoc
/// signature of `bundle.sh` included, is `development`. The team is not pinned here: the
/// designated requirement of the signed build, which names the team, is compared by the signing
/// step (`helper-sign-macos`) and checked by the host service's peer rule (27 9.4).
public enum MacSignature {
    /// The requirement of a Developer ID Application signature.
    public static let developerIDRequirement =
        "anchor apple generic and certificate 1[field.1.2.840.113635.100.6.2.6] exists"
        + " and certificate leaf[field.1.2.840.113635.100.6.1.13] exists"

    /// Checks the running process's code.
    /// - Returns: `.verified` when the signature is valid and satisfies the Developer ID
    ///   requirement, else `.development`.
    public static func current() -> SignatureState {
        var code: SecCode?
        var requirement: SecRequirement?
        guard SecCodeCopySelf([], &code) == errSecSuccess, let code,
            SecRequirementCreateWithString(developerIDRequirement as CFString, [], &requirement)
                == errSecSuccess,
            let requirement
        else { return .development }
        return SecCodeCheckValidity(code, [], requirement) == errSecSuccess ? .verified : .development
    }
}
