// SPDX-License-Identifier: Apache-2.0

/// Sign-in: the own sign-in methods in the settings of the desktop and the phone
/// (docs/spec/01-product.md 5.1 D8; 08 2.5, 3.3; design note docs/design/sign-in-methods.md).
library;

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:auth/auth.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart' show PlatformException;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import 'settings_providers.dart';
import 'settings_shell.dart';

/// The section "Sign-in": [MySignInView] over the REST source of the signed-in instance, the
/// passkey port of the platform (Windows Hello on Windows, the system on a phone build with a
/// public domain; none on a Mac, D-M4-18-9) and the login's browser and redirect for linking a
/// further login service.
class SignInSection extends ConsumerWidget {
  /// Creates the section; [clock] is the time the list shows relative days for.
  const SignInSection({super.key, this.clock = DateTime.now});

  /// The current time.
  final DateTime Function() clock;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final services = ref.watch(signedInServicesProvider);
    final zone = ref.watch(personTimeZoneProvider);
    final ports = ref.watch(appPortsProvider);
    return SectionFrame(
      title: l.settingsSectionSignIn,
      children: [
        if (services != null)
          MySignInView(
            key: ValueKey('signIn.${services.instance.instanceId}'),
            repository: ApiSignInRepository(services.auth.api),
            timeZone: zone,
            clock: clock,
            passwordEnabled: services.instance.passwordEnabled,
            createPasskey: switch (ports.passkeysFor(services.instance)) {
              final passkeys? => platformPasskeyCreator(passkeys),
              null => null,
            },
            noPasskeyHint: l.signInPasskeyUnavailable,
            providers: [
              for (final p in services.instance.providers)
                if (p.kind == 'oidc') SignInProvider(id: p.id, name: p.name),
            ],
            linkIdentity: appIdentityLinker(
              services.auth.login.linkIdentity,
              l,
            ),
          ),
      ],
    );
  }
}

/// [PasskeyCreator] over the passkey port of the platform: a cancelled or failed system prompt
/// ([PlatformException], for example a passkey this authenticator already holds) reads as "no
/// passkey was created".
PasskeyCreator platformPasskeyCreator(PasskeyAuthenticator passkeys) =>
    (options) async {
      try {
        return await passkeys.createCredential(options);
      } on PlatformException {
        return null;
      }
    };

/// The link flow of the login layer (`LoginService.linkIdentity`): null when linked, else the
/// reason.
typedef LinkFlow = Future<String?> Function(
  String providerId, {
  required String purposeText,
});

/// [IdentityLinker] over [link]: its outcome, and an answer of the server as [SelfActionFailed]
/// (a cancelled step-up or a lost session as its own error).
IdentityLinker appIdentityLinker(LinkFlow link, AppLocalizations l) =>
    (provider) async {
      final String? reason;
      try {
        reason = await link(
          provider.id,
          purposeText: l.stepUpPurposeLinkIdentity(provider.name),
        );
      } on DioException catch (e) {
        final cause = e.error;
        if (cause is AuthException) throw cause;
        final response = e.response;
        throw SelfActionFailed(
          ApiProblem.of(e) ??
              (response == null ? null : ApiProblem.fromResponse(response)),
        );
      }
      return switch (reason) {
        null => IdentityLinkOutcome.linked,
        'cancelled' => IdentityLinkOutcome.cancelled,
        _ => IdentityLinkOutcome.failed,
      };
    };
