// SPDX-License-Identifier: Apache-2.0

import Cocoa
import FlutterMacOS

/// The main window: the Flutter view, the plugins, the deep link channel and the channel of
/// "Start at login" (`LoginItemChannel`).
class MainFlutterWindow: NSWindow {
  override func awakeFromNib() {
    let flutterViewController = FlutterViewController()
    let windowFrame = self.frame
    self.contentViewController = flutterViewController
    self.setFrame(windowFrame, display: true)

    RegisterGeneratedPlugins(registry: flutterViewController)
    DeepLinkChannel.shared.attach(messenger: flutterViewController.engine.binaryMessenger)
    LoginItemChannel.attach(messenger: flutterViewController.engine.binaryMessenger)

    super.awakeFromNib()
  }
}
