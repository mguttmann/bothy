// SPDX-License-Identifier: Apache-2.0

import AppKit
import Foundation
import HelperCore
import Security

/// The frontmost app with bundle ID and team ID (brief item 21, 27 9.1).
///
/// The bundle ID and name come from `NSWorkspace`; the team ID comes from the code signature of
/// the running process (`SecCodeCopyGuestWithAttributes` by process ID), so it names who signed
/// the code that runs, not what its bundle claims. Apps of the system have no team ID; their
/// signer stays empty.
public final class MacAppInspector: AppInspector {
    /// Creates the inspector.
    public init() {}

    public func frontmostApp() -> AppIdentity {
        guard let app = MainThread.sync({ NSWorkspace.shared.frontmostApplication }) else {
            return .unknown
        }
        return AppIdentity(
            id: app.bundleIdentifier ?? "", name: app.localizedName ?? "",
            signer: Self.teamID(of: app.processIdentifier) ?? "")
    }

    /// The team ID of a running process's code signature.
    /// - Parameter pid: the process.
    /// - Returns: the team ID, or nil for unsigned or system code.
    static func teamID(of pid: pid_t) -> String? {
        var code: SecCode?
        let attributes = [kSecGuestAttributePid: pid] as CFDictionary
        guard SecCodeCopyGuestWithAttributes(nil, attributes, [], &code) == errSecSuccess,
            let code
        else { return nil }
        var staticCode: SecStaticCode?
        guard SecCodeCopyStaticCode(code, [], &staticCode) == errSecSuccess, let staticCode else {
            return nil
        }
        var information: CFDictionary?
        guard
            SecCodeCopySigningInformation(
                staticCode, SecCSFlags(rawValue: kSecCSSigningInformation), &information)
                == errSecSuccess,
            let dictionary = information as? [String: Any]
        else { return nil }
        return dictionary[kSecCodeInfoTeamIdentifier as String] as? String
    }
}
