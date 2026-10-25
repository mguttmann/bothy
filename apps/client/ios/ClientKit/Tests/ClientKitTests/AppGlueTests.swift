// SPDX-License-Identifier: Apache-2.0
//
// The pieces of the runner's push glue that live in ClientKit (docs/decisions.d/M5-04.md
// D-M5-04-9): the delegate of the notification center keeps the one it replaces, and the APNs
// environment of a registration follows the signing profile.

import Foundation
import Testing
import UserNotifications

@testable import ClientKit

/// A notification center stand-in; the real one needs an app bundle.
private final class Holder: NotificationDelegateHolder {
    var delegate: UNUserNotificationCenterDelegate?
}

/// A delegate like the local notifications plugin's.
private final class OtherDelegate: NSObject, UNUserNotificationCenterDelegate {}

@Test func testRouterKeepsTheDelegateItReplaces() {
    let center = Holder()
    let plugin = OtherDelegate()
    center.delegate = plugin
    let router = NotificationCenterRouter { _ in }
    router.install(on: center)
    #expect(center.delegate === router)
    #expect(router.previous === plugin)
    // A second install while the router is the delegate changes nothing.
    router.install(on: center, fallback: OtherDelegate())
    #expect(router.previous === plugin)
    // A component that took the center over later is kept as well.
    let later = OtherDelegate()
    center.delegate = later
    router.install(on: center)
    #expect(center.delegate === router && router.previous === later)
}

@Test func testRouterFallsBackToTheAppDelegate() {
    let center = Holder()
    let app = OtherDelegate()
    let router = NotificationCenterRouter { _ in }
    router.install(on: center, fallback: app)
    #expect(router.previous === app)
}

@Test func testRouterClaimsOnlyPushes() {
    #expect(NotificationCenterRouter.push(pushContent().userInfo) != nil)
    // A local notification of flutter_local_notifications carries its own keys, no `b`.
    #expect(NotificationCenterRouter.push(["payload": "/t/x?m=y", "presentAlert": true]) == nil)
    #expect(NotificationCenterRouter.push([:]) == nil)
}

private func profile(_ entitlements: [String: Any]?) -> Data {
    var plist: [String: Any] = ["Name": "Example profile"]
    if let entitlements { plist["Entitlements"] = entitlements }
    let xml = try! PropertyListSerialization.data(fromPropertyList: plist, format: .xml, options: 0)
    // The plist sits inside a CMS envelope; binary bytes around it, as in a real profile.
    return Data([0x30, 0x82, 0x01, 0x00, 0xff]) + xml + Data([0x00, 0xa0, 0x82])
}

@Test func testApsEnvironmentFollowsTheProfile() {
    #expect(ApsEnvironment.registration(profile: profile(["aps-environment": "development"])) == "sandbox")
    #expect(ApsEnvironment.registration(profile: profile(["aps-environment": "production"])) == "production")
    #expect(ApsEnvironment.registration(profile: profile(["com.apple.developer.aps-environment": "development"])) == "sandbox")
    // No profile: an App Store or TestFlight build, signed for production.
    #expect(ApsEnvironment.registration(profile: nil) == "production")
    #expect(ApsEnvironment.registration(profile: profile(nil)) == "production")
    #expect(ApsEnvironment.registration(profile: Data("garbage".utf8)) == "production")
}
