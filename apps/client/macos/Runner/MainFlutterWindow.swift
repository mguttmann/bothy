// SPDX-License-Identifier: Apache-2.0

import Cocoa
import FlutterMacOS

/// The main window: the Flutter view, the accessibility writes into text fields
/// (`AccessibilityTextInput`), the plugins, the deep link channel, the channel of
/// "Start at login" (`LoginItemChannel`), the clipboard of block copies
/// (`RichClipboardChannel`), the push channel (`PushApple`), the sharing channel
/// (`ShareApple`) and the updater of the Developer ID edition (`AppUpdater`, only when the
/// build carries Sparkle; its channel `bothy/updater` answers in every build).
class MainFlutterWindow: NSWindow {
  override func awakeFromNib() {
    let flutterViewController = FlutterViewController()
    let windowFrame = self.frame
    self.contentViewController = flutterViewController
    self.setFrame(windowFrame, display: true)

    // Dictation and text tools that write through the Accessibility API (AXSelectedText,
    // AXValue) reach Flutter's text model (FIX-macos-ax-text-insert); before the plugins, so
    // no text field can take focus first.
    AccessibilityTextInput.install()
    RegisterGeneratedPlugins(registry: flutterViewController)
    DeepLinkChannel.shared.attach(messenger: flutterViewController.engine.binaryMessenger)
    LoginItemChannel.attach(messenger: flutterViewController.engine.binaryMessenger)
    RichClipboardChannel.attach(messenger: flutterViewController.engine.binaryMessenger)
    // Push (M5-04): the notification center's delegate before launch finishes, so a tap that
    // started the app reaches Dart through `takePendingTaps`. It keeps the delegate that
    // flutter_local_notifications set while registering and forwards it every local
    // notification, so their clicks still open their thread (M4-30).
    PushApple.shared.installDelegate()
    PushApple.shared.attach(messenger: flutterViewController.engine.binaryMessenger)
    // Sharing (M5-06): the child session, the share settings and the share inbox.
    ShareApple.shared.attach(messenger: flutterViewController.engine.binaryMessenger)
    #if DEBUG
      AccessibilityTreeProbe.attach(
        messenger: flutterViewController.engine.binaryMessenger,
        engine: flutterViewController.engine,
        window: self)
    #endif
    AppUpdater.shared.start()
    // The update path of E1 and of "Update required" (brief M7-13).
    AppUpdater.shared.attach(messenger: flutterViewController.engine.binaryMessenger)

    super.awakeFromNib()
  }
}

#if DEBUG
  /// The probe of the platform accessibility tree for the integration test
  /// `integration_test/a11y_platform_tree_test.dart` (brief M4-32 Design note 2), over the method
  /// channel `bothy/a11y_probe`; debug builds only, so no release contains it.
  ///
  /// `enable` turns the accessibility of the engine on, as VoiceOver does when it starts (the
  /// notification the engine listens to, and its `semanticsEnabled` where it answers to it).
  /// `tree` walks the window's accessibility hierarchy through the same attributes VoiceOver
  /// reads (role, description, title, value, placeholder) and returns one map per element. It
  /// runs inside the app, so it needs no accessibility permission for another process and
  /// works in the app sandbox (docs/decisions.d/M4-32.md).
  ///
  /// For `integration_test/composer_ax_insert_test.dart` (FIX-macos-ax-text-insert):
  /// `activate` brings the app and its window to the front, `textInput` returns the text and selection of the window's first responder when it is the
  /// engine's text input, else nil; `axInsert` writes `text` into it as a dictation tool does,
  /// as the selected text or (`attribute` `value`) the whole value, through the setter of
  /// NSAccessibility or (`legacy`) through `accessibilitySetValue:forAttribute:`, and returns
  /// `inserted`, or `noTextInput` without a focused engine text input.
  enum AccessibilityTreeProbe {
    /// Connects the channel to the engine of the main window.
    static func attach(
      messenger: FlutterBinaryMessenger, engine: FlutterEngine, window: NSWindow
    ) {
      let channel = FlutterMethodChannel(name: "bothy/a11y_probe", binaryMessenger: messenger)
      channel.setMethodCallHandler { [weak window, weak engine] call, result in
        switch call.method {
        case "enable":
          NotificationCenter.default.post(
            name: NSNotification.Name(
              "NSApplicationDidChangeAccessibilityEnhancedUserInterfaceNotification"),
            object: nil,
            userInfo: ["AXEnhancedUserInterface": true])
          let setter = NSSelectorFromString("setSemanticsEnabled:")
          if let engine = engine, engine.responds(to: setter) {
            engine.setValue(true, forKey: "semanticsEnabled")
          }
          result(nil)
        case "activate":
          NSApp.activate(ignoringOtherApps: true)
          window?.makeKeyAndOrderFront(nil)
          result(nil)
        case "textInput":
          guard let input = focusedTextInput(window) else {
            result(nil)
            return
          }
          let selection = input.selectedRange()
          result([
            "text": input.string, "start": selection.location, "length": selection.length,
          ])
        case "axInsert":
          let arguments = call.arguments as? [String: Any] ?? [:]
          guard let input = focusedTextInput(window) else {
            result("noTextInput")
            return
          }
          let text = arguments["text"] as? String ?? ""
          let value = arguments["attribute"] as? String == "value"
          if arguments["legacy"] as? Bool == true {
            let attribute: NSAccessibility.Attribute = value ? .value : .selectedText
            _ = input.perform(
              NSSelectorFromString("accessibilitySetValue:forAttribute:"), with: text,
              with: attribute.rawValue)
          } else if value {
            input.setAccessibilityValue(text)
          } else {
            input.setAccessibilitySelectedText(text)
          }
          result("inserted")
        case "tree":
          var elements: [[String: String]] = []
          if let root = window?.contentView {
            walk(root, depth: 0, into: &elements)
          }
          result(elements)
        default:
          result(FlutterMethodNotImplemented)
        }
      }
    }

    /// The window's first responder when it is the engine's text input plugin.
    private static func focusedTextInput(_ window: NSWindow?) -> NSTextView? {
      guard let plugin = NSClassFromString(AccessibilityTextInput.pluginClassName),
        let responder = window?.firstResponder as? NSTextView, responder.isKind(of: plugin)
      else { return nil }
      return responder
    }

    /// The texts a screen reader reads as the name, with the new accessibility method and the
    /// attribute of the older API that VoiceOver asks when the method gives nothing.
    private static let attributes: [(String, String, String)] = [
      ("role", "accessibilityRole", "AXRole"),
      ("description", "accessibilityLabel", "AXDescription"),
      ("title", "accessibilityTitle", "AXTitle"),
      ("value", "accessibilityValue", "AXValue"),
      ("placeholder", "accessibilityPlaceholderValue", "AXPlaceholderValue"),
    ]

    /// The value of [method], else of the older attribute [legacy]; nil when the element
    /// answers to neither. Selectors keep the probe free of deprecated declarations.
    private static func read(_ object: NSObject, _ method: String, _ legacy: String) -> Any? {
      let modern = NSSelectorFromString(method)
      if object.responds(to: modern), let value = object.perform(modern)?.takeUnretainedValue() {
        return value
      }
      let older = NSSelectorFromString("accessibilityAttributeValue:")
      if object.responds(to: older) {
        return object.perform(older, with: legacy)?.takeUnretainedValue()
      }
      return nil
    }

    private static func walk(_ element: Any, depth: Int, into elements: inout [[String: String]]) {
      guard depth < 80, let object = element as? NSObject else { return }
      var entry: [String: String] = [:]
      for (key, method, legacy) in attributes {
        if let text = read(object, method, legacy) as? String, !text.isEmpty {
          entry[key] = text
        }
      }
      if !entry.isEmpty {
        elements.append(entry)
      }
      let children = read(object, "accessibilityChildren", "AXChildren") as? [Any] ?? []
      for child in children {
        walk(child, depth: depth + 1, into: &elements)
      }
    }
  }
#endif
