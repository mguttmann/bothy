// SPDX-License-Identifier: Apache-2.0

import Foundation

/// Modifier keys (D-CU-02-7). `cmd` and `win` both name the Command key on macOS.
public enum Modifier: String, Hashable, Sendable {
    case shift, control, option, command

    /// Reads a modifier name of D-CU-02-7, case-insensitive.
    ///
    /// - Parameter name: `shift`, `ctrl`/`control`, `alt`/`option`,
    ///   `cmd`/`command`/`super`/`meta`, or `win`/`windows`.
    /// - Returns: the modifier, or nil for any other name.
    public static func named(_ name: String) -> Modifier? {
        switch name.lowercased() {
        case "shift": return .shift
        case "ctrl", "control": return .control
        case "alt", "option": return .option
        case "cmd", "command", "super", "meta", "win", "windows": return .command
        default: return nil
        }
    }
}

/// Named keys of D-CU-02-7 (the xdotool names of the computer tool with their aliases).
public enum NamedKey: Hashable, Sendable {
    case `return`, tab, escape, backspace, delete, insert, space
    case up, down, left, right, home, end, pageUp, pageDown, capsLock
    /// `F1` to `F24`.
    case function(Int)

    /// Reads a key name, case-insensitive.
    ///
    /// - Parameter name: a name or alias of D-CU-02-7 (`Del` is accepted for `Delete`, because
    ///   the shortcut table of CU-T15 names `ctrl+alt+del`).
    /// - Returns: the key, or nil for any other name.
    public static func named(_ name: String) -> NamedKey? {
        let lower = name.lowercased()
        switch lower {
        case "return", "enter": return .return
        case "tab": return .tab
        case "escape", "esc": return .escape
        case "backspace": return .backspace
        case "delete", "del": return .delete
        case "insert": return .insert
        case "space": return .space
        case "up": return .up
        case "down": return .down
        case "left": return .left
        case "right": return .right
        case "home": return .home
        case "end": return .end
        case "page_up", "prior": return .pageUp
        case "page_down", "next": return .pageDown
        case "capslock": return .capsLock
        default:
            if lower.hasPrefix("f"), let number = Int(lower.dropFirst()), (1...24).contains(number),
                lower.count <= 3, !lower.dropFirst().hasPrefix("0")
            {
                return .function(number)
            }
            return nil
        }
    }
}

/// One key the input engine presses.
public enum Key: Hashable, Sendable {
    /// A modifier key.
    case modifier(Modifier)
    /// A named key.
    case named(NamedKey)
    /// A key that produces this character on the current keyboard layout.
    case character(Character)
}

/// Why a `keys` string is not valid (answered with `invalid_action`).
public enum KeyNameError: Error, Equatable {
    /// The string or one of its parts is empty.
    case empty
    /// A part is neither a modifier, a named key nor a single character.
    case unknownName(String)
    /// More than one key that is not a modifier.
    case severalKeys
}

/// A key combination of `do key`: modifiers held in order, then one key pressed and released.
///
/// Example: `try KeyCombo.parse("cmd+shift+t")` gives modifiers command and shift and the
/// character key `t`.
public struct KeyCombo: Equatable, Sendable {
    /// Modifiers in the order of the string; released in reverse order.
    public var modifiers: [Modifier]
    /// The key, or nil for a combination of modifiers only (for example `shift`).
    public var key: Key?

    /// Creates a combination.
    public init(modifiers: [Modifier], key: Key?) {
        self.modifiers = modifiers
        self.key = key
    }

    /// Every key of the combination in press order.
    public var keys: [Key] {
        modifiers.map(Key.modifier) + (key.map { [$0] } ?? [])
    }

    /// Parses a `keys` string of D-CU-02-7: `+` joins the parts, names ignore case.
    ///
    /// A `+` key is written as the last part after a joining `+` (`ctrl++`) or alone (`+`).
    /// - Parameter text: the `keys` member.
    /// - Returns: the combination.
    /// - Throws: `KeyNameError` for an empty part, an unknown name or two non-modifier keys.
    public static func parse(_ text: String) throws -> KeyCombo {
        guard !text.isEmpty else { throw KeyNameError.empty }
        var parts: [String]
        if text == "+" {
            parts = ["+"]
        } else if text.hasSuffix("++") {
            parts = text.dropLast(2).components(separatedBy: "+") + ["+"]
        } else {
            parts = text.components(separatedBy: "+")
        }
        var modifiers: [Modifier] = []
        var key: Key?
        for part in parts {
            guard !part.isEmpty else { throw KeyNameError.empty }
            let next: Key
            if let modifier = Modifier.named(part) {
                // A modifier after the key ("t+shift") is still a modifier of the combination.
                modifiers.append(modifier)
                continue
            } else if let named = NamedKey.named(part) {
                next = .named(named)
            } else if part.count == 1, let character = part.first {
                next = .character(character)
            } else {
                throw KeyNameError.unknownName(part)
            }
            guard key == nil else { throw KeyNameError.severalKeys }
            key = next
        }
        return KeyCombo(modifiers: modifiers, key: key)
    }
}
