// SPDX-License-Identifier: Apache-2.0

import 'package:auth/auth.dart' show StepUpCancelled;
import 'package:console/console.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/fakes.dart';
import 'terminal_fakes.dart';

const _challengeId = '0192f4a0-7b1c-747c-835d-13eca158d3c0';

const _passkeyOptions = {
  'challenge': 'Y2hhbGxlbmdl',
  'rpId': 'chat.example.com',
  'userVerification': 'preferred',
  'allowCredentials': [
    {'type': 'public-key', 'id': 'Y3JlZA'},
  ],
};

/// The step-up of the terminal over [b] with the session's [methods].
({PasskeyStepUpPrompt prompt, FakeSetupNotice notice, FakeStepUpPrompt dialog})
_prompt(
  TestBrowser b, {
  List<StepUpMethod> methods = const [
    StepUpMethod.passkey,
    StepUpMethod.passwordTotp,
    StepUpMethod.oidcReauth,
  ],
  bool withOptions = true,
}) {
  b.server.routes['POST /api/v1/auth/step-up/challenge'] = (_) => Answer(200, {
    'challenge_id': _challengeId,
    'challenge': 'Y2hhbGxlbmdl',
    'expires_at': '2026-10-10T09:35:00.000Z',
    if (withOptions) 'passkey_options': _passkeyOptions,
  });
  b.server.routes['POST /api/v1/auth/step-up/passkey'] = (_) =>
      const Answer(200, stepUpResult);
  final controller = ConsoleController(b.environment);
  final notice = FakeSetupNotice();
  final dialog = FakeStepUpPrompt();
  final prompt = PasskeyStepUpPrompt(
    api: () => controller.api,
    passkeys: b.passkeys,
    methods: () => methods,
    prompt: () => dialog,
    showSetupNotice: notice.show,
  );
  return (prompt: prompt, notice: notice, dialog: dialog);
}

void main() {
  group('PasskeyStepUpPrompt', () {
    test(
      'without a passkey in the session it shows the setup notice and cancels',
      () async {
        final b = TestBrowser();
        final p = _prompt(
          b,
          methods: const [StepUpMethod.passwordTotp, StepUpMethod.oidcReauth],
        );
        await expectLater(
          p.prompt.confirm(purposeText: 'Open a terminal on Build host'),
          throwsA(isA<StepUpCancelled>()),
        );
        expect(p.notice.shown, 1);
        // No challenge, no password or OIDC way, no browser prompt.
        expect(b.server.to('POST', '/api/v1/auth/step-up/challenge'), isEmpty);
        expect(p.dialog.asked, isEmpty);
        expect(b.passkeys.calls, isEmpty);
      },
    );

    test(
      'a challenge without passkey options shows the notice and cancels',
      () async {
        final b = TestBrowser();
        final p = _prompt(b, withOptions: false);
        await expectLater(
          p.prompt.confirm(purposeText: 'Open a terminal on Build host'),
          throwsA(isA<StepUpCancelled>()),
        );
        expect(p.notice.shown, 1);
        expect(p.dialog.asked, isEmpty);
        expect(b.passkeys.calls, isEmpty);
        expect(b.server.to('POST', '/api/v1/auth/step-up/password'), isEmpty);
      },
    );

    test(
      'with a passkey it asks the browser with userVerification required',
      () async {
        final b = TestBrowser();
        final p = _prompt(b);
        await p.prompt.confirm(purposeText: 'Open a terminal on Build host');
        expect(p.notice.shown, 0);
        // The dialog offers the passkey and nothing else, with the controller's purpose.
        expect(p.dialog.asked, hasLength(1));
        final (purpose, ways) = p.dialog.asked.single;
        expect(purpose, 'Open a terminal on Build host');
        expect(ways.passkey, isTrue);
        expect(ways.password, isFalse);
        expect(ways.oidc, isEmpty);
        expect(b.passkeys.calls, hasLength(1));
        expect(b.passkeys.calls.single['userVerification'], 'required');
        expect(b.passkeys.calls.single['challenge'], 'Y2hhbGxlbmdl');
        final complete = b.server.to('POST', '/api/v1/auth/step-up/passkey');
        expect(complete, hasLength(1));
        expect((complete.single.body! as Map)['challenge_id'], _challengeId);
        expect(b.server.to('POST', '/api/v1/auth/step-up/password'), isEmpty);
      },
    );

    test('a cancelled browser prompt cancels the step-up', () async {
      final b = TestBrowser();
      b.passkeys.failWith = const PasskeyCancelled();
      final p = _prompt(b);
      await expectLater(
        p.prompt.confirm(purposeText: 'Attach to the terminal on Build host'),
        throwsA(isA<StepUpCancelled>()),
      );
      expect(b.server.to('POST', '/api/v1/auth/step-up/passkey'), isEmpty);
    });

    test('a cancelled dialog sends nothing to the browser', () async {
      final b = TestBrowser();
      final p = _prompt(b);
      p.dialog.answer = null;
      await expectLater(
        p.prompt.confirm(purposeText: 'Open a terminal on Build host'),
        throwsA(isA<StepUpCancelled>()),
      );
      expect(b.passkeys.calls, isEmpty);
    });
  });

  testWidgets('the setup notice names the way to a passkey', (tester) async {
    final key = GlobalKey<NavigatorState>();
    await tester.pumpWidget(dsApp(key));
    final notice = DialogPasskeySetupNotice(() => key.currentContext);
    final shown = notice.show();
    await tester.pumpAndSettle();
    expect(
      find.byKey(const ValueKey('console.terminal.passkeySetup')),
      findsOneWidget,
    );
    expect(find.textContaining('needs a passkey'), findsOneWidget);
    await tester.tap(
      find.byKey(const ValueKey('console.terminal.passkeySetup.close')),
    );
    await tester.pumpAndSettle();
    await shown;
    expect(
      find.byKey(const ValueKey('console.terminal.passkeySetup')),
      findsNothing,
    );
  });
}
