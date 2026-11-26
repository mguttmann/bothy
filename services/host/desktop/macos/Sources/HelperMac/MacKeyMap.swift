// SPDX-License-Identifier: Apache-2.0

import Carbon.HIToolbox
import CoreGraphics
import Foundation
import HelperCore

/// Virtual key codes of macOS for the keys of D-CU-02-7.
///
/// Characters are looked up in the current keyboard layout first, so `cmd+z` presses the key
/// that types `z` on that layout (on a German layout that is not the key of the US `z`); the US
/// table is the fallback for a layout that cannot be read.
enum MacKeyMap {
    /// The key code of a modifier; the left-hand key of each pair.
    static func code(of modifier: Modifier) -> CGKeyCode {
        switch modifier {
        case .shift: return CGKeyCode(kVK_Shift)
        case .control: return CGKeyCode(kVK_Control)
        case .option: return CGKeyCode(kVK_Option)
        case .command: return CGKeyCode(kVK_Command)
        }
    }

    /// The event flag a held modifier adds to every later event.
    static func flag(of modifier: Modifier) -> CGEventFlags {
        switch modifier {
        case .shift: return .maskShift
        case .control: return .maskControl
        case .option: return .maskAlternate
        case .command: return .maskCommand
        }
    }

    /// Every modifier key code, both hands, for the release after a crash.
    static let allModifierCodes: [CGKeyCode] = [
        kVK_Shift, kVK_RightShift, kVK_Control, kVK_RightControl, kVK_Option, kVK_RightOption,
        kVK_Command, kVK_RightCommand, kVK_Function, kVK_CapsLock,
    ].map { CGKeyCode($0) }

    /// The key code of a named key, nil where a Mac keyboard has none (`Insert`, `F21` to `F24`).
    static func code(of key: NamedKey) -> CGKeyCode? {
        switch key {
        case .return: return CGKeyCode(kVK_Return)
        case .tab: return CGKeyCode(kVK_Tab)
        case .escape: return CGKeyCode(kVK_Escape)
        case .backspace: return CGKeyCode(kVK_Delete)
        case .delete: return CGKeyCode(kVK_ForwardDelete)
        case .insert: return nil
        case .space: return CGKeyCode(kVK_Space)
        case .up: return CGKeyCode(kVK_UpArrow)
        case .down: return CGKeyCode(kVK_DownArrow)
        case .left: return CGKeyCode(kVK_LeftArrow)
        case .right: return CGKeyCode(kVK_RightArrow)
        case .home: return CGKeyCode(kVK_Home)
        case .end: return CGKeyCode(kVK_End)
        case .pageUp: return CGKeyCode(kVK_PageUp)
        case .pageDown: return CGKeyCode(kVK_PageDown)
        case .capsLock: return CGKeyCode(kVK_CapsLock)
        case .function(let number):
            let codes = [
                kVK_F1, kVK_F2, kVK_F3, kVK_F4, kVK_F5, kVK_F6, kVK_F7, kVK_F8, kVK_F9, kVK_F10,
                kVK_F11, kVK_F12, kVK_F13, kVK_F14, kVK_F15, kVK_F16, kVK_F17, kVK_F18, kVK_F19,
                kVK_F20,
            ]
            return (1...codes.count).contains(number) ? CGKeyCode(codes[number - 1]) : nil
        }
    }

    /// The key code and whether Shift is needed for a character on the current layout.
    static func code(of character: Character) -> (code: CGKeyCode, shift: Bool)? {
        // The Text Input Sources calls must run on the main thread on current macOS.
        let layout = MainThread.sync { currentLayoutTable() }
        if let hit = layout[character] { return hit }
        let lower = Character(String(character).lowercased())
        if lower != character, let hit = layout[lower] { return (hit.code, true) }
        if let code = usTable[character] { return (code, false) }
        if let code = usTable[lower] { return (code, true) }
        return nil
    }

    /// Characters of the current keyboard layout without and with Shift, read with
    /// `UCKeyTranslate` for every key code.
    private static func currentLayoutTable() -> [Character: (code: CGKeyCode, shift: Bool)] {
        guard let source = TISCopyCurrentKeyboardLayoutInputSource()?.takeRetainedValue(),
            let pointer = TISGetInputSourceProperty(source, kTISPropertyUnicodeKeyLayoutData)
        else { return [:] }
        let data = Unmanaged<CFData>.fromOpaque(pointer).takeUnretainedValue() as Data
        var table: [Character: (code: CGKeyCode, shift: Bool)] = [:]
        data.withUnsafeBytes { raw in
            guard let layout = raw.baseAddress?.assumingMemoryBound(to: UCKeyboardLayout.self)
            else { return }
            for shift in [false, true] {
                let modifiers: UInt32 = shift ? UInt32(shiftKey >> 8) & 0xFF : 0
                for code in 0..<128 {
                    var deadKeys: UInt32 = 0
                    var length = 0
                    var chars = [UniChar](repeating: 0, count: 4)
                    let status = UCKeyTranslate(
                        layout, UInt16(code), UInt16(kUCKeyActionDown), modifiers,
                        UInt32(LMGetKbdType()), OptionBits(kUCKeyTranslateNoDeadKeysBit),
                        &deadKeys, chars.count, &length, &chars)
                    guard status == noErr, length > 0,
                        let character = String(utf16CodeUnits: chars, count: length).first
                    else { continue }
                    // Keep the first key found and prefer the unshifted one.
                    if table[character] == nil { table[character] = (CGKeyCode(code), shift) }
                }
            }
        }
        return table
    }

    /// The US ANSI layout, used when the current layout cannot be read.
    private static let usTable: [Character: CGKeyCode] = {
        let pairs: [(Character, Int)] = [
            ("a", kVK_ANSI_A), ("b", kVK_ANSI_B), ("c", kVK_ANSI_C), ("d", kVK_ANSI_D),
            ("e", kVK_ANSI_E), ("f", kVK_ANSI_F), ("g", kVK_ANSI_G), ("h", kVK_ANSI_H),
            ("i", kVK_ANSI_I), ("j", kVK_ANSI_J), ("k", kVK_ANSI_K), ("l", kVK_ANSI_L),
            ("m", kVK_ANSI_M), ("n", kVK_ANSI_N), ("o", kVK_ANSI_O), ("p", kVK_ANSI_P),
            ("q", kVK_ANSI_Q), ("r", kVK_ANSI_R), ("s", kVK_ANSI_S), ("t", kVK_ANSI_T),
            ("u", kVK_ANSI_U), ("v", kVK_ANSI_V), ("w", kVK_ANSI_W), ("x", kVK_ANSI_X),
            ("y", kVK_ANSI_Y), ("z", kVK_ANSI_Z), ("0", kVK_ANSI_0), ("1", kVK_ANSI_1),
            ("2", kVK_ANSI_2), ("3", kVK_ANSI_3), ("4", kVK_ANSI_4), ("5", kVK_ANSI_5),
            ("6", kVK_ANSI_6), ("7", kVK_ANSI_7), ("8", kVK_ANSI_8), ("9", kVK_ANSI_9),
            ("-", kVK_ANSI_Minus), ("=", kVK_ANSI_Equal), ("[", kVK_ANSI_LeftBracket),
            ("]", kVK_ANSI_RightBracket), ("\\", kVK_ANSI_Backslash), (";", kVK_ANSI_Semicolon),
            ("'", kVK_ANSI_Quote), (",", kVK_ANSI_Comma), (".", kVK_ANSI_Period),
            ("/", kVK_ANSI_Slash), ("`", kVK_ANSI_Grave), (" ", kVK_Space),
        ]
        return Dictionary(pairs.map { ($0.0, CGKeyCode($0.1)) }, uniquingKeysWith: { first, _ in first })
    }()
}
