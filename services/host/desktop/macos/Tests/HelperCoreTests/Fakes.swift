import Foundation
import HelperCore

/// One ordered log shared by every fake, so a test can check the order of effects across
/// components (for example: keys released and indicator hidden before the `stop` frame).
final class Journal {
    private(set) var entries: [String] = []

    func add(_ entry: String) { entries.append(entry) }

    func index(of entry: String) -> Int? { entries.firstIndex(of: entry) }
}

/// An input event as the fake engine saw it.
enum InjectedEvent: Equatable {
    case keyDown(Key)
    case keyUp(Key)
    case move(Point)
    case mouseDown(MouseButton, Point, Int)
    case mouseUp(MouseButton, Point, Int)
    case scroll(Point, ScrollDirection, Int)
    case text(String)
}

/// Fake input engine: records events, tracks what is held down, can refuse keys or fail.
final class FakeInjector: Injector {
    let journal: Journal
    var events: [InjectedEvent] = []
    var pressedKeys: Set<Key> = []
    var pressedButtons: Set<MouseButton> = []
    var releaseAllCount = 0
    var granted = true
    var unsupported: Set<Key> = []
    var failOnMouseDown = false
    var cursor = Point(x: 640.5, y: 400)
    /// Called after every key down, so a test can feed the event to the fake Esc tap.
    var onKeyDown: ((Key) -> Void)?

    init(journal: Journal) { self.journal = journal }

    func permissionGranted() -> Bool { granted }
    func supports(_ key: Key) -> Bool { !unsupported.contains(key) }

    func keyDown(_ key: Key) throws {
        events.append(.keyDown(key))
        pressedKeys.insert(key)
        onKeyDown?(key)
    }

    func keyUp(_ key: Key) throws {
        events.append(.keyUp(key))
        pressedKeys.remove(key)
    }

    func move(to point: Point) throws { events.append(.move(point)) }

    func mouseDown(_ button: MouseButton, at point: Point, clickCount: Int) throws {
        if failOnMouseDown { throw FakeError.refused }
        events.append(.mouseDown(button, point, clickCount))
        pressedButtons.insert(button)
    }

    func mouseUp(_ button: MouseButton, at point: Point, clickCount: Int) throws {
        events.append(.mouseUp(button, point, clickCount))
        pressedButtons.remove(button)
    }

    func scroll(at point: Point, direction: ScrollDirection, lines: Int) throws {
        events.append(.scroll(point, direction, lines))
    }

    func typeText(_ text: String) throws { events.append(.text(text)) }

    func cursorPosition() throws -> Point { cursor }

    func releaseAll() {
        releaseAllCount += 1
        pressedKeys.removeAll()
        pressedButtons.removeAll()
        journal.add("injector.releaseAll")
    }
}

enum FakeError: Error { case refused }

/// Fake capture: a configurable screen, records the requested pixel size and region.
final class FakeCapturer: Capturer {
    var granted = true
    var screen = ScreenGeometry(x: 0, y: 0, width: 1512, height: 982, scale: 2)
    var image = Data([0xFF, 0xD8, 0xFF, 0xD9])
    var fail = false
    var requests: [(region: Rect?, width: Int, height: Int, format: ImageFormat, quality: Int)] = []

    func permissionGranted() -> Bool { granted }
    func activeScreen() throws -> ScreenGeometry { screen }

    func capture(
        screen: ScreenGeometry, region: Rect?, pixelWidth: Int, pixelHeight: Int,
        format: ImageFormat, quality: Int
    ) throws -> Data {
        if fail { throw FakeError.refused }
        requests.append((region, pixelWidth, pixelHeight, format, quality))
        return image
    }
}

final class FakeAppInspector: AppInspector {
    var app = AppIdentity(id: "com.example.editor", name: "Editor", signer: "EXAMPLE123")
    func frontmostApp() -> AppIdentity { app }
}

final class FakeLockMonitor: LockMonitor {
    var locked = false
    var onChange: (() -> Void)?
    func start(onChange: @escaping () -> Void) { self.onChange = onChange }
    func stop() { onChange = nil }
    func isLocked() -> Bool { locked }

    /// Simulates a lock or unlock notification of the platform.
    func set(locked: Bool) {
        self.locked = locked
        onChange?()
    }
}

final class FakeWaker: Waker {
    let journal: Journal
    var awake = false
    init(journal: Journal) { self.journal = journal }
    func keepAwake() throws {
        awake = true
        journal.add("waker.keepAwake")
    }
    func allowSleep() {
        awake = false
        journal.add("waker.allowSleep")
    }
}

final class FakeEscTap: EscTap {
    let journal: Journal
    var handler: ((Bool) -> EscDecision)?
    init(journal: Journal) { self.journal = journal }
    var running: Bool { handler != nil }

    func start(handler: @escaping (Bool) -> EscDecision) throws {
        self.handler = handler
        journal.add("tap.start")
    }

    func stop() {
        handler = nil
        journal.add("tap.stop")
    }

    /// Feeds an Esc key down to the tap; without a running tap the event passes untouched.
    func press(injected: Bool) -> EscDecision {
        guard let handler else { return .pass }
        return handler(injected)
    }
}

final class FakeIndicator: Indicator {
    let journal: Journal
    var shownAgent: String?
    var onStop: (() -> Void)?
    var lastStatus: HelperStatus?
    init(journal: Journal) { self.journal = journal }

    func show(agentName: String, onStop: @escaping () -> Void) {
        shownAgent = agentName
        self.onStop = onStop
        journal.add("indicator.show")
    }

    func hide() {
        shownAgent = nil
        onStop = nil
        journal.add("indicator.hide")
    }

    func update(status: HelperStatus) { lastStatus = status }

    /// Simulates a click on the stop button.
    func clickStop() { onStop?() }
}

final class MemoryStore: EnablementStore {
    var state: EnablementState
    var saves: [EnablementState] = []
    var failSave = false
    init(_ state: EnablementState = EnablementState(enabled: false, generation: 0)) {
        self.state = state
    }
    func load() -> EnablementState { state }
    func save(_ state: EnablementState) throws {
        if failSave { throw FakeError.refused }
        self.state = state
        saves.append(state)
    }
}

/// Fake host side: records every frame the helper sends and plays scripted input.
final class FakeChannel: HostChannel {
    let journal: Journal
    var sent: [Frame] = []
    var incoming: [Data] = []
    var connectError: Error?
    var connects = 0
    var closed = 0
    private let codec = FrameCodec(maxFrameBytes: HostConfig.defaultFrameMaxBytes)

    init(journal: Journal) { self.journal = journal }

    func connect() throws {
        connects += 1
        if let connectError { throw connectError }
    }

    func send(_ data: Data) throws {
        let frame = try codec.decodeSingle(data)
        sent.append(frame)
        if case .message(let object) = frame, let op = object["op"]?.stringValue {
            journal.add("send.\(op)")
        } else {
            journal.add("send.image")
        }
    }

    func receive() throws -> Data {
        if incoming.isEmpty { throw FakeError.refused }
        return incoming.removeFirst()
    }

    func close() { closed += 1 }

    /// The JSON messages sent so far, without image frames.
    var messages: [JSONValue] {
        sent.compactMap { if case .message(let value) = $0 { return value } else { return nil } }
    }
}

/// A controller wired to fresh fakes, with the channel open and the hello consumed.
final class Rig {
    let journal = Journal()
    let capturer = FakeCapturer()
    let inspector = FakeAppInspector()
    let lock = FakeLockMonitor()
    lazy var injector = FakeInjector(journal: journal)
    lazy var waker = FakeWaker(journal: journal)
    lazy var tap = FakeEscTap(journal: journal)
    lazy var indicator = FakeIndicator(journal: journal)
    lazy var channel = FakeChannel(journal: journal)
    let store: MemoryStore
    lazy var controller = HelperController(
        capturer: capturer, injector: injector, appInspector: inspector, lockMonitor: lock,
        waker: waker, escTap: tap, indicator: indicator, store: store,
        info: HelperInfo(version: "1.2.3", signature: .development, osAccount: "alex"),
        frameMaxBytes: HostConfig.defaultFrameMaxBytes)

    static let sessionID = "0192f3a4-5b6c-7d8e-9f01-23456789abcd"

    init(enabled: Bool = true, generation: Int = 3, connect: Bool = true) {
        store = MemoryStore(EnablementState(enabled: enabled, generation: generation))
        controller.start()
        if connect { controller.channelOpened(channel) }
    }

    /// Sends one host message to the controller as the channel would deliver it.
    @discardableResult
    func host(_ object: JSONValue) -> Bool {
        controller.handle(.message(object))
    }

    func show(agent: String = "Builder") {
        host(["op": "show", "session_id": .string(Rig.sessionID), "agent_name": .string(agent)])
    }

    /// The last message the helper sent.
    var last: JSONValue? { channel.messages.last }
}
