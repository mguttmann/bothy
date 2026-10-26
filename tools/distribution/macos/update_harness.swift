// SPDX-License-Identifier: Apache-2.0
//
// Update harness of the desktop distribution trial (brief M7-12, Design note 2): drives Sparkle's
// own updater (`SPUUpdater` of the pinned Sparkle.framework) against an installed app bundle,
// the way Sparkle's command line tool does, without any user interface. It is no part of the
// app; tools/distribution/apple.py compiles it with `swiftc` against the framework of the pinned
// Sparkle release and runs it once per trial.
//
// Usage: update_harness <app bundle> [--channel <name>]...
// It checks the feed of the bundle (SUFeedURL of its Info.plist) for the given channels, takes
// every update Sparkle offers, and exits after Sparkle has handed the archive to its installer:
// 0 when an update was found and its installation started, 4 when the feed offers no newer
// version, 1 on any updater error (printed on stderr). The installer replaces the bundle in a
// separate process after this one exits; the caller waits for the new version on disk.

import Foundation
import Sparkle

/// A user driver that answers every question of the updater without a window: install the
/// update, install now, never ask for permission.
@MainActor
final class HeadlessUserDriver: NSObject, SPUUserDriver {
  func show(
    _ request: SPUUpdatePermissionRequest, reply: @escaping (SUUpdatePermissionResponse) -> Void
  ) {
    reply(SUUpdatePermissionResponse(automaticUpdateChecks: false, sendSystemProfile: false))
  }

  func showUserInitiatedUpdateCheck(cancellation: @escaping () -> Void) {
    log("checking for updates")
  }

  func showUpdateFound(
    with appcastItem: SUAppcastItem, state: SPUUserUpdateState,
    reply: @escaping (SPUUserUpdateChoice) -> Void
  ) {
    log("found version \(appcastItem.versionString) (\(appcastItem.displayVersionString))"
      + " channel \(appcastItem.channel ?? "default")")
    reply(appcastItem.isInformationOnlyUpdate ? .dismiss : .install)
  }

  func showUpdateReleaseNotes(with downloadData: SPUDownloadData) {}

  func showUpdateReleaseNotesFailedToDownloadWithError(_ error: Error) {}

  func showUpdateNotFoundWithError(_ error: Error, acknowledgement: @escaping () -> Void) {
    acknowledgement()
  }

  func showUpdaterError(_ error: Error, acknowledgement: @escaping () -> Void) {
    log("updater error: \(error.localizedDescription)")
    acknowledgement()
  }

  func showDownloadInitiated(cancellation: @escaping () -> Void) {
    log("downloading")
  }

  func showDownloadDidReceiveExpectedContentLength(_ expectedContentLength: UInt64) {}

  func showDownloadDidReceiveData(ofLength length: UInt64) {}

  func showDownloadDidStartExtractingUpdate() {
    log("extracting")
  }

  func showExtractionReceivedProgress(_ progress: Double) {}

  func showReady(toInstallAndRelaunch reply: @escaping (SPUUserUpdateChoice) -> Void) {
    log("installing")
    reply(.install)
  }

  func showInstallingUpdate(
    withApplicationTerminated applicationTerminated: Bool,
    retryTerminatingApplication: @escaping () -> Void
  ) {
    log("installer started")
  }

  func showUpdateInstalledAndRelaunched(_ relaunched: Bool, acknowledgement: @escaping () -> Void) {
    log("installed")
    acknowledgement()
  }

  func dismissUpdateInstallation() {}
}

/// The updater delegate: the channels of the run, no permission prompt, and the exit code at
/// the end of the update cycle.
@MainActor
final class HarnessDelegate: NSObject, SPUUpdaterDelegate {
  let channels: Set<String>

  init(channels: Set<String>) {
    self.channels = channels
  }

  func allowedChannels(for updater: SPUUpdater) -> Set<String> {
    channels
  }

  func updaterShouldPromptForPermissionToCheck(forUpdates updater: SPUUpdater) -> Bool {
    false
  }

  func updater(
    _ updater: SPUUpdater, didFinishUpdateCycleFor updateCheck: SPUUpdateCheck, error: Error?
  ) {
    guard let error = error as NSError? else {
      log("update cycle finished")
      exit(0)
    }
    if error.domain == SUSparkleErrorDomain && error.code == Int(SUError.noUpdateError.rawValue) {
      log("no update found")
      exit(4)
    }
    log("update failed: \(error.domain) \(error.code) \(error.localizedDescription)")
    exit(1)
  }
}

/// Writes one line to stderr, so stdout stays free for a caller that parses it.
func log(_ line: String) {
  FileHandle.standardError.write(Data("update_harness: \(line)\n".utf8))
}

@MainActor
func run() {
  var args = Array(CommandLine.arguments.dropFirst())
  guard !args.isEmpty else {
    log("usage: update_harness <app bundle> [--channel <name>]...")
    exit(2)
  }
  let path = args.removeFirst()
  var channels = Set<String>()
  while !args.isEmpty {
    let flag = args.removeFirst()
    guard flag == "--channel", !args.isEmpty else {
      log("unknown argument \(flag)")
      exit(2)
    }
    channels.insert(args.removeFirst())
  }
  guard let bundle = Bundle(path: path) else {
    log("no bundle at \(path)")
    exit(2)
  }
  let driver = HeadlessUserDriver()
  let delegate = HarnessDelegate(channels: channels)
  let updater = SPUUpdater(
    hostBundle: bundle, applicationBundle: bundle, userDriver: driver, delegate: delegate)
  do {
    try updater.start()
  } catch {
    log("updater did not start: \(error.localizedDescription)")
    exit(1)
  }
  updater.checkForUpdates()
  // Driver and delegate stay alive with the run loop; the delegate ends the process.
  withExtendedLifetime((driver, delegate, updater)) {
    RunLoop.main.run()
  }
}

MainActor.assumeIsolated {
  run()
}
