// SPDX-License-Identifier: Apache-2.0

// Accessibility of the in-app banner of the phone (13 4.6 item 1; 01 8; docs/decisions.d/
// AUD-client-app-01.md): the hint of a message with its two names, and a bundled hint with
// long names, which must neither overflow nor lose its close button at text scale 2. The
// banner is one button that reads title and text and is announced when it appears, and a
// close button with a name.

import 'package:client/notifications/in_app_banner.dart';
import 'package:client/notifications/toast_content.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

void _noop() {}

Widget _scene(ToastContent content) => Align(
  alignment: Alignment.topCenter,
  child: Padding(
    padding: const EdgeInsets.all(DsSpace.s8),
    child: InAppBannerView(content: content, onOpen: _noop, onDismiss: _noop),
  ),
);

void main() {
  final open = find.byKey(const ValueKey('notification.banner.open'));
  final dismiss = find.byKey(const ValueKey('notification.banner.dismiss'));

  phoneScreenTests(
    'the in-app banner of a hint',
    build: () => _scene(
      const ToastContent(
        title: 'general · Release notes',
        body: 'Sam: Build is green',
      ),
    ),
    reachable: (l) => [open, dismiss],
    semantics: (tester, l) {
      // One node reads both lines, is a button and announces itself when it appears.
      expect(
        tester.getSemantics(open),
        isSemantics(
          isButton: true,
          isLiveRegion: true,
          hasTapAction: true,
          label: 'general · Release notes\nSam: Build is green',
        ),
      );
      expect(
        tester.getSemantics(dismiss),
        isSemantics(
          isButton: true,
          hasTapAction: true,
          label: l.notificationBannerDismiss,
        ),
      );
    },
  );

  phoneScreenTests(
    'the in-app banner of a bundled hint with long names',
    build: () => _scene(
      const ToastContent(
        title:
            'customer-support-escalations · Printer on the third floor '
            'prints only every second page since the update',
        body:
            '12 new messages\nSam: The service partner answered and wants the '
            'serial number, the firmware version and a photo of the test page',
      ),
    ),
    reachable: (l) => [open, dismiss],
  );
}
