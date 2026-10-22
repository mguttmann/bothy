// SPDX-License-Identifier: Apache-2.0

/// The step-up of an administration action in the browser (brief M6-04 Design note 2,
/// docs/spec/08-identity.md 6.2, docs/decisions.d/M6-04.md D-M6-04-2): the confirmation dialog of
/// `admin_ui` comes first and sends the request; a `401 step_up_required` answer starts the
/// console's step-up, and after its success the request repeats once with the same
/// `Idempotency-Key`. A cancel sends nothing more.
library;

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart' show StepUpCancelled, stepUpPurposeExtra;
import 'package:built_collection/built_collection.dart';
import 'package:built_value/json_object.dart';
import 'package:dio/dio.dart';

import '../platform/browser.dart';
import '../session/console_controller.dart';

/// Key of `RequestOptions.extra`: `true` leaves a `step_up_required` answer of this request to
/// its caller (the console's own session reads, for example right after a stored view ended).
const String consoleNoStepUpExtra = 'console.no_step_up';

/// Marks a request the adapter already repeated, so a second `step_up_required` goes to the
/// caller instead of into a loop.
const String _stepUpRetried = 'console.step_up_retried';

/// The login and step-up operations: their `401` answers belong to their own flows.
const String _authPrefix = '${ApiClient.basePath}/auth/';

/// The ways a person can confirm an action, from the challenge and the session's
/// `step_up_methods_available` (08 6.2).
final class ConsoleStepUpWays {
  /// Creates the ways.
  const ConsoleStepUpWays({
    required this.passkey,
    required this.password,
    required this.oidc,
  });

  /// The person has a passkey: the challenge carries WebAuthn options.
  final bool passkey;

  /// Password with TOTP.
  final bool password;

  /// The OIDC providers for a new login.
  final List<ConsoleProvider> oidc;
}

/// The choice of the person in the step-up prompt.
sealed class ConsoleStepUpAnswer {
  const ConsoleStepUpAnswer();
}

/// Confirm with the passkey (WebAuthn with `userVerification = required`).
final class StepUpWithPasskey extends ConsoleStepUpAnswer {
  /// Creates the answer.
  const StepUpWithPasskey();
}

/// Confirm with password and the TOTP one-time code.
final class StepUpWithPassword extends ConsoleStepUpAnswer {
  /// Creates the answer with the typed [password] and [code].
  const StepUpWithPassword(this.password, this.code);

  /// The password as typed.
  final String password;

  /// The one-time code; surrounding spaces are removed before it is sent.
  final String code;
}

/// Confirm with a new OIDC login at [provider]. The tab leaves for the login service, so the
/// action is not repeated; the person makes the change again after the return.
final class StepUpWithOidc extends ConsoleStepUpAnswer {
  /// Creates the answer.
  const StepUpWithOidc(this.provider);

  /// The provider of the new login.
  final ConsoleProvider provider;
}

/// The dialog that explains the step-up and lets the person choose; the console shows it over
/// its navigator (`step_up_dialog.dart`), tests pass a fake.
abstract interface class ConsoleStepUpPrompt {
  /// Shows [purpose] (already in the person's language, may be empty) with [ways] and returns
  /// the choice, or null when the person cancels.
  Future<ConsoleStepUpAnswer?> ask(String purpose, ConsoleStepUpWays ways);
}

/// Runs one step-up of the console session for an action.
///
/// The order follows Design note 2: a passkey when the person has one (the challenge carries
/// `passkey_options`), otherwise the other methods of the session. Every request of the
/// step-up itself carries [consoleNoStepUpExtra], so a refused confirmation never opens a
/// second prompt.
///
/// Example:
/// ```dart
/// final stepUp = ConsoleActionStepUp(api: () => api, passkeys: env.passkeys, ...);
/// await stepUp.confirm('Create the group Support Team');
/// ```
class ConsoleActionStepUp {
  /// Creates the runner. [api] is the console's API, [methods] and [oidcProviders] read the
  /// current session and discovery, [prompt] returns the dialog (null when no window is there,
  /// which cancels), [onOidc] starts the OIDC re-login of the session and [onConfirmed] runs
  /// after a successful step-up (the console reads its session again).
  ConsoleActionStepUp({
    required this.api,
    required this.passkeys,
    required this.methods,
    required this.oidcProviders,
    required this.prompt,
    required this.onOidc,
    required this.onConfirmed,
  });

  /// The console's API.
  final ApiClient Function() api;

  /// The browser's WebAuthn API.
  final PasskeyPort passkeys;

  /// `step_up_methods_available` of the session.
  final List<StepUpMethod> Function() methods;

  /// The OIDC providers of discovery.
  final List<ConsoleProvider> Function() oidcProviders;

  /// The dialog, or null.
  final ConsoleStepUpPrompt? Function() prompt;

  /// Starts the OIDC re-login of the session at a provider (the tab leaves).
  final void Function(ConsoleProvider provider) onOidc;

  /// Runs after a successful step-up.
  final Future<void> Function() onConfirmed;

  static const Map<String, Object?> _own = {consoleNoStepUpExtra: true};

  Future<void>? _running;

  /// Runs one step-up for an action whose purpose is [purpose].
  ///
  /// Throws [StepUpCancelled] when the person cancels, the browser offers no passkey, no window
  /// can show the prompt or the person chose the OIDC re-login; the `DioException` of a refused
  /// challenge or confirmation otherwise.
  ///
  /// While one step-up runs, a second request that needs one waits for it instead of opening a
  /// second dialog (a page often loads several things at once); after a cancelled or failed
  /// first step-up the waiting request ends with the same error.
  Future<void> confirm(String purpose) {
    final running = _running;
    if (running != null) return running;
    final next = _confirm(purpose);
    _running = next;
    return next.whenComplete(() => _running = null);
  }

  Future<void> _confirm(String purpose) async {
    final dialog = prompt();
    if (dialog == null) throw const StepUpCancelled();
    final auth = api().getAuthApi();
    final challenge = (await auth.createStepUpChallenge(extra: _own)).data!;
    final available = methods();
    final options = challenge.passkeyOptions;
    final ways = ConsoleStepUpWays(
      passkey: options != null && available.contains(StepUpMethod.passkey),
      password:
          options == null && available.contains(StepUpMethod.passwordTotp),
      oidc: options == null && available.contains(StepUpMethod.oidcReauth)
          ? oidcProviders()
          : const [],
    );
    final answer = await dialog.ask(purpose, ways);
    switch (answer) {
      case null:
        throw const StepUpCancelled();
      case StepUpWithPasskey():
        if (options == null) throw const StepUpCancelled();
        final Map<String, Object?> credential;
        try {
          credential = await passkeys.get(_plain(options));
        } on PasskeyCancelled {
          throw const StepUpCancelled();
        }
        await auth.completeStepUpWithPasskey(
          stepUpPasskeyRequest: StepUpPasskeyRequest(
            (b) => b
              ..challengeId = challenge.challengeId
              ..credential.replace(_json(credential)),
          ),
          extra: _own,
        );
      case StepUpWithPassword(:final password, :final code):
        await auth.stepUpWithPassword(
          stepUpPasswordRequest: StepUpPasswordRequest(
            (b) => b
              ..challengeId = challenge.challengeId
              ..password = password
              ..totpCode = code.trim(),
          ),
          extra: _own,
        );
      case StepUpWithOidc(:final provider):
        onOidc(provider);
        throw const StepUpCancelled();
    }
    await onConfirmed();
  }

  static BuiltMap<String, JsonObject?> _json(Map<String, Object?> map) =>
      BuiltMap<String, JsonObject?>({
        for (final e in map.entries)
          e.key: e.value == null ? null : JsonObject(e.value),
      });

  static Map<String, Object?> _plain(BuiltMap<String, JsonObject?> map) => {
    for (final e in map.entries) e.key: e.value?.value,
  };
}

/// On `401 step_up_required` of an administration request runs [stepUp] once and repeats the
/// request with the same options: the same `Idempotency-Key` (from the request's extra), the
/// same body and the CSRF header again (08 6.2; 06 7).
///
/// Untouched: requests while [active] is false (before the console is signed in), the login
/// and step-up operations below `/api/v1/auth/`, requests with [consoleNoStepUpExtra] and a
/// request the adapter already repeated. A failed or cancelled step-up ends the request with
/// that error ([StepUpCancelled] inside a `DioException`, which `admin_ui` treats as a cancel).
class ConsoleStepUpInterceptor extends Interceptor {
  /// Creates the adapter over [dio], the Dio of the console's API.
  ConsoleStepUpInterceptor({
    required this.dio,
    required this.stepUp,
    required this.active,
  });

  /// The Dio that repeats the request.
  final Dio Function() dio;

  /// Runs one step-up for the purpose text of the request.
  final Future<void> Function(String purpose) stepUp;

  /// Whether the console is signed in with its first step-up.
  final bool Function() active;

  @override
  Future<void> onError(
    DioException err,
    ErrorInterceptorHandler handler,
  ) async {
    final options = err.requestOptions;
    final response = err.response;
    if (response == null ||
        response.statusCode != 401 ||
        options.extra[consoleNoStepUpExtra] == true ||
        options.extra[_stepUpRetried] == true ||
        options.uri.path.startsWith(_authPrefix) ||
        !active() ||
        ApiProblem.fromResponse(response).code != 'step_up_required') {
      return handler.next(err);
    }
    options.extra[_stepUpRetried] = true;
    try {
      await stepUp(options.extra[stepUpPurposeExtra] as String? ?? '');
    } on DioException catch (e) {
      return handler.next(e);
    } on Object catch (e) {
      return handler.next(
        DioException(requestOptions: options, error: e, message: '$e'),
      );
    }
    final data = options.data;
    if (data is FormData) {
      options.data = data.clone();
    } else if (data is Stream) {
      // A streamed body is gone after the first attempt; the caller repeats.
      return handler.next(err);
    }
    try {
      handler.resolve(await dio().fetch<Object?>(options));
    } on DioException catch (e) {
      handler.next(e);
    }
  }
}
