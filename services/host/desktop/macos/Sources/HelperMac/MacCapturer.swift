import AppKit
import CoreGraphics
import Foundation
import HelperCore
import ImageIO
import ScreenCaptureKit
import UniformTypeIdentifiers

/// Screen capture with ScreenCaptureKit (`SCScreenshotManager`, macOS 14; 27 9.1).
///
/// The content filter excludes every window of the helper's own process, which holds the
/// indicator border, its stop button and the menu, so the indicator is never in a screenshot
/// (brief item 16). Images are encoded with ImageIO into memory only; nothing is written to disk.
public final class MacCapturer: Capturer {
    /// Creates the capturer.
    public init() {}

    public func permissionGranted() -> Bool {
        // Preflight only: the helper never asks for or grants itself a permission (brief item 22).
        CGPreflightScreenCaptureAccess()
    }

    public func activeScreen() throws -> ScreenGeometry {
        let display = Self.displayOfFrontmostWindow() ?? CGMainDisplayID()
        return try Self.geometry(of: display)
    }

    public func capture(
        screen: ScreenGeometry, region: Rect?, pixelWidth: Int, pixelHeight: Int,
        format: ImageFormat, quality: Int
    ) throws -> Data {
        let displayID = try Self.display(at: screen)
        let image = try Self.wait { () async throws -> CGImage in
            let content = try await SCShareableContent.excludingDesktopWindows(
                false, onScreenWindowsOnly: true)
            guard let display = content.displays.first(where: { $0.displayID == displayID }) else {
                throw CaptureError.displayGone
            }
            let own = content.applications.filter { $0.processID == getpid() }
            let filter = SCContentFilter(
                display: display, excludingApplications: own, exceptingWindows: [])
            let configuration = SCStreamConfiguration()
            configuration.width = pixelWidth
            configuration.height = pixelHeight
            configuration.showsCursor = false
            configuration.scalesToFit = true
            if let region {
                // sourceRect is in points relative to the display's top left corner.
                configuration.sourceRect = CGRect(
                    x: region.x - screen.x, y: region.y - screen.y, width: region.width,
                    height: region.height)
            }
            return try await SCScreenshotManager.captureImage(
                contentFilter: filter, configuration: configuration)
        }
        return try Self.encode(image, format: format, quality: quality)
    }

    // MARK: Screens

    /// The display under the center of the frontmost app's frontmost normal window.
    private static func displayOfFrontmostWindow() -> CGDirectDisplayID? {
        guard let pid = MainThread.sync({ NSWorkspace.shared.frontmostApplication?.processIdentifier }),
            let windows = CGWindowListCopyWindowInfo(
                [.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID)
                as? [[String: Any]]
        else { return nil }
        // The list is ordered front to back; layer 0 holds normal windows.
        for window in windows {
            guard (window[kCGWindowOwnerPID as String] as? Int32) == pid,
                (window[kCGWindowLayer as String] as? Int) == 0,
                let bounds = window[kCGWindowBounds as String] as? NSDictionary,
                let rect = CGRect(dictionaryRepresentation: bounds)
            else { continue }
            var display: CGDirectDisplayID = 0
            var count: UInt32 = 0
            if CGGetDisplaysWithPoint(CGPoint(x: rect.midX, y: rect.midY), 1, &display, &count)
                == .success, count == 1
            {
                return display
            }
        }
        return nil
    }

    private static func geometry(of display: CGDirectDisplayID) throws -> ScreenGeometry {
        let bounds = CGDisplayBounds(display)
        guard bounds.width > 0, let mode = CGDisplayCopyDisplayMode(display) else {
            throw CaptureError.displayGone
        }
        let scale = Double(mode.pixelWidth) / Double(bounds.width)
        return ScreenGeometry(
            x: bounds.origin.x, y: bounds.origin.y, width: bounds.width, height: bounds.height,
            scale: scale > 0 ? scale : 1)
    }

    /// The display whose bounds match a geometry read before; it may have gone since.
    private static func display(at screen: ScreenGeometry) throws -> CGDirectDisplayID {
        var display: CGDirectDisplayID = 0
        var count: UInt32 = 0
        let center = CGPoint(x: screen.x + screen.width / 2, y: screen.y + screen.height / 2)
        guard CGGetDisplaysWithPoint(center, 1, &display, &count) == .success, count == 1 else {
            throw CaptureError.displayGone
        }
        return display
    }

    // MARK: Encoding

    private static func encode(_ image: CGImage, format: ImageFormat, quality: Int) throws -> Data {
        let data = NSMutableData()
        let type = format == .png ? UTType.png : UTType.jpeg
        guard
            let destination = CGImageDestinationCreateWithData(
                data as CFMutableData, type.identifier as CFString, 1, nil)
        else { throw CaptureError.encodingFailed }
        var options: [CFString: Any] = [:]
        if format == .jpeg {
            options[kCGImageDestinationLossyCompressionQuality] = Double(quality) / 100
        }
        CGImageDestinationAddImage(destination, image, options as CFDictionary)
        guard CGImageDestinationFinalize(destination) else { throw CaptureError.encodingFailed }
        return data as Data
    }

    /// Runs an async ScreenCaptureKit call from the controller's synchronous context.
    ///
    /// The controller's queue is not the main thread and ScreenCaptureKit completes on its own
    /// queues, so waiting here does not block what the call needs.
    private static func wait<T>(_ work: @escaping () async throws -> T) throws -> T {
        let semaphore = DispatchSemaphore(value: 0)
        var result: Result<T, Error> = .failure(CaptureError.displayGone)
        Task.detached {
            do {
                result = .success(try await work())
            } catch {
                result = .failure(error)
            }
            semaphore.signal()
        }
        semaphore.wait()
        return try result.get()
    }
}

/// Errors of the capturer; the controller answers them with `os_refused`.
public enum CaptureError: Error {
    /// The display was removed or reconfigured.
    case displayGone
    /// ImageIO could not encode the image.
    case encodingFailed
}
