// SPDX-License-Identifier: Apache-2.0

import Cocoa
import FlutterMacOS

/// The main window: the Flutter view, the plugins, the deep link channel, the channel of
/// "Start at login" (`LoginItemChannel`) and the push channel (`PushApple`).
class MainFlutterWindow: NSWindow {
  override func awakeFromNib() {
    let flutterViewController = FlutterViewController()
    let windowFrame = self.frame
    self.contentViewController = flutterViewController
    self.setFrame(windowFrame, display: true)

    RegisterGeneratedPlugins(registry: flutterViewController)
    DeepLinkChannel.shared.attach(messenger: flutterViewController.engine.binaryMessenger)
    LoginItemChannel.attach(messenger: flutterViewController.engine.binaryMessenger)
    // Push (M5-04): the notification center's delegate before launch finishes, so a tap that
    // started the app reaches Dart through `takePendingTaps`. It keeps the delegate that
    // flutter_local_notifications set while registering and forwards it every local
    // notification, so their clicks still open their thread (M4-30).
    PushApple.shared.installDelegate()
    PushApple.shared.attach(messenger: flutterViewController.engine.binaryMessenger)

    super.awakeFromNib()
  }
}
