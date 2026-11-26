import Foundation
import HelperCore
import IOKit.pwr_mgt

/// Keeps the display awake with an `IOPMAssertion` of type
/// `kIOPMAssertionTypePreventUserIdleDisplaySleep` while a session is shown (27 8.2 item 5).
public final class MacWaker: Waker {
    private var assertion: IOPMAssertionID?

    /// Creates the waker without an assertion.
    public init() {}

    public func keepAwake() throws {
        guard assertion == nil else { return }
        var id = IOPMAssertionID(0)
        let result = IOPMAssertionCreateWithName(
            kIOPMAssertionTypePreventUserIdleDisplaySleep as CFString,
            IOPMAssertionLevel(kIOPMAssertionLevelOn),
            "A Bothy agent controls this computer" as CFString, &id)
        guard result == kIOReturnSuccess else { throw WakerError.refused(result) }
        assertion = id
    }

    public func allowSleep() {
        guard let id = assertion else { return }
        IOPMAssertionRelease(id)
        assertion = nil
    }
}

/// Errors of the waker.
public enum WakerError: Error {
    /// IOKit refused the assertion with this return code.
    case refused(IOReturn)
}
