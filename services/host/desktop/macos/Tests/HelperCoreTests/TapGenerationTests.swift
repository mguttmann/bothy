// SPDX-License-Identifier: Apache-2.0

import Foundation
import HelperCore
import XCTest

/// The generations of the Esc tap (27 9.6 item 5): an old tap's late callback or teardown never
/// acts on a newer tap.
final class TapGenerationTests: XCTestCase {
    func testOnlyTheCurrentGenerationActs() {
        let slot = TapGeneration<String>()
        XCTAssertNil(slot.currentValue)
        let first = slot.begin("tap 1")
        XCTAssertEqual(slot.value(for: first), "tap 1")
        XCTAssertEqual(slot.endCurrent(), "tap 1")
        let second = slot.begin("tap 2")
        XCTAssertGreaterThan(second, first)
        // The late callback and the late teardown of the first tap find nothing.
        XCTAssertNil(slot.value(for: first))
        XCTAssertNil(slot.end(generation: first))
        XCTAssertEqual(slot.currentValue, "tap 2")
        XCTAssertEqual(slot.value(for: second), "tap 2")
        // Its own teardown ends it once.
        XCTAssertEqual(slot.end(generation: second), "tap 2")
        XCTAssertNil(slot.end(generation: second))
        XCTAssertNil(slot.currentValue)
    }

    func testANewStartWithoutStopEndsTheOldTap() {
        let slot = TapGeneration<String>()
        let first = slot.begin("tap 1")
        XCTAssertEqual(slot.endCurrent(), "tap 1")
        _ = slot.begin("tap 2")
        XCTAssertNil(slot.value(for: first))
        XCTAssertGreaterThan(first, 0, "generations start above zero")
    }
}
