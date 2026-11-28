// SPDX-License-Identifier: Apache-2.0

import AppKit
import Foundation
import HelperCore
import QuartzCore

/// The indicator, the session's menu bar item and the settings window (brief items 15, 16, 22;
/// 27 9.1; decisions.d/FIX-CU-HELPER-NO-ICON.md).
///
/// While a session is shown: a borderless, click-through border window on every screen in the
/// fixed color `#FF6A00` (D-CU-02-9), a stop button panel, the pointer mark (a ring in the same
/// color that follows the pointer and a short pulse at every click the helper sends) and a menu
/// bar item with the text "Agent <name> controls this computer, Esc to stop" and a stop item.
/// Outside a session the helper shows nothing: no menu bar item, no window.
///
/// The settings window holds the switch "Allow computer control", which only a click toggles,
/// buttons that register a missing permission with the system and open the Screen Recording and
/// Accessibility panes of the system settings (the helper never grants itself anything), and the
/// version. It opens when the person opens the helper app again (Finder or Spotlight; the system
/// sends the running helper a reopen event), and by itself at start while the switch was never
/// switched on (`EnablementState.needsSetup`).
///
/// Every window of this process is excluded from capture by `MacCapturer`'s content filter;
/// `sharingType = .none` is set as well for capture paths outside ScreenCaptureKit.
public final class MacIndicator: NSObject, Indicator {
    /// The border color sRGB `#FF6A00` (D-CU-02-9), which the native CI jobs look for.
    public static let borderColor = NSColor(
        srgbRed: 0xFF / 255.0, green: 0x6A / 255.0, blue: 0x00 / 255.0, alpha: 1)

    /// URL of the Screen Recording pane of the system settings.
    public static let screenRecordingPane = URL(
        string: "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture")!
    /// URL of the Accessibility pane of the system settings.
    public static let accessibilityPane = URL(
        string: "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility")!

    /// The title of the settings window and of the session's menu bar item outside its text.
    public static let settingsTitle = "Computer Helper"

    /// Edge of the pointer ring's window and of the click pulse's window, in points.
    static let ringSize: CGFloat = 40
    static let pulseSize: CGFloat = 72
    /// How long a click pulse shows, in seconds.
    static let pulseDuration: CFTimeInterval = 0.4

    private let dispatch: (@escaping () -> Void) -> Void
    private let interrupt: ActionInterrupt
    private let onToggle: (Bool) -> Void
    private let onSettingsOpen: () -> Void
    private let onPermissionLink: (PermissionKind) -> Void
    private var onStop: (() -> Void)?
    private var statusItem: NSStatusItem?
    private var borders: [NSWindow] = []
    private var stopPanel: NSPanel?
    private var ring: NSPanel?
    private var ringTimer: Timer?
    private var pulses: [NSPanel] = []
    private var status: HelperStatus?

    private var settings: NSWindow?
    private let switchBox = NSButton(
        checkboxWithTitle: "Allow computer control", target: nil, action: nil)
    private let captureButton = NSButton(title: "", target: nil, action: nil)
    private let inputButton = NSButton(title: "", target: nil, action: nil)
    private let versionLabel = NSTextField(labelWithString: "")

    /// Creates the indicator. Nothing shows until a session or the settings window opens.
    /// - Parameters:
    ///   - dispatch: runs a closure asynchronously in the controller's context.
    ///   - interrupt: set at once by the stop button, so a running action stops between steps.
    ///   - onToggle: "Allow computer control" was clicked, with true when the click came from input
    ///     the helper injected (its marker); runs in the controller's context.
    ///   - onSettingsOpen: the settings window opens; runs in the controller's context, to
    ///     refresh the permissions.
    ///   - onPermissionLink: a permission button was clicked; runs in the controller's context
    ///     before the pane opens, to register a missing permission with the system.
    public init(
        dispatch: @escaping (@escaping () -> Void) -> Void, interrupt: ActionInterrupt,
        onToggle: @escaping (Bool) -> Void, onSettingsOpen: @escaping () -> Void,
        onPermissionLink: @escaping (PermissionKind) -> Void
    ) {
        self.dispatch = dispatch
        self.interrupt = interrupt
        self.onToggle = onToggle
        self.onSettingsOpen = onSettingsOpen
        self.onPermissionLink = onPermissionLink
        super.init()
    }

    public func show(agentName: String, onStop: @escaping () -> Void) {
        MainThread.sync {
            self.onStop = onStop
            self.removeSessionViews()
            for screen in NSScreen.screens { self.borders.append(Self.borderWindow(for: screen)) }
            self.stopPanel = self.makeStopPanel()
            self.statusItem = self.makeStatusItem(agentName: agentName)
            self.ring = Self.ringPanel()
            self.followPointer()
            // In the common modes, so the ring keeps following while a menu is open.
            let timer = Timer(timeInterval: 1.0 / 60, repeats: true) { [weak self] _ in
                self?.followPointer()
            }
            RunLoop.main.add(timer, forMode: .common)
            self.ringTimer = timer
        }
    }

    public func hide() {
        MainThread.sync {
            self.onStop = nil
            self.removeSessionViews()
        }
    }

    public func update(status: HelperStatus) {
        MainThread.sync {
            self.status = status
            self.refreshSettings()
        }
    }

    public func markClick(at point: Point) {
        // Asynchronous: the action goes on while the pulse shows.
        DispatchQueue.main.async { [weak self] in
            guard let self, self.onStop != nil else { return }
            let panel = Self.pulse(at: Self.appKitPoint(point))
            self.pulses.append(panel)
            DispatchQueue.main.asyncAfter(deadline: .now() + Self.pulseDuration) { [weak self] in
                panel.orderOut(nil)
                self?.pulses.removeAll { $0 === panel }
            }
        }
    }

    /// Opens the settings window and brings it to the front. Main thread only.
    public func openSettings() {
        let window = settings ?? makeSettingsWindow()
        settings = window
        refreshSettings()
        let refresh = onSettingsOpen
        dispatch { refresh() }
        NSApp.activate()
        window.makeKeyAndOrderFront(nil)
    }

    // MARK: Session menu bar item

    private func makeStatusItem(agentName: String) -> NSStatusItem {
        let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
        item.button?.title = "Agent active"
        let menu = NSMenu()
        menu.autoenablesItems = false
        let text = NSMenuItem(
            title: "Agent \(agentName) controls this computer, Esc to stop", action: nil,
            keyEquivalent: "")
        text.isEnabled = false
        let stop = NSMenuItem(title: "Stop the agent", action: #selector(stopClicked), keyEquivalent: "")
        stop.target = self
        menu.addItem(text)
        menu.addItem(stop)
        item.menu = menu
        return item
    }

    @objc private func stopClicked() {
        guard let stop = onStop else { return }
        interrupt.requestStop(.indicatorStop)
        dispatch { stop() }
    }

    // MARK: Settings window

    private func makeSettingsWindow() -> NSWindow {
        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 420, height: 190),
            styleMask: [.titled, .closable], backing: .buffered, defer: false)
        window.title = Self.settingsTitle
        window.isReleasedWhenClosed = false
        window.sharingType = .none
        switchBox.target = self
        switchBox.action = #selector(switchClicked)
        captureButton.target = self
        captureButton.action = #selector(openScreenRecording)
        inputButton.target = self
        inputButton.action = #selector(openAccessibility)
        let note = NSTextField(
            wrappingLabelWithString:
                "Shows nothing while no agent controls this computer. Open Computer Helper again to come back here.")
        note.textColor = .secondaryLabelColor
        let stack = NSStackView(views: [switchBox, captureButton, inputButton, versionLabel, note])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 10
        stack.edgeInsets = NSEdgeInsets(top: 18, left: 20, bottom: 18, right: 20)
        stack.translatesAutoresizingMaskIntoConstraints = false
        let content = NSView()
        content.addSubview(stack)
        NSLayoutConstraint.activate([
            stack.leadingAnchor.constraint(equalTo: content.leadingAnchor),
            stack.trailingAnchor.constraint(equalTo: content.trailingAnchor),
            stack.topAnchor.constraint(equalTo: content.topAnchor),
            stack.bottomAnchor.constraint(equalTo: content.bottomAnchor),
            note.widthAnchor.constraint(equalToConstant: 380),
        ])
        window.contentView = content
        window.center()
        return window
    }

    @objc private func switchClicked() {
        // The box shows the stored state again once the controller answered with update(status:).
        // The helper's marker in the event that made the click means the agent clicked, which may
        // switch off but never on (R-CU-03).
        let injected =
            NSApp.currentEvent?.cgEvent?.getIntegerValueField(.eventSourceUserData) == MacInjector.marker
        let toggle = onToggle
        dispatch { toggle(injected) }
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

    private func refreshSettings() {
        guard let status else { return }
        switchBox.state = status.enabled ? .on : .off
        captureButton.title =
            "Screen Recording: \(status.capture == .granted ? "allowed" : "not allowed") (open settings)"
        inputButton.title =
            "Accessibility: \(status.input == .granted ? "allowed" : "not allowed") (open settings)"
        let build = status.info.signature == .verified ? "signed" : "development build"
        versionLabel.stringValue =
            "Version \(status.info.version), \(build), \(status.connected ? "connected" : "not connected")"
    }

    // MARK: Session windows

    private func removeSessionViews() {
        for window in borders { window.orderOut(nil) }
        borders.removeAll()
        stopPanel?.orderOut(nil)
        stopPanel = nil
        ringTimer?.invalidate()
        ringTimer = nil
        ring?.orderOut(nil)
        ring = nil
        for pulse in pulses { pulse.orderOut(nil) }
        pulses.removeAll()
        if let item = statusItem { NSStatusBar.system.removeStatusItem(item) }
        statusItem = nil
    }

    /// Places the ring's center on the pointer, wherever the agent or the person moved it.
    private func followPointer() {
        guard let ring else { return }
        let at = NSEvent.mouseLocation
        let half = Self.ringSize / 2
        ring.setFrameOrigin(NSPoint(x: at.x - half, y: at.y - half))
    }

    /// The AppKit point (origin at the bottom left of the primary screen) of a point in the
    /// helper's screen space (origin at the top left of the main display, y downwards).
    static func appKitPoint(_ point: Point) -> NSPoint {
        let height = NSScreen.screens.first?.frame.height ?? 0
        return NSPoint(x: point.x, y: height - point.y)
    }

    /// A borderless, click-through panel above everything that takes no focus and shows on every
    /// space; the base of the border pieces, the ring and the pulse.
    private static func overlayPanel(frame: NSRect) -> NSPanel {
        let panel = NSPanel(
            contentRect: frame, styleMask: [.borderless, .nonactivatingPanel], backing: .buffered,
            defer: false)
        panel.isReleasedWhenClosed = false
        // The helper becomes active while its settings window is in front; the indicator must
        // stay when it is no longer.
        panel.hidesOnDeactivate = false
        panel.isOpaque = false
        panel.backgroundColor = .clear
        panel.hasShadow = false
        panel.ignoresMouseEvents = true
        panel.level = .screenSaver
        panel.sharingType = .none
        panel.collectionBehavior = [.canJoinAllSpaces, .stationary, .fullScreenAuxiliary, .ignoresCycle]
        return panel
    }

    private static func borderWindow(for screen: NSScreen) -> NSWindow {
        let window = overlayPanel(frame: screen.frame)
        window.contentView = BorderView(frame: NSRect(origin: .zero, size: screen.frame.size))
        window.setFrame(screen.frame, display: true)
        window.orderFrontRegardless()
        return window
    }

    private static func ringPanel() -> NSPanel {
        let size = NSSize(width: ringSize, height: ringSize)
        let panel = overlayPanel(frame: NSRect(origin: .zero, size: size))
        let view = NSView(frame: NSRect(origin: .zero, size: size))
        view.wantsLayer = true
        let ring = CAShapeLayer()
        ring.frame = CGRect(origin: .zero, size: size)
        ring.path = CGPath(
            ellipseIn: CGRect(origin: .zero, size: size).insetBy(dx: 8, dy: 8), transform: nil)
        ring.fillColor = borderColor.withAlphaComponent(0.18).cgColor
        ring.strokeColor = borderColor.cgColor
        ring.lineWidth = 3
        // The glow.
        ring.shadowColor = borderColor.cgColor
        ring.shadowOpacity = 0.9
        ring.shadowRadius = 4
        ring.shadowOffset = .zero
        view.layer?.addSublayer(ring)
        panel.contentView = view
        panel.orderFrontRegardless()
        return panel
    }

    private static func pulse(at center: NSPoint) -> NSPanel {
        let size = NSSize(width: pulseSize, height: pulseSize)
        let panel = overlayPanel(
            frame: NSRect(
                origin: NSPoint(x: center.x - pulseSize / 2, y: center.y - pulseSize / 2), size: size))
        let view = NSView(frame: NSRect(origin: .zero, size: size))
        view.wantsLayer = true
        let ring = CAShapeLayer()
        let bounds = CGRect(origin: .zero, size: size)
        ring.frame = bounds
        ring.path = CGPath(ellipseIn: bounds.insetBy(dx: 4, dy: 4), transform: nil)
        ring.fillColor = NSColor.clear.cgColor
        ring.strokeColor = borderColor.cgColor
        ring.lineWidth = 4
        view.layer?.addSublayer(ring)
        panel.contentView = view
        panel.orderFrontRegardless()
        let grow = CABasicAnimation(keyPath: "transform.scale")
        grow.fromValue = 0.25
        grow.toValue = 1.0
        let fade = CABasicAnimation(keyPath: "opacity")
        fade.fromValue = 1.0
        fade.toValue = 0.0
        let group = CAAnimationGroup()
        group.animations = [grow, fade]
        group.duration = pulseDuration
        group.fillMode = .forwards
        group.isRemovedOnCompletion = false
        ring.add(group, forKey: "pulse")
        return panel
    }

    private func makeStopPanel() -> NSPanel? {
        guard let screen = NSScreen.main ?? NSScreen.screens.first else { return nil }
        let size = NSSize(width: 220, height: 36)
        let origin = NSPoint(
            x: screen.frame.midX - size.width / 2, y: screen.visibleFrame.maxY - size.height - 12)
        let panel = Self.overlayPanel(frame: NSRect(origin: origin, size: size))
        // The stop button takes clicks; everything else of the indicator lets them through.
        panel.ignoresMouseEvents = false
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
