// SPDX-License-Identifier: Apache-2.0

import Cocoa
import FlutterMacOS

/// The main window: the Flutter view, the plugins and the deep link channel.
class MainFlutterWindow: NSWindow {
  override func awakeFromNib() {
    let flutterViewController = FlutterViewController()
    let windowFrame = self.frame
    self.contentViewController = flutterViewController
    self.setFrame(windowFrame, display: true)

    RegisterGeneratedPlugins(registry: flutterViewController)
    DeepLinkChannel.shared.attach(messenger: flutterViewController.engine.binaryMessenger)

    super.awakeFromNib()
  }
}
