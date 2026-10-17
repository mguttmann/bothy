// SPDX-License-Identifier: Apache-2.0

// Codes (01 5.6 and 8; brief M4-18 Design note 6): grouped, in the mono font, copyable, read
// character by character; pasted input with spaces, hyphens and lower case is normalized.

import 'package:client/onboarding/code_text.dart';
import 'package:client/onboarding/e3_enrollment_code.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/instances.dart';
import '../support/widgets.dart';

void main() {
  test('groupCode and spellCode ignore spaces and hyphens', () {
    expect(groupCode('HQ7P3MXR', 4), ['HQ7P', '3MXR']);
    expect(groupCode('481 207', 3), ['481', '207']);
    expect(groupCode('ABCDE', 4), ['ABCD', 'E']);
    expect(spellCode('48-12 07'), '4, 8, 1, 2, 0, 7');
  });

  testWidgets(
    'a code is grouped in the mono font and spelled for screen readers',
    (tester) async {
      final semantics = tester.ensureSemantics();
      await tester.pumpWidget(
        hostScreen(const Center(child: CodeText('HQ7P3MXR'))),
      );
      final text = tester.widget<Text>(
        find.byKey(const ValueKey('code.groups')),
      );
      expect(text.data, 'HQ7P  3MXR');
      expect(text.style?.fontFamily, DsType.code.fontFamily);
      expect(
        find.bySemanticsLabel(en.onboardingCodeLabel('H, Q, 7, P, 3, M, X, R')),
        findsOneWidget,
      );
      expect(find.bySemanticsLabel(RegExp('HQ7P')), findsNothing);
      semantics.dispose();
    },
  );

  testWidgets('the code can be copied without its spaces', (tester) async {
    final copied = <String>[];
    tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
      SystemChannels.platform,
      (call) async {
        if (call.method == 'Clipboard.setData') {
          copied.add((call.arguments as Map)['text'] as String);
        }
        return null;
      },
    );
    await tester.pumpWidget(
      hostScreen(const Center(child: CodeText('481 207', groupSize: 3))),
    );
    await tester.tap(find.byKey(const ValueKey('code.copy')));
    await tester.pump();
    expect(copied, ['481207']);
    expect(find.text(en.onboardingCopied), findsOneWidget);
    tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
      SystemChannels.platform,
      null,
    );
  });

  testWidgets('a pasted enrollment code is normalized before it is sent', (
    tester,
  ) async {
    desktopWindow(tester);
    final sent = <(String, EnrollMethod)>[];
    await tester.pumpWidget(
      hostScreen(
        EnrollmentCodeScreen(
          instance: oidcOnly,
          passkeysAvailable: false,
          onRedeem: (code, method) => sent.add((code, method)),
          onBack: () {},
        ),
      ),
    );
    await tester.enterText(
      find.byKey(const ValueKey('e3.code')),
      'en_ 7k2m-9qxa',
    );
    await tester.tap(find.byKey(const ValueKey('e3.continue')));
    expect(sent.single.$1, 'en_7K2M9QXA');
    expect(sent.single.$2, isA<EnrollThisDevice>());
  });
}
