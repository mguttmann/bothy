// SPDX-License-Identifier: Apache-2.0

import Foundation
import HelperCore
import XCTest

/// The pointer mark of the indicator (27 9.1, decisions.d/FIX-CU-HELPER-NO-ICON.md): every click
/// the helper sends for the agent pulses at its point; other actions pulse nothing. The helper
/// shows its settings window at start only while the switch was never switched on.
final class IndicatorMarkTests: XCTestCase {
    func testEveryClickPulsesAtItsPoint() {
        let rig = Rig()
        rig.show()
        rig.host(["op": "do", "seq": 1, "action": "left_click", "x": 10, "y": 20])
        XCTAssertEqual(rig.last?["code"], "ok")
        XCTAssertEqual(rig.indicator.clicks, [Point(x: 10, y: 20)])
        rig.host(["op": "do", "seq": 2, "action": "double_click", "x": 30, "y": 40])
        XCTAssertEqual(
            rig.indicator.clicks, [Point(x: 10, y: 20), Point(x: 30, y: 40), Point(x: 30, y: 40)])
        rig.host(["op": "do", "seq": 3, "action": "right_click", "x": 5, "y": 6])
        XCTAssertEqual(rig.indicator.clicks.last, Point(x: 5, y: 6))
    }

    func testOtherActionsPulseNothing() {
        let rig = Rig()
        rig.show()
        rig.host(["op": "do", "seq": 1, "action": "mouse_move", "x": 10, "y": 20])
        rig.host(["op": "do", "seq": 2, "action": "type", "text": "hi"])
        rig.host(["op": "do", "seq": 3, "action": "key", "keys": "Return", "repeat": 1])
        rig.host([
            "op": "do", "seq": 4, "action": "scroll", "x": 10, "y": 20, "direction": "down",
            "amount": 2,
        ])
        XCTAssertEqual(rig.channel.messages.filter { $0["code"] == "ok" }.count, 4)
        XCTAssertTrue(rig.indicator.clicks.isEmpty)
    }

    func testAStoppedClickPulsesNothing() {
        let rig = Rig()
        rig.show()
        rig.indicator.clickStop()
        rig.host(["op": "do", "seq": 1, "action": "left_click", "x": 10, "y": 20])
        XCTAssertEqual(rig.last?["code"], "invalid_action")
        XCTAssertTrue(rig.indicator.clicks.isEmpty)
    }

    func testSettingsShowAtStartOnlyBeforeTheFirstSwitchOn() {
        XCTAssertTrue(EnablementState.off.needsSetup)
        XCTAssertFalse(EnablementState(enabled: true, generation: 1).needsSetup)
        // Switched off later, locally or by a remote disable: the person knows the helper.
        XCTAssertFalse(EnablementState(enabled: false, generation: 2).needsSetup)
    }
}

/// R-CU-03: a click on the switch that the helper injected for the agent may switch off, never on.
final class InjectedSwitchTests: XCTestCase {
    func testAnInjectedClickOnTheSwitchNeverSwitchesOn() {
        let rig = Rig()
        rig.show()
        let generation = rig.store.state.generation
        rig.controller.toggleEnablementByClick(injected: true)
        XCTAssertFalse(rig.store.state.enabled)
        XCTAssertNil(rig.indicator.shownAgent)
        rig.controller.toggleEnablementByClick(injected: true)
        XCTAssertFalse(rig.store.state.enabled)
        XCTAssertEqual(rig.store.state.generation, generation)
        rig.controller.toggleEnablementByClick()
        XCTAssertTrue(rig.store.state.enabled)
        XCTAssertEqual(rig.store.state.generation, generation + 1)
    }
}
