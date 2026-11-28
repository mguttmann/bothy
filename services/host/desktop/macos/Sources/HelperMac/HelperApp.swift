// SPDX-License-Identifier: Apache-2.0

import AppKit
import Foundation
import HelperCore
import os

/// Wires the platform implementations to `HelperController` and runs the accessory app.
///
/// Threads: AppKit on the main thread; the controller on one serial queue; the channel reader
/// on its own thread, which queues frames for the controller; the Esc tap on its own run loop.
/// The tap, the stop button, the lock monitor and the reader also set the shared
/// `ActionInterrupt` directly, because the controller's queue may be busy with an action.
/// The main thread enters the controller only with `async`, and the controller may wait for the
/// main thread (`MainThread.sync`), so the two never wait for each other.
public final class HelperApp: NSObject, NSApplicationDelegate {
    /// The version for `hello` and `--version`: the bundle's `CFBundleShortVersionString`, which
    /// `bundle.sh` fills in; a build outside a bundle reports `0.0.0-dev`.
    public static var version: String {
        Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String
            ?? "0.0.0-dev"
    }

    private let queue = DispatchQueue(label: "com.example.bothy-computer-helper.controller")
    private let logger = Logger(subsystem: "com.example.bothy-computer-helper", category: "helper")
    private var controller: HelperController?
    private var indicator: MacIndicator?
    private var axObserver: NSObjectProtocol?
    private var settingsObserver: NSObjectProtocol?

    /// Creates the app delegate.
    override public init() {
        super.init()
    }

    /// The distributed notification with which a second start asks the running helper to open
    /// its settings window (decisions.d/FIX-CU-HELPER-NO-ICON.md); it opens nothing else.
    static let openSettingsNotification = Notification.Name("com.example.bothy-computer-helper.open-settings")

    /// Starts the helper once AppKit runs.
    public func applicationDidFinishLaunching(_ notification: Notification) {
        // The person opened the app while the helper already runs, and the system started a
        // second process instead of sending the running one a reopen event (the running helper
        // comes from launchd, not from LaunchServices): ask that one to open its settings and end,
        // so two helpers never compete for the channel.
        if let id = Bundle.main.bundleIdentifier,
            NSRunningApplication.runningApplications(withBundleIdentifier: id)
                .contains(where: { $0.processIdentifier != getpid() })
        {
            // Only a start by the person asks for the window; a start by the LaunchAgent (its
            // label in XPC_SERVICE_NAME) ends silently, so a restart loop of launchd shows nothing.
            if ProcessInfo.processInfo.environment["XPC_SERVICE_NAME"] != id {
                DistributedNotificationCenter.default().postNotificationName(
                    Self.openSettingsNotification, object: nil, userInfo: nil, deliverImmediately: true)
            }
            NSApp.terminate(nil)
            return
        }
        let queue = self.queue
        let dispatch: (@escaping () -> Void) -> Void = { work in queue.async(execute: work) }
        let signature = MacSignature.current()
        let info = HelperInfo(version: Self.version, signature: signature, osAccount: NSUserName())
        let injector = MacInjector()
        let frameMax = HostConfig.frameMaxBytes()
        let interrupt = ActionInterrupt()
        var controllerRef: HelperController?
        let indicator = MacIndicator(
            dispatch: dispatch, interrupt: interrupt, onToggle: { controllerRef?.toggleEnablementByClick() },
            onSettingsOpen: { controllerRef?.refreshStatus() },
            onPermissionLink: { controllerRef?.permissionLinkClicked($0) })
        let logger = self.logger
        let controller = HelperController(
            capturer: MacCapturer(), injector: injector, appInspector: MacAppInspector(),
            lockMonitor: MacLockMonitor(interrupt: interrupt, dispatch: dispatch),
            waker: MacWaker(),
            escTap: MacEscTap(
                ownSourceStateID: injector.sourceStateID, interrupt: interrupt, dispatch: dispatch),
            indicator: indicator, store: FileEnablementStore(folder: FileEnablementStore.defaultFolder()),
            info: info, frameMaxBytes: frameMax, interrupt: interrupt,
            log: { logger.notice("\($0, privacy: .public)") })
        controllerRef = controller
        self.controller = controller
        self.indicator = indicator
        // The accessibility list changed: report the permission without a timer.
        axObserver = DistributedNotificationCenter.default().addObserver(
            forName: .init("com.apple.accessibility.api"), object: nil, queue: nil
        ) { _ in dispatch { controller.refreshStatus() } }
        settingsObserver = DistributedNotificationCenter.default().addObserver(
            forName: Self.openSettingsNotification, object: nil, queue: .main
        ) { [weak self] _ in self?.indicator?.openSettings() }
        // Asynchronous, because start() waits for the main thread to hide the indicator. The
        // settings window opens by itself only while the switch was never switched on, so the
        // person finds it after installing; otherwise the helper shows nothing until a session.
        dispatch {
            controller.start()
            let status = controller.status
            if EnablementState(enabled: status.enabled, generation: status.generation).needsSetup {
                DispatchQueue.main.async { indicator.openSettings() }
            }
        }

        let channel = UnixSocketChannel.forBuild(
            signature: signature, environment: ProcessInfo.processInfo.environment)
        if !channel.checksPeer {
            logger.notice("development build: channel override in use, peer check skipped")
        }
        let runner = ChannelRunner(
            channel: channel, controller: controller, frameMaxBytes: frameMax,
            // Asynchronous, so the reader keeps reading during a long action and a hide,
            // disable or break reaches the interrupt flag at once.
            perform: { work in queue.async(execute: work) },
            log: { logger.notice("\($0, privacy: .public)") })
        let thread = Thread { runner.run() }
        thread.name = "channel"
        thread.start()
    }

    /// The person opened the helper app while it runs (Finder, Spotlight, `open`): the system
    /// sends the running helper a reopen event, which opens the settings window.
    public func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows flag: Bool) -> Bool {
        indicator?.openSettings()
        return false
    }

    /// Runs the helper as an accessory app (no Dock icon, no menu bar item, no window outside a
    /// session or its settings) until it is ended.
    public static func main() {
        let app = NSApplication.shared
        app.setActivationPolicy(.accessory)
        let delegate = HelperApp()
        app.delegate = delegate
        withExtendedLifetime(delegate) { app.run() }
    }
}
