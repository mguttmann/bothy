// SPDX-License-Identifier: Apache-2.0

import Foundation

/// The helper's logic: enablement and generation, `show` and `hide`, the stop paths, lock gating,
/// capture and input (docs/spec/27-computer-use.md sections 4, 6.4, 6.5, 8.2 to 8.4, 9.4;
/// docs/decisions.d/CU-02.md D-CU-02-4 to D-CU-02-8).
///
/// The controller is not thread-safe: every method, and every callback it hands to the
/// platform, runs in one serial context (the app uses one serial dispatch queue; tests call it
/// directly). It never refuses a key, app or setting (R-CU-02); the only answers other than
/// `ok` are the ones of 27 9.4.
///
/// Example:
/// ```swift
/// let controller = HelperController(capturer: ..., injector: ..., ..., frameMaxBytes: limit)
/// controller.start()
/// controller.channelOpened(channel)  // sends hello
/// _ = controller.handle(frame)        // false: protocol fault, close the channel
/// ```
public final class HelperController {
    private let capturer: Capturer
    private let injector: Injector
    private let appInspector: AppInspector
    private let lockMonitor: LockMonitor
    private let waker: Waker
    private let escTap: EscTap
    private let indicator: Indicator
    private let store: EnablementStore
    private let info: HelperInfo
    private let codec: FrameCodec
    private let log: (String) -> Void
    /// The flag a running action checks between its steps; the platform layer sets it from the
    /// Esc tap, the stop button, the lock monitor and the channel reader.
    public let interrupt: ActionInterrupt

    private var enablement = EnablementState.off
    private var locked = false
    private var session: (id: String, agent: String)?
    private var channel: HostChannel?
    /// The hello members last sent, for the change set of `state`.
    private var reported: [String: JSONValue] = [:]
    /// The geometry of the last screen read, for the `screen` member of answers that read none.
    private var lastScreen = ScreenGeometry.unknown

    /// Creates the controller.
    ///
    /// - Parameters:
    ///   - capturer: screen capture.
    ///   - injector: synthetic input.
    ///   - appInspector: frontmost app.
    ///   - lockMonitor: lock state.
    ///   - waker: display assertion.
    ///   - escTap: the Esc tap.
    ///   - indicator: border, stop button and menu bar item.
    ///   - store: the persisted enablement.
    ///   - info: version, signature state and account.
    ///   - frameMaxBytes: `host_service.computer_frame_max_bytes` (D-CU-02-8).
    ///   - interrupt: the flag shared with the platform layer; a fresh one by default.
    ///   - log: receives short lines without image bytes or message contents.
    public init(
        capturer: Capturer, injector: Injector, appInspector: AppInspector,
        lockMonitor: LockMonitor, waker: Waker, escTap: EscTap, indicator: Indicator,
        store: EnablementStore, info: HelperInfo, frameMaxBytes: Int,
        interrupt: ActionInterrupt = ActionInterrupt(),
        log: @escaping (String) -> Void = { _ in }
    ) {
        self.interrupt = interrupt
        self.capturer = capturer
        self.injector = injector
        self.appInspector = appInspector
        self.lockMonitor = lockMonitor
        self.waker = waker
        self.escTap = escTap
        self.indicator = indicator
        self.store = store
        self.info = info
        self.codec = FrameCodec(maxFrameBytes: frameMaxBytes)
        self.log = log
    }

    /// The decision of the Esc tap for one Esc key down: a real Esc is swallowed (it ends the
    /// session), an Esc the helper injected passes to the app (27 8.4 item 4, R-CU-09).
    ///
    /// Pure, so a platform tap can answer at once on its own thread and leave the end of the
    /// session to `escapePressed(injected:)` in the controller's context.
    public static func escDecision(injected: Bool) -> EscDecision {
        injected ? .pass : .swallow
    }

    /// What the menu shows now.
    public var status: HelperStatus {
        HelperStatus(
            enabled: enablement.enabled, generation: enablement.generation,
            capture: capturer.permissionGranted() ? .granted : .denied,
            input: injector.permissionGranted() ? .granted : .denied,
            locked: locked, connected: channel != nil, agentName: session?.agent, info: info)
    }

    // MARK: Life cycle

    /// Runs once at process start: releases every key and button a crashed earlier run may have
    /// left pressed, undoes indicator, tap and display assertion (27 8.4 item 2), reads the
    /// enablement and starts the lock monitor.
    public func start() {
        endLocally()
        enablement = store.load()
        lockMonitor.start { [weak self] in self?.lockStateChanged() }
        locked = lockMonitor.isLocked()
        interrupt.setLocked(locked)
        indicator.update(status: status)
    }

    /// Takes a newly connected channel and sends `hello`, which must be its first frame.
    /// - Parameter channel: the connected channel.
    public func channelOpened(_ channel: HostChannel) {
        self.channel = channel
        locked = lockMonitor.isLocked()
        let hello = helloMembers()
        reported = hello
        var message = hello
        message["op"] = "hello"
        send(.object(message))
        indicator.update(status: status)
    }

    /// The channel broke or was closed: the session ends in the helper (27 8.4 item 2).
    public func channelBroken() {
        channel = nil
        endLocally()
        indicator.update(status: status)
    }

    // MARK: Local events

    /// An Esc key down seen by the tap.
    /// - Parameter injected: whether the helper itself sent it.
    /// - Returns: `.swallow` for a real Esc, which also ends the session with `local_escape`;
    ///   `.pass` for an injected one, which stops nothing.
    @discardableResult
    public func escapePressed(injected: Bool) -> EscDecision {
        let decision = Self.escDecision(injected: injected)
        if decision == .swallow { stopByPerson(.localEscape) }
        return decision
    }

    /// The stop button of the indicator or the stop item of the menu was clicked.
    public func indicatorStopClicked() {
        stopByPerson(.indicatorStop)
    }

    /// The person clicked "Allow computer control" (brief item 15). This is the only way to
    /// switch on: no op of the channel, argument or file does it while the helper runs.
    ///
    /// Switching on raises the generation by one and takes effect only once persisted;
    /// switching off takes effect at once and ends a shown session.
    public func toggleEnablementByClick() {
        if enablement.enabled {
            switchOff()
        } else {
            let next = EnablementState(enabled: true, generation: enablement.generation + 1)
            do {
                try store.save(next)
                enablement = next
            } catch {
                log("enablement not saved, the switch stays off")
            }
            reportChanges()
        }
        indicator.update(status: status)
    }

    /// The lock monitor saw a change. Locking releases every key and button, keeps the
    /// indicator and the display assertion (27 8.2) and is reported as `state`.
    public func lockStateChanged() {
        let now = lockMonitor.isLocked()
        interrupt.setLocked(now)
        if now && !locked { injector.releaseAll() }
        locked = now
        reportChanges()
        indicator.update(status: status)
    }

    /// Reads permissions and lock state again and reports what changed. The app calls it when
    /// the menu opens and when the system reports an accessibility change; there is no timer.
    public func refreshStatus() {
        locked = lockMonitor.isLocked()
        interrupt.setLocked(locked)
        reportChanges()
        indicator.update(status: status)
    }

    /// The person clicked the menu item of a permission, which then opens its pane of the
    /// system settings. A missing permission is registered with the system first: a preflight
    /// alone never lists the helper in the Screen Recording pane, so the person could only add
    /// it by hand or switch on another app (live finding 26.11.). The person still decides.
    /// - Parameter kind: the permission of the clicked item.
    public func permissionLinkClicked(_ kind: PermissionKind) {
        switch kind {
        case .capture:
            if !capturer.permissionGranted() { capturer.registerForPermission() }
        case .input:
            if !injector.permissionGranted() { injector.registerForPermission() }
        }
        refreshStatus()
    }

    // MARK: Host messages

    /// Handles one frame from the host service.
    ///
    /// - Parameter frame: the frame.
    /// - Returns: false for a protocol fault (an image frame, a message without a known `op`,
    ///   a `show` or `hide` without its members); the caller then closes the channel, which
    ///   ends the session. A bad `capture`, `do` or `position` with a `seq` is no fault: it is
    ///   answered with `invalid_action`.
    @discardableResult
    public func handle(_ frame: Frame) -> Bool {
        guard case .message(let message) = frame, let op = message["op"]?.stringValue else {
            log("channel fault: no message")
            return false
        }
        switch op {
        case "show":
            guard let id = message["session_id"]?.stringValue,
                let agent = message["agent_name"]?.stringValue
            else { return false }
            show(sessionID: id, agent: agent)
        case "hide":
            guard let id = message["session_id"]?.stringValue else { return false }
            if session?.id == id { endLocally() }
        case "release":
            injector.releaseAll()
        case "disable":
            // Always switch off: the generation names what the server saw, and a stale
            // disable can only make the person click again, never enable anything.
            if enablement.enabled { switchOff() } else { endLocally() }
        case "capture", "do", "position":
            guard let seq = message["seq"]?.intValue else { return false }
            request(op: op, seq: seq, message: message)
        default:
            log("channel fault: unknown op")
            return false
        }
        indicator.update(status: status)
        return true
    }

    // MARK: Sessions

    private func show(sessionID: String, agent: String) {
        guard enablement.enabled else {
            // D-CU-02-6: nothing is shown; the explicit state makes the host service end the
            // session with device_disabled.
            send(["op": "state", "enabled": false])
            return
        }
        if let current = session, current.id == sessionID { return }
        if session != nil { endLocally() }
        if let detail = interrupt.begin(sessionID: sessionID) {
            // The person stopped while this show was on its way: the new session never shows
            // and is answered with the person's stop (27 9.6 item 6).
            endLocally()
            send(["op": "stop", "session_id": .string(sessionID), "detail": .string(detail.rawValue)])
            return
        }
        injector.releaseAll()
        session = (sessionID, agent)
        indicator.show(agentName: agent) { [weak self] in self?.indicatorStopClicked() }
        do {
            try escTap.start { [weak self] injected in
                self?.escapePressed(injected: injected) ?? HelperController.escDecision(injected: injected)
            }
        } catch {
            // The stop button and every client still stop the session (27 16).
            log("Esc tap refused by the system")
        }
        do {
            try waker.keepAwake()
        } catch {
            log("display assertion refused by the system")
        }
    }

    /// Ends a session by the person at the machine: everything is undone before the `stop`
    /// frame goes out (D-CU-02-6), so the person never waits for the host service.
    private func stopByPerson(_ detail: StopDetail) {
        // The platform raised the flag before handing the stop here; acting on it consumes it,
        // whether a session is still shown or a refused show already answered it.
        interrupt.consumePersonStop()
        guard let current = session else { return }
        endLocally()
        send(["op": "stop", "session_id": .string(current.id), "detail": .string(detail.rawValue)])
        indicator.update(status: status)
    }

    private func switchOff() {
        endLocally()
        enablement.enabled = false
        do {
            try store.save(enablement)
        } catch {
            // Off in memory at once; the file may still say on until the next save.
            log("enablement not saved")
        }
        reportChanges()
    }

    /// Releases input, hides the indicator, stops the tap and the display assertion. Every
    /// step runs even without a session, because the start uses it to clean up after a crash.
    private func endLocally() {
        session = nil
        interrupt.finish()
        injector.releaseAll()
        indicator.hide()
        escTap.stop()
        waker.allowSleep()
    }

    // MARK: Requests

    private func request(op: String, seq: Int64, message: JSONValue) {
        guard session != nil else {
            answer(seq: seq, code: .invalidAction)
            return
        }
        locked = lockMonitor.isLocked()
        interrupt.setLocked(locked)
        if locked {
            answer(seq: seq, code: .screenLocked)
            return
        }
        switch op {
        case "capture": capture(seq: seq, message: message)
        case "do": perform(seq: seq, message: message)
        default: position(seq: seq)
        }
    }

    private func capture(seq: Int64, message: JSONValue) {
        guard let maxEdge = message["max_edge"]?.intValue, maxEdge >= 1, maxEdge <= Int64(Int32.max),
            let formatName = message["format"]?.stringValue,
            let format = ImageFormat(rawValue: formatName)
        else {
            answer(seq: seq, code: .invalidAction)
            return
        }
        let quality = Int(message["quality"]?.intValue ?? 100)
        if format == .jpeg, !(1...100).contains(quality) {
            answer(seq: seq, code: .invalidAction)
            return
        }
        guard capturer.permissionGranted() else {
            answer(seq: seq, code: .osRefused)
            return
        }
        let screen: ScreenGeometry
        do {
            screen = try capturer.activeScreen()
            lastScreen = screen
        } catch {
            answer(seq: seq, code: .osRefused)
            return
        }
        var region: Rect?
        if let value = message["region"] {
            guard let x = value["x"]?.doubleValue, let y = value["y"]?.doubleValue,
                let width = value["width"]?.doubleValue, let height = value["height"]?.doubleValue,
                width > 0, height > 0
            else {
                answer(seq: seq, code: .invalidAction)
                return
            }
            let rect = Rect(x: x, y: y, width: width, height: height)
            guard screen.contains(rect) else {
                answer(seq: seq, code: .invalidAction)
                return
            }
            region = rect
        }
        let area = region ?? Rect(x: screen.x, y: screen.y, width: screen.width, height: screen.height)
        let size = Scaling.pixelSize(
            width: area.width, height: area.height, scale: screen.scale, maxEdge: Int(maxEdge))
        let bytes: Data
        do {
            bytes = try capturer.capture(
                screen: screen, region: region, pixelWidth: size.width, pixelHeight: size.height,
                format: format, quality: quality)
        } catch {
            answer(seq: seq, code: .osRefused)
            return
        }
        guard bytes.count + 1 <= codec.maxFrameBytes else {
            // The host service would close the channel on a longer frame (D-CU-02-8).
            answer(seq: seq, code: .osRefused)
            return
        }
        answer(
            seq: seq, code: .ok, screen: screen,
            extra: [
                "image": [
                    "format": .string(format.rawValue), "width": .int(Int64(size.width)),
                    "height": .int(Int64(size.height)), "bytes": .int(Int64(bytes.count)),
                ]
            ])
        send(frame: .image(bytes))
    }

    private func position(seq: Int64) {
        do {
            let cursor = try injector.cursorPosition()
            answer(
                seq: seq, code: .ok,
                extra: ["cursor": ["x": .coordinate(cursor.x), "y": .coordinate(cursor.y)]])
        } catch {
            answer(seq: seq, code: .osRefused)
        }
    }

    /// The events of one `do`, checked completely before the first event, so an invalid request
    /// injects nothing.
    private enum Step {
        case move(Point)
        case down(Key)
        case up(Key)
        case mouseDown(MouseButton, Point, Int)
        case mouseUp(MouseButton, Point, Int)
        case scroll(Point, ScrollDirection, Int)
        case text(String)
    }

    /// The units of one `do` (27 9.6 item 1): a press and release of the combination per
    /// `repeat`, a character of `type` (a line break included), a line of `scroll`, a click of a
    /// click action, a move. Units are made one at a time, so a long action allocates nothing
    /// in advance.
    private struct Plan {
        let count: Int
        let unit: (Int) -> [Step]
    }

    private func perform(seq: Int64, message: JSONValue) {
        guard let plan = plan(message) else {
            answer(seq: seq, code: .invalidAction)
            return
        }
        guard injector.permissionGranted() else {
            answer(seq: seq, code: .osRefused)
            return
        }
        do {
            for index in 0..<plan.count {
                injector.willStartUnit(index)
                // Checked before every unit, so a stop, an end or a lock that arrives on another
                // thread takes effect within one unit, however long the action.
                if let reason = interrupt.reason {
                    injector.releaseAll()
                    abort(seq: seq, reason: reason)
                    return
                }
                for step in plan.unit(index) {
                    switch step {
                    case .move(let point): try injector.move(to: point)
                    case .down(let key): try injector.keyDown(key)
                    case .up(let key): try injector.keyUp(key)
                    case .mouseDown(let button, let point, let count):
                        try injector.mouseDown(button, at: point, clickCount: count)
                    case .mouseUp(let button, let point, let count):
                        try injector.mouseUp(button, at: point, clickCount: count)
                    case .scroll(let point, let direction, let lines):
                        try injector.scroll(at: point, direction: direction, lines: lines)
                    case .text(let text): try injector.typeText(text)
                    }
                }
            }
        } catch {
            // Nothing may stay pressed after a half-done action.
            injector.releaseAll()
            answer(seq: seq, code: .osRefused)
            return
        }
        answer(seq: seq, code: .ok)
    }

    /// Ends an action that an interrupt stopped (27 9.6 item 2): a lock answers `screen_locked`
    /// and keeps the session; the person's stop ends the session with its `stop` frame first
    /// and answers `invalid_action`; a `hide` or `disable` answers `invalid_action` and leaves
    /// the session to the queued message; a broken channel answers nothing.
    private func abort(seq: Int64, reason: ActionInterrupt.Reason) {
        switch reason {
        case .locked:
            answer(seq: seq, code: .screenLocked)
        case .person(let detail):
            stopByPerson(detail)
            answer(seq: seq, code: .invalidAction)
        case .ended:
            answer(seq: seq, code: .invalidAction)
        case .channelBroken:
            break
        }
    }

    private func plan(_ message: JSONValue) -> Plan? {
        guard let action = message["action"]?.stringValue else { return nil }
        switch action {
        case "left_click", "right_click", "middle_click", "double_click", "triple_click":
            guard let point = point(message), let modifiers = modifiers(message) else { return nil }
            let button: MouseButton =
                action == "right_click" ? .right : action == "middle_click" ? .middle : .left
            let count = action == "double_click" ? 2 : action == "triple_click" ? 3 : 1
            let keys = modifiers.map(Key.modifier)
            guard keys.allSatisfy(injector.supports) else { return nil }
            // Unit 0 is the move, then one unit per click; the modifiers go down with the first
            // click and up with the last (an interrupt in between releases them).
            return Plan(count: count + 1) { index in
                if index == 0 { return [.move(point)] }
                var steps: [Step] = index == 1 ? keys.map(Step.down) : []
                steps += [.mouseDown(button, point, index), .mouseUp(button, point, index)]
                if index == count { steps += keys.reversed().map(Step.up) }
                return steps
            }
        case "mouse_move":
            return point(message).map { point in Plan(count: 1) { _ in [.move(point)] } }
        case "scroll":
            guard let point = point(message),
                let direction = message["direction"]?.stringValue.flatMap(ScrollDirection.init),
                // The host service enforces computer.scroll_amount_max; the helper holds its
                // default as an outer bound (27 9.6 item 3).
                let amount = message["amount"]?.intValue, amount >= 1,
                amount <= Int64(Limits.scrollAmountMax)
            else { return nil }
            return Plan(count: Int(amount) + 1) { index in
                index == 0 ? [.move(point)] : [.scroll(point, direction, 1)]
            }
        case "type":
            // A text that does not fit one frame of host_service.computer_frame_max_bytes (with
            // its type byte) cannot have come from the host service; it is refused before
            // anything is typed.
            guard let text = message["text"]?.stringValue, text.utf8.count + 1 <= codec.maxFrameBytes
            else { return nil }
            let characters = Array(
                text.replacingOccurrences(of: "\r\n", with: "\n").replacingOccurrences(of: "\r", with: "\n"))
            return Plan(count: characters.count) { index in
                // A line break is Return (D-CU-02-7); every other character is Unicode input.
                characters[index] == "\n"
                    ? [.down(.named(.return)), .up(.named(.return))] : [.text(String(characters[index]))]
            }
        case "key":
            guard let keys = message["keys"]?.stringValue,
                let combo = try? KeyCombo.parse(keys),
                combo.keys.allSatisfy(injector.supports)
            else { return nil }
            let repeatCount = message["repeat"]?.intValue ?? 1
            // The host service enforces computer.key_repeat_max; the helper holds its default
            // as an outer bound, so no request builds an unbounded action.
            guard repeatCount >= 1, repeatCount <= Int64(Limits.keyRepeatMax) else { return nil }
            let once = combo.keys.map(Step.down) + combo.keys.reversed().map(Step.up)
            return Plan(count: Int(repeatCount)) { _ in once }
        default:
            return nil
        }
    }

    private func point(_ message: JSONValue) -> Point? {
        guard let x = message["x"]?.doubleValue, let y = message["y"]?.doubleValue,
            x.isFinite, y.isFinite
        else { return nil }
        return Point(x: x, y: y)
    }

    private func modifiers(_ message: JSONValue) -> [Modifier]? {
        guard let value = message["modifiers"] else { return [] }
        guard let names = value.arrayValue else { return nil }
        var result: [Modifier] = []
        for name in names {
            guard let text = name.stringValue, let modifier = Modifier.named(text) else {
                return nil
            }
            result.append(modifier)
        }
        return result
    }

    // MARK: Answers and reports

    /// Sends a `done` with the frontmost app and the screen geometry (27 9.4).
    private func answer(
        seq: Int64, code: ResultCode, screen: ScreenGeometry? = nil, extra: [String: JSONValue] = [:]
    ) {
        let geometry = screen ?? currentScreen()
        let app = code == .screenLocked ? AppIdentity.unknown : appInspector.frontmostApp()
        var members: [String: JSONValue] = [
            "op": "done", "seq": .int(seq), "code": .string(code.rawValue), "app": app.json,
            "screen": geometry.json,
        ]
        members.merge(extra) { _, new in new }
        send(.object(members))
    }

    private func currentScreen() -> ScreenGeometry {
        if let screen = try? capturer.activeScreen() { lastScreen = screen }
        return lastScreen
    }

    private func helloMembers() -> [String: JSONValue] {
        let current = status
        return [
            "enabled": .bool(current.enabled), "generation": .int(Int64(current.generation)),
            "os_account": .string(info.osAccount),
            "permissions": [
                "capture": .string(current.capture.rawValue), "input": .string(current.input.rawValue),
            ],
            "helper_version": .string(info.version), "signature": .string(info.signature.rawValue),
            "locked": .bool(current.locked),
        ]
    }

    /// Sends `state` with the hello members that changed since the last report.
    private func reportChanges() {
        guard channel != nil else { return }
        let now = helloMembers()
        var changed: [String: JSONValue] = [:]
        for (key, value) in now where reported[key] != value { changed[key] = value }
        guard !changed.isEmpty else { return }
        reported = now
        changed["op"] = "state"
        send(.object(changed))
    }

    private func send(_ message: JSONValue) {
        send(frame: .message(message))
    }

    private func send(frame: Frame) {
        guard let channel else { return }
        do {
            try channel.send(try codec.encode(frame))
        } catch {
            // The reader of the channel sees the broken connection and ends the session.
            log("channel send failed")
            channel.close()
        }
    }
}
