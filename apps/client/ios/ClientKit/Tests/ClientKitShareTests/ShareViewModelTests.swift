// SPDX-License-Identifier: Apache-2.0
//
// The state and logic of the share sheet (brief M5-06 Design notes 1, 2 and 4; 01 5.3 M6):
// Send only with a target, the recent targets newest first and capped, the debounced search,
// the instance limits, and what each outcome of the send does to the sheet.

import ClientKit
import CoreGraphics
import Foundation
import Testing

@testable import ClientKitShare

/// What the sheet asked its host to do, and which staged files it discarded.
final class SheetHost {
    var finished: [ShareViewModel.Finish] = []
    var discarded: [[ShareItem]] = []
}

@MainActor
private func model(
    settings: ShareSettings = settings(), recent: [ShareTarget] = [threadTarget, forumTarget, channelTarget],
    search: TargetSearching? = FakeSearch(answers: [:]), sender: ShareSending = FakeSender(outcome: .sent),
    loaded: ShareLoadResult = ShareLoadResult(items: [item("photo.jpg", size: 1000)], text: "", failures: 0),
    gate: Gate = { let gate = Gate(); gate.open(); return gate }(), host: SheetHost = SheetHost(),
    thumbnail: @escaping (ShareItem) async -> CGImage? = { _ in nil }
) -> ShareViewModel {
    ShareViewModel(dependencies: ShareSheetDependencies(
        settings: settings, recent: recent, search: search, sender: sender, load: { loaded },
        discard: { host.discarded.append($0) }, thumbnail: thumbnail, wait: { await gate.wait($0) },
        finish: { host.finished.append($0) }))
}

@MainActor
@Test func testSendDisabledUntilATargetIsChosen() async {
    let vm = model()
    #expect(!vm.canSend)
    await vm.load()
    #expect(vm.items.count == 1 && !vm.loading)
    #expect(!vm.canSend)
    vm.select(threadTarget)
    #expect(vm.selected == threadTarget)
    #expect(vm.canSend)
}

@MainActor
@Test func testSendDisabledWhileLoadingAndWithoutContent() async {
    let vm = model(loaded: ShareLoadResult(items: [], text: "", failures: 0))
    vm.select(threadTarget)
    // Still loading: nothing to send yet.
    #expect(vm.loading && !vm.canSend)
    await vm.load()
    // Neither a file nor text.
    #expect(!vm.canSend)
    vm.comment = "  "
    #expect(!vm.canSend)
    vm.comment = "See error above"
    #expect(vm.canSend)
}

@MainActor
@Test func testRecentTargetsNewestFirstAndCapped() async {
    let a = ShareTarget(kind: .thread, channelID: "c", threadID: "a", path: "x > A")
    let b = ShareTarget(kind: .thread, channelID: "c", threadID: "b", path: "x > B")
    let c = ShareTarget(kind: .thread, channelID: "c", threadID: "c", path: "x > C")
    // `client.share.recent_targets` as the app wrote it caps what is shown, in the stored order.
    let vm = model(settings: settings(recentMax: 2), recent: [c, a, b])
    #expect(vm.recent == [c, a])
    #expect(vm.shownTargets == [c, a])
    #expect(model(settings: settings(recentMax: 0), recent: [a]).shownTargets.isEmpty)
}

@MainActor
@Test func testSearchIsDebouncedAndShowsResults() async {
    let hit = ShareTarget(kind: .thread, channelID: "c-9", threadID: "t-9", path: "server > Gateway: retry")
    let search = FakeSearch(answers: ["gate": [hit, channelTarget]])
    let gate = Gate()
    let vm = model(search: search, gate: gate)

    vm.query = "g"
    vm.query = "gate"
    await gate.reached(2)
    // Both keystrokes wait; nothing has asked the server yet.
    #expect(gate.waits == [.searchDebounce, .searchDebounce])
    #expect(search.queries.isEmpty)
    #expect(vm.searching)
    gate.open()
    await vm.searchTask?.value
    // Only the last text was searched.
    #expect(search.queries == ["gate"])
    #expect(vm.results == [hit, channelTarget])
    #expect(vm.shownTargets == [hit, channelTarget])
    #expect(!vm.searching)

    // Clearing the search shows the recent targets again, without asking the server.
    vm.query = ""
    #expect(vm.shownTargets == vm.recent)
    #expect(search.queries == ["gate"])
}

@MainActor
@Test func testSearchFailureAndNoSessionShowNoResults() async {
    let failing = model(search: FakeSearch(answers: [:], failing: true))
    failing.query = "x"
    await failing.searchTask?.value
    #expect(failing.results.isEmpty && !failing.searching && failing.shownTargets.isEmpty)

    // Without a child session there is no search; the recent targets stay.
    let offline = model(search: nil)
    #expect(!offline.canSearch)
    offline.query = "x"
    #expect(offline.searchTask == nil)
    #expect(offline.shownTargets.isEmpty)
}

@MainActor
@Test func testLimitsOfTheInstanceDisableSend() async {
    let files = [item("a.jpg", size: 10), item("b.mov", size: 2000), item("c.pdf", size: 50)]
    let loaded = ShareLoadResult(items: files, text: "", failures: 0)

    // `file_max_upload_bytes` and `message_max_attachments` unknown: the server checks.
    let open = model(settings: settings(), loaded: loaded)
    await open.load()
    open.select(threadTarget)
    #expect(open.issues.isEmpty && open.canSend)

    let limited = model(settings: settings(maxUploadBytes: 1000, maxAttachments: 2), loaded: loaded)
    await limited.load()
    limited.select(threadTarget)
    #expect(limited.issues == [.tooMany(limit: 2), .tooLarge(name: "b.mov", limit: 1000)])
    #expect(!limited.canSend)
    #expect(testStrings.issue(.tooMany(limit: 2)) == "at most 2")
    #expect(testStrings.issue(.tooLarge(name: "b.mov", limit: 1000)).hasPrefix("b.mov over "))

    // Exactly at the limits is allowed.
    let exact = model(settings: settings(maxUploadBytes: 2000, maxAttachments: 3), loaded: loaded)
    await exact.load()
    exact.select(threadTarget)
    #expect(exact.issues.isEmpty && exact.canSend)
}

@MainActor
@Test func testUnreadableItemDisablesSend() async {
    let vm = model(loaded: ShareLoadResult(items: [item("a.jpg", size: 1)], text: "", failures: 1))
    await vm.load()
    vm.select(threadTarget)
    #expect(vm.issues == [.loadFailed])
    #expect(!vm.canSend)
}

@MainActor
@Test func testSharedTextFillsTheComment() async {
    let vm = model(loaded: ShareLoadResult(items: [], text: "https://chat.example.com/docs", failures: 0))
    await vm.load()
    #expect(vm.comment == "https://chat.example.com/docs")
}

@MainActor
@Test func testNewPostTitlePrefill() async {
    let fromComment = model(loaded: ShareLoadResult(items: [item("diagram.png", size: 5)], text: "", failures: 0))
    await fromComment.load()
    fromComment.comment = "\n  Login fails on step two  \nmore details"
    #expect(!fromComment.showsTitle)
    fromComment.select(forumTarget)
    #expect(fromComment.showsTitle)
    #expect(fromComment.title == "Login fails on step two")

    let fromFile = model(loaded: ShareLoadResult(items: [item("diagram.png", size: 5)], text: "", failures: 0))
    await fromFile.load()
    fromFile.select(forumTarget)
    #expect(fromFile.title == "diagram.png")
    // A title the person typed is kept when the target changes back and forth.
    fromFile.title = "Diagram for Robin"
    fromFile.select(threadTarget)
    fromFile.select(forumTarget)
    #expect(fromFile.title == "Diagram for Robin")
    // A new post needs a title.
    fromFile.title = " "
    #expect(!fromFile.canSend)
}

@MainActor
@Test func testSentClosesTheSheet() async {
    let sender = FakeSender(outcome: .sent)
    let host = SheetHost()
    let vm = model(sender: sender, host: host)
    await vm.load()
    vm.select(forumTarget)
    vm.comment = "For Sam"
    vm.title = "Photo"
    await vm.send()
    #expect(sender.sent == [ShareDraft(target: forumTarget, comment: "For Sam", title: "Photo", items: [item("photo.jpg", size: 1000)])])
    #expect(host.finished == [.completed])
    #expect(host.discarded.isEmpty)
}

@MainActor
@Test func testThreadDraftHasNoTitle() async {
    let sender = FakeSender(outcome: .sent)
    let vm = model(sender: sender)
    await vm.load()
    vm.select(forumTarget)
    vm.select(threadTarget)
    await vm.send()
    #expect(sender.sent.first?.title == nil)
    #expect(sender.sent.first?.target == threadTarget)
}

@MainActor
@Test func testSendDisabledWhileSending() async {
    let hold = Gate()
    let sender = FakeSender(outcome: .sent, hold: hold)
    let host = SheetHost()
    let vm = model(sender: sender, host: host)
    await vm.load()
    vm.select(threadTarget)
    let sending = Task { await vm.send() }
    await hold.reached(1)
    #expect(vm.sending)
    #expect(!vm.canSend && !vm.canCancel)
    // A second tap does nothing.
    await vm.send()
    #expect(sender.sent.count == 1)
    hold.open()
    await sending.value
    #expect(host.finished == [.completed])
}

@MainActor
@Test func testQueuedShowsTheNoticeThenCloses() async {
    let gate = Gate()
    let host = SheetHost()
    let vm = model(sender: FakeSender(outcome: .queued), gate: gate, host: host)
    await vm.load()
    vm.select(threadTarget)
    let sending = Task { await vm.send() }
    await gate.reached(1)
    #expect(gate.waits == [.queuedNotice])
    #expect(vm.notice == .queued)
    #expect(host.finished.isEmpty)
    gate.open()
    await sending.value
    #expect(host.finished == [.completed])
}

@MainActor
@Test func testRefusedShowsTheReasonAndKeepsTheSheetOpen() async {
    let host = SheetHost()
    let vm = model(sender: FakeSender(outcome: .refused(status: 403, code: "forbidden")), host: host)
    await vm.load()
    vm.select(threadTarget)
    await vm.send()
    #expect(vm.refusal == ShareRefusal(status: 403, code: "forbidden"))
    #expect(host.finished.isEmpty)
    #expect(!vm.sending)
    // The send discarded the staged files: they cannot go out again from this sheet.
    #expect(!vm.canSend)
    #expect(vm.canCancel)
    #expect(testStrings.refusal(vm.refusal!) == "Not sent: SHARE_REASON_FORBIDDEN")

    // A text-only share can try another target.
    let text = model(sender: FakeSender(outcome: .refused(status: 404, code: "not_found")),
                     loaded: ShareLoadResult(items: [], text: "Link", failures: 0))
    await text.load()
    text.select(threadTarget)
    await text.send()
    #expect(text.refusal?.status == 404)
    text.select(channelTarget)
    #expect(text.refusal == nil)
    #expect(text.canSend)
}

@Test func testRefusalReasons() {
    let cases: [(Int, String)] = [
        (403, "Not sent: SHARE_REASON_FORBIDDEN"), (413, "Not sent: SHARE_REASON_TOO_LARGE"),
        (415, "Not sent: SHARE_REASON_TYPE"), (404, "Not sent: SHARE_REASON_GONE"), (410, "Not sent: SHARE_REASON_GONE"),
        (0, "Not sent: SHARE_REASON_SAVE"), (422, "Not sent: error 422"), (409, "Not sent: error 409"),
    ]
    for (status, text) in cases {
        #expect(testStrings.refusal(ShareRefusal(status: status, code: nil)) == text)
    }
}

@MainActor
@Test func testCancelDiscardsTheStagedFiles() async {
    let host = SheetHost()
    let vm = model(host: host)
    await vm.load()
    vm.cancel()
    #expect(host.finished == [.cancelled])
    #expect(host.discarded == [[item("photo.jpg", size: 1000)]])
    // The request ends once.
    vm.cancel()
    await vm.send()
    #expect(host.finished == [.cancelled])
}

@MainActor
@Test func testThumbnailOfTheFirstImage() async {
    let image = patternImage(width: 4, height: 4)
    var asked: [String] = []
    let vm = model(
        loaded: ShareLoadResult(items: [item("a.jpg", size: 1), item("b.pdf", size: 1, mime: "application/pdf")], text: "", failures: 0),
        thumbnail: { asked.append($0.name); return image })
    await vm.load()
    #expect(asked == ["a.jpg"])
    #expect(vm.thumbnail != nil)
    #expect(vm.preview == item("a.jpg", size: 1))
    #expect(vm.moreCount == 1)

    let pdf = model(loaded: ShareLoadResult(items: [item("b.pdf", size: 1, mime: "application/pdf")], text: "", failures: 0),
                    thumbnail: { _ in Issue.record("no thumbnail for a PDF"); return nil })
    await pdf.load()
    #expect(pdf.thumbnail == nil)
}

@Test func testStringsOfTheSheet() {
    #expect(testStrings.newPost("requests") == "requests (new post)")
    #expect(testStrings.moreFiles(1) == "and 1 more file")
    #expect(testStrings.moreFiles(3) == "and 3 more files")
    #expect(testStrings.targetLabel(forumTarget) == "requests (new post)")
    #expect(testStrings.targetLabel(threadTarget) == "server > Gateway: resume")
    #expect(testStrings.sheetTitle == "SHARE_SHEET_TITLE")
}

@Test func testEveryKeyIsInTheGeneratedStrings() throws {
    // The native strings come from the ARB files through tools/catalog/apple_strings.py.
    let localizations = URL(fileURLWithPath: #filePath)
        .deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        .deletingLastPathComponent().appendingPathComponent("Localizations")
    for language in ["en", "de"] {
        let file = localizations.appendingPathComponent("\(language).lproj/Localizable.strings")
        let table = try #require(NSDictionary(contentsOf: file) as? [String: String], "\(file.path)")
        for key in ShareStrings.keys {
            #expect(table[key] != nil, "\(language): \(key)")
        }
        let strings = ShareStrings(lookup: { table[$0] ?? "" })
        #expect(strings.refusal(ShareRefusal(status: 418, code: nil)).contains("418"))
        #expect(strings.moreFiles(2).contains("2"))
    }
}
