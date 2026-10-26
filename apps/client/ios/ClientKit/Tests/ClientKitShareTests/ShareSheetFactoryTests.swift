// SPDX-License-Identifier: Apache-2.0
//
// The wiring of the sheet in the extension (brief M5-06 Design note 1): without the shared
// groups or before the app signed in, the sheet only explains that the app must be opened.

import Foundation
import Testing

@testable import ClientKitShare

@MainActor
@Test func testWithoutSharedGroupsTheSheetIsUnavailable() {
    var finished: [ShareViewModel.Finish] = []
    // The test bundle has no `ClientKitAppGroup`, like a build without a signing team.
    let content = ShareSheetFactory.content(bundle: Bundle(for: SheetHost.self), inputItems: [], finish: { finished.append($0) })
    guard case .unavailable(let cancel) = content else {
        Issue.record("expected the unavailable sheet")
        return
    }
    cancel()
    #expect(finished == [.cancelled])
}

@Test func testAttachmentsOfTheExtensionItems() {
    let first = NSExtensionItem()
    first.attachments = [NSItemProvider(item: "a" as NSString, typeIdentifier: "public.plain-text")]
    first.attributedContentText = NSAttributedString(string: "Page title")
    let second = NSExtensionItem()
    second.attachments = [
        NSItemProvider(item: "b" as NSString, typeIdentifier: "public.plain-text"),
        NSItemProvider(item: "c" as NSString, typeIdentifier: "public.plain-text"),
    ]
    let empty = NSExtensionItem()
    empty.attributedContentText = NSAttributedString(string: "  ")
    let input = ShareSheetFactory.input(of: [first, second, empty, "not an item"])
    #expect(input.attachments.count == 3)
    #expect(input.texts == ["Page title"])
}
