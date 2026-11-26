import Foundation
import HelperCore
import XCTest

/// Hello, enablement and generation (27 section 4, brief item 15, D-CU-02-4).
final class EnablementTests: XCTestCase {
    func testHelloReportsEveryMember() throws {
        let rig = Rig(enabled: false, generation: 0, connect: false)
        rig.injector.granted = false
        rig.controller.channelOpened(rig.channel)
        XCTAssertEqual(
            rig.channel.messages.first,
            [
                "op": "hello", "enabled": false, "generation": 0, "os_account": "alex",
                "permissions": ["capture": "granted", "input": "denied"],
                "helper_version": "1.2.3", "signature": "development", "locked": false,
            ])
    }

    func testSwitchOnByClickRaisesTheGenerationAndPersists() {
        let rig = Rig(enabled: false, generation: 0)
        rig.controller.toggleEnablementByClick()
        XCTAssertEqual(rig.store.saves, [EnablementState(enabled: true, generation: 1)])
        XCTAssertEqual(rig.last, ["op": "state", "enabled": true, "generation": 1])
        rig.controller.toggleEnablementByClick()
        XCTAssertEqual(rig.store.state, EnablementState(enabled: false, generation: 1))
        XCTAssertEqual(rig.last, ["op": "state", "enabled": false])
        rig.controller.toggleEnablementByClick()
        XCTAssertEqual(rig.store.state, EnablementState(enabled: true, generation: 2))
        XCTAssertEqual(rig.indicator.lastStatus?.enabled, true)
        XCTAssertEqual(rig.indicator.lastStatus?.generation, 2)
    }

    func testSwitchOnThatCannotBePersistedStaysOff() {
        let rig = Rig(enabled: false, generation: 4)
        rig.store.failSave = true
        rig.controller.toggleEnablementByClick()
        XCTAssertFalse(rig.controller.status.enabled)
        XCTAssertEqual(rig.controller.status.generation, 4)
    }

    func testDisableFromTheHostSwitchesOffAndEndsTheSession() {
        let rig = Rig(enabled: true, generation: 3)
        rig.show()
        rig.host(["op": "disable", "generation": 3])
        XCTAssertEqual(rig.store.state, EnablementState(enabled: false, generation: 3))
        XCTAssertNil(rig.indicator.shownAgent)
        XCTAssertFalse(rig.tap.running)
        XCTAssertFalse(rig.waker.awake)
        XCTAssertEqual(rig.last, ["op": "state", "enabled": false])
        // Only a click switches on again; the host has no op for it.
        rig.show()
        XCTAssertNil(rig.indicator.shownAgent)
    }

    func testLocalSwitchOffEndsTheSessionWithoutStopFrame() {
        let rig = Rig()
        rig.show()
        rig.controller.toggleEnablementByClick()
        XCTAssertNil(rig.indicator.shownAgent)
        XCTAssertFalse(rig.channel.messages.contains { $0["op"] == "stop" })
        XCTAssertEqual(rig.last, ["op": "state", "enabled": false])
        XCTAssertLessThan(
            try XCTUnwrap(rig.journal.index(of: "indicator.hide")),
            try XCTUnwrap(rig.journal.entries.lastIndex(of: "send.state")))
    }

    func testPermissionChangeIsReportedAsState() {
        let rig = Rig()
        rig.injector.granted = false
        rig.controller.refreshStatus()
        XCTAssertEqual(
            rig.last, ["op": "state", "permissions": ["capture": "granted", "input": "denied"]])
        let count = rig.channel.messages.count
        rig.controller.refreshStatus()
        XCTAssertEqual(rig.channel.messages.count, count, "no state without a change")
    }

    func testFileStoreKeepsSwitchAndGenerationPrivately() throws {
        let base = FileManager.default.temporaryDirectory
            .appendingPathComponent("helper-store-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: base) }
        let folder = base.appendingPathComponent("bothy-computer-helper")
        let store = FileEnablementStore(folder: folder)
        XCTAssertEqual(store.load(), EnablementState(enabled: false, generation: 0))
        try store.save(EnablementState(enabled: true, generation: 7))
        XCTAssertEqual(store.load(), EnablementState(enabled: true, generation: 7))
        let file = folder.appendingPathComponent("state.json")
        let fileMode = try FileManager.default.attributesOfItem(atPath: file.path)[.posixPermissions]
        let folderMode = try FileManager.default.attributesOfItem(atPath: folder.path)[
            .posixPermissions]
        XCTAssertEqual((fileMode as? NSNumber)?.intValue, 0o600)
        XCTAssertEqual((folderMode as? NSNumber)?.intValue, 0o700)
        XCTAssertEqual(
            try JSONValue.parse(Data(contentsOf: file)),
            ["v": 1, "enabled": true, "generation": 7])
        try Data("not json".utf8).write(to: file)
        XCTAssertEqual(store.load(), EnablementState(enabled: false, generation: 0))
        let leftovers = try FileManager.default.contentsOfDirectory(atPath: folder.path)
        XCTAssertEqual(leftovers, ["state.json"])
    }
}

/// `show`, `hide`, the stop paths and the release of every key and button (brief items 16, 17,
/// CU-T14, CU-T16 part, D-CU-02-6).
final class ShowHideTests: XCTestCase {
    func testShowStartsIndicatorTapAndWaker() {
        let rig = Rig()
        rig.show(agent: "Builder")
        XCTAssertEqual(rig.indicator.shownAgent, "Builder")
        XCTAssertTrue(rig.tap.running)
        XCTAssertTrue(rig.waker.awake)
        XCTAssertEqual(rig.controller.status.agentName, "Builder")
    }

    func testHideUndoesAllThreeAndReleases() {
        let rig = Rig()
        rig.show()
        let before = rig.injector.releaseAllCount
        rig.host(["op": "hide", "session_id": .string(Rig.sessionID)])
        XCTAssertNil(rig.indicator.shownAgent)
        XCTAssertFalse(rig.tap.running)
        XCTAssertFalse(rig.waker.awake)
        XCTAssertGreaterThan(rig.injector.releaseAllCount, before)
        XCTAssertNil(rig.controller.status.agentName)
    }

    func testHideOfAnotherSessionKeepsTheCurrentOne() {
        let rig = Rig()
        rig.show()
        rig.host(["op": "hide", "session_id": "0192f3a4-0000-7000-8000-000000000000"])
        XCTAssertEqual(rig.indicator.shownAgent, "Builder")
    }

    func testShowWhileDisabledShowsNothingAndReportsDisabled() {
        let rig = Rig(enabled: false, generation: 2)
        rig.show()
        XCTAssertNil(rig.indicator.shownAgent)
        XCTAssertFalse(rig.tap.running)
        XCTAssertFalse(rig.waker.awake)
        XCTAssertEqual(rig.last, ["op": "state", "enabled": false])
    }

    func testRealEscEndsTheSessionAndIsSwallowed() throws {
        let rig = Rig()
        rig.show()
        XCTAssertEqual(rig.tap.press(injected: false), .swallow)
        XCTAssertEqual(
            rig.last,
            ["op": "stop", "session_id": .string(Rig.sessionID), "detail": "local_escape"])
        XCTAssertNil(rig.indicator.shownAgent)
        XCTAssertFalse(rig.tap.running)
        XCTAssertFalse(rig.waker.awake)
        let stop = try XCTUnwrap(rig.journal.index(of: "send.stop"))
        XCTAssertLessThan(try XCTUnwrap(rig.journal.entries.lastIndex(of: "injector.releaseAll")), stop)
        XCTAssertLessThan(try XCTUnwrap(rig.journal.entries.lastIndex(of: "indicator.hide")), stop)
        XCTAssertLessThan(try XCTUnwrap(rig.journal.entries.lastIndex(of: "waker.allowSleep")), stop)
    }

    func testInjectedEscPassesAndStopsNothing() {
        let rig = Rig()
        rig.show()
        var decisions: [EscDecision] = []
        rig.injector.onKeyDown = { key in
            if key == .named(.escape) { decisions.append(rig.tap.press(injected: true)) }
        }
        rig.host(["op": "do", "seq": 5, "action": "key", "keys": "Escape"])
        XCTAssertEqual(decisions, [.pass])
        XCTAssertEqual(rig.last?["code"], "ok")
        XCTAssertEqual(rig.indicator.shownAgent, "Builder")
        XCTAssertTrue(rig.tap.running)
        XCTAssertFalse(rig.channel.messages.contains { $0["op"] == "stop" })
        XCTAssertEqual(HelperController.escDecision(injected: true), .pass)
        XCTAssertEqual(HelperController.escDecision(injected: false), .swallow)
    }

    func testStopButtonEndsTheSession() {
        let rig = Rig()
        rig.show()
        rig.indicator.clickStop()
        XCTAssertEqual(
            rig.last,
            ["op": "stop", "session_id": .string(Rig.sessionID), "detail": "indicator_stop"])
        XCTAssertNil(rig.indicator.shownAgent)
        XCTAssertFalse(rig.waker.awake)
    }

    func testBrokenChannelEndsTheSessionAndReleases() {
        let rig = Rig()
        rig.show()
        rig.injector.pressedButtons.insert(.left)
        rig.controller.channelBroken()
        XCTAssertNil(rig.indicator.shownAgent)
        XCTAssertFalse(rig.tap.running)
        XCTAssertFalse(rig.waker.awake)
        XCTAssertTrue(rig.injector.pressedButtons.isEmpty)
        XCTAssertTrue(rig.injector.pressedKeys.isEmpty)
    }

    func testStartReleasesAndUndoesEverything() {
        let rig = Rig(connect: false)
        XCTAssertEqual(rig.injector.releaseAllCount, 1)
        for entry in ["indicator.hide", "tap.stop", "waker.allowSleep", "injector.releaseAll"] {
            XCTAssertNotNil(rig.journal.index(of: entry), entry)
        }
        XCTAssertTrue(rig.channel.messages.isEmpty)
    }

    func testReleaseOpReleasesNow() {
        let rig = Rig()
        rig.show()
        rig.injector.pressedKeys.insert(.modifier(.shift))
        rig.host(["op": "release"])
        XCTAssertTrue(rig.injector.pressedKeys.isEmpty)
    }

    func testRunnerSendsHelloAndReleasesWhenTheChannelBreaks() throws {
        let rig = Rig(connect: false)
        let codec = FrameCodec(maxFrameBytes: HostConfig.defaultFrameMaxBytes)
        rig.channel.incoming = [
            try codec.encode(
                .message([
                    "op": "show", "session_id": .string(Rig.sessionID), "agent_name": "Builder",
                ]))
        ]
        let runner = ChannelRunner(
            channel: rig.channel, controller: rig.controller,
            frameMaxBytes: HostConfig.defaultFrameMaxBytes, perform: { $0() })
        runner.runOnce()
        XCTAssertEqual(rig.channel.messages.first?["op"], "hello")
        XCTAssertNotNil(rig.journal.index(of: "indicator.show"))
        XCTAssertNil(rig.indicator.shownAgent, "the broken channel hid it again")
        XCTAssertEqual(rig.channel.closed, 1)
    }

    func testRunnerClosesOnAFrameOverTheLimit() {
        let rig = Rig(connect: false)
        rig.channel.incoming = [Data([0, 0, 0, 200, 1])]
        let runner = ChannelRunner(
            channel: rig.channel, controller: rig.controller, frameMaxBytes: 100,
            perform: { $0() })
        runner.runOnce()
        XCTAssertEqual(rig.channel.closed, 1)
    }
}

/// Lock and secure desktop gating (brief item 20, CU-T13 part).
final class LockTests: XCTestCase {
    func testLockIsReportedAndReleasesInput() {
        let rig = Rig()
        rig.show()
        let before = rig.injector.releaseAllCount
        rig.lock.set(locked: true)
        XCTAssertEqual(rig.last, ["op": "state", "locked": true])
        XCTAssertGreaterThan(rig.injector.releaseAllCount, before)
        XCTAssertEqual(rig.indicator.shownAgent, "Builder", "a pause keeps the indicator")
        XCTAssertTrue(rig.waker.awake)
        rig.lock.set(locked: false)
        XCTAssertEqual(rig.last, ["op": "state", "locked": false])
    }

    func testActionsWhileLockedAnswerScreenLockedAndInjectNothing() {
        let rig = Rig()
        rig.show()
        rig.lock.set(locked: true)
        rig.host(["op": "do", "seq": 3, "action": "left_click", "x": 10, "y": 20])
        XCTAssertEqual(rig.last?["code"], "screen_locked")
        XCTAssertEqual(rig.last?["seq"], 3)
        XCTAssertEqual(
            rig.last?["screen"], ["x": 0, "y": 0, "width": 1512, "height": 982, "scale": 2])
        rig.host(["op": "do", "seq": 4, "action": "key", "keys": "Return"])
        XCTAssertEqual(rig.last?["code"], "screen_locked")
        rig.host(["op": "capture", "seq": 5, "max_edge": 1280, "format": "jpeg", "quality": 80])
        XCTAssertEqual(rig.last?["code"], "screen_locked")
        XCTAssertNil(rig.last?["image"])
        XCTAssertTrue(rig.injector.events.isEmpty)
        XCTAssertTrue(rig.capturer.requests.isEmpty)
        XCTAssertFalse(rig.channel.sent.contains { if case .image = $0 { true } else { false } })
    }
}
