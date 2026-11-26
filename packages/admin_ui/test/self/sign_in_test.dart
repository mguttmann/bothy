// SPDX-License-Identifier: Apache-2.0

// The area "Sign-in" (V1-14, docs/design/sign-in-methods.md): what getMyCredentials says, the
// password with the one-time code in two steps (QR code and key, a wrong code keeps the dialog,
// the purpose text of the step-up, a mismatch and a too short password), the password change
// alone, a new code, "Add passkey" over the platform and its cancel, the hint without a passkey
// port, "Link with" only for unlinked services and each outcome, a cancelled step-up that says
// nothing, and a failed load with its retry.

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart' show StepUpCancelled;
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

const _uri = 'otpauth://totp/chat.example.com:alex?secret=JBSWY3DPEHPK3PXP';

class FakeSignIn implements SignInRepository {
  FakeSignIn({this.local = true});

  final bool local;
  bool passwordSet = false;
  bool totp = false;
  List<SelfPasskey> passkeys = [];
  List<SelfIdentity> linked = [];
  final List<String> calls = [];
  final List<String> purposes = [];
  Object? fail;
  int loads = 0;
  bool loadFails = false;

  @override
  Future<SelfCredentials> credentials() async {
    loads++;
    if (loadFails) throw const SelfActionFailed(null);
    return SelfCredentials(
      identities: [
        if (local)
          SelfIdentity(
            id: 'i-local',
            providerId: 'p-local',
            providerKind: 'local',
            subject: 'alex',
            linkedAt: DateTime.utc(2026, 10, 1),
          ),
        ...linked,
      ],
      passkeys: passkeys,
      passwordSet: passwordSet,
      totpEnrolled: totp,
    );
  }

  void _maybeFail() {
    final f = fail;
    if (f != null) {
      fail = null;
      throw f;
    }
  }

  @override
  Future<String?> setPassword(
    String? password, {
    required bool enrollTotp,
    required String purpose,
  }) async {
    calls.add('set ${password ?? '-'} ${enrollTotp ? 'enroll' : ''}'.trim());
    purposes.add(purpose);
    _maybeFail();
    if (password != null) passwordSet = true;
    return enrollTotp ? _uri : null;
  }

  @override
  Future<void> confirmTotp(String code, {required String purpose}) async {
    calls.add('confirm $code');
    purposes.add(purpose);
    if (code != '492039') {
      throw const SelfActionFailed(
        ApiProblem(
          status: 400,
          code: 'validation_failed',
          errors: [
            ProblemFieldError(pointer: '/totp_code', code: 'totp_invalid'),
          ],
        ),
      );
    }
    totp = true;
  }

  @override
  Future<SelfPasskey?> addPasskey(
    String name, {
    required PasskeyCreator create,
    required String purpose,
  }) async {
    calls.add('passkey $name');
    purposes.add(purpose);
    _maybeFail();
    final credential = await create({'challenge': 'Zm9v'});
    if (credential == null) return null;
    final p = SelfPasskey(
      id: 'pk-${passkeys.length}',
      name: name,
      createdAt: testNow,
    );
    passkeys = [...passkeys, p];
    return p;
  }
}

void main() {
  late FakeSignIn repository;

  setUp(() => repository = FakeSignIn());

  Future<void> pump(
    WidgetTester tester, {
    bool passwordEnabled = true,
    PasskeyCreator? create,
    String? noPasskeyHint,
    IdentityLinker? link,
  }) => pumpInApp(
    tester,
    MySignInView(
      repository: repository,
      timeZone: 'Europe/Berlin',
      clock: testClock,
      passwordEnabled: passwordEnabled,
      createPasskey: create,
      noPasskeyHint: noPasskeyHint,
      providers: const [
        SignInProvider(id: 'p-company', name: 'Company login'),
        SignInProvider(id: 'p-other', name: 'Other login'),
      ],
      linkIdentity: link,
    ),
  );

  Future<void> tap(WidgetTester tester, String key) async {
    await tester.tap(find.byKey(ValueKey(key)));
    await tester.pumpAndSettle();
  }

  Future<void> type(WidgetTester tester, String key, String text) async {
    await tester.enterText(find.byKey(ValueKey(key)), text);
    await tester.pump();
  }

  testWidgets('a first password and one-time code in two steps', (
    tester,
  ) async {
    await pump(tester);
    expect(find.text('No local password'), findsOneWidget);
    await tap(tester, 'signIn.password.set');
    await type(tester, 'signIn.password.new', 'correct horse');
    await type(tester, 'signIn.password.repeat', 'correct hors');
    await tap(tester, 'signIn.password.submit');
    expect(find.text('The two passwords are not the same.'), findsOneWidget);
    expect(repository.calls, isEmpty);

    await type(tester, 'signIn.password.repeat', 'correct horse');
    await tap(tester, 'signIn.password.submit');
    expect(repository.calls, ['set correct horse enroll']);
    expect(find.byKey(const ValueKey('signIn.totp.qr')), findsOneWidget);
    expect(find.text('Key: JBSW Y3DP EHPK 3PXP'), findsOneWidget);

    await type(tester, 'signIn.totp.code', '111111');
    await tap(tester, 'signIn.password.submit');
    expect(
      find.text('The code was not accepted. Enter the code the app shows now.'),
      findsOneWidget,
    );
    expect(find.byKey(const ValueKey('signIn.totp.qr')), findsOneWidget);

    await type(tester, 'signIn.totp.code', '492039');
    await tap(tester, 'signIn.password.submit');
    expect(find.byKey(const ValueKey('signIn.password.dialog')), findsNothing);
    expect(
      find.text('Your password and the one-time code are set up.'),
      findsOneWidget,
    );
    expect(find.text('Local password set'), findsOneWidget);
    expect(find.text('One-time code app set up'), findsOneWidget);
    expect(repository.purposes.toSet(), {'Set the password of your account'});
  });

  testWidgets('a too short password stays in the dialog with its rule', (
    tester,
  ) async {
    repository.fail = const SelfActionFailed(
      ApiProblem(
        status: 400,
        code: 'validation_failed',
        errors: [ProblemFieldError(pointer: '/password', code: 'too_short')],
      ),
    );
    await pump(tester);
    await tap(tester, 'signIn.password.set');
    await type(tester, 'signIn.password.new', 'short');
    await type(tester, 'signIn.password.repeat', 'short');
    await tap(tester, 'signIn.password.submit');
    expect(find.text('The password is too short.'), findsOneWidget);
    expect(find.byKey(const ValueKey('signIn.totp.qr')), findsNothing);
  });

  testWidgets('a cancelled step-up keeps the dialog and says nothing', (
    tester,
  ) async {
    repository.fail = const StepUpCancelled();
    await pump(tester);
    await tap(tester, 'signIn.password.set');
    await type(tester, 'signIn.password.new', 'correct horse');
    await type(tester, 'signIn.password.repeat', 'correct horse');
    await tap(tester, 'signIn.password.submit');
    expect(find.byKey(const ValueKey('signIn.password.error')), findsNothing);
    expect(find.byKey(const ValueKey('signIn.password.new')), findsOneWidget);
  });

  testWidgets(
    'with a confirmed code: change the password alone or a new code',
    (tester) async {
      repository
        ..passwordSet = true
        ..totp = true;
      await pump(tester);
      expect(find.byKey(const ValueKey('signIn.password.set')), findsNothing);
      await tap(tester, 'signIn.password.change');
      await type(tester, 'signIn.password.new', 'new horse');
      await type(tester, 'signIn.password.repeat', 'new horse');
      await tap(tester, 'signIn.password.submit');
      expect(repository.calls, ['set new horse']);
      expect(find.text('Your password was changed.'), findsOneWidget);

      await tap(tester, 'signIn.totp.new');
      expect(find.byKey(const ValueKey('signIn.password.new')), findsNothing);
      await tap(tester, 'signIn.password.submit');
      expect(repository.calls.last, 'set - enroll');
      expect(find.byKey(const ValueKey('signIn.totp.qr')), findsOneWidget);
    },
  );

  testWidgets('a cancel after the first step shows the saved password', (
    tester,
  ) async {
    await pump(tester);
    await tap(tester, 'signIn.password.set');
    await type(tester, 'signIn.password.new', 'correct horse');
    await type(tester, 'signIn.password.repeat', 'correct horse');
    await tap(tester, 'signIn.password.submit');
    expect(find.byKey(const ValueKey('signIn.totp.qr')), findsOneWidget);
    await tap(tester, 'signIn.password.cancel');
    expect(find.text('Local password set'), findsOneWidget);
    expect(find.byKey(const ValueKey('signIn.totp.finish')), findsOneWidget);
  });

  testWidgets('no password part without a local identity or local passwords', (
    tester,
  ) async {
    repository = FakeSignIn(local: false);
    await pump(tester);
    expect(find.byKey(const ValueKey('signIn.password.state')), findsNothing);
    repository = FakeSignIn();
    await pump(tester, passwordEnabled: false);
    expect(find.byKey(const ValueKey('signIn.password.state')), findsNothing);
  });

  testWidgets('add a passkey over the platform, and a cancel', (tester) async {
    var created = 0;
    Map<String, Object?>? options;
    var answer = true;
    await pump(
      tester,
      create: (o) async {
        created++;
        options = o;
        return answer ? {'id': 'cred'} : null;
      },
    );
    expect(find.byKey(const ValueKey('signIn.passkeys.none')), findsOneWidget);
    await tap(tester, 'signIn.passkey.add');
    await type(tester, 'signIn.passkey.name', '  Work laptop ');
    await tap(tester, 'signIn.passkey.submit');
    expect(created, 1);
    expect(options, {'challenge': 'Zm9v'});
    expect(repository.calls, ['passkey Work laptop']);
    expect(repository.purposes, ['Add a passkey to your account']);
    expect(find.text('The passkey Work laptop was added.'), findsOneWidget);
    expect(find.byKey(const ValueKey('signIn.passkey.pk-0')), findsOneWidget);

    answer = false;
    await tap(tester, 'signIn.passkey.add');
    await type(tester, 'signIn.passkey.name', 'Phone');
    await tap(tester, 'signIn.passkey.submit');
    expect(find.text('No passkey was created.'), findsOneWidget);
  });

  testWidgets('without a passkey port the hint shows instead of the button', (
    tester,
  ) async {
    await pump(tester, noPasskeyHint: 'Add one in the web console.');
    expect(find.byKey(const ValueKey('signIn.passkey.add')), findsNothing);
    expect(find.text('Add one in the web console.'), findsOneWidget);
  });

  testWidgets('link only unlinked services and show each outcome', (
    tester,
  ) async {
    repository.linked = [
      SelfIdentity(
        id: 'i-other',
        providerId: 'p-other',
        providerKind: 'oidc',
        subject: 'alex@example.com',
        issuer: 'https://login.example.com',
        linkedAt: DateTime.utc(2026, 10, 2),
      ),
    ];
    var outcome = IdentityLinkOutcome.failed;
    final asked = <String>[];
    await pump(
      tester,
      link: (p) async {
        asked.add(p.id);
        return outcome;
      },
    );
    expect(find.byKey(const ValueKey('signIn.link.p-other')), findsNothing);
    expect(find.textContaining('Other login'), findsOneWidget);

    await tap(tester, 'signIn.link.p-company');
    expect(find.text('The link was not made. Try again.'), findsOneWidget);
    outcome = IdentityLinkOutcome.cancelled;
    await tap(tester, 'signIn.link.p-company');
    expect(
      find.text('The link was cancelled at the login service.'),
      findsOneWidget,
    );
    outcome = IdentityLinkOutcome.linked;
    await tap(tester, 'signIn.link.p-company');
    expect(
      find.text('Your account is now linked with Company login.'),
      findsOneWidget,
    );
    expect(asked, ['p-company', 'p-company', 'p-company']);
  });

  testWidgets('a failed load offers to try again', (tester) async {
    repository.loadFails = true;
    await pump(tester);
    expect(find.byKey(const ValueKey('signIn.loadFailed')), findsOneWidget);
    repository.loadFails = false;
    await tester.tap(find.text('Try again'));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('signIn.loadFailed')), findsNothing);
    expect(repository.loads, 2);
  });

  test('totpSecretOf groups the key and ignores a URI without one', () {
    expect(totpSecretOf(_uri), 'JBSW Y3DP EHPK 3PXP');
    expect(totpSecretOf('otpauth://totp/x?secret=ABCDEF'), 'ABCD EF');
    expect(totpSecretOf('otpauth://totp/x'), isNull);
  });
}
