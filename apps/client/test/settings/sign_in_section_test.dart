// SPDX-License-Identifier: Apache-2.0

// The platform side of the section "Sign-in" (V1-14): a cancelled or failed system passkey
// prompt reads as "no passkey created"; the link flow's outcome maps to the view's outcomes
// with the step-up purpose naming the service, and an answer of the server becomes
// SelfActionFailed while a cancelled step-up passes as it is.

import 'dart:ui' show Locale;

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:auth/auth.dart';
import 'package:client/settings/sign_in_section.dart';
import 'package:dio/dio.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

class _Passkeys implements PasskeyAuthenticator {
  Object? error;

  @override
  Future<Map<String, Object?>> createCredential(
    Map<String, Object?> creationOptions,
  ) async {
    final e = error;
    if (e != null) throw e;
    return {'id': 'cred', 'options': creationOptions};
  }

  @override
  Future<Map<String, Object?>> getAssertion(
    Map<String, Object?> requestOptions,
  ) => throw UnimplementedError();
}

void main() {
  final l = lookupAppLocalizations(const Locale('en'));
  const company = SignInProvider(id: 'p1', name: 'Company login');

  test('a passkey of the platform, and a cancelled or failed prompt', () async {
    final passkeys = _Passkeys();
    final create = platformPasskeyCreator(passkeys);
    expect(await create({'challenge': 'Zm9v'}), {
      'id': 'cred',
      'options': {'challenge': 'Zm9v'},
    });
    passkeys.error = PlatformException(code: 'cancelled');
    expect(await create({}), isNull);
    passkeys.error = PlatformException(code: 'failed', message: 'NotAllowed');
    expect(await create({}), isNull);
  });

  test('the outcome of the link flow and the purpose of its step-up', () async {
    final purposes = <String>[];
    String? reason;
    final link = appIdentityLinker((id, {required purposeText}) async {
      purposes.add('$id $purposeText');
      return reason;
    }, l);
    expect(await link(company), IdentityLinkOutcome.linked);
    reason = 'cancelled';
    expect(await link(company), IdentityLinkOutcome.cancelled);
    reason = 'flow_expired';
    expect(await link(company), IdentityLinkOutcome.failed);
    expect(purposes.first, 'p1 Link your account with Company login');
  });

  test(
    'an answer of the server is SelfActionFailed, a cancelled step-up passes',
    () async {
      final options = RequestOptions(path: '/me/identities/begin');
      Object error = DioException(
        requestOptions: options,
        error: const ApiProblem(status: 409, code: 'conflict'),
      );
      final link = appIdentityLinker(
        (id, {required purposeText}) async => throw error,
        l,
      );
      await expectLater(
        link(company),
        throwsA(
          isA<SelfActionFailed>().having(
            (e) => e.problem?.code,
            'code',
            'conflict',
          ),
        ),
      );
      error = DioException(
        requestOptions: options,
        error: const StepUpCancelled(),
      );
      await expectLater(link(company), throwsA(isA<StepUpCancelled>()));
    },
  );
}
