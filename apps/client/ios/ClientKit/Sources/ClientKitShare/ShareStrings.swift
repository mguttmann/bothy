// SPDX-License-Identifier: Apache-2.0

import ClientKit
import Foundation

/// The texts of the share sheet (brief M5-06 rule 8; M5-04 Design note 6).
///
/// The texts have one source, the ARB files of `packages/l10n`: `tools/catalog/apple_strings.py`
/// writes them as `SHARE_*` keys into the `Localizable.strings` files of `ios/Localizations`,
/// which the Share Extensions carry as resources (`make apple-strings`). Every argument is passed
/// as a text (`%1$@`, numbers formatted before), as the generator writes every placeholder. A
/// plural becomes two keys, `_ONE` for a count of 1 and `_OTHER` otherwise, which is right for
/// the app's languages.
///
/// Example:
///
///     let strings = ShareStrings.fromBundle(Bundle(for: ShareViewController.self))
///     Text(strings.sheetTitle)
public struct ShareStrings {
    /// Every native key the sheet reads; a test checks each against the generated files.
    public static let keys = [
        "SHARE_SHEET_TITLE", "SHARE_CANCEL", "SHARE_SEND", "SHARE_COMMENT", "SHARE_POST_TITLE", "SHARE_TARGETS",
        "SHARE_SEARCH", "SHARE_WRITE_ONLY", "SHARE_NEW_POST", "SHARE_MORE_FILES_ONE", "SHARE_MORE_FILES_OTHER",
        "SHARE_FILE_TOO_LARGE", "SHARE_TOO_MANY_FILES", "SHARE_NO_TARGET", "SHARE_QUEUED", "SHARE_REFUSED",
        "SHARE_UNAVAILABLE", "SHARE_LOAD_FAILED", "SHARE_REASON_FORBIDDEN", "SHARE_REASON_TOO_LARGE",
        "SHARE_REASON_TYPE", "SHARE_REASON_GONE", "SHARE_REASON_SAVE", "SHARE_REASON_STATUS",
    ]

    private let lookup: (String) -> String

    /// Creates the texts over a lookup.
    ///
    /// - Parameter lookup: returns the text or format of a native key.
    public init(lookup: @escaping (String) -> String) {
        self.lookup = lookup
    }

    /// The texts of `bundle`'s `Localizable.strings` in the person's language.
    ///
    /// - Parameter bundle: the Share Extension's bundle.
    /// - Returns: the texts.
    public static func fromBundle(_ bundle: Bundle) -> ShareStrings {
        ShareStrings(lookup: { bundle.localizedString(forKey: $0, value: nil, table: nil) })
    }

    /// "Share to", the title between Cancel and Send.
    public var sheetTitle: String { lookup("SHARE_SHEET_TITLE") }
    /// "Cancel".
    public var cancel: String { lookup("SHARE_CANCEL") }
    /// "Send".
    public var send: String { lookup("SHARE_SEND") }
    /// The label of the comment field.
    public var comment: String { lookup("SHARE_COMMENT") }
    /// The label of the title field of a new forum post.
    public var postTitle: String { lookup("SHARE_POST_TITLE") }
    /// The heading of the target list.
    public var targets: String { lookup("SHARE_TARGETS") }
    /// The hint of the search field.
    public var search: String { lookup("SHARE_SEARCH") }
    /// "Only targets with write permission."
    public var writeOnly: String { lookup("SHARE_WRITE_ONLY") }
    /// Shown when the list has no target.
    public var noTarget: String { lookup("SHARE_NO_TARGET") }
    /// Shown briefly when the share went to the inbox.
    public var queued: String { lookup("SHARE_QUEUED") }
    /// Shown instead of the sheet when the extension cannot send (no shared groups, nobody
    /// signed in).
    public var unavailable: String { lookup("SHARE_UNAVAILABLE") }

    /// A forum target: its path with the "(new post)" suffix.
    ///
    /// - Parameter path: the channel's display path.
    /// - Returns: the label.
    public func newPost(_ path: String) -> String {
        format("SHARE_NEW_POST", path)
    }

    /// The label of a target in the list: the display path, for a new post with its suffix.
    ///
    /// - Parameter target: the target.
    /// - Returns: the label.
    public func targetLabel(_ target: ShareTarget) -> String {
        target.kind == .newPost ? newPost(target.path) : target.path
    }

    /// The line under the preview when more files are shared.
    ///
    /// - Parameter count: the number of files besides the first.
    /// - Returns: the line.
    public func moreFiles(_ count: Int) -> String {
        format(count == 1 ? "SHARE_MORE_FILES_ONE" : "SHARE_MORE_FILES_OTHER", String(count))
    }

    /// A file size in the person's locale, for example "1.2 MB".
    ///
    /// - Parameter bytes: the size.
    /// - Returns: the text.
    public func size(_ bytes: Int64) -> String {
        ByteCountFormatter.string(fromByteCount: bytes, countStyle: .file)
    }

    /// Why Send is disabled.
    ///
    /// - Parameter issue: the problem of the shared items.
    /// - Returns: the text.
    public func issue(_ issue: ShareIssue) -> String {
        switch issue {
        case .loadFailed:
            return lookup("SHARE_LOAD_FAILED")
        case let .tooMany(limit):
            return format("SHARE_TOO_MANY_FILES", String(limit))
        case let .tooLarge(name, limit):
            return format("SHARE_FILE_TOO_LARGE", name, size(limit))
        }
    }

    /// "Not sent: <reason>" for a refusal of the server or of the inbox.
    ///
    /// - Parameter refusal: the status and problem code.
    /// - Returns: the text.
    public func refusal(_ refusal: ShareRefusal) -> String {
        format("SHARE_REFUSED", reason(refusal))
    }

    /// The reason of a refusal by its status (Design note 4): 403 no write permission, 413 too
    /// large, 415 type not allowed, 404 and 410 the target is gone, 0 the inbox could not be
    /// written, every other status by its number.
    private func reason(_ refusal: ShareRefusal) -> String {
        switch refusal.status {
        case 403:
            return lookup("SHARE_REASON_FORBIDDEN")
        case 413:
            return lookup("SHARE_REASON_TOO_LARGE")
        case 415:
            return lookup("SHARE_REASON_TYPE")
        case 404, 410:
            return lookup("SHARE_REASON_GONE")
        case 0:
            return lookup("SHARE_REASON_SAVE")
        default:
            return format("SHARE_REASON_STATUS", String(refusal.status))
        }
    }

    private func format(_ key: String, _ arguments: String...) -> String {
        String(format: lookup(key), arguments: arguments)
    }
}
