// swift-tools-version: 5.9
// SPDX-License-Identifier: Apache-2.0
//
// The Swift package of the Flutter plugin `device_key` for iOS and macOS (Swift Package Manager,
// the default of the pinned Flutter). Flutter links this folder into the app's build and provides
// `../FlutterFramework`. The device key code itself is the package `DeviceKeyCore` in the subfolder
// of the same name, which the app extensions of M5 use directly; it sits inside this folder because
// Flutter links the plugin package by a symbolic link, so a path that leaves the folder would break.

import PackageDescription

let package = Package(
    name: "device_key",
    platforms: [
        .iOS("15.0"),
        .macOS("12.0"),
    ],
    products: [
        .library(name: "device-key", targets: ["device_key"]),
    ],
    dependencies: [
        .package(name: "FlutterFramework", path: "../FlutterFramework"),
        .package(name: "DeviceKeyCore", path: "DeviceKeyCore"),
    ],
    targets: [
        .target(
            name: "device_key",
            dependencies: [
                .product(name: "FlutterFramework", package: "FlutterFramework"),
                .product(name: "DeviceKeyCore", package: "DeviceKeyCore"),
            ]
        ),
    ]
)
