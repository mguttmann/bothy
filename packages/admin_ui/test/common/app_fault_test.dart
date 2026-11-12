// SPDX-License-Identifier: Apache-2.0

// A fault of the app in a screen of the administration (ruling AUD-AM1/3): an error that is
// neither an answer of the server nor an error of the login layer is no server error. The two
// readings every screen uses (`loadProblem` after a load, `selfFailure` after an action, with
// `adminFailure` and `runChange` on top) hand it to the reporter of the package, once, and say
// that the app failed; the default reporter is the error handler of the app. A problem of the
// server shows its correlation ID where only the server's log can tell more.

import 'package:admin_ui/admin_ui.dart';
import 'package:admin_ui/src/self/self_feedback.dart';
import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import '../admin/support.dart';

const String _correlation = '4bf92f35-77b3-4da6-a3ce-929d0e0e4736';

/// What a wrong cast in the app throws.
Object _typeError() {
  try {
    // ignore: unnecessary_cast
    return (Object() as dynamic) as String;
  } on TypeError catch (e) {
    return e;
  }
}

/// A repository whose group list fails in the code of the app.
class _BrokenGroups extends FakeAdminRepository {
  _BrokenGroups() : super(prompt: RecordingPrompt());

  @override
  Future<AdminPage<AdminGroup>> groups({String? cursor}) async =>
      throw StateError('no element');
}

void main() {
  final l = lookupAppLocalizations(const Locale('en'));
  late List<(Object, StackTrace)> reported;

  setUp(() {
    reported = [];
    final before = adminFaultReporter;
    adminFaultReporter = (error, stack) => reported.add((error, stack));
    addTearDown(() => adminFaultReporter = before);
  });

  group('a fault of the app', () {
    test('after a load: reported once, and not called a server error', () {
      final error = _typeError();
      final problem = loadProblem(error)!;
      expect(problem.code, appFaultCode);
      expect(problem.status, isNot(500));
      // A screen that keeps the error reads it again at every build.
      loadProblem(error);
      expect(reported, hasLength(1));
      expect(reported.single.$1, same(error));
      // The stack of the place that threw, which an `Error` carries.
      expect(reported.single.$2, same((error as Error).stackTrace));
    });

    test('after an action: reported, and the app is named', () {
      final error = StateError('no element');
      final failure = selfFailure(l, error)!;
      expect(failure.text, 'Something went wrong in the app');
      expect(failure.text, isNot(l.problemInternalError));
      expect(failure.denied, isFalse);
      expect(adminFailure(l, error)!.text, failure.text);
      expect(reported, hasLength(1));
      expect(reported.single.$1, same(error));
    });

    test('an exception without a stack gets the one of the reading', () {
      const error = FormatException('ImportSource.id is missing');
      selfFailure(l, error);
      expect(reported.single.$1, same(error));
      expect('${reported.single.$2}', contains('app_fault_test.dart'));
    });

    test(
      'an answer of the server and an error of the login layer are none',
      () {
        const refused = SelfActionFailed(
          ApiProblem(status: 403, code: 'forbidden'),
        );
        for (final Object error in [
          refused,
          const SelfActionFailed(null),
          const SelfActionFailed(
            ApiProblem(status: 500, code: 'internal_error'),
          ),
          const StepUpCancelled(),
          const StepUpUnavailable(),
          const RefreshUnavailable(),
          const SessionEnded(SignOutReason.sessionRevoked),
        ]) {
          expect(isAppFault(error), isFalse, reason: '$error');
          selfFailure(l, error);
          loadProblem(error);
        }
        expect(reported, isEmpty);
        expect(
          loadProblem(
            const SelfActionFailed(
              ApiProblem(status: 500, code: 'internal_error'),
            ),
          )!.code,
          'internal_error',
        );
      },
    );

    test('the default reporter is the error handler of the app', () {
      final seen = <FlutterErrorDetails>[];
      final before = FlutterError.onError;
      FlutterError.onError = seen.add;
      addTearDown(() => FlutterError.onError = before);
      adminFaultReporter = reportFaultToFlutter;

      final error = _typeError();
      loadProblem(error);

      expect(seen, hasLength(1));
      expect(seen.single.exception, same(error));
      expect(seen.single.stack, same((error as Error).stackTrace));
      expect(seen.single.library, 'admin_ui');
    });

    testWidgets(
      'a list that fails in the app says so and offers to try again',
      (tester) async {
        await pumpInApp(
          tester,
          GroupsList(repository: _BrokenGroups(), onOpen: (_) {}),
        );

        final banner = find.byKey(const ValueKey('admin.loadFailed'));
        expect(banner, findsOneWidget);
        expect(
          find.descendant(
            of: banner,
            matching: find.text('Something went wrong in the app'),
          ),
          findsOneWidget,
        );
        expect(find.text('Something went wrong on the server'), findsNothing);
        expect(find.text('Try again'), findsOneWidget);
        expect(reported, hasLength(1));
        expect(reported.single.$1, isStateError);
      },
    );

    testWidgets('a change that fails in the app says so in its notice', (
      tester,
    ) async {
      final repository = FakeAdminRepository(prompt: RecordingPrompt())
        ..stepUpFresh = true;
      await pumpInApp(tester, _Changer(repository: repository));
      await tester.tap(find.byKey(const ValueKey('changer.run')));
      await tester.pumpAndSettle();

      expect(find.text('Something went wrong in the app'), findsOneWidget);
      expect(reported.single.$1, isStateError);
    });
  });

  group('the correlation ID of the server', () {
    Future<void> banner(WidgetTester tester, ApiProblem problem) =>
        pumpInApp(tester, ProblemBanner(problem: problem));

    testWidgets('a server error shows it after its text', (tester) async {
      await banner(
        tester,
        const ApiProblem(
          status: 500,
          code: 'internal_error',
          correlationId: _correlation,
        ),
      );
      expect(
        find.text(
          'Something went wrong on the server (correlation ID $_correlation)',
        ),
        findsOneWidget,
      );
    });

    testWidgets('a code this build has no text for shows it too', (
      tester,
    ) async {
      await banner(
        tester,
        const ApiProblem(
          status: 200,
          code: 'http_200',
          correlationId: _correlation,
        ),
      );
      expect(
        find.text(
          'Something went wrong (http_200) (correlation ID $_correlation)',
        ),
        findsOneWidget,
      );
    });

    testWidgets('a refusal the person can read does not', (tester) async {
      await banner(
        tester,
        const ApiProblem(
          status: 403,
          code: 'forbidden',
          reason: 'owner_only',
          correlationId: _correlation,
        ),
      );
      expect(find.textContaining(_correlation), findsNothing);
      expect(find.text(l.problemForbiddenOwnerOnly), findsOneWidget);
    });

    test('after an action: with a server error, not with a conflict', () {
      expect(
        selfFailure(
          l,
          const SelfActionFailed(
            ApiProblem(
              status: 503,
              code: 'unavailable',
              correlationId: _correlation,
            ),
          ),
        )!.text,
        '${l.problemUnavailable} (correlation ID $_correlation)',
      );
      expect(
        selfFailure(
          l,
          const SelfActionFailed(
            ApiProblem(
              status: 409,
              code: 'conflict',
              correlationId: _correlation,
            ),
          ),
        )!.text,
        l.problemConflict,
      );
      // Without an ID the text stays as it is.
      expect(
        selfFailure(
          l,
          const SelfActionFailed(
            ApiProblem(status: 500, code: 'internal_error'),
          ),
        )!.text,
        l.problemInternalError,
      );
    });
  });
}

/// A screen with one change that fails in the app.
class _Changer extends StatefulWidget {
  const _Changer({required this.repository});

  final AdminRepository repository;

  @override
  State<_Changer> createState() => _ChangerState();
}

class _ChangerState extends State<_Changer> with AdminActionState<_Changer> {
  @override
  Widget build(BuildContext context) => Column(
    mainAxisSize: MainAxisSize.min,
    children: [
      noticeBanner(),
      TextButton(
        key: const ValueKey('changer.run'),
        onPressed: () => runChange('run', () async => throw StateError('x')),
        child: const Text('Run'),
      ),
    ],
  );
}
