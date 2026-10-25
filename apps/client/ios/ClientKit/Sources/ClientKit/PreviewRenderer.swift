// SPDX-License-Identifier: Apache-2.0

import Foundation

/// The localized texts of a notification, from the `Localizable.strings` the generator writes
/// from the ARB files of `packages/l10n` (brief M5-04 Design note 6).
public struct PushStrings: Equatable, Sendable {
    /// The table keys; `tools/catalog/apple_strings.py` writes exactly these.
    public enum Key {
        /// "Message not available" (13 5.1, 6.4).
        public static let notAvailable = "PUSH_MESSAGE_NOT_AVAILABLE"
        /// The title format `<channel> · <thread title>`, two `%@` arguments.
        public static let previewTitle = "PUSH_PREVIEW_TITLE"
        /// The body format `<sender>: <excerpt>`, two `%@` arguments.
        public static let previewText = "PUSH_PREVIEW_TEXT"
    }

    /// "Message not available".
    public let notAvailable: String
    /// The title format with two positional `%@` arguments.
    public let titleFormat: String
    /// The body format with two positional `%@` arguments.
    public let textFormat: String

    /// Creates the texts.
    ///
    /// - Parameters:
    ///   - notAvailable: "Message not available".
    ///   - titleFormat: the title format, for example `%1$@ · %2$@`.
    ///   - textFormat: the body format, for example `%1$@: %2$@`.
    public init(notAvailable: String, titleFormat: String, textFormat: String) {
        self.notAvailable = notAvailable
        self.titleFormat = titleFormat
        self.textFormat = textFormat
    }

    /// The texts of `bundle` in the current language of the system.
    ///
    /// - Parameter bundle: the bundle that holds `Localizable.strings` (the extension's own).
    /// - Returns: the texts.
    public static func fromBundle(_ bundle: Bundle = .main) -> PushStrings {
        PushStrings(
            notAvailable: bundle.localizedString(forKey: Key.notAvailable, value: nil, table: nil),
            titleFormat: bundle.localizedString(forKey: Key.previewTitle, value: nil, table: nil),
            textFormat: bundle.localizedString(forKey: Key.previewText, value: nil, table: nil))
    }
}

/// Title and body of a notification with a preview (docs/spec/01-product.md 10.2; brief M5-04
/// Design note 4), by the same rules as the desktop toast (`toast_content.dart`).
public enum PreviewRenderer {
    /// What the extension shows for `preview`.
    ///
    /// The title is `<channel> · <thread title>`, only the channel when the thread has no
    /// title, and the placeholder title when no channel name came (a DM, or `none`). The body is
    /// `<sender>: <excerpt>`; without an excerpt (`notify.preview_mode = names`) the excerpt is
    /// the placeholder body, without a sender the body is the placeholder alone. Only members
    /// the response carries are shown.
    ///
    /// - Parameters:
    ///   - preview: the server's answer.
    ///   - placeholderTitle: the title the push brought (the localized `PUSH_TITLE`).
    ///   - placeholderBody: the body the push brought (the localized `loc-key` of the kind).
    ///   - strings: the format texts.
    /// - Returns: title and body.
    public static func render(
        _ preview: PushPreview, placeholderTitle: String, placeholderBody: String, strings: PushStrings
    ) -> (title: String, body: String) {
        let channel = present(preview.channelName)
        let thread = present(preview.threadTitle)
        let title: String
        switch (channel, thread) {
        case let (.some(c), .some(t)):
            title = format(strings.titleFormat, c, t)
        case let (.some(c), .none):
            title = c
        default:
            title = placeholderTitle
        }
        let body: String
        if let sender = present(preview.senderDisplayName) {
            body = format(strings.textFormat, sender, present(preview.excerpt) ?? placeholderBody)
        } else {
            body = placeholderBody
        }
        return (title, body)
    }

    private static func present(_ value: String?) -> String? {
        guard let value, !value.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { return nil }
        return value
    }

    /// Formats with the positional arguments `%1$@` and `%2$@` (and `%%`) in one pass, so an
    /// argument is never read as a format. A format without both arguments (a broken strings
    /// file) falls back to the plain join, so no name is lost.
    static func format(_ format: String, _ first: String, _ second: String) -> String {
        guard format.contains("%1$@"), format.contains("%2$@") else { return "\(first) \(second)" }
        var out = ""
        var rest = Substring(format)
        while let percent = rest.firstIndex(of: "%") {
            out += rest[..<percent]
            let tail = rest[percent...]
            if tail.hasPrefix("%1$@") {
                out += first
                rest = tail.dropFirst(4)
            } else if tail.hasPrefix("%2$@") {
                out += second
                rest = tail.dropFirst(4)
            } else if tail.hasPrefix("%%") {
                out += "%"
                rest = tail.dropFirst(2)
            } else {
                out += "%"
                rest = tail.dropFirst()
            }
        }
        return out + rest
    }
}
