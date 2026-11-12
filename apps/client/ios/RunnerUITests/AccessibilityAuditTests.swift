// SPDX-License-Identifier: Apache-2.0
//
// The accessibility audit of Xcode on the phone screens (brief M5-10 Design note 3; 01 8): each
// test starts the audit entry of the app (integration_test/a11y_audit/audit_app.dart, built by
// tool/run_ios_a11y_audit.sh) with one screen and runs `performAccessibilityAudit` with every
// audit type on the iOS simulator. An issue the audit reports fails the test, unless it is a
// known limitation of the Flutter engine listed in `knownLimitations` with its reason (and in
// docs/quality/accessibility-mobile.md); a listed limitation is never ignored silently, the
// test prints every issue it lets pass. An audit that XCTest ends at its own time limit has
// no verdict about the screen: the screen starts again and is audited once more
// (`AuditTimeLimit`, docs/decisions.d/FIX-MAIN-RED-1705.md).

import XCTest

/// The time limit XCTest gives one audit. It ends a longer audit with the error "Audit failed
/// to complete in time" and reports neither that the screen is clean nor that it is not. The
/// first start of the freshly installed app on a loaded runner can take that long.
enum AuditTimeLimit {
  /// Domain and code of that error.
  static let domain = "com.apple.xcode.xctest.accessibilityAudit"
  static let code = -56

  /// Whether [error] is the error of XCTest for an audit it ended at its time limit.
  static func ended(_ error: Error) -> Bool {
    let error = error as NSError
    return error.domain == domain && error.code == code
  }

  /// Runs [audit] and, when XCTest ended it at its time limit, once more. Only an audit
  /// without a verdict is repeated: every other error and a second time limit are thrown, and
  /// an audit that completed is never run again.
  static func completing(_ screen: String, _ audit: () throws -> Void) throws {
    do {
      try audit()
    } catch let error where ended(error) {
      print("\(screen): XCTest ended the audit at its time limit; the screen is audited once more")
      try audit()
    }
  }
}

final class AccessibilityAuditTests: XCTestCase {
  /// The upper bound for the first frame of a screen: the debug build of the audit entry
  /// starts from its kernel file, which takes long on a loaded runner.
  private static let patience: TimeInterval = 300

  /// Issues of the audit that come from the Flutter engine, not from the app, with the reason
  /// and where the rule is checked instead (docs/quality/accessibility-mobile.md, section
  /// "Platform audits: known limitations"). Each one matches by the audit type and the element,
  /// never by the audit type alone.
  @available(iOS 17.0, *)
  private static let knownLimitations:
    [(type: XCUIAccessibilityAuditType, matches: (XCUIElement?) -> Bool, reason: String)] = [
      (
        .sufficientElementDescription,
        { $0?.elementType == .scrollView && ($0?.label.isEmpty ?? false) },
        "the engine presents every scrollable list as a scroll view of its own, which Flutter "
          + "gives no name; VoiceOver reads the items of the list, which the widget tests name"
      ),
    ]

  override func setUpWithError() throws {
    continueAfterFailure = false
  }

  /// Starts the audit entry with [screen], waits until [text] is on the screen and runs the
  /// audit with every audit type; fails on every issue that is no known limitation.
  private func audit(_ screen: String, showing text: String) throws {
    let issues = try findings(screen, showing: text)
    XCTAssertTrue(issues.isEmpty, issues.joined(separator: "\n"))
  }

  /// The issues of the audit on [screen] that are no known limitation.
  private func findings(_ screen: String, showing text: String) throws -> [String] {
    guard #available(iOS 17.0, *) else {
      // A skip would let the job pass without an audit: an older simulator is a failure.
      XCTFail("performAccessibilityAudit needs an iOS 17 simulator or later")
      return []
    }
    // The issues of every audit of the screen count, also those an audit reported before
    // XCTest ended it at its time limit.
    var issues: [String] = []
    try AuditTimeLimit.completing(screen) {
      let app = XCUIApplication()
      // The engine reads its initial route from the process arguments.
      app.launchArguments = ["--route=/\(screen)"]
      app.launch()
      defer { app.terminate() }
      let shown = app.descendants(matching: .any)
        .matching(NSPredicate(format: "label CONTAINS %@", text)).firstMatch
      XCTAssertTrue(
        shown.waitForExistence(timeout: Self.patience),
        "\(screen): \"\(text)\" did not appear"
      )
      try app.performAccessibilityAudit(for: .all) { issue in
        let element = issue.element.map { "\($0.elementType) \"\($0.label)\" at \($0.frame)" } ?? "no element"
        let line = "\(screen): \(issue.compactDescription) (\(element)): \(issue.detailedDescription)"
        if let known = Self.knownLimitations.first(where: {
          $0.type == issue.auditType && $0.matches(issue.element)
        }) {
          print("\(line): known limitation: \(known.reason)")
        } else if !issues.contains(line) {
          issues.append(line)
        }
        // Every issue is handled here: the test decides with the list.
        return true
      }
    }
    return issues
  }

  /// The audit finds the gaps of a screen made to have them (a tappable square of 20 by 20
  /// without a name), so a green screen is no empty run. The size of that square and the grey
  /// text on grey of the screen are no issue of the audit on Flutter elements (measured with
  /// Xcode 27); the widget tests check targets and contrast (docs/quality/
  /// accessibility-mobile.md, section "Platform audits: known limitations").
  func testTheAuditFindsKnownGaps() throws {
    let issues = try findings("known_gaps", showing: "Audit self-check")
    print(issues.joined(separator: "\n"))
    XCTAssertTrue(
      issues.contains { $0.contains("Element has no description") },
      "no issue on the screen with known gaps"
    )
  }

  /// The error of XCTest for an audit it ended at its time limit, as the log of the CI run
  /// shows it in which the first audit of the job did not complete.
  private static let timeLimit = NSError(
    domain: "com.apple.xcode.xctest.accessibilityAudit",
    code: -56,
    userInfo: [NSLocalizedDescriptionKey: "Audit failed to complete in time"]
  )

  func testAnAuditEndedAtTheTimeLimitRunsOnceMore() throws {
    var audits = 0
    try AuditTimeLimit.completing("screen") {
      audits += 1
      if audits == 1 { throw Self.timeLimit }
    }
    XCTAssertEqual(audits, 2)
  }

  func testASecondTimeLimitFailsTheAudit() {
    var audits = 0
    XCTAssertThrowsError(
      try AuditTimeLimit.completing("screen") {
        audits += 1
        throw Self.timeLimit
      }
    ) { XCTAssertTrue(AuditTimeLimit.ended($0)) }
    XCTAssertEqual(audits, 2)
  }

  func testAnotherErrorOfAnAuditFailsWithoutASecondAudit() {
    var audits = 0
    let other = NSError(domain: "com.apple.xcode.xctest.accessibilityAudit", code: -1)
    XCTAssertThrowsError(
      try AuditTimeLimit.completing("screen") {
        audits += 1
        throw other
      }
    ) { XCTAssertFalse(AuditTimeLimit.ended($0)) }
    XCTAssertEqual(audits, 1)
  }

  func testACompletedAuditRunsOnce() throws {
    var audits = 0
    try AuditTimeLimit.completing("screen") { audits += 1 }
    XCTAssertEqual(audits, 1)
  }

  func testM1Spaces() throws { try audit("m1_spaces", showing: "Jump to") }

  func testM2PostList() throws { try audit("m2_post_list", showing: "New post") }

  func testM3HistoryRunning() throws { try audit("m3_running", showing: "Allow once") }

  func testM3HistoryAfterRun() throws {
    try audit("m3_collapsed", showing: "Please include expired threads.")
  }

  func testM4LargeComponents() throws {
    try audit("m4_components", showing: "Back to requests")
  }

  func testM5Recording() throws { try audit("m5_recording", showing: "Send voice message") }

  func testM6ShareSheet() throws { try audit("m6_share_sheet", showing: "Share to") }

  func testM7Offline() throws { try audit("m7_offline", showing: "Offline") }

  func testM8Terminal() throws { try audit("m8_terminal", showing: "Reattach") }

  func testM9Hosts() throws { try audit("m9_hosts", showing: "Build host") }

  func testM11RunningNow() throws { try audit("m11_running", showing: "Running now") }

  func testM12RunGraph() throws { try audit("m12_run_graph", showing: "Run graph") }

  func testE1ServerAddress() throws { try audit("e1_server", showing: "Connect") }

  func testE2Login() throws { try audit("e2_login", showing: "Log in to") }

  func testE4PendingDevice() throws { try audit("e4_pending", showing: "Code") }

  func testE5Pairing() throws { try audit("e5_pairing", showing: "Code") }

  func testE6AppLock() throws { try audit("e6_lock", showing: "Unlock") }

  func testE6AppLockUnavailable() throws {
    try audit("e6_lock_unavailable", showing: "Check again")
  }

  func testE7SignedOut() throws { try audit("e7_signed_out", showing: "You are logged out") }
}
