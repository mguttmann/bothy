// SPDX-License-Identifier: Apache-2.0

// The frame of the audit tests (brief M4-29 Tests): the audit view needs a bounded height, so
// it runs in a box of the window's height; the fake repository is the administration fake with
// its audit log.

import 'dart:async';

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';

export '../admin/support.dart';

/// A save target that keeps what the export writes, as a file would.
class MemoryTarget {
  /// The bytes written.
  final List<int> bytes = [];

  /// The file names asked for.
  final List<String> names = [];

  /// Whether the sink was closed.
  bool closed = false;

  /// The target to pass to the view.
  Future<StreamConsumer<List<int>>?> call(String name) async {
    names.add(name);
    final controller = StreamController<List<int>>();
    controller.stream.listen(bytes.addAll, onDone: () => closed = true);
    return controller;
  }
}

/// Pumps the audit view over [repository] in [timeZone].
Future<void> pumpAudit(
  WidgetTester tester,
  FakeAdminRepository repository, {
  String timeZone = 'Europe/Berlin',
  AuditSaveTarget? saveTarget,
  GlobalKey<NavigatorState>? navigatorKey,
  Locale locale = const Locale('en'),
}) => pumpInApp(
  tester,
  SizedBox(
    height: 900,
    child: AuditLogView(
      repository: repository,
      timeZone: timeZone,
      clock: testClock,
      saveTarget: saveTarget,
    ),
  ),
  navigatorKey: navigatorKey,
  locale: locale,
);
