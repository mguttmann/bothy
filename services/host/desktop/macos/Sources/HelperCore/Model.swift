import Foundation

/// A point in the platform's screen space: macOS points of the global display space, origin at
/// the top left of the main display, y growing downwards (27 6.5, 9.4).
public struct Point: Equatable, Sendable {
    /// Horizontal coordinate in points.
    public var x: Double
    /// Vertical coordinate in points.
    public var y: Double

    /// Creates a point.
    public init(x: Double, y: Double) {
        self.x = x
        self.y = y
    }
}

/// A rectangle in the platform's screen space (points of the global display space).
public struct Rect: Equatable, Sendable {
    /// Left edge.
    public var x: Double
    /// Top edge.
    public var y: Double
    /// Width in points.
    public var width: Double
    /// Height in points.
    public var height: Double

    /// Creates a rectangle.
    public init(x: Double, y: Double, width: Double, height: Double) {
        self.x = x
        self.y = y
        self.width = width
        self.height = height
    }
}

/// The geometry of the captured screen, sent as the `screen` member of every `done` (27 9.4).
public struct ScreenGeometry: Equatable, Sendable {
    /// Left edge in the global display space.
    public var x: Double
    /// Top edge in the global display space.
    public var y: Double
    /// Width in points.
    public var width: Double
    /// Height in points.
    public var height: Double
    /// Pixels per point (2 on a Retina display).
    public var scale: Double

    /// Creates a screen geometry.
    public init(x: Double, y: Double, width: Double, height: Double, scale: Double) {
        self.x = x
        self.y = y
        self.width = width
        self.height = height
        self.scale = scale
    }

    /// Whether `rect` lies inside the screen; a tolerance of a hundredth of a point absorbs the
    /// rounding of coordinates to two decimals.
    public func contains(_ rect: Rect) -> Bool {
        let slack = 0.01
        return rect.x >= x - slack && rect.y >= y - slack
            && rect.x + rect.width <= x + width + slack
            && rect.y + rect.height <= y + height + slack
    }

    /// The `screen` member of a `done`.
    public var json: JSONValue {
        [
            "x": .coordinate(x), "y": .coordinate(y), "width": .coordinate(width),
            "height": .coordinate(height), "scale": .coordinate(scale),
        ]
    }

    /// A geometry of zero size, used in a `done` when no screen could be read.
    public static let unknown = ScreenGeometry(x: 0, y: 0, width: 0, height: 0, scale: 1)
}

/// The identity of the frontmost app, the `app` member of every `done` (brief item 21).
public struct AppIdentity: Equatable, Sendable {
    /// The bundle ID (macOS).
    public var id: String
    /// The display name.
    public var name: String
    /// The team ID of the code signature, empty for an app without one.
    public var signer: String

    /// Creates an identity.
    public init(id: String, name: String, signer: String) {
        self.id = id
        self.name = name
        self.signer = signer
    }

    /// The identity used when nothing is known (all members empty, as in the vectors).
    public static let unknown = AppIdentity(id: "", name: "", signer: "")

    /// The `app` member of a `done`.
    public var json: JSONValue {
        ["id": .string(id), "name": .string(name), "signer": .string(signer)]
    }
}

/// Image formats of `capture` (27 6.5).
public enum ImageFormat: String, Sendable {
    /// Screenshots, with the quality of `computer.jpeg_quality`.
    case jpeg
    /// `zoom`, lossless.
    case png
}

/// The `code` of a `done` (27 9.4).
public enum ResultCode: String, Sendable {
    /// The request ran.
    case ok
    /// The screen is locked (lock, screen saver, user switch, display sleep); nothing ran.
    case screenLocked = "screen_locked"
    /// The secure desktop is shown (Windows only).
    case secureDesktop = "secure_desktop"
    /// The operating system refused capture or input, or a permission is missing.
    case osRefused = "os_refused"
    /// The request is not valid here: outside a shown session, unknown action, key or argument.
    case invalidAction = "invalid_action"
}

/// The `detail` of a `stop` (27 9.4, 8.3).
public enum StopDetail: String, Sendable {
    /// Esc at the real keyboard.
    case localEscape = "local_escape"
    /// The stop button of the indicator or the stop item of the menu bar.
    case indicatorStop = "indicator_stop"
}

/// The state of an operating system permission as reported in `hello` and `state`.
///
/// The preflight calls cannot tell "never asked" from "denied", so both are `denied`.
public enum PermissionState: String, Sendable {
    /// The permission is granted.
    case granted
    /// The permission is not granted.
    case denied
}

/// Whether the helper's own code signature verifies against the Developer ID requirement.
public enum SignatureState: String, Sendable {
    /// A Developer ID signature that verifies.
    case verified
    /// Any other build (D-CU-01-29); it may use the development channel override (D-CU-02-5).
    case development
}

/// Mouse buttons of the click actions.
public enum MouseButton: Hashable, Sendable {
    case left, right, middle
}

/// Directions of `scroll` (27 6.4).
public enum ScrollDirection: String, Sendable {
    case up, down, left, right
}

/// Facts about the running helper that do not change while it runs.
public struct HelperInfo: Equatable, Sendable {
    /// `helper_version`, from the bundle's `CFBundleShortVersionString`.
    public var version: String
    /// `signature`.
    public var signature: SignatureState
    /// `os_account`, the short name of the user the helper runs as.
    public var osAccount: String

    /// Creates the facts.
    public init(version: String, signature: SignatureState, osAccount: String) {
        self.version = version
        self.signature = signature
        self.osAccount = osAccount
    }
}

/// The persisted enablement (D-CU-02-4): the switch and its generation.
public struct EnablementState: Equatable, Sendable {
    /// Whether computer control is allowed.
    public var enabled: Bool
    /// Raised by one on every switch-on by a click (27 section 4 item 1).
    public var generation: Int

    /// Creates a state.
    public init(enabled: Bool, generation: Int) {
        self.enabled = enabled
        self.generation = generation
    }

    /// The state after installation and for a missing or unreadable file.
    public static let off = EnablementState(enabled: false, generation: 0)
}

/// What the menu bar shows; handed to the `Indicator` on every change.
public struct HelperStatus: Equatable, Sendable {
    /// The switch "Allow computer control".
    public var enabled: Bool
    /// The enablement generation.
    public var generation: Int
    /// Screen recording.
    public var capture: PermissionState
    /// Accessibility.
    public var input: PermissionState
    /// Whether the screen counts as locked.
    public var locked: Bool
    /// Whether the channel to the host service is open.
    public var connected: Bool
    /// The agent of the shown session, nil without one.
    public var agentName: String?
    /// Version and signature for the menu's information line.
    public var info: HelperInfo
}

/// A size in pixels.
public struct PixelSize: Equatable, Sendable {
    /// Width in pixels.
    public var width: Int
    /// Height in pixels.
    public var height: Int

    /// Creates a size.
    public init(width: Int, height: Int) {
        self.width = width
        self.height = height
    }
}

/// The scaling rule of 27 6.5 item 2.
public enum Scaling {
    /// The pixel size of an image of an area captured in native resolution and scaled, keeping
    /// the aspect ratio, so that the long edge is at most `maxEdge`; never scaled up.
    ///
    /// - Parameters:
    ///   - width: width of the area in points.
    ///   - height: height of the area in points.
    ///   - scale: pixels per point of the screen.
    ///   - maxEdge: the `max_edge` of the request (`computer.image_max_edge`).
    /// - Returns: the output size, at least one pixel per side.
    ///
    /// Example: a 1512 by 982 point Retina screen with a long edge of 1280 gives 1280 by 831.
    public static func pixelSize(width: Double, height: Double, scale: Double, maxEdge: Int)
        -> PixelSize
    {
        let nativeWidth = (width * scale).rounded()
        let nativeHeight = (height * scale).rounded()
        let longEdge = max(nativeWidth, nativeHeight)
        let factor = longEdge > Double(maxEdge) ? Double(maxEdge) / longEdge : 1
        return PixelSize(
            width: max(1, Int((nativeWidth * factor).rounded())),
            height: max(1, Int((nativeHeight * factor).rounded())))
    }
}
