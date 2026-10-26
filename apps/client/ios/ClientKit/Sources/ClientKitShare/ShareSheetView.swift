// SPDX-License-Identifier: Apache-2.0

import ClientKit
import SwiftUI

/// The share sheet (01 5.3 M6; brief M5-06 Design note 1): Cancel, "Share to" and Send in the
/// header, the preview of the first item, the comment, the title of a new forum post, the target
/// list with the search, and the note "Only targets with write permission.".
///
/// The view only shows `ShareViewModel`; every control carries an accessibility identifier for
/// the device tests (`share.cancel`, `share.send`, `share.comment`, `share.title`,
/// `share.search`, `share.target.<index>`, `share.note`, `share.error`, `share.queued`).
public struct ShareSheetView: View {
    @ObservedObject private var model: ShareViewModel
    private let strings: ShareStrings

    /// Creates the sheet.
    ///
    /// - Parameters:
    ///   - model: the sheet's state.
    ///   - strings: the texts.
    public init(model: ShareViewModel, strings: ShareStrings) {
        self.model = model
        self.strings = strings
    }

    /// The sheet.
    public var body: some View {
        VStack(spacing: 0) {
            header
            Divider()
            ScrollView {
                VStack(alignment: .leading, spacing: 12) {
                    previewRow
                    commentField
                    if model.showsTitle { titleField }
                    messages
                    targetList
                }
                .padding()
            }
            Divider()
            Text(strings.writeOnly)
                .font(.footnote)
                .foregroundColor(.secondary)
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding()
                .accessibilityIdentifier("share.note")
        }
    }

    private var header: some View {
        HStack {
            Button(strings.cancel) { model.cancel() }
                .disabled(!model.canCancel)
                .accessibilityIdentifier("share.cancel")
            Spacer()
            Text(strings.sheetTitle).font(.headline)
            Spacer()
            if model.sending { ProgressView() }
            Button(strings.send) { Task { await model.send() } }
                .disabled(!model.canSend)
                .accessibilityIdentifier("share.send")
        }
        .padding()
    }

    @ViewBuilder private var previewRow: some View {
        if model.loading {
            ProgressView()
        } else if let first = model.preview {
            HStack(spacing: 12) {
                if let thumbnail = model.thumbnail {
                    Image(decorative: thumbnail, scale: 1)
                        .resizable()
                        .scaledToFill()
                        .frame(width: 48, height: 48)
                        .clipped()
                } else {
                    Image(systemName: Self.icon(first.mimeType))
                        .font(.title)
                        .frame(width: 48, height: 48)
                }
                VStack(alignment: .leading, spacing: 2) {
                    Text(first.name).lineLimit(1)
                    Text(strings.size(first.sizeBytes)).font(.caption).foregroundColor(.secondary)
                    if model.moreCount > 0 {
                        Text(strings.moreFiles(model.moreCount)).font(.caption).foregroundColor(.secondary)
                    }
                }
            }
        }
    }

    private var commentField: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(strings.comment).font(.caption).foregroundColor(.secondary)
            TextEditor(text: $model.comment)
                .frame(minHeight: 60, maxHeight: 120)
                .overlay(RoundedRectangle(cornerRadius: 6).stroke(Color.secondary.opacity(0.3)))
                .accessibilityLabel(strings.comment)
                .accessibilityIdentifier("share.comment")
        }
    }

    private var titleField: some View {
        TextField(strings.postTitle, text: $model.title)
            .textFieldStyle(.roundedBorder)
            .accessibilityIdentifier("share.title")
    }

    @ViewBuilder private var messages: some View {
        let lines = model.issues.map(strings.issue) + (model.refusal.map { [strings.refusal($0)] } ?? [])
        if !lines.isEmpty {
            Text(lines.joined(separator: "\n"))
                .foregroundColor(.red)
                .accessibilityIdentifier("share.error")
        }
        if model.notice == .queued {
            Text(strings.queued).accessibilityIdentifier("share.queued")
        }
    }

    private var targetList: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(strings.targets).font(.headline)
            if model.canSearch {
                HStack {
                    Image(systemName: "magnifyingglass").foregroundColor(.secondary)
                    TextField(strings.search, text: $model.query)
                        .accessibilityIdentifier("share.search")
                }
            }
            ForEach(Array(model.shownTargets.enumerated()), id: \.offset) { index, target in
                Button {
                    model.select(target)
                } label: {
                    HStack {
                        Image(systemName: Self.glyph(target.kind)).frame(width: 20)
                        Text(strings.targetLabel(target)).lineLimit(1)
                        Spacer()
                        if model.selected == target { Image(systemName: "checkmark") }
                    }
                    .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .accessibilityAddTraits(model.selected == target ? .isSelected : [])
                .accessibilityIdentifier("share.target.\(index)")
            }
            if model.searching {
                ProgressView()
            } else if model.shownTargets.isEmpty, !model.loading {
                Text(strings.noTarget).foregroundColor(.secondary)
            }
        }
    }

    /// The glyph of a target kind: a thread, a channel's main timeline (`#`) or a forum.
    private static func glyph(_ kind: ShareTargetKind) -> String {
        switch kind {
        case .thread: return "text.bubble"
        case .channel: return "number"
        case .newPost: return "square.and.pencil"
        }
    }

    /// The icon of an item without a thumbnail.
    private static func icon(_ mimeType: String) -> String {
        if mimeType.hasPrefix("image/") { return "photo" }
        if mimeType.hasPrefix("video/") { return "film" }
        if mimeType.hasPrefix("audio/") { return "waveform" }
        if mimeType.hasPrefix("text/") { return "doc.text" }
        return "doc"
    }
}

/// Shown instead of the sheet when the extension cannot send: the build has no shared groups or
/// the app has not signed in, so it must be opened first.
public struct ShareUnavailableView: View {
    private let strings: ShareStrings
    private let cancel: () -> Void

    /// Creates the view.
    ///
    /// - Parameters:
    ///   - strings: the texts.
    ///   - cancel: ends the request.
    public init(strings: ShareStrings, cancel: @escaping () -> Void) {
        self.strings = strings
        self.cancel = cancel
    }

    /// The view.
    public var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            HStack {
                Button(strings.cancel, action: cancel).accessibilityIdentifier("share.cancel")
                Spacer()
                Text(strings.sheetTitle).font(.headline)
                Spacer()
            }
            Text(strings.unavailable).accessibilityIdentifier("share.unavailable")
            Spacer()
        }
        .padding()
    }
}
