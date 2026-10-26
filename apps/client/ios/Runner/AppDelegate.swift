// SPDX-License-Identifier: Apache-2.0

import Flutter
import UIKit

/// The application delegate of the app on iOS.
///
/// The engine is the implicit engine of the scene (`SceneDelegate`); once it runs, the
/// generated plugins and the runner's own plugin (`IosPlatformPlugin`: login, passkeys, file
/// protection and deep links) register on it, and so does the push channel (`PushApple`, brief
/// M5-04), which is the notification center's delegate from launch on, so a tap that started
/// the app is not lost; it forwards every notification that is no push to the delegate it
/// replaced, or to this app delegate, which hands it to the plugins. The sharing channel
/// (`ShareApple`, brief M5-06) registers on the engine the same way.
@main
@objc class AppDelegate: FlutterAppDelegate, FlutterImplicitEngineDelegate {
  override func application(
    _ application: UIApplication,
    didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
  ) -> Bool {
    PushApple.shared.installDelegate(fallback: self)
    return super.application(application, didFinishLaunchingWithOptions: launchOptions)
  }

  override func application(
    _ application: UIApplication, didRegisterForRemoteNotificationsWithDeviceToken deviceToken: Data
  ) {
    PushApple.shared.didRegister(deviceToken: deviceToken)
  }

  override func application(
    _ application: UIApplication, didFailToRegisterForRemoteNotificationsWithError error: Error
  ) {
    PushApple.shared.didFailToRegister()
  }

  func didInitializeImplicitFlutterEngine(_ engineBridge: FlutterImplicitEngineBridge) {
    GeneratedPluginRegistrant.register(with: engineBridge.pluginRegistry)
    if let registrar = engineBridge.pluginRegistry.registrar(forPlugin: "IosPlatformPlugin") {
      IosPlatformPlugin.register(with: registrar)
    }
    if let registrar = engineBridge.pluginRegistry.registrar(forPlugin: "PushApple") {
      PushApple.shared.attach(messenger: registrar.messenger())
    }
    if let registrar = engineBridge.pluginRegistry.registrar(forPlugin: "ShareApple") {
      ShareApple.shared.attach(messenger: registrar.messenger())
    }
    // A plugin may have taken the notification center over while it registered.
    PushApple.shared.installDelegate(fallback: self)
  }
}
