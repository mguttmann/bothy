// swift-tools-version:5.9
// The Bothy Computer Helper for macOS (docs/spec/27-computer-use.md section 9.1,
// docs/decisions.d/CU-02.md D-CU-02-1). HelperCore holds the logic behind the platform
// interfaces and uses Foundation only, so it runs against fakes on any Mac; HelperMac holds
// the thin platform layer; the executable only starts the accessory app.
import PackageDescription

let package = Package(
    name: "BothyComputerHelper",
    platforms: [.macOS(.v14)],
    products: [
        .library(name: "HelperCore", targets: ["HelperCore"]),
        .library(name: "HelperMac", targets: ["HelperMac"]),
        .executable(name: "bothy-computer-helper", targets: ["bothy-computer-helper"]),
    ],
    targets: [
        .target(name: "HelperCore"),
        .target(
            name: "HelperMac",
            dependencies: ["HelperCore"],
            linkerSettings: [
                .linkedFramework("AppKit"),
                .linkedFramework("ApplicationServices"),
                .linkedFramework("Carbon"),
                .linkedFramework("CoreGraphics"),
                .linkedFramework("ImageIO"),
                .linkedFramework("IOKit"),
                .linkedFramework("ScreenCaptureKit"),
                .linkedFramework("Security"),
                .linkedFramework("UniformTypeIdentifiers"),
            ]
        ),
        .executableTarget(name: "bothy-computer-helper", dependencies: ["HelperMac", "HelperCore"]),
        .testTarget(name: "HelperCoreTests", dependencies: ["HelperCore"]),
    ]
)
