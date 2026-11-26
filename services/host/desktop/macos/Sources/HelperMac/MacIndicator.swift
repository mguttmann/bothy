// SPDX-License-Identifier: Apache-2.0

import AppKit
import Foundation
import HelperCore

/// The indicator and the menu bar item (brief items 15, 16, 22; 27 9.1).
///
/// While a session is shown: a borderless, click-through border window on every screen in the
/// fixed color `#FF6A00` (D-CU-02-9), a stop button panel, and the menu text "Agent <name>
/// controls this computer, Esc to stop" with a stop item. Always: the switch "Allow computer
/// control", which only a click in the menu toggles, and items that register a missing permission
/// with the system and open the Screen Recording and Accessibility panes of the system settings
/// (the helper never grants itself anything).
///
/// Every window of this process is excluded from capture by `MacCapturer`'s content filter;
/// `sharingType = .none` is set as well for capture paths outside ScreenCaptureKit.
public final class MacIndicator: NSObject, Indicator, NSMenuDelegate {
    /// The border color sRGB `#FF6A00` (D-CU-02-9), which the native CI jobs look for.
    public static let borderColor = NSColor(
        srgbRed: 0xFF / 255.0, green: 0x6A / 255.0, blue: 0x00 / 255.0, alpha: 1)

    /// URL of the Screen Recording pane of the system settings.
    public static let screenRecordingPane = URL(
        string: "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture")!
    /// URL of the Accessibility pane of the system settings.
    public static let accessibilityPane = URL(
        string: "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility")!

    private let dispatch: (@escaping () -> Void) -> Void
    private let interrupt: ActionInterrupt
    private let onToggle: () -> Void
    private let onMenuOpen: () -> Void
    private let onPermissionLink: (PermissionKind) -> Void
    private var onStop: (() -> Void)?
    private var statusItem: NSStatusItem?
    private var borders: [NSWindow] = []
    private var stopPanel: NSPanel?
    private var status: HelperStatus?

    private let sessionItem = NSMenuItem(title: "", action: nil, keyEquivalent: "")
    private let stopItem = NSMenuItem(title: "Stop the agent", action: nil, keyEquivalent: "")
    private let sessionSeparator = NSMenuItem.separator()
    private let switchItem = NSMenuItem(title: "Allow computer control", action: nil, keyEquivalent: "")
    private let captureItem = NSMenuItem(title: "", action: nil, keyEquivalent: "")
    private let inputItem = NSMenuItem(title: "", action: nil, keyEquivalent: "")
    private let versionItem = NSMenuItem(title: "", action: nil, keyEquivalent: "")

    /// Creates the indicator; call `install()` on the main thread once the app runs.
    /// - Parameters:
    ///   - dispatch: runs a closure asynchronously in the controller's context.
    ///   - interrupt: set at once by the stop button, so a running action stops between steps.
    ///   - onToggle: the person clicked "Allow computer control"; runs in the controller's context.
    ///   - onMenuOpen: the menu opens; runs in the controller's context, to refresh permissions.
    ///   - onPermissionLink: a permission item was clicked; runs in the controller's context
    ///     before the pane opens, to register a missing permission with the system.
    public init(
        dispatch: @escaping (@escaping () -> Void) -> Void, interrupt: ActionInterrupt,
        onToggle: @escaping () -> Void, onMenuOpen: @escaping () -> Void,
        onPermissionLink: @escaping (PermissionKind) -> Void
    ) {
        self.dispatch = dispatch
        self.interrupt = interrupt
        self.onToggle = onToggle
        self.onMenuOpen = onMenuOpen
        self.onPermissionLink = onPermissionLink
        super.init()
    }

    /// Creates the menu bar item. Main thread only.
    public func install() {
        let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
        item.button?.title = "Computer Helper"
        let menu = NSMenu()
        menu.autoenablesItems = false
        menu.delegate = self
        sessionItem.isEnabled = false
        stopItem.target = self
        stopItem.action = #selector(stopClicked)
        switchItem.target = self
        switchItem.action = #selector(switchClicked)
        captureItem.target = self
        captureItem.action = #selector(openScreenRecording)
        inputItem.target = self
        inputItem.action = #selector(openAccessibility)
        versionItem.isEnabled = false
        for entry in [
            sessionItem, stopItem, sessionSeparator, switchItem, .separator(), captureItem,
            inputItem, .separator(), versionItem,
        ] {
            menu.addItem(entry)
        }
        item.menu = menu
        statusItem = item
        refreshMenu()
    }

    public func show(agentName: String, onStop: @escaping () -> Void) {
        MainThread.sync {
            self.onStop = onStop
            self.removeWindows()
            for screen in NSScreen.screens { self.borders.append(Self.borderWindow(for: screen)) }
            self.stopPanel = self.makeStopPanel()
            self.sessionItem.title = "Agent \(agentName) controls this computer, Esc to stop"
            self.refreshMenu()
        }
    }

    public func hide() {
        MainThread.sync {
            self.onStop = nil
            self.removeWindows()
            self.sessionItem.title = ""
            self.refreshMenu()
        }
    }

    public func update(status: HelperStatus) {
        MainThread.sync {
            self.status = status
            self.refreshMenu()
        }
    }

    // MARK: Menu

    public func menuWillOpen(_ menu: NSMenu) {
        let refresh = onMenuOpen
        dispatch { refresh() }
    }

    @objc private func switchClicked() {
        let toggle = onToggle
        dispatch { toggle() }
    }

    @objc private func stopClicked() {
        guard let stop = onStop else { return }
        interrupt.requestStop(.indicatorStop)
        dispatch { stop() }
    }

    @objc private func openScreenRecording() {
        openPane(Self.screenRecordingPane, for: .capture)
    }

    @objc private func openAccessibility() {
        openPane(Self.accessibilityPane, for: .input)
    }

    /// Registers the permission in the controller's context, then opens the pane, so the helper
    /// is already in the pane's list when it shows.
    private func openPane(_ pane: URL, for kind: PermissionKind) {
        let link = onPermissionLink
        dispatch {
            link(kind)
            DispatchQueue.main.async { NSWorkspace.shared.open(pane) }
        }
    }

    private func refreshMenu() {
        let live = onStop != nil
        sessionItem.isHidden = !live
        stopItem.isHidden = !live
        sessionSeparator.isHidden = !live
        statusItem?.button?.title = live ? "Agent active" : "Computer Helper"
        guard let status else { return }
        switchItem.state = status.enabled ? .on : .off
        captureItem.title =
            "Screen Recording: \(status.capture == .granted ? "allowed" : "not allowed") (open settings)"
        inputItem.title =
            "Accessibility: \(status.input == .granted ? "allowed" : "not allowed") (open settings)"
        let build = status.info.signature == .verified ? "signed" : "development build"
        versionItem.title =
            "Version \(status.info.version), \(build), \(status.connected ? "connected" : "not connected")"
    }

    // MARK: Windows

    private func removeWindows() {
        for window in borders { window.orderOut(nil) }
        borders.removeAll()
        stopPanel?.orderOut(nil)
        stopPanel = nil
    }

    private static func borderWindow(for screen: NSScreen) -> NSWindow {
        let window = NSWindow(
            contentRect: screen.frame, styleMask: .borderless, backing: .buffered, defer: false)
        window.isReleasedWhenClosed = false
        window.isOpaque = false
        window.backgroundColor = .clear
        window.hasShadow = false
        window.ignoresMouseEvents = true
        window.level = .screenSaver
        window.sharingType = .none
        window.collectionBehavior = [.canJoinAllSpaces, .stationary, .fullScreenAuxiliary, .ignoresCycle]
        window.contentView = BorderView(frame: NSRect(origin: .zero, size: screen.frame.size))
        window.setFrame(screen.frame, display: true)
        window.orderFrontRegardless()
        return window
    }

    private func makeStopPanel() -> NSPanel? {
        guard let screen = NSScreen.main ?? NSScreen.screens.first else { return nil }
        let size = NSSize(width: 220, height: 36)
        let origin = NSPoint(
            x: screen.frame.midX - size.width / 2, y: screen.visibleFrame.maxY - size.height - 12)
        let panel = NSPanel(
            contentRect: NSRect(origin: origin, size: size),
            styleMask: [.borderless, .nonactivatingPanel], backing: .buffered, defer: false)
        panel.isReleasedWhenClosed = false
        panel.level = .screenSaver
        panel.sharingType = .none
        panel.isOpaque = false
        panel.backgroundColor = .clear
        panel.collectionBehavior = [.canJoinAllSpaces, .stationary, .fullScreenAuxiliary, .ignoresCycle]
        let button = NSButton(title: "Stop the agent (Esc)", target: self, action: #selector(stopClicked))
        button.bezelStyle = .rounded
        button.bezelColor = Self.borderColor
        button.frame = NSRect(origin: .zero, size: size)
        panel.contentView = button
        panel.orderFrontRegardless()
        return panel
    }
}

/// Draws the border of the indicator.
final class BorderView: NSView {
    override func draw(_ dirtyRect: NSRect) {
        MacIndicator.borderColor.setStroke()
        let path = NSBezierPath(rect: bounds.insetBy(dx: 3, dy: 3))
        path.lineWidth = 6
        path.stroke()
    }
}
