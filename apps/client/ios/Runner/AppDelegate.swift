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
/// (`ShareApple`, brief M5-06) registers on the engine the same way, and so does the clipboard
/// of block copies (`RichClipboardChannel`, brief M4-48). A debug build also connects the text
/// input probe of the phone integration tests (`TextInputProbeChannel`, DICTATION-MOBILE).
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
    if let registrar = engineBridge.pluginRegistry.registrar(forPlugin: "RichClipboard") {
      RichClipboardChannel.attach(messenger: registrar.messenger())
    }
    #if DEBUG
      if let registrar = engineBridge.pluginRegistry.registrar(forPlugin: "TextInputProbe") {
        TextInputProbeChannel.attach(messenger: registrar.messenger())
      }
    #endif
    // A plugin may have taken the notification center over while it registered.
    PushApple.shared.installDelegate(fallback: self)
  }
}

/// The clipboard of the block copies over the method channel `bothy/clipboard` (docs/spec/
/// 01-product.md 6.12 item 4, brief M4-48; `PlatformRichClipboard` in
/// lib/app/rich_clipboard.dart): `write` puts the plain text and the HTML form of a quote on the
/// general pasteboard as one item, so a rich editor pastes the formatting and every other app
/// the text.
enum RichClipboardChannel {
  /// Connects the channel to [messenger].
  static func attach(messenger: FlutterBinaryMessenger) {
    let channel = FlutterMethodChannel(name: "bothy/clipboard", binaryMessenger: messenger)
    channel.setMethodCallHandler { call, result in
      guard call.method == "write" else {
        result(FlutterMethodNotImplemented)
        return
      }
      guard let args = call.arguments as? [String: Any],
        let text = args["text"] as? String, let html = args["html"] as? String
      else {
        result(FlutterError(code: "clipboard", message: "text and html expected", details: nil))
        return
      }
      UIPasteboard.general.setItems(
        [["public.html": html, "public.utf8-plain-text": text]], options: [:])
      result(nil)
    }
  }
}

#if DEBUG
  /// The text input probe of the phone integration tests (DICTATION-MOBILE), in debug builds
  /// only (`SWIFT_ACTIVE_COMPILATION_CONDITIONS` of ios/Flutter/Debug.xcconfig).
  ///
  /// The method channel `bothy/text_input_probe` drives the focused `UITextInput` (Flutter's text
  /// input view of the focused field) as input sources outside the app do: `insertText` (what a
  /// keyboard extension's `textDocumentProxy.insertText` and a dictation app's insert arrive as),
  /// `keyboardInsert` (an extension that reads the text before the cursor and inserts its chunks,
  /// with a space before the first when that text does not end in white space), `dictate` (the
  /// system dictation: each partial result as marked text, then `unmarkText`) and `paste` (the
  /// text on the general pasteboard, then the action `paste:` sent to the first responder, as the
  /// edit menu does). `text` changes nothing. Every call answers the text of the field as the text
  /// input holds it afterwards; without a focused text input it fails with `no_input`.
  enum TextInputProbeChannel {
    /// Connects the channel to [messenger].
    static func attach(messenger: FlutterBinaryMessenger) {
      let channel = FlutterMethodChannel(
        name: "bothy/text_input_probe", binaryMessenger: messenger)
      channel.setMethodCallHandler { call, result in
        guard let input = focusedInput() else {
          result(FlutterError(code: "no_input", message: "no focused text input", details: nil))
          return
        }
        let args = call.arguments as? [String: Any] ?? [:]
        switch call.method {
        case "insertText":
          input.insertText(args["text"] as? String ?? "")
        case "keyboardInsert":
          var chunks = args["chunks"] as? [String] ?? []
          if let last = textBeforeCursor(input).last, !last.isWhitespace, !chunks.isEmpty {
            chunks[0] = " " + chunks[0]
          }
          for chunk in chunks {
            input.insertText(chunk)
          }
        case "dictate":
          for partial in args["partials"] as? [String] ?? [] {
            input.setMarkedText(
              partial, selectedRange: NSRange(location: (partial as NSString).length, length: 0))
          }
          input.unmarkText()
        case "paste":
          UIPasteboard.general.string = args["text"] as? String ?? ""
          let paste = #selector(UIResponderStandardEditActions.paste(_:))
          guard input.canPerformAction(paste, withSender: nil) else {
            result(FlutterError(code: "no_paste", message: "paste: is not offered", details: nil))
            return
          }
          UIApplication.shared.sendAction(paste, to: nil, from: nil, for: nil)
        case "text":
          break
        default:
          result(FlutterMethodNotImplemented)
          return
        }
        result(fullText(input))
      }
    }

    /// The first responder of the app's windows that is a text input, or nil.
    private static func focusedInput() -> (UIView & UITextInput)? {
      for scene in UIApplication.shared.connectedScenes {
        guard let windows = (scene as? UIWindowScene)?.windows else { continue }
        for window in windows {
          if let input = firstResponder(in: window) { return input }
        }
      }
      return nil
    }

    private static func firstResponder(in view: UIView) -> (UIView & UITextInput)? {
      if view.isFirstResponder, let input = view as? UIView & UITextInput { return input }
      for sub in view.subviews {
        if let input = firstResponder(in: sub) { return input }
      }
      return nil
    }

    private static func fullText(_ input: UITextInput) -> String {
      guard let all = input.textRange(from: input.beginningOfDocument, to: input.endOfDocument)
      else { return "" }
      return input.text(in: all) ?? ""
    }

    private static func textBeforeCursor(_ input: UITextInput) -> String {
      guard let selection = input.selectedTextRange,
        let before = input.textRange(from: input.beginningOfDocument, to: selection.start)
      else { return "" }
      return input.text(in: before) ?? ""
    }
  }
#endif
