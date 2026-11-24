// SPDX-License-Identifier: Apache-2.0

import 'dart:convert';

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart' show StepUpCancelled, stepUpPurposeExtra;
import 'package:console/console.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/fakes.dart';

const _challengeId = '0192f4a0-7b1c-747c-835d-13eca158d3c0';
const _providerId = '0192f4a0-7b1c-7474-8636-4dbb076ee0db';

/// A prompt that answers with [answer] and records what it was shown.
class FakePrompt implements ConsoleStepUpPrompt {
  FakePrompt(this.answer);

  ConsoleStepUpAnswer? answer;
  final List<String> purposes = [];
  final List<ConsoleStepUpWays> ways = [];

  @override
  Future<ConsoleStepUpAnswer?> ask(String purpose, ConsoleStepUpWays w) async {
    purposes.add(purpose);
    ways.add(w);
    return answer;
  }
}

/// The API of a console whose action step-up uses [prompt] over [b].
class Harness {
  /// The console's clock: two minutes before the `expires_at` of the fake challenge.
  DateTime now = DateTime.utc(2026, 10, 10, 9, 33);

  Harness(
    this.b,
    this.prompt, {
    this.methods = const [StepUpMethod.passkey, StepUpMethod.passwordTotp],
    this.active = true,
    bool passkeyOptions = true,
  }) {
    b.server.routes['POST /api/v1/auth/step-up/challenge'] = (_) =>
        Answer(200, {
          'challenge_id': _challengeId,
          'challenge': 'Zm9vYmFy',
          if (passkeyOptions)
            'passkey_options': {
              'challenge': 'Zm9vYmFy',
              'rpId': 'chat.example.com',
              'userVerification': 'required',
            },
          'expires_at': '2026-10-10T09:35:00.000Z',
        });
    b.server.routes['POST /api/v1/auth/step-up/passkey'] = (_) =>
        const Answer(200, stepUpResult);
    b.server.routes['POST /api/v1/auth/step-up/password'] = (_) =>
        const Answer(200, stepUpResult);
    late final ApiClient client;
    stepUp = ConsoleActionStepUp(
      api: () => client,
      passkeys: b.passkeys,
      methods: () => methods,
      oidcProviders: () => const [
        ConsoleProvider(
          id: _providerId,
          kind: ConsoleProviderKind.oidc,
          name: 'Company SSO',
        ),
      ],
      prompt: () => prompt,
      onOidc: oidc.add,
      onConfirmed: () async => confirmed++,
      clock: () => now,
    );
    client = createConsoleApiClient(
      origin: Uri.parse('https://chat.example.com'),
      adapter: b.server,
      onUnauthorized: (p) => unauthorized.add(p.code),
      interceptors: [
        ConsoleStepUpInterceptor(
          dio: () => client.dio,
          stepUp: stepUp.confirm,
          active: () => active,
        ),
      ],
    );
    api = client;
  }

  final TestBrowser b;
  final FakePrompt prompt;
  final List<StepUpMethod> methods;
  final bool active;
  final List<ConsoleProvider> oidc = [];
  final List<String> unauthorized = [];
  int confirmed = 0;
  late final ConsoleActionStepUp stepUp;
  late final ApiClient api;

  /// A changing request as the repositories of admin_ui send it.
  Future<Response<Object?>> createGroup({
    Map<String, Object?> extra = const {},
  }) => api.dio.post<Object?>(
    '/groups',
    data: {'name': 'Support Team'},
    options: Options(
      extra: {
        idempotencyKeyExtra: '0192f4a0-7b1c-7000-8000-000000000001',
        stepUpPurposeExtra: 'Create the group Support Team',
        ...extra,
      },
    ),
  );
}

/// Answers `401 step_up_required` [times] times, then `201`.
void stepUpFirst(FakeServer server, String route, {int times = 1}) {
  var left = times;
  server.routes[route] = (_) {
    if (left-- > 0) {
      return const Answer(401, {
        'code': 'step_up_required',
        'status': 401,
        'title': 'Step-up required',
      });
    }
    return const Answer(201, {'id': '0192f4a0-7b1c-7000-8000-0000000000aa'});
  };
}

void main() {
  /// A password step-up over [h] whose prompt answers after [delay] on the console's clock;
  /// returns the number of challenges and the challenge the confirmation used.
  Future<(int, Object?)> passwordStepUp(Duration delay) async {
    final b = TestBrowser();
    late final Harness h;
    final prompt = _DelayedPrompt(
      const StepUpWithPassword('secret-1', ' 492039 '),
      () => h.now = h.now.add(delay),
    );
    h = Harness(
      b,
      prompt,
      methods: const [StepUpMethod.passwordTotp],
      passkeyOptions: false,
    );
    var issued = 0;
    b.server.routes['POST /api/v1/auth/step-up/challenge'] = (_) {
      issued++;
      return Answer(200, {
        'challenge_id': '0192f4a0-7b1c-747c-835d-13eca158d3c$issued',
        'challenge': 'Zm9vYmFy',
        'expires_at': '2026-10-10T09:35:00.000Z',
      });
    };
    stepUpFirst(b.server, 'POST /api/v1/groups');
    await h.createGroup();
    final confirm = b.server.to('POST', '/api/v1/auth/step-up/password').single;
    return (issued, (confirm.body! as Map)['challenge_id']);
  }

  test(
    'a slow answer confirms over a challenge created after the dialog answered',
    () async {
      // fn-sign-in-devices-15: a person who needs longer than auth.step_up_challenge_ttl in the
      // dialog must not confirm over an expired challenge.
      final (issued, used) = await passwordStepUp(const Duration(seconds: 90));
      expect(issued, 2);
      expect(used, '0192f4a0-7b1c-747c-835d-13eca158d3c2');
    },
  );

  test(
    'a quick answer confirms over the first challenge, without a second one',
    () async {
      // Every challenge counts in rl.public_auth of the client address (burst 10): two per
      // step-up ran the administration suites of console-e2e into 429.
      final (issued, used) = await passwordStepUp(const Duration(seconds: 5));
      expect(issued, 1);
      expect(used, '0192f4a0-7b1c-747c-835d-13eca158d3c1');
    },
  );

  test(
    'a step_up_required that ended the stored view repeats without a dialog',
    () async {
      // console-terminal-ui-10: the end of a view is the server's answer, not the browser's clock.
      final b = TestBrowser();
      final prompt = FakePrompt(const StepUpWithPasskey());
      var viewEnded = 0;
      late final ApiClient client;
      client = createConsoleApiClient(
        origin: Uri.parse('https://chat.example.com'),
        adapter: b.server,
        onUnauthorized: (_) {},
        interceptors: [
          ConsoleStepUpInterceptor(
            dio: () => client.dio,
            stepUp: (_) async => prompt.ask(
              '',
              const ConsoleStepUpWays(passkey: true, password: false, oidc: []),
            ),
            active: () => true,
            viewEnded: () async {
              viewEnded++;
              return true;
            },
          ),
        ],
      );
      stepUpFirst(b.server, 'POST /api/v1/groups');
      final r = await client.dio.post<Object?>('/groups', data: {'name': 'x'});
      expect(r.statusCode, 201);
      expect(viewEnded, 1);
      expect(prompt.purposes, isEmpty);
      expect(b.server.to('POST', '/api/v1/groups'), hasLength(2));
    },
  );

  test('a step_up_required action runs the passkey step-up and repeats once with the same key', () async {
    final h = Harness(TestBrowser(), FakePrompt(const StepUpWithPasskey()));
    stepUpFirst(h.b.server, 'POST /api/v1/groups');
    final r = await h.createGroup();
    expect(r.statusCode, 201);
    expect(h.prompt.purposes, ['Create the group Support Team']);
    expect(h.prompt.ways.single.passkey, isTrue);
    final sent = h.b.server.to('POST', '/api/v1/groups');
    expect(sent, hasLength(2));
    expect(
      sent[0].headers['idempotency-key'],
      sent[1].headers['idempotency-key'],
    );
    expect(sent[1].headers['x-console-request'], '1');
    expect(sent[1].body, {'name': 'Support Team'});
    final complete = h.b.server
        .to('POST', '/api/v1/auth/step-up/passkey')
        .single;
    expect((complete.body! as Map)['challenge_id'], _challengeId);
    // WebAuthn got the server's options; the port itself asks userVerification required.
    expect(h.b.passkeys.calls.single['challenge'], 'Zm9vYmFy');
    expect(h.confirmed, 1);
  });

  test(
    'a streamed download that needs a step-up runs it instead of logging out',
    () async {
      // The audit export asks for a stream; the step-up of its read can end between the check
      // entry and the download (a CI run of A6 was logged out with http_401 that way).
      final h = Harness(TestBrowser(), FakePrompt(const StepUpWithPasskey()));
      stepUpFirst(h.b.server, 'GET /api/v1/audit/export');
      final r = await h.api.dio.get<ResponseBody>(
        '/audit/export',
        options: Options(
          responseType: ResponseType.stream,
          extra: {stepUpPurposeExtra: 'Export the audit log'},
        ),
      );
      expect(r.statusCode, 201);
      expect(await utf8.decodeStream(r.data!.stream), contains('"id"'));
      expect(h.prompt.purposes, ['Export the audit log']);
      expect(h.b.server.to('GET', '/api/v1/audit/export'), hasLength(2));
      expect(h.unauthorized, ['step_up_required']);
    },
  );

  test('two requests that need a step-up at once share one prompt', () async {
    final h = Harness(TestBrowser(), FakePrompt(const StepUpWithPasskey()));
    stepUpFirst(h.b.server, 'POST /api/v1/groups', times: 2);
    final both = await Future.wait([h.createGroup(), h.createGroup()]);
    expect(both.map((r) => r.statusCode), [201, 201]);
    expect(h.prompt.purposes, hasLength(1));
    expect(h.b.server.to('POST', '/api/v1/auth/step-up/passkey'), hasLength(1));
    expect(h.b.server.to('POST', '/api/v1/groups'), hasLength(4));
  });

  test('without a passkey the prompt offers the password with TOTP', () async {
    final h = Harness(
      TestBrowser(),
      FakePrompt(const StepUpWithPassword('pw-Aa1!', ' 492039 ')),
      passkeyOptions: false,
    );
    stepUpFirst(h.b.server, 'POST /api/v1/groups');
    expect((await h.createGroup()).statusCode, 201);
    final ways = h.prompt.ways.single;
    expect(ways.passkey, isFalse);
    expect(ways.password, isTrue);
    expect(h.b.server.to('POST', '/api/v1/auth/step-up/password').single.body, {
      'challenge_id': _challengeId,
      'password': 'pw-Aa1!',
      'totp_code': '492039',
    });
    expect(h.b.passkeys.calls, isEmpty);
  });

  test(
    'a cancelled prompt sends nothing more and ends with StepUpCancelled',
    () async {
      final h = Harness(TestBrowser(), FakePrompt(null));
      stepUpFirst(h.b.server, 'POST /api/v1/groups');
      final error = await h.createGroup().then<Object?>(
        (_) => null,
        onError: (Object e) => e,
      );
      expect(error, isA<DioException>());
      expect((error! as DioException).error, isA<StepUpCancelled>());
      expect(h.b.server.to('POST', '/api/v1/groups'), hasLength(1));
      expect(h.b.server.to('POST', '/api/v1/auth/step-up/passkey'), isEmpty);
      expect(h.confirmed, 0);
    },
  );

  test(
    'a cancelled passkey prompt of the browser counts as a cancel',
    () async {
      final b = TestBrowser()..passkeys.failWith = const PasskeyCancelled();
      final h = Harness(b, FakePrompt(const StepUpWithPasskey()));
      stepUpFirst(b.server, 'POST /api/v1/groups');
      final error = await h.createGroup().then<Object?>(
        (_) => null,
        onError: (Object e) => e,
      );
      expect((error! as DioException).error, isA<StepUpCancelled>());
      expect(b.server.to('POST', '/api/v1/groups'), hasLength(1));
    },
  );

  test(
    'only OIDC: the tab goes to the re-login and the change is not repeated',
    () async {
      final h = Harness(
        TestBrowser(),
        FakePrompt(
          const StepUpWithOidc(
            ConsoleProvider(
              id: _providerId,
              kind: ConsoleProviderKind.oidc,
              name: 'Company SSO',
            ),
          ),
        ),
        methods: const [StepUpMethod.oidcReauth],
        passkeyOptions: false,
      );
      stepUpFirst(h.b.server, 'POST /api/v1/groups');
      final error = await h.createGroup().then<Object?>(
        (_) => null,
        onError: (Object e) => e,
      );
      expect((error! as DioException).error, isA<StepUpCancelled>());
      expect(h.oidc.single.id, _providerId);
      expect(h.prompt.ways.single.oidc.single.name, 'Company SSO');
      expect(h.b.server.to('POST', '/api/v1/groups'), hasLength(1));
    },
  );

  test(
    'a second step_up_required after the step-up goes to the caller',
    () async {
      final h = Harness(TestBrowser(), FakePrompt(const StepUpWithPasskey()));
      stepUpFirst(h.b.server, 'POST /api/v1/groups', times: 5);
      final error = await h.createGroup().then<Object?>(
        (_) => null,
        onError: (Object e) => e,
      );
      expect((error! as DioException).response?.statusCode, 401);
      expect(h.b.server.to('POST', '/api/v1/groups'), hasLength(2));
      expect(h.prompt.purposes, hasLength(1));
    },
  );

  test(
    'a refused confirmation ends the request with the server answer',
    () async {
      final h = Harness(TestBrowser(), FakePrompt(const StepUpWithPasskey()));
      h.b.server.routes['POST /api/v1/auth/step-up/passkey'] = (_) =>
          const Answer(401, {'code': 'step_up_required', 'status': 401});
      stepUpFirst(h.b.server, 'POST /api/v1/groups');
      final error = await h.createGroup().then<Object?>(
        (_) => null,
        onError: (Object e) => e,
      );
      expect((error! as DioException).response?.statusCode, 401);
      expect(h.b.server.to('POST', '/api/v1/groups'), hasLength(1));
      // The refused confirmation itself never opened a second prompt.
      expect(h.prompt.purposes, hasLength(1));
    },
  );

  test(
    'requests marked for the caller and the auth operations pass unchanged',
    () async {
      final h = Harness(TestBrowser(), FakePrompt(const StepUpWithPasskey()));
      stepUpFirst(h.b.server, 'POST /api/v1/groups');
      final marked = await h
          .createGroup(extra: {consoleNoStepUpExtra: true})
          .then<Object?>((_) => null, onError: (Object e) => e);
      expect((marked! as DioException).response?.statusCode, 401);
      stepUpFirst(h.b.server, 'POST /api/v1/auth/logout');
      final auth = await h.api.dio
          .post<Object?>('/auth/logout')
          .then<Object?>((_) => null, onError: (Object e) => e);
      expect((auth! as DioException).response?.statusCode, 401);
      expect(h.prompt.purposes, isEmpty);
    },
  );

  test('outside a signed-in console the adapter does nothing', () async {
    final h = Harness(
      TestBrowser(),
      FakePrompt(const StepUpWithPasskey()),
      active: false,
    );
    stepUpFirst(h.b.server, 'POST /api/v1/groups');
    final error = await h.createGroup().then<Object?>(
      (_) => null,
      onError: (Object e) => e,
    );
    expect((error! as DioException).response?.statusCode, 401);
    expect(h.prompt.purposes, isEmpty);
  });
}

/// A prompt that runs [onAsk] (the time the person needs) before it answers.
class _DelayedPrompt extends FakePrompt {
  _DelayedPrompt(super.answer, this.onAsk);

  final void Function() onAsk;

  @override
  Future<ConsoleStepUpAnswer?> ask(String purpose, ConsoleStepUpWays w) {
    onAsk();
    return super.ask(purpose, w);
  }
}
