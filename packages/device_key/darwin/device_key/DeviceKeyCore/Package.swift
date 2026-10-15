// swift-tools-version:5.9
// SPDX-License-Identifier: Apache-2.0
//
// DeviceKeyCore: the device keys of docs/spec/08-identitaet.md 6.1 for iOS and macOS. The Flutter
// plugin `device_key` (the package one folder up) and the app extensions of M5 depend on this
// package, so the plugin and the extensions sign with one implementation (15 9).

import PackageDescription

let package = Package(
    name: "DeviceKeyCore",
    platforms: [.iOS(.v15), .macOS(.v12)],
    products: [
        .library(name: "DeviceKeyCore", targets: ["DeviceKeyCore"]),
    ],
    targets: [
        .target(name: "DeviceKeyCore"),
        .testTarget(name: "DeviceKeyCoreTests", dependencies: ["DeviceKeyCore"]),
    ]
)
