// SPDX-License-Identifier: Apache-2.0

// The checks of `Runner/AccessibilityTextInput.swift` without the engine (FIX-macos-ax-text-insert):
// a text view that stands in for the engine's `FlutterTextInputPlugin` keeps its own text model
// and hears only `insertText(_:replacementRange:)`, as the engine does. Dictation and text tools
// write through the accessibility setters; the checks prove that AppKit's own setters bypass
// that method and that the patch routes them to it. `run_tests.sh` builds and runs this file.

import AppKit

/// The stand-in of the engine's plugin: its model changes only through `insertText`, like the
/// engine's `TextInputModel`, and the view mirrors the model as the engine does.
@objc(FakeTextInputPlugin)
final class FakeTextInputPlugin: NSTextView {
  var model = ""
  var cursor = 0
  var inserts: [(String, NSRange)] = []

  func setModel(_ text: String, cursor: Int) {
    model = text
    self.cursor = cursor
    string = text
    setSelectedRange(NSRange(location: cursor, length: 0))
  }

  override func insertText(_ string: Any, replacementRange: NSRange) {
    let text = (string as? NSAttributedString)?.string ?? (string as? String) ?? ""
    inserts.append((text, replacementRange))
    let range = replacementRange.location == NSNotFound
      ? NSRange(location: cursor, length: 0) : replacementRange
    model = (model as NSString).replacingCharacters(in: range, with: text)
    cursor = range.location + (text as NSString).length
    self.string = model
    setSelectedRange(NSRange(location: cursor, length: 0))
  }
}

/// A plugin class that writes its own value setter: the patch must leave it alone.
@objc(FakeOwnSetterPlugin)
final class FakeOwnSetterPlugin: NSTextView {
  var ownValue: Any?
  var inserts = 0
  override func setAccessibilityValue(_ accessibilityValue: Any?) { ownValue = accessibilityValue }
  override func insertText(_ string: Any, replacementRange: NSRange) { inserts += 1 }
}

/// An uninstalled copy of the stand-in, for the behaviour of AppKit without the patch.
@objc(FakeUnpatchedPlugin)
final class FakeUnpatchedPlugin: NSTextView {
  var inserts = 0
  override func insertText(_ string: Any, replacementRange: NSRange) { inserts += 1 }
}

var failures = 0
func check(_ condition: Bool, _ what: String) {
  if condition {
    print("ok   \(what)")
  } else {
    failures += 1
    print("FAIL \(what)")
  }
}

/// The legacy setter of the informal accessibility protocol, through the runtime because the
/// SDK marks it deprecated; AppKit's accessibility server still calls it for writes of AX
/// clients in other processes.
func legacySet(_ view: NSView, _ value: Any, _ attribute: String) {
  let selector = NSSelectorFromString("accessibilitySetValue:forAttribute:")
  typealias Setter = @convention(c) (AnyObject, Selector, Any?, NSString) -> Void
  let imp = unsafeBitCast(view.method(for: selector), to: Setter.self)
  imp(view, selector, value, attribute as NSString)
}

func frame() -> NSRect { NSRect(x: 0, y: 0, width: 200, height: 40) }

// The cause: without the patch AppKit writes the text into the view's storage and never calls
// insertText, so a model kept beside the view never hears of it.
do {
  let view = FakeUnpatchedPlugin(frame: frame())
  view.string = "ab"
  view.setSelectedRange(NSRange(location: 1, length: 0))
  view.setAccessibilitySelectedText("Hallo")
  check(view.inserts == 0, "unpatched: setAccessibilitySelectedText bypasses insertText")
  check(view.string == "aHallob", "unpatched: the text lands in the view's storage only")
}

// A missing class: nothing happens and nothing crashes.
check(AccessibilityTextInput.install(className: "NoSuchTextInputPlugin") == nil,
  "missing class: install reports nothing installed")

let added = AccessibilityTextInput.install(className: "FakeTextInputPlugin") ?? []
check(Set(added) == [
  "setAccessibilitySelectedText:", "setAccessibilityValue:", "accessibilitySetValue:forAttribute:",
], "install adds the three setters: \(added)")
check(AccessibilityTextInput.install(className: "FakeTextInputPlugin") == [],
  "a second install adds nothing")

do {
  let view = FakeTextInputPlugin(frame: frame())
  view.setModel("ab", cursor: 1)
  view.setAccessibilitySelectedText("Hallo")
  check(view.inserts.count == 1 && view.inserts[0].0 == "Hallo"
    && view.inserts[0].1 == NSRange(location: 1, length: 0),
    "selected text: insertText(\"Hallo\", {1, 0})")
  check(view.model == "aHallob" && view.cursor == 6, "selected text: model aHallob, cursor 6")
  check(view.accessibilityValue() == "aHallob", "selected text: AXValue follows the model")
}

do {
  let view = FakeTextInputPlugin(frame: frame())
  view.setModel("abcd", cursor: 0)
  view.setSelectedRange(NSRange(location: 1, length: 2))
  view.setAccessibilitySelectedText("Z")
  check(view.inserts.last?.1 == NSRange(location: 1, length: 2) && view.model == "aZd",
    "selected text replaces the selection")
  view.setAccessibilitySelectedText(nil)
  check(view.inserts.count == 2 && view.inserts[1].0 == "", "selected text nil inserts nothing")
}

do {
  let view = FakeTextInputPlugin(frame: frame())
  view.setModel("ab", cursor: 1)
  view.setAccessibilityValue("X")
  check(view.inserts.count == 1 && view.inserts[0].1 == NSRange(location: 0, length: 2),
    "value: insertText over the whole string")
  check(view.model == "X" && view.cursor == 1, "value: model X, cursor 1")
  view.setAccessibilityValue(NSAttributedString(string: "Yz"))
  check(view.model == "Yz", "value: an attributed string inserts its text")
  view.setAccessibilityValue(42)
  check(view.model == "Yz" && view.inserts.count == 2, "value: a non-text value changes nothing")
}

do {
  let view = FakeTextInputPlugin(frame: frame())
  view.setModel("ab", cursor: 1)
  legacySet(view, "Hallo", "AXSelectedText")
  check(view.model == "aHallob", "legacy AXSelectedText routes to insertText")
  legacySet(view, "Q", "AXValue")
  check(view.model == "Q", "legacy AXValue routes to insertText")
  let before = view.inserts.count
  view.setModel("abc", cursor: 0)
  legacySet(view, NSValue(range: NSRange(location: 2, length: 0)), "AXSelectedTextRange")
  check(view.inserts.count == before && view.selectedRange() == NSRange(location: 2, length: 0),
    "legacy AXSelectedTextRange keeps the original setter")
}

do {
  let added = AccessibilityTextInput.install(className: "FakeOwnSetterPlugin") ?? []
  check(!added.contains("setAccessibilityValue:"), "a setter of the class itself stays: \(added)")
  let view = FakeOwnSetterPlugin(frame: frame())
  view.setAccessibilityValue("X")
  check(view.ownValue as? String == "X" && view.inserts == 0, "the own setter still runs")
  view.setAccessibilitySelectedText("S")
  check(view.inserts == 1, "the setter the class lacks is added")
}

if failures > 0 {
  print("\(failures) check(s) failed")
  exit(1)
}
print("all checks passed")
