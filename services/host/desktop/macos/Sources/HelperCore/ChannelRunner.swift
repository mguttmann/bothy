import Foundation

/// Keeps the channel to the host service: connect, `hello`, read frames, and on every break end
/// the session in the helper and connect again (27 8.4 item 2, D-CU-02-8).
///
/// The runner blocks in `HostChannel.receive()` on its own thread and hands every frame to the
/// controller through `perform`, which runs a closure synchronously in the controller's serial
/// context.
///
/// Example:
/// ```swift
/// let runner = ChannelRunner(channel: socket, controller: controller,
///                            frameMaxBytes: limit, perform: { work in queue.sync(execute: work) })
/// Thread { runner.run() }.start()
/// ```
public final class ChannelRunner {
    private let channel: HostChannel
    private let controller: HelperController
    private let frameMaxBytes: Int
    private let perform: (() -> Void) -> Void
    private let log: (String) -> Void

    /// Creates a runner.
    /// - Parameters:
    ///   - channel: the connection to the host service.
    ///   - controller: the helper's logic.
    ///   - frameMaxBytes: `host_service.computer_frame_max_bytes`.
    ///   - perform: runs a closure synchronously in the controller's context.
    ///   - log: receives short lines without message contents.
    public init(
        channel: HostChannel, controller: HelperController, frameMaxBytes: Int,
        perform: @escaping (() -> Void) -> Void, log: @escaping (String) -> Void = { _ in }
    ) {
        self.channel = channel
        self.controller = controller
        self.frameMaxBytes = frameMaxBytes
        self.perform = perform
        self.log = log
    }

    /// One connection from connect to break.
    /// - Returns: whether the connection was established, so the caller can pace retries.
    @discardableResult
    public func runOnce() -> Bool {
        do {
            try channel.connect()
        } catch {
            log("channel not connected")
            return false
        }
        perform { self.controller.channelOpened(self.channel) }
        var decoder = FrameDecoder(maxFrameBytes: frameMaxBytes)
        reading: while true {
            do {
                decoder.append(try channel.receive())
                while let frame = try decoder.next() {
                    var accepted = true
                    perform { accepted = self.controller.handle(frame) }
                    if !accepted { break reading }
                }
            } catch {
                // A closed, broken or too long frame: the connection is unusable.
                break reading
            }
        }
        channel.close()
        perform { self.controller.channelBroken() }
        log("channel closed")
        return true
    }

    /// Connects again and again for the life of the process.
    ///
    /// The pause between attempts doubles from `minimumDelay` up to `maximumDelay` while the
    /// endpoint is missing and starts over after a connection that was established, so a
    /// restarted host service is found again quickly without a busy loop.
    /// - Parameters:
    ///   - minimumDelay: the first pause in seconds.
    ///   - maximumDelay: the longest pause in seconds.
    public func run(minimumDelay: TimeInterval = 0.5, maximumDelay: TimeInterval = 10) -> Never {
        var delay = minimumDelay
        while true {
            delay = runOnce() ? minimumDelay : min(delay * 2, maximumDelay)
            Thread.sleep(forTimeInterval: delay)
        }
    }
}
