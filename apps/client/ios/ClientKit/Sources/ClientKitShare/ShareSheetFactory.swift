// SPDX-License-Identifier: Apache-2.0

import ClientKit
import Foundation
import SwiftUI

/// What the extension shows: the sheet, or the note that the app must be opened first.
public enum ShareSheetContent {
    /// No shared groups or nobody signed in; the closure cancels the request.
    case unavailable(cancel: () -> Void)
    /// The sheet over its model.
    case sheet(ShareViewModel)
}

/// Wires the share sheet of the running extension (brief M5-06 Design notes 1 to 4): the live
/// environment of `ShareEnvironment.live`, the attachments of the extension's input items, the
/// loader over the App Group's staging folder and the sheet's model. The platform view
/// controllers (`ClientKitShareViewController`) only host the result.
@MainActor
public enum ShareSheetFactory {
    /// The content for the extension's `bundle` and input items.
    ///
    /// - Parameters:
    ///   - bundle: the extension's bundle (Info.plist groups, strings).
    ///   - inputItems: `extensionContext.inputItems`.
    ///   - finish: ends the request: `completeRequest` or `cancelRequest`.
    /// - Returns: the sheet, or `unavailable` when `ShareEnvironment.live` gives nothing.
    public static func content(bundle: Bundle, inputItems: [Any], finish: @escaping (ShareViewModel.Finish) -> Void) -> ShareSheetContent {
        guard let environment = ShareEnvironment.live(bundle: bundle) else {
            return .unavailable(cancel: { finish(.cancelled) })
        }
        let shared = input(of: inputItems)
        let inbox = environment.inbox
        let loader = ShareItemLoader(fileSystem: LiveStagingFileSystem(makeFolder: inbox.stagingFolder))
        return .sheet(ShareViewModel(dependencies: ShareSheetDependencies(
            settings: environment.settings, recent: environment.targets.recent(), search: environment.search,
            sender: environment.sender, load: { await loader.load(shared.attachments, texts: shared.texts) },
            discard: { inbox.removeStaged($0) }, thumbnail: ShareSheetDependencies.liveThumbnail,
            wait: ShareSheetDependencies.liveWait, finish: finish)))
    }

    /// The attachments and texts of the extension's input items, in order.
    ///
    /// - Parameter inputItems: `extensionContext.inputItems`; anything but `NSExtensionItem` is
    ///   ignored.
    /// - Returns: every attachment, and the non-empty `attributedContentText` of the items (a
    ///   page title, a post's text).
    public nonisolated static func input(of inputItems: [Any]) -> (attachments: [ShareAttachment], texts: [String]) {
        let items = inputItems.compactMap { $0 as? NSExtensionItem }
        let attachments: [ShareAttachment] = items.flatMap { ($0.attachments ?? []).map(ItemProviderAttachment.init) }
        let texts = items.compactMap { item -> String? in
            guard let text = item.attributedContentText?.string.trimmingCharacters(in: .whitespacesAndNewlines), !text.isEmpty else {
                return nil
            }
            return text
        }
        return (attachments, texts)
    }
}

/// The root view the platform controllers host.
public struct ShareSheetRoot: View {
    private let content: ShareSheetContent
    private let strings: ShareStrings

    /// Creates the root.
    ///
    /// - Parameters:
    ///   - content: the sheet or the unavailable note.
    ///   - strings: the texts.
    public init(content: ShareSheetContent, strings: ShareStrings) {
        self.content = content
        self.strings = strings
    }

    /// The sheet, which loads the items when it appears, or the note.
    public var body: some View {
        switch content {
        case let .unavailable(cancel):
            ShareUnavailableView(strings: strings, cancel: cancel)
        case let .sheet(model):
            ShareSheetView(model: model, strings: strings)
                .task { await model.load() }
        }
    }
}
