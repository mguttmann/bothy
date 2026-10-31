// SPDX-License-Identifier: Apache-2.0

// The writer of another process for integration_test/composer_ax_insert_test.dart
// (FIX-macos-ax-text-insert): it writes text into the focused text field of a running app
// through the Accessibility API, as dictation and text tools do (AXUIElementSetAttributeValue
// with kAXSelectedTextAttribute). tool/run_composer_ax_insert_test.sh builds and starts it.
//
//   ax_write_probe check
//     exit 0 when this process may use the Accessibility API, else 1.
//   ax_write_probe watch <marker> <text> <seconds>
//     polls the focused element of every regular app until one shows exactly <marker> as its
//     value, then sets <text> as its selected text and exits 0; exit 1 after <seconds>.

import AppKit
import ApplicationServices

func attribute(_ element: AXUIElement, _ name: String) -> AnyObject? {
  var value: AnyObject?
  guard AXUIElementCopyAttributeValue(element, name as CFString, &value) == .success else {
    return nil
  }
  return value
}

/// The focused element of the app [pid] when its value is [marker].
func markedElement(pid: pid_t, marker: String) -> AXUIElement? {
  let app = AXUIElementCreateApplication(pid)
  AXUIElementSetMessagingTimeout(app, 0.5)
  guard let focused = attribute(app, kAXFocusedUIElementAttribute),
    CFGetTypeID(focused) == AXUIElementGetTypeID()
  else { return nil }
  let element = focused as! AXUIElement
  guard attribute(element, kAXValueAttribute) as? String == marker else { return nil }
  return element
}

let arguments = CommandLine.arguments
switch arguments.dropFirst().first {
case "check":
  exit(AXIsProcessTrusted() ? 0 : 1)
case "watch" where arguments.count == 5:
  let marker = arguments[2]
  let text = arguments[3]
  let end = Date().addingTimeInterval(TimeInterval(arguments[4]) ?? 600)
  while Date() < end {
    for app in NSWorkspace.shared.runningApplications where app.activationPolicy == .regular {
      guard let element = markedElement(pid: app.processIdentifier, marker: marker) else {
        continue
      }
      let role = attribute(element, kAXRoleAttribute) as? String ?? "?"
      let status = AXUIElementSetAttributeValue(
        element, kAXSelectedTextAttribute as CFString, text as CFString)
      let after = attribute(element, kAXValueAttribute) as? String ?? ""
      print(
        "ax_write_probe: pid \(app.processIdentifier) role \(role): set AXSelectedText "
          + "status \(status.rawValue), AXValue now \"\(after)\"")
      exit(status == .success ? 0 : 1)
    }
    Thread.sleep(forTimeInterval: 0.2)
  }
  print("ax_write_probe: no focused element showed the marker in time")
  exit(1)
default:
  print("usage: ax_write_probe check | watch <marker> <text> <seconds>")
  exit(2)
}
