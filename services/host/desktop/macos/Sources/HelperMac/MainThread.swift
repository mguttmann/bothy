import Foundation

/// Runs AppKit and Text Input Sources work on the main thread.
///
/// The controller's queue may wait for the main thread, never the other way round: every
/// callback from the main thread into the controller is asynchronous, so this cannot deadlock.
enum MainThread {
    /// Runs `work` on the main thread and returns its result.
    static func sync<T>(_ work: () -> T) -> T {
        if Thread.isMainThread { return work() }
        return DispatchQueue.main.sync(execute: work)
    }
}
