// swift-tools-version:5.9
// SPDX-License-Identifier: Apache-2.0
//
// ClientKit: what the native extensions of the app share (docs/adr/0056, brief M5-04 Design
// note 1): the shared keychain items of the push registration, the small App Group state, DPoP
// proofs signed with the `dpop` device key of DeviceKeyCore, the preview fetch and the
// Notification Service Extension's logic. Foundation, Security, CryptoKit and UserNotifications
// only, never Flutter: the iOS and macOS runners and their Notification Service Extensions link
// it, and the Share Extension of M5-06 extends it.

import PackageDescription

let package = Package(
    name: "ClientKit",
    platforms: [.iOS(.v15), .macOS(.v12)],
    products: [
        .library(name: "ClientKit", targets: ["ClientKit"]),
    ],
    dependencies: [
        .package(name: "DeviceKeyCore", path: "../../../../packages/device_key/darwin/device_key/DeviceKeyCore"),
    ],
    targets: [
        .target(
            name: "ClientKit",
            dependencies: [.product(name: "DeviceKeyCore", package: "DeviceKeyCore")]
        ),
        .testTarget(
            name: "ClientKitTests",
            dependencies: ["ClientKit"],
            exclude: ["Fixtures"]
        ),
    ]
)
