// The entry point of the Bothy Computer Helper (docs/spec/27-computer-use.md section 9.1).
// `--version` prints the version and exits without touching the screen, input or the channel,
// so packaging and doctor checks can call it safely.
import Foundation
import HelperMac

if CommandLine.arguments.dropFirst().contains("--version") {
    print(HelperApp.version)
    exit(0)
}
HelperApp.main()
