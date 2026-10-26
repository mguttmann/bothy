// SPDX-License-Identifier: Apache-2.0

import Cocoa
import FlutterMacOS
import ServiceManagement

/// The application delegate of the desktop app on macOS.
///
/// Links of the URL scheme of the build branding (`BRANDING_URL_SCHEME`, docs/spec/
/// 15-client-architecture.md 11) arrive through `application(_:open:)` and go to Dart over the
/// channel of `DeepLinkChannel`. The APNs token callbacks go to `PushApple` (brief M5-04).
@main
class AppDelegate: FlutterAppDelegate {
  override func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
    return true
  }

  override func applicationSupportsSecureRestorableState(_ app: NSApplication) -> Bool {
    return true
  }

  override func application(_ application: NSApplication, didRegisterForRemoteNotificationsWithDeviceToken deviceToken: Data) {
    PushApple.shared.didRegister(deviceToken: deviceToken)
  }

  override func application(_ application: NSApplication, didFailToRegisterForRemoteNotificationsWithError error: Error) {
    PushApple.shared.didFailToRegister()
  }

  override func application(_ application: NSApplication, open urls: [URL]) {
    for url in urls {
      DeepLinkChannel.shared.receive(url.absoluteString)
    }
  }
}

/// Forwards deep links to Dart over the method channel `bothy/deep_links`.
///
/// A link that arrives before the Flutter engine runs (the app was started by the link) waits
/// in `pending` until Dart asks for it with `takePending`; later links go out at once with
/// `open`. Only the Dart side parses the link (`lib/app/deep_links.dart`).
final class DeepLinkChannel {
  /// The one instance of the app.
  static let shared = DeepLinkChannel()

  private var channel: FlutterMethodChannel?
  private var pending: [String] = []

  /// Connects the channel to the engine of the main window.
  func attach(messenger: FlutterBinaryMessenger) {
    let channel = FlutterMethodChannel(name: "bothy/deep_links", binaryMessenger: messenger)
    channel.setMethodCallHandler { [weak self] call, result in
      guard call.method == "takePending" else {
        result(FlutterMethodNotImplemented)
        return
      }
      let links = self?.pending ?? []
      self?.pending.removeAll()
      result(links)
    }
    self.channel = channel
  }

  /// Hands a link to Dart, or keeps it until Dart asks.
  func receive(_ link: String) {
    guard let channel = channel else {
      pending.append(link)
      return
    }
    channel.invokeMethod("open", arguments: link)
  }
}

/// "Start at login" over the method channel `bothy/start_at_login` (docs/spec/
/// 15-client-architecture.md 10.3, docs/DECISIONS.md D-M4-30-4): the login item of the app
/// (`SMAppService.mainApp`, macOS 13 and later). The system keeps the value; on macOS 12 the
/// channel reports `isSupported` false and the settings say so.
enum LoginItemChannel {
  /// Connects the channel to the engine of the main window.
  static func attach(messenger: FlutterBinaryMessenger) {
    let channel = FlutterMethodChannel(name: "bothy/start_at_login", binaryMessenger: messenger)
    channel.setMethodCallHandler { call, result in
      guard #available(macOS 13.0, *) else {
        result(call.method == "isSupported" ? false : FlutterMethodNotImplemented)
        return
      }
      switch call.method {
      case "isSupported":
        result(true)
      case "isEnabled":
        result(SMAppService.mainApp.status == .enabled)
      case "setEnabled":
        do {
          if (call.arguments as? Bool) == true {
            try SMAppService.mainApp.register()
          } else {
            try SMAppService.mainApp.unregister()
          }
          result(nil)
        } catch {
          result(FlutterError(code: "login_item", message: nil, details: nil))
        }
      default:
        result(FlutterMethodNotImplemented)
      }
    }
  }
}

/// Automatic updates of the Developer ID edition with Sparkle 2 (docs/spec/18-release.md 8,
/// docs/decisions.d/M7-12.md D-M7-12-3).
///
/// The release build (tools/distribution/apple.py) embeds Sparkle.framework and writes the feed,
/// the public EdDSA key and the installer service into `Info.plist`; any other build has none of
/// them, and the updater stays off. Sparkle is loaded at run time, so the project links no Sparkle
/// and debug and test builds need none. The installation follows the default (stable) channel and
/// the channels of the user default `UpdateChannels` (an array of names, for example `beta`).
final class AppUpdater: NSObject {
  /// The one instance; it is also the updater's delegate, which Sparkle holds weakly.
  static let shared = AppUpdater()

  private var controller: NSObject?

  /// Starts Sparkle's standard updater once, when the build carries Sparkle and a feed.
  func start() {
    guard controller == nil,
      Bundle.main.object(forInfoDictionaryKey: "SUFeedURL") != nil,
      let frameworks = Bundle.main.privateFrameworksPath,
      let sparkle = Bundle(path: "\(frameworks)/Sparkle.framework"), sparkle.load(),
      let type = NSClassFromString("SPUStandardUpdaterController") as? NSObject.Type
    else { return }
    let initializer = NSSelectorFromString("initWithUpdaterDelegate:userDriverDelegate:")
    // `alloc` hands over one reference that `init` consumes; `init` returns one that we keep.
    guard
      let allocated = type.perform(NSSelectorFromString("alloc"))?.takeUnretainedValue()
        as? NSObject,
      allocated.responds(to: initializer)
    else { return }
    controller = allocated.perform(initializer, with: self, with: nil)?.takeRetainedValue()
      as? NSObject
  }

  /// `SPUUpdaterDelegate`: the channels besides the default one this installation follows.
  @objc(allowedChannelsForUpdater:)
  func allowedChannels(forUpdater updater: NSObject) -> Set<String> {
    Set(UserDefaults.standard.stringArray(forKey: "UpdateChannels") ?? [])
  }
}
