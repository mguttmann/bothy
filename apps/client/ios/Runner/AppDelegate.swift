// SPDX-License-Identifier: Apache-2.0

import Flutter
import UIKit

/// The application delegate of the app on iOS.
///
/// The engine is the implicit engine of the scene (`SceneDelegate`); once it runs, the
/// generated plugins and the runner's own plugin (`IosPlatformPlugin`: login, passkeys, file
/// protection and deep links) register on it.
@main
@objc class AppDelegate: FlutterAppDelegate, FlutterImplicitEngineDelegate {
  override func application(
    _ application: UIApplication,
    didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
  ) -> Bool {
    return super.application(application, didFinishLaunchingWithOptions: launchOptions)
  }

  func didInitializeImplicitFlutterEngine(_ engineBridge: FlutterImplicitEngineBridge) {
    GeneratedPluginRegistrant.register(with: engineBridge.pluginRegistry)
    if let registrar = engineBridge.pluginRegistry.registrar(forPlugin: "IosPlatformPlugin") {
      IosPlatformPlugin.register(with: registrar)
    }
  }
}
