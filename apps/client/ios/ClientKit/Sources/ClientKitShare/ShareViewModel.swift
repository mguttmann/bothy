// SPDX-License-Identifier: Apache-2.0

import ClientKit
import CoreGraphics
import Foundation

/// Why Send is disabled although a target is chosen (brief M5-06 Design note 2).
public enum ShareIssue: Equatable, Sendable {
    /// An attachment could not be loaded; sending the rest would drop it unseen.
    case loadFailed
    /// More files than `message_max_attachments` allows.
    case tooMany(limit: Int)
    /// A file is larger than `file_max_upload_bytes`.
    case tooLarge(name: String, limit: Int64)
}

/// A refusal shown in the sheet: the HTTP status and the problem `code` (Design note 4), or
/// status 0 when the inbox could not be written.
public struct ShareRefusal: Equatable, Sendable {
    /// The HTTP status, or 0.
    public let status: Int
    /// The problem `code`, or nil.
    public let code: String?

    /// Creates the refusal.
    ///
    /// - Parameters:
    ///   - status: the HTTP status, or 0.
    ///   - code: the problem `code`, or nil.
    public init(status: Int, code: String?) {
        self.status = status
        self.code = code
    }
}

/// What the sheet needs from the outside; the extension wires the live pieces
/// (`ShareSheetFactory`), the tests fakes.
public struct ShareSheetDependencies {
    /// The settings the app wrote: `client.share.recent_targets` and the instance limits.
    public var settings: ShareSettings
    /// The recent targets of `TargetStore`, newest first.
    public var recent: [ShareTarget]
    /// The search, or nil without a child session.
    public var search: TargetSearching?
    /// The send.
    public var sender: ShareSending
    /// Loads the shared items into the staging folder.
    public var load: () async -> ShareLoadResult
    /// Removes staged copies the person did not send (Cancel).
    public var discard: ([ShareItem]) -> Void
    /// The preview of an image item, or nil.
    public var thumbnail: (ShareItem) async -> CGImage?
    /// Waits before a search starts and while the queued notice shows; the tests open a gate
    /// instead of letting time pass.
    public var wait: (ShareViewModel.Wait) async -> Void
    /// Ends the extension request.
    public var finish: (ShareViewModel.Finish) -> Void

    /// Creates the dependencies.
    ///
    /// - Parameters: the fields of the same names.
    public init(
        settings: ShareSettings, recent: [ShareTarget], search: TargetSearching?, sender: ShareSending,
        load: @escaping () async -> ShareLoadResult, discard: @escaping ([ShareItem]) -> Void,
        thumbnail: @escaping (ShareItem) async -> CGImage?, wait: @escaping (ShareViewModel.Wait) async -> Void,
        finish: @escaping (ShareViewModel.Finish) -> Void
    ) {
        self.settings = settings
        self.recent = recent
        self.search = search
        self.sender = sender
        self.load = load
        self.discard = discard
        self.thumbnail = thumbnail
        self.wait = wait
        self.finish = finish
    }

    /// How long a search waits for the next keystroke; a feel of the interface, not a limit.
    public static let searchDebounceNanoseconds: UInt64 = 300_000_000
    /// How long "Saved; the app sends it later" stays before the sheet closes; a feel of the
    /// interface, not a limit.
    public static let queuedNoticeNanoseconds: UInt64 = 1_500_000_000
    /// The longer side of the preview thumbnail in pixels; a display size, not a limit.
    public static let thumbnailPixels = 240

    /// The extension's wait: a sleep that ends early when its task is cancelled (a newer
    /// keystroke).
    public static let liveWait: (ShareViewModel.Wait) async -> Void = { kind in
        let nanoseconds = kind == .searchDebounce ? searchDebounceNanoseconds : queuedNoticeNanoseconds
        try? await Task.sleep(nanoseconds: nanoseconds)
    }

    /// The extension's thumbnail: ImageIO's subsampled decode, off the main thread.
    public static let liveThumbnail: (ShareItem) async -> CGImage? = { item in
        await Task.detached(priority: .userInitiated) { ShareThumbnail.make(item.fileURL, maxPixelSize: thumbnailPixels) }.value
    }
}

/// The state and logic of the share sheet (01 5.3 M6; brief M5-06 Design notes 1, 2 and 4); the
/// SwiftUI views only show it.
///
/// The target list shows the recent targets (newest first, at most `client.share.recent_targets`)
/// or, while the person types, the results of the search, which starts after a short pause and
/// only for the last text. Send is enabled with a chosen target, something to send (a file or
/// text), a title for a new forum post, while not sending, and while every item is within
/// `file_max_upload_bytes` and their number within `message_max_attachments` (both from the
/// settings the app wrote; unknown limits are left to the server). The sheet shows only what the
/// server delivered and what the person used before; it never shows a target's messages and
/// adds no permission logic: a send into a target that lost the right is refused by the server
/// and the reason is shown (15 1 principle 1).
///
/// Outcomes: `sent` closes the sheet; `queued` shows that the app sends it later, then closes;
/// `refused` shows the reason and keeps the sheet open (the send removed the staged files, so a
/// share with files can only be cancelled; a text-only share may pick another target).
///
/// Example:
///
///     let model = ShareViewModel(dependencies: dependencies)
///     await model.load()
///     model.select(target)
///     await model.send()
@MainActor
public final class ShareViewModel: ObservableObject {
    /// What the sheet waits for.
    public enum Wait: Equatable, Sendable {
        /// The pause after a keystroke before the search starts.
        case searchDebounce
        /// The time the queued notice shows.
        case queuedNotice
    }

    /// How the sheet ends the extension request.
    public enum Finish: Equatable, Sendable {
        /// Sent or in the inbox: `completeRequest`.
        case completed
        /// Cancelled: `cancelRequest`.
        case cancelled
    }

    /// A notice shown before the sheet closes.
    public enum Notice: Equatable, Sendable {
        /// The share is in the inbox; the app sends it later.
        case queued
    }

    /// Whether the items are still loading.
    @Published public private(set) var loading = true
    /// The staged items.
    @Published public private(set) var items: [ShareItem] = []
    /// The comment, the message text; the shared text and links fill it first.
    @Published public var comment = ""
    /// The title of a new forum post.
    @Published public var title = ""
    /// The search text.
    @Published public var query = "" {
        didSet {
            if query != oldValue { scheduleSearch() }
        }
    }
    /// The search results of the last text.
    @Published public private(set) var results: [ShareTarget] = []
    /// Whether a search is pending or running.
    @Published public private(set) var searching = false
    /// The chosen target.
    @Published public private(set) var selected: ShareTarget?
    /// Whether the send runs.
    @Published public private(set) var sending = false
    /// The refusal of the last send, shown until another target is chosen.
    @Published public private(set) var refusal: ShareRefusal?
    /// The notice before the sheet closes.
    @Published public private(set) var notice: Notice?
    /// The preview of the first item when it is an image.
    @Published public private(set) var thumbnail: CGImage?

    /// The recent targets shown, newest first, at most `client.share.recent_targets`.
    public let recent: [ShareTarget]

    private let dependencies: ShareSheetDependencies
    private var failures = 0
    private var loadStarted = false
    private var finished = false
    /// The send removed the staged files after a refusal.
    private var itemsGone = false
    /// The pending search; the tests await it.
    private(set) var searchTask: Task<Void, Never>?

    /// Creates the sheet's model.
    ///
    /// - Parameter dependencies: the live pieces or fakes.
    public init(dependencies: ShareSheetDependencies) {
        self.dependencies = dependencies
        recent = Array(dependencies.recent.prefix(max(0, dependencies.settings.recentMax)))
    }

    /// The targets the list shows: the recent ones, or the search results while there is text.
    public var shownTargets: [ShareTarget] {
        Self.trimmed(query).isEmpty ? recent : results
    }

    /// Whether the search field is offered: only with a child session.
    public var canSearch: Bool { dependencies.search != nil }

    /// Whether the chosen target is a new forum post, which needs a title.
    public var showsTitle: Bool { selected?.kind == .newPost }

    /// The first item, shown in the preview.
    public var preview: ShareItem? { items.first }

    /// The number of items besides the first.
    public var moreCount: Int { max(0, items.count - 1) }

    /// Why Send is disabled, from the items and the instance limits; empty when nothing blocks.
    public var issues: [ShareIssue] {
        var issues: [ShareIssue] = []
        if failures > 0 { issues.append(.loadFailed) }
        if let limit = dependencies.settings.maxAttachments, items.count > limit {
            issues.append(.tooMany(limit: limit))
        }
        if let limit = dependencies.settings.maxUploadBytes {
            issues += items.filter { $0.sizeBytes > limit }.map { .tooLarge(name: $0.name, limit: limit) }
        }
        return issues
    }

    /// Whether Send is enabled.
    public var canSend: Bool {
        guard let selected, !loading, !sending, !finished, !itemsGone, issues.isEmpty else { return false }
        guard !items.isEmpty || !Self.trimmed(comment).isEmpty else { return false }
        return selected.kind != .newPost || !Self.trimmed(title).isEmpty
    }

    /// Whether Cancel is enabled: not while sending, so a send is never cut off halfway.
    public var canCancel: Bool { !sending && !finished }

    /// Loads the shared items once, fills the comment with the shared text and makes the
    /// preview of a first image.
    public func load() async {
        guard !loadStarted else { return }
        loadStarted = true
        let result = await dependencies.load()
        guard !finished else {
            // Cancelled while loading: nothing may stay in the staging folder.
            if !result.items.isEmpty { dependencies.discard(result.items) }
            return
        }
        items = result.items
        failures = result.failures
        if comment.isEmpty { comment = result.text }
        loading = false
        if let first = items.first, first.mimeType.hasPrefix("image/") {
            thumbnail = await dependencies.thumbnail(first)
        }
    }

    /// Chooses `target`; a new post gets a title from the comment's first line or the first
    /// file's name unless the person typed one.
    ///
    /// - Parameter target: a target of the list.
    public func select(_ target: ShareTarget) {
        selected = target
        refusal = nil
        if target.kind == .newPost, Self.trimmed(title).isEmpty {
            title = Self.suggestedTitle(comment: comment, items: items)
        }
    }

    /// Sends to the chosen target and ends the request on `sent` and `queued`.
    public func send() async {
        guard canSend, let target = selected else { return }
        sending = true
        refusal = nil
        let draft = ShareDraft(
            target: target, comment: comment, title: target.kind == .newPost ? Self.trimmed(title) : nil, items: items)
        switch await dependencies.sender.send(draft) {
        case .sent:
            finish(.completed)
        case .queued:
            notice = .queued
            await dependencies.wait(.queuedNotice)
            finish(.completed)
        case let .refused(status, code):
            refusal = ShareRefusal(status: status, code: code)
            itemsGone = !items.isEmpty
            sending = false
        }
    }

    /// Cancels: the staged copies are removed and the request is cancelled.
    public func cancel() {
        guard canCancel else { return }
        searchTask?.cancel()
        if !items.isEmpty { dependencies.discard(items) }
        finish(.cancelled)
    }

    private func finish(_ how: Finish) {
        guard !finished else { return }
        finished = true
        dependencies.finish(how)
    }

    private func scheduleSearch() {
        searchTask?.cancel()
        searchTask = nil
        let text = Self.trimmed(query)
        guard !text.isEmpty, let search = dependencies.search else {
            results = []
            searching = false
            return
        }
        searching = true
        let wait = dependencies.wait
        searchTask = Task { [weak self] in
            await wait(.searchDebounce)
            guard !Task.isCancelled else { return }
            let found = (try? await search.search(text)) ?? []
            guard !Task.isCancelled, let self else { return }
            self.results = found
            self.searching = false
        }
    }

    private static func suggestedTitle(comment: String, items: [ShareItem]) -> String {
        let firstLine = comment.split(whereSeparator: \.isNewline).lazy.map { trimmed(String($0)) }.first { !$0.isEmpty }
        return firstLine ?? items.first?.name ?? ""
    }

    private static func trimmed(_ text: String) -> String {
        text.trimmingCharacters(in: .whitespacesAndNewlines)
    }
}
