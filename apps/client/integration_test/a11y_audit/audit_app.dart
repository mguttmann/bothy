// SPDX-License-Identifier: Apache-2.0

// The entry of the platform accessibility audits (brief M5-10 Design note 3): an app that
// shows one phone screen with the fixtures of the widget tests, so the audit of the platform
// (`performAccessibilityAudit` on the iOS simulator, the Accessibility Test Framework on
// Android) checks what VoiceOver and TalkBack receive from that screen. The native tests
// choose the screen as the initial route `/<screen>` of the engine: on iOS the launch argument
// `--route=/<screen>`, which the engine reads from the process arguments (the environment is
// empty for Dart on iOS), on Android the intent extra `route` of the activity. It lives in a test
// folder, so no release build contains it; it needs no server and opens no port.

import 'dart:io' show Platform;
import 'dart:ui' show PlatformDispatcher;

import 'package:client/app/form_factor.dart';
import 'package:design_system/testing.dart' show dsPhoneA11yHost;
import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';

import 'audit_screens.dart';

/// Shows the screen the native test asked for, or the list of the screens' names when the name
/// is unknown (so a typo fails the native test, which waits for the screen's text).
void main() {
  WidgetsFlutterBinding.ensureInitialized();
  // The tree must exist before an audit asks for it: the audit tools are no screen reader
  // the engine would turn the semantics on for.
  SemanticsBinding.instance.ensureSemantics();
  final name = PlatformDispatcher.instance.defaultRouteName.replaceFirst(
    '/',
    '',
  );
  final screen = auditScreens[name];
  runApp(
    dsPhoneA11yHost(
      screen == null
          ? Text('unknown screen "$name": ${auditScreens.keys.join(', ')}')
          : screen(),
      platform: Platform.isAndroid
          ? TargetPlatform.android
          : TargetPlatform.iOS,
      wrap: (child) =>
          FormFactorScope(formFactor: FormFactor.phone, child: child),
    ),
  );
}
