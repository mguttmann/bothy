import Foundation
import HelperCore
import XCTest

/// The shared stop contract of 27 9.6: every case of
/// `services/host/testdata/computer/stop-vectors.json` against `HelperController` and the fakes
/// (D-CU-02-16). The file is read, never changed; its description defines units and events.
final class StopVectorTests: XCTestCase {
    static func load() throws -> JSONValue {
        var url = URL(fileURLWithPath: #filePath)
        for _ in 0..<5 { url.deleteLastPathComponent() }  // up to services/host
        url.appendPathComponent("testdata/computer/stop-vectors.json")
        return try JSONValue.parse(Data(contentsOf: url))
    }

    func testKeyRepeatMaxMatchesTheHelper() throws {
        XCTAssertEqual(try Self.load()["key_repeat_max"]?.intValue, Int64(Limits.keyRepeatMax))
    }

    func testEveryCase() throws {
        let cases = try XCTUnwrap(Self.load()["cases"]?.arrayValue)
        XCTAssertGreaterThanOrEqual(cases.count, 14)
        for item in cases {
            try run(item)
        }
    }

    private func run(_ item: JSONValue) throws {
        let name = item["name"]?.stringValue ?? "?"
        let frameMax = item["frame_max_bytes"]?.intValue.map(Int.init) ?? HostConfig.defaultFrameMaxBytes
        // The fake host decodes with the default limit; frame_max_bytes is the helper's.
        let rig = Rig(enabled: item["enabled"]?.boolValue ?? true, generation: 1, frameMaxBytes: frameMax)
        var sentAtLastShow = 0
        for event in try XCTUnwrap(item["events"]?.arrayValue, name) {
            if let frame = event["host"] {
                if frame["op"] == "show" { sentAtLastShow = rig.channel.sent.count }
                if frame["op"] == "do" {
                    rig.injector.units = 0
                    rig.injector.events = []
                }
                rig.host(frame)
            } else if let kind = event["interrupt"]?.stringValue {
                let unit = Int(try XCTUnwrap(event["before_unit"]?.intValue, name))
                rig.injector.onUnit = { index in
                    guard index == unit else { return }
                    rig.injector.onUnit = nil
                    Self.raise(kind, on: rig)
                }
            } else if let direction = event["esc"]?.stringValue {
                let decision = rig.tap.key(
                    down: direction == "down", injected: event["injected"]?.boolValue ?? false)
                let swallow = try XCTUnwrap(event["swallow"]?.boolValue, name)
                XCTAssertEqual(decision, swallow ? .swallow : .pass, "\(name): esc \(direction)")
            } else {
                XCTFail("\(name): unknown event \(event)")
            }
        }
        let expect = try XCTUnwrap(item["expect"], name)
        XCTAssertEqual(rig.injector.units, Int(expect["units"]?.intValue ?? -1), "\(name): units")
        if expect["released_after"]?.boolValue == true {
            XCTAssertTrue(rig.injector.pressedKeys.isEmpty, "\(name): keys held")
            XCTAssertTrue(rig.injector.pressedButtons.isEmpty, "\(name): buttons held")
        }
        let sent = rig.channel.sent.dropFirst(sentAtLastShow).compactMap { frame -> JSONValue? in
            guard case .message(let message) = frame, ["stop", "done"].contains(message["op"]?.stringValue)
            else { return nil }
            return message
        }
        let expected = try XCTUnwrap(expect["sent"]?.arrayValue, name)
        let compare = {
            XCTAssertEqual(sent.count, expected.count, "\(name): frames \(sent)")
            for (got, want) in zip(sent, expected) {
                for (key, value) in want.objectValue ?? [:] {
                    XCTAssertEqual(got[key], value, "\(name): member \(key)")
                }
            }
        }
        if frameMax < Self.smallestDone, !expected.isEmpty {
            // A done carries app and screen and is longer than such a small frame limit, so the
            // helper's codec cannot send it (D-CU-02-8). Reported to the coordinator; strict, so
            // a corrected vector makes this test fail until the exception is removed.
            XCTExpectFailure("\(name): a done does not fit frame_max_bytes \(frameMax)", strict: true) {
                compare()
            }
        } else {
            compare()
        }
    }

    /// The length of the shortest `done` the controller sends with the fakes (type byte and
    /// content), measured instead of assumed.
    static let smallestDone: Int = {
        let rig = Rig()
        rig.host(["op": "position", "seq": 1])
        guard let done = rig.channel.messages.last,
            let bytes = try? FrameCodec(maxFrameBytes: HostConfig.defaultFrameMaxBytes).encode(.message(done))
        else { return 0 }
        return bytes.count - 4
    }()

    /// Raises an interrupt the way the platform layer does, from outside the controller.
    private static func raise(_ kind: String, on rig: Rig) {
        let interrupt = rig.controller.interrupt
        switch kind {
        case "escape":
            // The tap sets the flag at once and queues the end of the session behind the do.
            interrupt.requestStop(.localEscape)
        case "stop_button":
            interrupt.requestStop(.indicatorStop)
        case "lock":
            rig.lock.locked = true
            interrupt.setLocked(true)
        case "hide":
            interrupt.note(["op": "hide", "session_id": "s1"])
        case "channel_broken":
            interrupt.requestChannelBroken()
        default:
            XCTFail("unknown interrupt \(kind)")
        }
    }
}
