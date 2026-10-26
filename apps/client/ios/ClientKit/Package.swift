// swift-tools-version:5.9
// SPDX-License-Identifier: Apache-2.0
//
// ClientKit: what the native extensions of the app share (docs/adr/0056, brief M5-04 Design
// note 1): the shared keychain items of the push registration, the small App Group state, DPoP
// proofs signed with the `dpop` device key of DeviceKeyCore, the preview fetch and the
// Notification Service Extension's logic. Foundation, Security, CryptoKit and UserNotifications
// only, never Flutter: the iOS and macOS runners and their Notification Service Extensions link
// it, and the Share Extension of M5-06 extends it.
//
// ClientKitShare: the Share Extension's sheet and item loading (brief M5-06 Design notes 1 and
// 2): SwiftUI, ImageIO, UniformTypeIdentifiers and UIKit on iOS or AppKit on macOS, over
// ClientKit. Only the Share Extensions link it, so the runners and the Notification Service
// Extensions stay free of SwiftUI.

import PackageDescription

let package = Package(
    name: "ClientKit",
    platforms: [.iOS(.v15), .macOS(.v12)],
    products: [
        .library(name: "ClientKit", targets: ["ClientKit"]),
        .library(name: "ClientKitShare", targets: ["ClientKitShare"]),
    ],
    dependencies: [
        .package(name: "DeviceKeyCore", path: "../../../../packages/device_key/darwin/device_key/DeviceKeyCore"),
    ],
    targets: [
        .target(
            name: "ClientKit",
            dependencies: [.product(name: "DeviceKeyCore", package: "DeviceKeyCore")]
        ),
        .target(
            name: "ClientKitShare",
            dependencies: ["ClientKit"]
        ),
        .testTarget(
            name: "ClientKitTests",
            dependencies: ["ClientKit"],
            exclude: ["Fixtures"]
        ),
        // Loads one shared file through ClientKitShare in a process of its own and prints its
        // memory figures, so the memory tests measure nothing but that load.
        .executableTarget(
            name: "ShareMemoryProbe",
            dependencies: ["ClientKitShare"],
            path: "Tests/ShareMemoryProbe"
        ),
        .testTarget(
            name: "ClientKitShareTests",
            dependencies: ["ClientKitShare", "ClientKit", "ShareMemoryProbe"]
        ),
    ]
)
