// SPDX-License-Identifier: Apache-2.0

import ApplicationServices
import CoreGraphics
import Foundation
import HelperCore

/// Synthetic input with `CGEvent` (27 9.1).
///
/// Every event comes from the helper's own private event source and carries
/// `MacInjector.marker` in `eventSourceUserData`, so `MacEscTap` lets an Esc the helper sends
/// pass and swallows only the real one (27 8.4 item 4). The injector tracks what it holds down
/// so `releaseAll` can let go of it; at start it also releases whatever the session's combined
/// state reports as held, because a crashed earlier run may have left a key down.
public final class MacInjector: Injector {
    /// The value in `eventSourceUserData` of every event the helper posts. Any value works; this
    /// one spells a short ASCII tag so it is recognizable in an event trace.
    public static let marker: Int64 = 0x0042_4F54_4859_4355

    private let source: CGEventSource?
    private var heldKeys: [Key: CGKeyCode] = [:]
    private var heldButtons: Set<MouseButton> = []
    private var flags: CGEventFlags = []

    /// Creates the injector with a private event source, whose state is the helper's alone.
    public init() {
        source = CGEventSource(stateID: .privateState)
    }

    /// The state ID of the helper's event source; the Esc tap compares it as a second sign.
    public var sourceStateID: Int64? {
        source.map { Int64($0.sourceStateID.rawValue) }
    }

    public func permissionGranted() -> Bool {
        AXIsProcessTrusted()
    }

    public func supports(_ key: Key) -> Bool {
        switch key {
        case .modifier: return true
        case .named(let named): return MacKeyMap.code(of: named) != nil
        // A character without a key on the layout is typed as Unicode input.
        case .character: return true
        }
    }

    public func keyDown(_ key: Key) throws {
        switch key {
        case .modifier(let modifier):
            flags.insert(MacKeyMap.flag(of: modifier))
            try postKey(MacKeyMap.code(of: modifier), down: true, type: .flagsChanged)
            heldKeys[key] = MacKeyMap.code(of: modifier)
        case .named(let named):
            guard let code = MacKeyMap.code(of: named) else { throw InjectionError.noKey }
            try postKey(code, down: true)
            heldKeys[key] = code
        case .character(let character):
            if let hit = MacKeyMap.code(of: character) {
                try postKey(hit.code, down: true, extraFlags: hit.shift ? .maskShift : [])
                heldKeys[key] = hit.code
            } else {
                try postUnicode(String(character), down: true)
            }
        }
    }

    public func keyUp(_ key: Key) throws {
        switch key {
        case .modifier(let modifier):
            flags.remove(MacKeyMap.flag(of: modifier))
            try postKey(MacKeyMap.code(of: modifier), down: false, type: .flagsChanged)
        case .named, .character:
            if let code = heldKeys[key] {
                let shift: CGEventFlags =
                    if case .character(let character) = key, MacKeyMap.code(of: character)?.shift == true {
                        .maskShift
                    } else { [] }
                try postKey(code, down: false, extraFlags: shift)
            } else if case .character(let character) = key {
                try postUnicode(String(character), down: false)
            }
        }
        heldKeys[key] = nil
    }

    public func move(to point: Point) throws {
        let type: CGEventType = heldButtons.contains(.left) ? .leftMouseDragged : .mouseMoved
        try postMouse(type, button: .left, at: point, clickCount: 0)
    }

    public func mouseDown(_ button: MouseButton, at point: Point, clickCount: Int) throws {
        let type: CGEventType =
            switch button {
            case .left: .leftMouseDown
            case .right: .rightMouseDown
            case .middle: .otherMouseDown
            }
        try postMouse(type, button: button, at: point, clickCount: clickCount)
        heldButtons.insert(button)
    }

    public func mouseUp(_ button: MouseButton, at point: Point, clickCount: Int) throws {
        let type: CGEventType =
            switch button {
            case .left: .leftMouseUp
            case .right: .rightMouseUp
            case .middle: .otherMouseUp
            }
        try postMouse(type, button: button, at: point, clickCount: clickCount)
        heldButtons.remove(button)
    }

    public func scroll(at point: Point, direction: ScrollDirection, lines: Int) throws {
        // Positive wheel1 scrolls up, positive wheel2 scrolls left.
        let amount = Int32(clamping: lines)
        let vertical: Int32 = direction == .up ? amount : direction == .down ? -amount : 0
        let horizontal: Int32 = direction == .left ? amount : direction == .right ? -amount : 0
        guard
            let event = CGEvent(
                scrollWheelEvent2Source: source, units: .line, wheelCount: 2, wheel1: vertical,
                wheel2: horizontal, wheel3: 0)
        else { throw InjectionError.eventNotCreated }
        event.location = CGPoint(x: point.x, y: point.y)
        post(event)
    }

    public func typeText(_ text: String) throws {
        // A keyboard event carries a short Unicode string; split on character boundaries so no
        // surrogate pair or combined character is cut in half.
        var chunk = ""
        for character in text {
            if chunk.utf16.count + String(character).utf16.count > 16 {
                try postUnicode(chunk, down: true)
                try postUnicode(chunk, down: false)
                chunk = ""
            }
            chunk.append(character)
        }
        if !chunk.isEmpty {
            try postUnicode(chunk, down: true)
            try postUnicode(chunk, down: false)
        }
    }

    public func cursorPosition() throws -> Point {
        guard let event = CGEvent(source: nil) else { throw InjectionError.eventNotCreated }
        return Point(x: event.location.x, y: event.location.y)
    }

    public func releaseAll() {
        let location = CGEvent(source: nil)?.location ?? .zero
        let point = Point(x: location.x, y: location.y)
        let buttons: [(MouseButton, CGMouseButton)] = [(.left, .left), (.right, .right), (.middle, .center)]
        for (button, cgButton) in buttons
        where heldButtons.contains(button)
            || CGEventSource.buttonState(.combinedSessionState, button: cgButton)
        {
            try? mouseUp(button, at: point, clickCount: 1)
        }
        heldButtons.removeAll()
        for (key, code) in heldKeys {
            if case .modifier = key { continue }
            try? postKey(code, down: false)
        }
        heldKeys.removeAll()
        flags = []
        for code in MacKeyMap.allModifierCodes
        where CGEventSource.keyState(.combinedSessionState, key: code) {
            try? postKey(code, down: false, type: .flagsChanged)
        }
        // Keys other than modifiers that a crashed run left down.
        for code in CGKeyCode(0)..<CGKeyCode(128)
        where !MacKeyMap.allModifierCodes.contains(code)
            && CGEventSource.keyState(.combinedSessionState, key: code)
        {
            try? postKey(code, down: false)
        }
    }

    // MARK: Posting

    private func postKey(
        _ code: CGKeyCode, down: Bool, type: CGEventType? = nil, extraFlags: CGEventFlags = []
    ) throws {
        guard let event = CGEvent(keyboardEventSource: source, virtualKey: code, keyDown: down)
        else { throw InjectionError.eventNotCreated }
        if let type { event.type = type }
        event.flags = flags.union(extraFlags)
        post(event)
    }

    private func postUnicode(_ text: String, down: Bool) throws {
        guard let event = CGEvent(keyboardEventSource: source, virtualKey: 0, keyDown: down)
        else { throw InjectionError.eventNotCreated }
        let units = Array(text.utf16)
        event.keyboardSetUnicodeString(stringLength: units.count, unicodeString: units)
        event.flags = flags
        post(event)
    }

    private func postMouse(_ type: CGEventType, button: MouseButton, at point: Point, clickCount: Int)
        throws
    {
        let cgButton: CGMouseButton =
            switch button {
            case .left: .left
            case .right: .right
            case .middle: .center
            }
        guard
            let event = CGEvent(
                mouseEventSource: source, mouseType: type,
                mouseCursorPosition: CGPoint(x: point.x, y: point.y), mouseButton: cgButton)
        else { throw InjectionError.eventNotCreated }
        if clickCount > 0 { event.setIntegerValueField(.mouseEventClickState, value: Int64(clickCount)) }
        event.flags = flags
        post(event)
    }

    private func post(_ event: CGEvent) {
        event.setIntegerValueField(.eventSourceUserData, value: Self.marker)
        // The HID tap is upstream of the session tap, so MacEscTap sees the injected Esc and
        // can let it pass.
        event.post(tap: .cghidEventTap)
    }
}

/// Errors of the injector; the controller answers them with `os_refused`.
public enum InjectionError: Error {
    /// The operating system did not create the event.
    case eventNotCreated
    /// The key has no key code on this platform.
    case noKey
}
