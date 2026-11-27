// SPDX-License-Identifier: Apache-2.0

import Foundation
import HelperCore
import XCTest

/// A stop, a lock or an end reaches a running action between its steps, and the helper bounds
/// `repeat` and `text` itself (coordinator security finding 1 on CU-02).
final class InterruptTests: XCTestCase {
    private func downCount(_ rig: Rig) -> Int {
        rig.injector.events.filter { $0 == .keyDown(.named(.down)) }.count
    }

    func testEscDuringALongRepeatAbortsReleasesAndStops() {
        let rig = Rig()
        rig.show()
        var presses = 0
        rig.injector.onKeyDown = { _ in
            presses += 1
            // The platform tap sets the flag on its own thread while the action runs.
            if presses == 3 { rig.controller.interrupt.requestStop(.localEscape) }
        }
        rig.host([
            "op": "do", "seq": 9, "action": "key", "keys": "shift+Down",
            "repeat": .int(Int64(Limits.keyRepeatMax)),
        ])
        XCTAssertLessThanOrEqual(downCount(rig), 2)
        XCTAssertTrue(rig.injector.pressedKeys.isEmpty)
        XCTAssertNil(rig.indicator.shownAgent)
        XCTAssertFalse(rig.waker.awake)
        let stops = rig.channel.messages.filter { $0["op"] == "stop" }
        XCTAssertEqual(
            stops, [["op": "stop", "session_id": .string(Rig.sessionID), "detail": "local_escape"]])
        XCTAssertEqual(rig.last?["op"], "done")
        XCTAssertEqual(rig.last?["seq"], 9)
        XCTAssertEqual(rig.last?["code"], "invalid_action")
        // The tap's own handler arrives later and must not stop twice.
        XCTAssertEqual(rig.controller.escapePressed(injected: false), .swallow)
        XCTAssertEqual(rig.channel.messages.filter { $0["op"] == "stop" }.count, 1)
    }

    func testStopButtonDuringALongRepeatAbortsAndStops() {
        let rig = Rig()
        rig.show()
        rig.injector.onKeyDown = { _ in rig.controller.interrupt.requestStop(.indicatorStop) }
        rig.host(["op": "do", "seq": 1, "action": "key", "keys": "Down", "repeat": 40])
        XCTAssertEqual(downCount(rig), 1)
        XCTAssertTrue(rig.injector.pressedKeys.isEmpty)
        XCTAssertTrue(rig.channel.messages.contains { $0["detail"] == "indicator_stop" })
    }

    func testLockDuringALongRepeatAbortsWithScreenLocked() {
        let rig = Rig()
        rig.show()
        var presses = 0
        rig.injector.onKeyDown = { _ in
            presses += 1
            if presses == 2 {
                rig.lock.locked = true
                rig.controller.interrupt.setLocked(true)
            }
        }
        rig.host(["op": "do", "seq": 4, "action": "key", "keys": "Down", "repeat": 30])
        XCTAssertEqual(downCount(rig), 2)
        XCTAssertTrue(rig.injector.pressedKeys.isEmpty)
        XCTAssertEqual(rig.last?["code"], "screen_locked")
        XCTAssertEqual(rig.indicator.shownAgent, "Builder", "a lock pauses, it does not end")
    }

    func testHideDisableAndBrokenChannelInterrupt() throws {
        let rig = Rig()
        rig.show()
        let interrupt = rig.controller.interrupt
        XCTAssertNil(interrupt.reason)
        interrupt.note(["op": "hide", "session_id": "0192f3a4-0000-7000-8000-000000000000"])
        XCTAssertNil(interrupt.reason, "a hide of another session does not interrupt")
        interrupt.note(["op": "hide", "session_id": .string(Rig.sessionID)])
        XCTAssertEqual(interrupt.reason, .ended)
        rig.host(["op": "hide", "session_id": .string(Rig.sessionID)])
        rig.host(["op": "show", "session_id": "0192f3a4-1111-7000-8000-000000000000", "agent_name": "B"])
        XCTAssertNil(interrupt.reason)
        interrupt.note(["op": "disable", "generation": 3])
        XCTAssertEqual(interrupt.reason, .ended)

        let other = Rig(connect: false)
        let codec = FrameCodec(maxFrameBytes: HostConfig.defaultFrameMaxBytes)
        other.channel.incoming = [
            try codec.encode(
                .message(["op": "show", "session_id": .string(Rig.sessionID), "agent_name": "B"]))
        ]
        ChannelRunner(
            channel: other.channel, controller: other.controller,
            frameMaxBytes: HostConfig.defaultFrameMaxBytes, perform: { $0() }
        ).runOnce()
        XCTAssertEqual(other.controller.interrupt.reason, .channelBroken)
    }

    func testEndedDuringAnActionAbortsWithoutStopFrame() {
        let rig = Rig()
        rig.show()
        rig.injector.onKeyDown = { _ in rig.controller.interrupt.requestEnd() }
        rig.host(["op": "do", "seq": 2, "action": "type", "text": "a\nb\nc"])
        XCTAssertTrue(rig.injector.pressedKeys.isEmpty)
        XCTAssertFalse(rig.injector.events.contains(.text("c")))
        XCTAssertFalse(rig.channel.messages.contains { $0["op"] == "stop" })
    }

    func testRepeatAboveTheLimitIsInvalidAndInjectsNothing() {
        let rig = Rig()
        rig.show()
        rig.host([
            "op": "do", "seq": 1, "action": "key", "keys": "Down",
            "repeat": .int(Int64(Limits.keyRepeatMax) + 1),
        ])
        XCTAssertEqual(rig.last?["code"], "invalid_action")
        rig.host(["op": "do", "seq": 2, "action": "key", "keys": "Down", "repeat": .int(Int64(Int32.max))])
        XCTAssertEqual(rig.last?["code"], "invalid_action")
        XCTAssertTrue(rig.injector.events.isEmpty)
        rig.host([
            "op": "do", "seq": 3, "action": "key", "keys": "Down",
            "repeat": .int(Int64(Limits.keyRepeatMax)),
        ])
        XCTAssertEqual(rig.last?["code"], "ok")
        XCTAssertEqual(Limits.keyRepeatMax, 50, "the default of computer.key_repeat_max (27 section 18)")
    }

    func testTextLongerThanAFrameIsInvalidAndInjectsNothing() {
        let rig = Rig(frameMaxBytes: 1024)
        rig.show()
        rig.host(["op": "do", "seq": 1, "action": "type", "text": .string(String(repeating: "a", count: 1025))])
        XCTAssertEqual(rig.last?["code"], "invalid_action")
        XCTAssertTrue(rig.injector.events.isEmpty)
        rig.host(["op": "do", "seq": 2, "action": "type", "text": .string(String(repeating: "a", count: 512))])
        XCTAssertEqual(rig.last?["code"], "ok")
    }
}

/// The host side check of the channel (coordinator security finding 2, D-CU-02-5).
final class PeerRuleTests: XCTestCase {
    static let socket = "/Library/Application Support/bothy-host/computer/computer.sock"
    static let own: UInt32 = 501
    static let service: UInt32 = 260

    static func goodTree() -> [String: FileStatus] {
        [
            "/": FileStatus(uid: 0, mode: 0o755, kind: .directory),
            "/Library": FileStatus(uid: 0, mode: 0o755, kind: .directory),
            "/Library/Application Support": FileStatus(uid: 0, mode: 0o755, kind: .directory),
            "/Library/Application Support/bothy-host": FileStatus(uid: 0, mode: 0o755, kind: .directory),
            "/Library/Application Support/bothy-host/computer": FileStatus(uid: service, mode: 0o755, kind: .directory),
            socket: FileStatus(uid: service, mode: 0o666, kind: .socket),
        ]
    }

    private func check(
        _ tree: [String: FileStatus], peer: UInt32 = service, serviceUID: UInt32? = service
    ) throws {
        try PeerRule.check(
            socketPath: Self.socket, ownUID: Self.own, serviceUID: serviceUID, peerUID: peer,
            status: { tree[$0] })
    }

    func testAllGood() throws {
        XCTAssertNoThrow(try check(Self.goodTree()))
        var rootTree = Self.goodTree()
        rootTree[Self.socket] = FileStatus(uid: 0, mode: 0o666, kind: .socket)
        XCTAssertNoThrow(try check(rootTree, peer: 0), "root may run the host service")
    }

    func testForeignOwnedFolderIsRefused() {
        var tree = Self.goodTree()
        tree["/Library/Application Support/bothy-host"] = FileStatus(uid: Self.service, mode: 0o755, kind: .directory)
        XCTAssertThrowsError(try check(tree)) {
            XCTAssertEqual($0 as? PeerRule.Refusal, .folderOwner("/Library/Application Support/bothy-host"))
        }
        tree = Self.goodTree()
        tree["/Library/Application Support/bothy-host/computer"] = FileStatus(uid: Self.own, mode: 0o700, kind: .directory)
        XCTAssertThrowsError(try check(tree))
        tree = Self.goodTree()
        tree["/Library/Application Support/bothy-host/computer"] = nil
        XCTAssertThrowsError(try check(tree))
    }

    func testWritableFolderIsRefused() {
        for path in ["/Library/Application Support", "/Library/Application Support/bothy-host/computer"] {
            for mode: UInt16 in [0o775, 0o757] {
                var tree = Self.goodTree()
                tree[path]?.mode = mode
                XCTAssertThrowsError(try check(tree), "\(path) \(String(mode, radix: 8))") {
                    XCTAssertEqual($0 as? PeerRule.Refusal, .folderWritable(path))
                }
            }
        }
    }

    func testSocketOwnedByAnotherUserOrNotASocketIsRefused() {
        var tree = Self.goodTree()
        tree[Self.socket] = FileStatus(uid: 777, mode: 0o666, kind: .socket)
        XCTAssertThrowsError(try check(tree)) { XCTAssertEqual($0 as? PeerRule.Refusal, .socketOwner) }
        tree[Self.socket] = FileStatus(uid: Self.service, mode: 0o666, kind: .other)
        XCTAssertThrowsError(try check(tree)) { XCTAssertEqual($0 as? PeerRule.Refusal, .notASocket) }
    }

    func testPeerChecks() {
        var tree = Self.goodTree()
        tree[Self.socket] = FileStatus(uid: Self.own, mode: 0o666, kind: .socket)
        XCTAssertThrowsError(try check(tree, peer: Self.own)) {
            XCTAssertEqual($0 as? PeerRule.Refusal, .peerIsHelper)
        }
        tree[Self.socket] = FileStatus(uid: 777, mode: 0o666, kind: .socket)
        XCTAssertThrowsError(try check(tree, peer: 777)) {
            XCTAssertEqual($0 as? PeerRule.Refusal, .peerNotService)
        }
        XCTAssertThrowsError(try check(Self.goodTree(), serviceUID: nil)) {
            XCTAssertEqual($0 as? PeerRule.Refusal, .noServiceUser)
        }
        XCTAssertThrowsError(try check(Self.goodTree(), serviceUID: Self.own)) {
            XCTAssertEqual($0 as? PeerRule.Refusal, .serviceUserIsHelper)
        }
    }
}
