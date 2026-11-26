import Foundation
import HelperCore
import XCTest

/// Key names of D-CU-02-7 and the shortcut table of CU-T15: every key and combination passes to
/// the engine unchanged and the helper refuses none.
final class ShortcutTests: XCTestCase {
    func testKeyNamesParse() throws {
        XCTAssertEqual(
            try KeyCombo.parse("cmd+shift+t"),
            KeyCombo(modifiers: [.command, .shift], key: .character("t")))
        XCTAssertEqual(try KeyCombo.parse("Return"), KeyCombo(modifiers: [], key: .named(.return)))
        XCTAssertEqual(try KeyCombo.parse("enter"), KeyCombo(modifiers: [], key: .named(.return)))
        XCTAssertEqual(
            try KeyCombo.parse("CONTROL+Option+Page_Down"),
            KeyCombo(modifiers: [.control, .option], key: .named(.pageDown)))
        XCTAssertEqual(
            try KeyCombo.parse("super+Prior"), KeyCombo(modifiers: [.command], key: .named(.pageUp)))
        XCTAssertEqual(
            try KeyCombo.parse("win+Next"), KeyCombo(modifiers: [.command], key: .named(.pageDown)))
        XCTAssertEqual(try KeyCombo.parse("F24"), KeyCombo(modifiers: [], key: .named(.function(24))))
        XCTAssertEqual(try KeyCombo.parse("shift"), KeyCombo(modifiers: [.shift], key: nil))
        XCTAssertEqual(try KeyCombo.parse("ctrl++"), KeyCombo(modifiers: [.control], key: .character("+")))
        XCTAssertEqual(try KeyCombo.parse("+"), KeyCombo(modifiers: [], key: .character("+")))
        XCTAssertEqual(try KeyCombo.parse("ä"), KeyCombo(modifiers: [], key: .character("ä")))
        XCTAssertEqual(try KeyCombo.parse("Esc"), KeyCombo(modifiers: [], key: .named(.escape)))
        XCTAssertEqual(try KeyCombo.parse("meta+BackSpace"), KeyCombo(modifiers: [.command], key: .named(.backspace)))
        for bad in ["", "Hyper+x", "F25", "F0", "ctrl+", "cmd+ab", "shift+Return+t"] {
            XCTAssertThrowsError(try KeyCombo.parse(bad), bad)
        }
    }

    func testShortcutTablePassesUnchanged() {
        let cmd = Key.modifier(.command)
        let ctrl = Key.modifier(.control)
        let alt = Key.modifier(.option)
        let shift = Key.modifier(.shift)
        let table: [(String, [InjectedEvent])] = [
            ("cmd+q", [.keyDown(cmd), .keyDown(.character("q")), .keyUp(.character("q")), .keyUp(cmd)]),
            (
                "ctrl+alt+del",
                [
                    .keyDown(ctrl), .keyDown(alt), .keyDown(.named(.delete)), .keyUp(.named(.delete)),
                    .keyUp(alt), .keyUp(ctrl),
                ]
            ),
            ("cmd+space", [.keyDown(cmd), .keyDown(.named(.space)), .keyUp(.named(.space)), .keyUp(cmd)]),
            (
                "cmd+shift+t",
                [
                    .keyDown(cmd), .keyDown(shift), .keyDown(.character("t")), .keyUp(.character("t")),
                    .keyUp(shift), .keyUp(cmd),
                ]
            ),
            ("F5", [.keyDown(.named(.function(5))), .keyUp(.named(.function(5)))]),
            ("Return", [.keyDown(.named(.return)), .keyUp(.named(.return))]),
            ("cmd+tab", [.keyDown(cmd), .keyDown(.named(.tab)), .keyUp(.named(.tab)), .keyUp(cmd)]),
            ("ctrl+c", [.keyDown(ctrl), .keyDown(.character("c")), .keyUp(.character("c")), .keyUp(ctrl)]),
            ("cmd+alt+Escape", [.keyDown(cmd), .keyDown(alt), .keyDown(.named(.escape)), .keyUp(.named(.escape)), .keyUp(alt), .keyUp(cmd)]),
            ("shift", [.keyDown(shift), .keyUp(shift)]),
        ]
        for (keys, expected) in table {
            let rig = Rig()
            rig.show()
            rig.host(["op": "do", "seq": 1, "action": "key", "keys": .string(keys), "repeat": 1])
            XCTAssertEqual(rig.last?["code"], "ok", keys)
            XCTAssertEqual(rig.injector.events, expected, keys)
            XCTAssertTrue(rig.injector.pressedKeys.isEmpty, keys)
        }
    }

    func testRepeatPressesTheCombinationAgain() {
        let rig = Rig()
        rig.show()
        rig.host(["op": "do", "seq": 1, "action": "key", "keys": "Down", "repeat": 3])
        XCTAssertEqual(rig.injector.events.filter { $0 == .keyDown(.named(.down)) }.count, 3)
    }

    func testUnknownOrUnsupportedKeysAreInvalidAndInjectNothing() {
        let rig = Rig()
        rig.show()
        rig.host(["op": "do", "seq": 1, "action": "key", "keys": "Hyper+x"])
        XCTAssertEqual(rig.last?["code"], "invalid_action")
        rig.injector.unsupported = [.named(.insert)]
        rig.host(["op": "do", "seq": 2, "action": "key", "keys": "shift+Insert"])
        XCTAssertEqual(rig.last?["code"], "invalid_action")
        rig.host(["op": "do", "seq": 3, "action": "key", "keys": "a", "repeat": 0])
        XCTAssertEqual(rig.last?["code"], "invalid_action")
        XCTAssertTrue(rig.injector.events.isEmpty)
    }
}

/// Clicks, move, scroll, type and position (brief item 19).
final class InputTests: XCTestCase {
    func testClickWithModifiers() {
        let rig = Rig()
        rig.show()
        rig.host(["op": "do", "seq": 3, "action": "left_click", "x": 640.5, "y": 400, "modifiers": ["cmd"]])
        let point = Point(x: 640.5, y: 400)
        XCTAssertEqual(
            rig.injector.events,
            [
                .move(point), .keyDown(.modifier(.command)), .mouseDown(.left, point, 1),
                .mouseUp(.left, point, 1), .keyUp(.modifier(.command)),
            ])
        XCTAssertEqual(rig.last?["code"], "ok")
        XCTAssertEqual(rig.last?["seq"], 3)
        XCTAssertEqual(rig.last?["app"], ["id": "com.example.editor", "name": "Editor", "signer": "EXAMPLE123"])
    }

    func testDoubleTripleRightMiddle() {
        let rig = Rig()
        rig.show()
        let point = Point(x: 1, y: 2)
        rig.host(["op": "do", "seq": 1, "action": "triple_click", "x": 1, "y": 2])
        XCTAssertEqual(
            rig.injector.events.filter { if case .mouseDown = $0 { true } else { false } },
            [.mouseDown(.left, point, 1), .mouseDown(.left, point, 2), .mouseDown(.left, point, 3)])
        rig.injector.events = []
        rig.host(["op": "do", "seq": 2, "action": "double_click", "x": 1, "y": 2])
        XCTAssertEqual(rig.injector.events.count, 5)
        rig.injector.events = []
        rig.host(["op": "do", "seq": 3, "action": "right_click", "x": 1, "y": 2])
        XCTAssertTrue(rig.injector.events.contains(.mouseDown(.right, point, 1)))
        rig.injector.events = []
        rig.host(["op": "do", "seq": 4, "action": "middle_click", "x": 1, "y": 2])
        XCTAssertTrue(rig.injector.events.contains(.mouseDown(.middle, point, 1)))
    }

    func testMoveScrollType() {
        let rig = Rig()
        rig.show()
        rig.host(["op": "do", "seq": 1, "action": "mouse_move", "x": 5, "y": 6])
        rig.host(["op": "do", "seq": 2, "action": "scroll", "x": 10, "y": 20, "direction": "down", "amount": 3])
        rig.host(["op": "do", "seq": 3, "action": "type", "text": "naïve café\n世界"])
        XCTAssertEqual(
            rig.injector.events,
            [
                .move(Point(x: 5, y: 6)), .move(Point(x: 10, y: 20)),
                .scroll(Point(x: 10, y: 20), .down, 3), .text("naïve café"),
                .keyDown(.named(.return)), .keyUp(.named(.return)), .text("世界"),
            ])
    }

    func testPositionCarriesTheCursor() {
        let rig = Rig()
        rig.show()
        rig.injector.cursor = Point(x: 640.504, y: 400)
        rig.host(["op": "position", "seq": 7])
        XCTAssertEqual(rig.last?["cursor"], ["x": 640.5, "y": 400])
        XCTAssertEqual(rig.last?["code"], "ok")
    }

    func testOutsideASessionEverythingIsInvalid() {
        let rig = Rig()
        rig.host(["op": "do", "seq": 1, "action": "left_click", "x": 1, "y": 2])
        XCTAssertEqual(rig.last?["code"], "invalid_action")
        rig.host(["op": "capture", "seq": 2, "max_edge": 1280, "format": "jpeg", "quality": 80])
        XCTAssertEqual(rig.last?["code"], "invalid_action")
        rig.host(["op": "position", "seq": 3])
        XCTAssertEqual(rig.last?["code"], "invalid_action")
        XCTAssertTrue(rig.injector.events.isEmpty)
        XCTAssertTrue(rig.capturer.requests.isEmpty)
    }

    func testBadArgumentsAreInvalid() {
        let rig = Rig()
        rig.show()
        let bad: [JSONValue] = [
            ["op": "do", "seq": 1, "action": "screenshot"],
            ["op": "do", "seq": 2, "action": "drag", "x": 1, "y": 1],
            ["op": "do", "seq": 3, "action": "left_click", "x": 1],
            ["op": "do", "seq": 4, "action": "left_click", "x": 1, "y": 1, "modifiers": ["hyper"]],
            ["op": "do", "seq": 5, "action": "scroll", "x": 1, "y": 1, "direction": "sideways", "amount": 1],
            ["op": "do", "seq": 6, "action": "scroll", "x": 1, "y": 1, "direction": "up", "amount": 0],
            ["op": "do", "seq": 7, "action": "type"],
        ]
        for message in bad {
            rig.host(message)
            XCTAssertEqual(rig.last?["code"], "invalid_action", "\(message)")
            XCTAssertEqual(rig.last?["seq"], message["seq"])
        }
        XCTAssertTrue(rig.injector.events.isEmpty)
    }

    func testPlatformErrorsAnswerOsRefusedAndRelease() {
        let rig = Rig()
        rig.show()
        rig.injector.failOnMouseDown = true
        rig.host(["op": "do", "seq": 1, "action": "left_click", "x": 1, "y": 2, "modifiers": ["shift"]])
        XCTAssertEqual(rig.last?["code"], "os_refused")
        XCTAssertTrue(rig.injector.pressedKeys.isEmpty)
        rig.injector.failOnMouseDown = false
        rig.injector.granted = false
        rig.host(["op": "do", "seq": 2, "action": "mouse_move", "x": 1, "y": 2])
        XCTAssertEqual(rig.last?["code"], "os_refused")
        rig.capturer.fail = true
        rig.host(["op": "capture", "seq": 3, "max_edge": 1280, "format": "png", "quality": 100])
        XCTAssertEqual(rig.last?["code"], "os_refused")
        rig.capturer.fail = false
        rig.capturer.granted = false
        rig.host(["op": "capture", "seq": 4, "max_edge": 1280, "format": "png", "quality": 100])
        XCTAssertEqual(rig.last?["code"], "os_refused")
    }

    func testProtocolFaultsCloseTheChannel() {
        let rig = Rig()
        XCTAssertFalse(rig.host(["op": "teleport"]))
        XCTAssertFalse(rig.host(["seq": 1]))
        XCTAssertFalse(rig.controller.handle(.image(Data([1]))))
        XCTAssertTrue(rig.host(["op": "release"]))
    }
}

/// Capture, scaling into `max_edge`, the `screen` member and `region` (brief item 18, 27 6.5).
final class CaptureTests: XCTestCase {
    func testScalingKeepsTheAspectRatioAndNeverUpscales() {
        XCTAssertEqual(Scaling.pixelSize(width: 1512, height: 982, scale: 2, maxEdge: 1280), PixelSize(width: 1280, height: 831))
        XCTAssertEqual(Scaling.pixelSize(width: 982, height: 1512, scale: 2, maxEdge: 1280), PixelSize(width: 831, height: 1280))
        XCTAssertEqual(Scaling.pixelSize(width: 800, height: 600, scale: 1, maxEdge: 1280), PixelSize(width: 800, height: 600))
        XCTAssertEqual(Scaling.pixelSize(width: 320, height: 240, scale: 2, maxEdge: 1280), PixelSize(width: 640, height: 480))
        XCTAssertEqual(Scaling.pixelSize(width: 1920, height: 1080, scale: 1.5, maxEdge: 1280), PixelSize(width: 1280, height: 720))
        XCTAssertEqual(Scaling.pixelSize(width: 4000, height: 1, scale: 1, maxEdge: 100), PixelSize(width: 100, height: 1))
    }

    func testScreenshotOfARetinaScreen() throws {
        let rig = Rig()
        rig.show()
        rig.host(["op": "capture", "seq": 1, "max_edge": 1280, "format": "jpeg", "quality": 80])
        let done = try XCTUnwrap(rig.channel.messages.last)
        XCTAssertEqual(
            done,
            [
                "op": "done", "seq": 1, "code": "ok",
                "app": ["id": "com.example.editor", "name": "Editor", "signer": "EXAMPLE123"],
                "screen": ["x": 0, "y": 0, "width": 1512, "height": 982, "scale": 2],
                "image": ["format": "jpeg", "width": 1280, "height": 831, "bytes": 4],
            ])
        XCTAssertEqual(rig.channel.sent.last, .image(Data([0xFF, 0xD8, 0xFF, 0xD9])))
        let request = try XCTUnwrap(rig.capturer.requests.first)
        XCTAssertNil(request.region)
        XCTAssertEqual(request.width, 1280)
        XCTAssertEqual(request.height, 831)
        XCTAssertEqual(request.format, .jpeg)
        XCTAssertEqual(request.quality, 80)
    }

    func testZoomCapturesTheRegionInNativeResolution() throws {
        let rig = Rig()
        rig.capturer.screen = ScreenGeometry(x: -1512, y: 0, width: 1512, height: 982, scale: 2)
        rig.show()
        rig.host([
            "op": "capture", "seq": 2, "max_edge": 1280, "format": "png", "quality": 100,
            "region": ["x": -1412, "y": 200, "width": 320, "height": 240],
        ])
        let request = try XCTUnwrap(rig.capturer.requests.first)
        XCTAssertEqual(request.region, Rect(x: -1412, y: 200, width: 320, height: 240))
        XCTAssertEqual(request.width, 640)
        XCTAssertEqual(request.height, 480)
        XCTAssertEqual(rig.last?["op"], "done")
        XCTAssertEqual(rig.channel.messages.last?["screen"], ["x": -1512, "y": 0, "width": 1512, "height": 982, "scale": 2])
        XCTAssertEqual(rig.channel.messages.last?["image"]?["format"], "png")
    }

    func testRegionOutsideTheScreenOrBadRequestIsInvalid() {
        let rig = Rig()
        rig.show()
        rig.host([
            "op": "capture", "seq": 2, "max_edge": 1280, "format": "png", "quality": 100,
            "region": ["x": 1400, "y": 0, "width": 320, "height": 240],
        ])
        XCTAssertEqual(rig.last?["code"], "invalid_action")
        rig.host(["op": "capture", "seq": 3, "max_edge": 0, "format": "png", "quality": 100])
        XCTAssertEqual(rig.last?["code"], "invalid_action")
        rig.host(["op": "capture", "seq": 4, "max_edge": 1280, "format": "gif", "quality": 100])
        XCTAssertEqual(rig.last?["code"], "invalid_action")
        rig.host(["op": "capture", "seq": 5, "max_edge": 1280, "format": "jpeg", "quality": 0])
        XCTAssertEqual(rig.last?["code"], "invalid_action")
        XCTAssertTrue(rig.capturer.requests.isEmpty)
    }

    func testImageOverTheFrameLimitIsNotSent() {
        let rig = Rig()
        rig.capturer.image = Data(count: HostConfig.defaultFrameMaxBytes)
        rig.show()
        rig.host(["op": "capture", "seq": 1, "max_edge": 1280, "format": "png", "quality": 100])
        XCTAssertEqual(rig.last?["code"], "os_refused")
        XCTAssertFalse(rig.channel.sent.contains { if case .image = $0 { true } else { false } })
    }
}
