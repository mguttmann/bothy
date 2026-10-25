// SPDX-License-Identifier: Apache-2.0

import Foundation

/// The APNs environment of a registration, from the provisioning profile the app was signed
/// with (docs/spec/13-push-notifications.md 5.1, 13; docs/decisions.d/M5-04.md D-M5-04-10).
///
/// A token is valid only in the environment of the `aps-environment` entitlement the app was
/// signed with, and that value comes from the profile, not from the build configuration: a
/// development profile grants `development`, the export for TestFlight, the App Store or
/// Developer ID re-signs with `production`. So the app reads its embedded profile
/// (`embedded.mobileprovision` on iOS, `Contents/embedded.provisionprofile` on macOS); a build
/// without one was installed from the App Store and is `production`.
public enum ApsEnvironment {
    /// The `environment` of `registerPush` for a profile.
    ///
    /// - Parameter profile: the bytes of the embedded provisioning profile, or nil.
    /// - Returns: `sandbox` for `development`, `production` otherwise.
    public static func registration(profile: Data?) -> String {
        guard let profile, let entitlements = entitlements(of: profile) else { return "production" }
        let value = entitlements["aps-environment"] ?? entitlements["com.apple.developer.aps-environment"]
        return (value as? String) == "development" ? "sandbox" : "production"
    }

    /// The environment of the running app from its embedded profile.
    ///
    /// - Parameter bundle: the app bundle.
    /// - Returns: `sandbox` or `production`.
    public static func ofBundle(_ bundle: Bundle = .main) -> String {
        let candidates = [
            bundle.bundleURL.appendingPathComponent("embedded.mobileprovision"),
            bundle.bundleURL.appendingPathComponent("Contents/embedded.provisionprofile"),
        ]
        let data = candidates.lazy.compactMap { try? Data(contentsOf: $0) }.first
        return registration(profile: data)
    }

    /// The `Entitlements` dictionary of the property list inside a profile's CMS envelope.
    private static func entitlements(of profile: Data) -> [String: Any]? {
        guard let start = profile.range(of: Data("<?xml".utf8)),
              let end = profile.range(of: Data("</plist>".utf8), in: start.lowerBound..<profile.endIndex)
        else {
            return nil
        }
        let xml = profile.subdata(in: start.lowerBound..<end.upperBound)
        let plist = try? PropertyListSerialization.propertyList(from: xml, options: [], format: nil)
        return (plist as? [String: Any])?["Entitlements"] as? [String: Any]
    }
}
