import Foundation

// The platform interfaces of brief item 13. Every call into the operating system sits behind one
// of them, so `HelperController` runs against fakes in unit tests. `HelperController` calls them
// from its one serial context; an implementation that needs another thread (AppKit's main
// thread, an event tap's run loop) hops there itself.

/// Screen capture (27 6.5, 9.1).
public protocol Capturer: AnyObject {
    /// Whether screen recording is granted; checked without asking the person.
    func permissionGranted() -> Bool

    /// The screen that holds the frontmost window, else the main screen.
    /// - Returns: its geometry in the global display space.
    /// - Throws: when the operating system reports no screen.
    func activeScreen() throws -> ScreenGeometry

    /// Captures the screen or a region of it, without the indicator, scaled to a pixel size.
    ///
    /// The image stays in memory; it is never written to disk.
    /// - Parameters:
    ///   - screen: the screen from `activeScreen()`.
    ///   - region: an area inside the screen in the global display space, nil for all of it.
    ///   - pixelWidth: width of the output image.
    ///   - pixelHeight: height of the output image.
    ///   - format: JPEG or PNG.
    ///   - quality: JPEG quality from 1 to 100 (`computer.jpeg_quality`); ignored for PNG.
    /// - Returns: the encoded image.
    /// - Throws: when the operating system refuses the capture or the encoding fails.
    func capture(
        screen: ScreenGeometry, region: Rect?, pixelWidth: Int, pixelHeight: Int,
        format: ImageFormat, quality: Int
    ) throws -> Data
}

/// Synthetic input (27 6.4, 9.1). Events carry a marker of the helper, so the Esc tap can tell
/// them from the real keyboard (27 8.4 item 4).
public protocol Injector: AnyObject {
    /// Whether accessibility is granted, without which macOS drops synthetic events.
    func permissionGranted() -> Bool

    /// Whether the platform has a key for `key` (D-CU-02-7: a name without a key is
    /// `invalid_action`). Checked for every key of a request before the first event.
    func supports(_ key: Key) -> Bool

    /// Presses a key and keeps it held until `keyUp` or `releaseAll`.
    /// - Throws: when the event cannot be created or posted.
    func keyDown(_ key: Key) throws

    /// Releases a key.
    /// - Throws: when the event cannot be created or posted.
    func keyUp(_ key: Key) throws

    /// Moves the pointer.
    /// - Throws: when the event cannot be created or posted.
    func move(to point: Point) throws

    /// Presses a mouse button at a point.
    /// - Parameter clickCount: 1 for a single click, 2 and 3 for the second and third press of
    ///   a double or triple click.
    /// - Throws: when the event cannot be created or posted.
    func mouseDown(_ button: MouseButton, at point: Point, clickCount: Int) throws

    /// Releases a mouse button at a point.
    /// - Throws: when the event cannot be created or posted.
    func mouseUp(_ button: MouseButton, at point: Point, clickCount: Int) throws

    /// Scrolls at a point by whole lines.
    /// - Throws: when the event cannot be created or posted.
    func scroll(at point: Point, direction: ScrollDirection, lines: Int) throws

    /// Types text as Unicode input, independent of the keyboard layout. The text holds no line
    /// break; `HelperController` sends those as `Return`.
    /// - Throws: when the events cannot be created or posted.
    func typeText(_ text: String) throws

    /// The pointer position.
    /// - Throws: when the operating system gives no position.
    func cursorPosition() throws -> Point

    /// Releases every key and button that may be held, including ones a crashed earlier run of
    /// the helper left pressed. Never fails: it is the cleanup of every end.
    func releaseAll()
}

/// The identity of the frontmost app (brief item 21).
public protocol AppInspector: AnyObject {
    /// The frontmost app; `AppIdentity.unknown` members when the platform gives none.
    func frontmostApp() -> AppIdentity
}

/// Lock, screen saver, user switch and display sleep (27 8.2, 9.1).
public protocol LockMonitor: AnyObject {
    /// Starts watching; `onChange` runs in the controller's context after every change.
    func start(onChange: @escaping () -> Void)
    /// Stops watching.
    func stop()
    /// Whether the screen counts as locked now.
    func isLocked() -> Bool
}

/// Keeps the display awake while a session is shown (27 8.2 item 5).
public protocol Waker: AnyObject {
    /// Holds the display awake until `allowSleep`.
    /// - Throws: when the operating system refuses the assertion.
    func keepAwake() throws
    /// Releases the hold; harmless without one.
    func allowSleep()
}

/// What the Esc tap does with an Esc.
public enum EscDecision: Equatable, Sendable {
    /// Remove the event, so the app never sees it.
    case swallow
    /// Let the event through.
    case pass
}

/// The Esc tap (27 8.4 item 4, 9.1).
public protocol EscTap: AnyObject {
    /// Starts the tap.
    /// - Parameter handler: called for every Esc key down with whether the helper injected it;
    ///   its answer decides the event. The platform tap may answer from
    ///   `HelperController.escDecision(injected:)` itself and call the handler later in the
    ///   controller's context, because a tap must not wait.
    /// - Throws: when the operating system refuses the tap (no accessibility grant).
    func start(handler: @escaping (_ injected: Bool) -> EscDecision) throws
    /// Stops the tap; harmless when it is not running.
    func stop()
}

/// The indicator and the menu bar item (brief item 16, 27 9.1).
public protocol Indicator: AnyObject {
    /// Shows the border around the screen with its stop button and the menu bar text
    /// "Agent <name> controls this computer, Esc to stop".
    /// - Parameters:
    ///   - agentName: the `agent_name` of `show`.
    ///   - onStop: runs in the controller's context when the person clicks the stop button or
    ///     the stop item of the menu.
    func show(agentName: String, onStop: @escaping () -> Void)
    /// Hides the border and the session text; harmless when nothing is shown.
    func hide()
    /// Updates the menu (switch, permissions, version) after a change.
    func update(status: HelperStatus)
}

/// The persisted enablement (D-CU-02-4).
public protocol EnablementStore: AnyObject {
    /// Reads the state; a missing or unreadable file reads as `EnablementState.off`.
    func load() -> EnablementState
    /// Writes the state atomically.
    /// - Throws: when the file cannot be written.
    func save(_ state: EnablementState) throws
}

/// The connection to the host service (27 9.4).
public protocol HostChannel: AnyObject {
    /// Opens the connection and checks the host side (D-CU-02-5).
    /// - Throws: when the endpoint is missing or the peer check fails.
    func connect() throws
    /// Sends bytes; safe to call while another thread waits in `receive`.
    /// - Throws: when the connection is broken.
    func send(_ data: Data) throws
    /// Waits for the next bytes.
    /// - Returns: at least one byte.
    /// - Throws: when the connection is closed or broken.
    func receive() throws -> Data
    /// Closes the connection; harmless when it is closed.
    func close()
}
