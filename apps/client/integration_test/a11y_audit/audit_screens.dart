// SPDX-License-Identifier: Apache-2.0

// The phone screens of the platform accessibility audits (brief M5-10 Design note 3), by the
// names the native tests use (ios/RunnerUITests/AccessibilityAuditTests.swift,
// android/app/src/androidTest/.../AccessibilityChecksTest.kt): the scenes of the widget
// tests in test/a11y/phone.

import 'package:client/lock/lock_screen.dart';
import 'package:client/onboarding/e6_app_lock.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';

import '../../test/goldens/phone/onboarding_phone_golden_test.dart'
    as onboarding;
import '../../test/goldens/phone/run_graph_phone_golden_test.dart' as run_graph;
import '../../test/goldens/phone/voice_golden_test.dart' as voice;
import '../../test/goldens/share/share_sheet_golden_test.dart' as share;
import '../../test/phone/support/phone_scenes.dart';
import '../../test/phone/support/terminal_scenes.dart';

void _noop() {}

/// Whether the audit runs on Android (the native tests start this entry on the device).
bool get _android => defaultTargetPlatform == TargetPlatform.android;

/// [scene] once the host of the terminal scenes exists: its key pair comes from the platform's
/// crypto, which the synchronous scene builders cannot wait for. The scenes use the ports of
/// the tests, so no secure window hides the screen from the audit's screenshot.
Widget _terminal(Widget Function() scene) => FutureBuilder<void>(
  future: prepareTerminalScenes(),
  builder: (context, snapshot) =>
      snapshot.connectionState == ConnectionState.done
      ? scene()
      : const SizedBox.shrink(),
);

/// A screen with known gaps, so the native tests prove that their audit finds something: a
/// tappable square of 20 by 20 without a name or a role, and grey text on grey.
Widget _knownGaps() => Column(
  mainAxisSize: MainAxisSize.min,
  children: [
    const Text('Audit self-check'),
    GestureDetector(
      onTap: _noop,
      child: Container(width: 20, height: 20, color: const Color(0xFF777777)),
    ),
    Container(
      color: const Color(0xFF888888),
      child: const Text(
        'Low contrast text',
        style: TextStyle(color: Color(0xFF999999), fontSize: 11),
      ),
    ),
  ],
);

/// Every screen of the audits by name; `known_gaps` is the self-check of the audits.
final Map<String, Widget Function()> auditScreens = {
  'known_gaps': _knownGaps,
  'm1_spaces': spacesScene,
  'm2_post_list': postListScene,
  'm3_running': runningScene,
  'm3_collapsed': collapsedScene,
  'm4_components': componentsScene,
  'm5_recording': () => Center(child: voice.voiceScenes['locked']!()),
  'm6_share_sheet': () => share.shareSheetScene(refused: false),
  'm7_offline': offlineScene,
  // The terminal scenes take the platform's frame and ports, so each audit sees its own.
  'm8_terminal': () => _terminal(() => terminalScene(android: _android)),
  'm9_hosts': () => _terminal(() => hostsScene(android: _android)),
  'm11_running': run_graph.scenes['m11_running_portrait']!,
  'm12_run_graph': run_graph.scenes['m12_run_graph_portrait']!,
  'e1_server': onboarding.screens()['e1_idle']!,
  'e2_login': onboarding.screens()['e2_local_passkey_password']!,
  'e4_pending': onboarding.screens()['e4']!,
  'e5_pairing': onboarding.screens()['e5']!,
  'e6_lock': () => const AppLockScreen(
    productName: 'Team Chat',
    onUnlock: _noop,
    onLogout: _noop,
  ),
  'e6_lock_unavailable': () => const AppLockUnavailableScreen(
    productName: 'Team Chat',
    onCheckAgain: _noop,
    onLogout: _noop,
  ),
  'e7_signed_out': onboarding.screens()['e7_disabled']!,
};
