// SPDX-License-Identifier: Apache-2.0

import AppKit
import os.log

/// Text that dictation and text tools write through the Accessibility API reaches the composer
/// (FIX-macos-ax-text-insert, docs/decisions.d/FIX-macos-ax-text-insert.md).
///
/// The focused accessibility element of a Flutter text field is the engine's private
/// `FlutterTextInputPlugin`, an `NSTextView` that keeps the engine's text model beside its own
/// storage and hears edits only through `insertText(_:replacementRange:)`. AppKit's
/// `setAccessibilitySelectedText(_:)` and `setAccessibilityValue(_:)` write into the view's text
/// storage directly: the tool sees the new value and reports success, while the text model of
/// Flutter, and so the composer, never changes. This adds both setters to the plugin's class
/// and lets them insert through `insertText(_:replacementRange:)`, the path of typing and of
/// the input methods; the legacy `accessibilitySetValue(_:forAttribute:)`, which AppKit's
/// accessibility server calls for writes of other processes, routes the selected text and the
/// value to them and keeps its original behaviour for every other attribute.
///
/// A setter the class already writes itself (a later engine) stays untouched. Without the
/// class the patch logs once and does nothing.
enum AccessibilityTextInput {
  /// The engine's text input plugin.
  static let pluginClassName = "FlutterTextInputPlugin"

  private static let log = OSLog(
    subsystem: Bundle.main.bundleIdentifier ?? "Runner", category: "accessibility")

  private static let selectedTextSetter = #selector(
    NSTextView.setAccessibilitySelectedText(_:))
  private static let valueSetter = #selector(NSTextView.setAccessibilityValue(_:))
  private static let legacySetter = NSSelectorFromString("accessibilitySetValue:forAttribute:")

  /// Adds the setters to the class named [className] where the class itself has none; returns
  /// the names of the added selectors, or nil when there is no such `NSTextView` subclass.
  @discardableResult
  static func install(className: String = pluginClassName) -> [String]? {
    guard let target = NSClassFromString(className), target is NSTextView.Type,
      let parent = class_getSuperclass(target)
    else {
      os_log(
        "accessibility text input: no text view class %{public}@, dictation stays unpatched",
        log: log, type: .error, className)
      return nil
    }
    let own = ownSelectors(target)
    var added: [String] = []
    func add(_ selector: Selector, _ block: Any, _ types: String) {
      if own.contains(selector) {
        os_log(
          "accessibility text input: %{public}@ already implements %{public}@",
          log: log, type: .info, className, NSStringFromSelector(selector))
        return
      }
      if class_addMethod(target, selector, imp_implementationWithBlock(block), types) {
        added.append(NSStringFromSelector(selector))
      }
    }

    let setSelectedText: @convention(block) (NSTextView, AnyObject?) -> Void = { view, value in
      guard let text = text(of: value) else { return }
      view.insertText(text, replacementRange: view.selectedRange())
    }
    let setValue: @convention(block) (NSTextView, AnyObject?) -> Void = { view, value in
      guard let text = text(of: value) else { return }
      let whole = NSRange(location: 0, length: (view.string as NSString).length)
      view.insertText(text, replacementRange: whole)
    }
    typealias LegacySetter = @convention(c) (AnyObject, Selector, AnyObject?, NSString) -> Void
    let legacy = legacySetter
    let setLegacy: @convention(block) (NSTextView, AnyObject?, NSString) -> Void = {
      view, value, attribute in
      switch attribute as String {
      case NSAccessibility.Attribute.selectedText.rawValue:
        view.perform(selectedTextSetter, with: value)
      case NSAccessibility.Attribute.value.rawValue:
        view.perform(valueSetter, with: value)
      default:
        guard let imp = class_getMethodImplementation(parent, legacy) else { return }
        unsafeBitCast(imp, to: LegacySetter.self)(view, legacy, value, attribute)
      }
    }

    add(selectedTextSetter, setSelectedText, "v@:@")
    add(valueSetter, setValue, "v@:@")
    add(legacySetter, setLegacy, "v@:@@")
    return added
  }

  /// The text of an accessibility value: a string, the string of an attributed string, empty
  /// for nil; nil for any other value, which then changes nothing.
  private static func text(of value: AnyObject?) -> String? {
    switch value {
    case nil: return ""
    case let string as String: return string
    case let attributed as NSAttributedString: return attributed.string
    default: return nil
    }
  }

  /// The selectors the class implements itself, without those of its superclasses.
  private static func ownSelectors(_ target: AnyClass) -> Set<Selector> {
    var count: UInt32 = 0
    guard let methods = class_copyMethodList(target, &count) else { return [] }
    defer { free(methods) }
    return Set((0..<Int(count)).map { method_getName(methods[$0]) })
  }
}
