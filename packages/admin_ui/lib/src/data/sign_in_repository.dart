// SPDX-License-Identifier: Apache-2.0

/// The own sign-in methods: the linked identities, the passkeys and the password with its
/// one-time code, and the calls that set them up (docs/spec/08-identity.md 2.5 and 3.3; design
/// note docs/design/sign-in-methods.md; plan row V1-14).
library;

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/json_object.dart';
import 'package:dio/dio.dart';

import 'self_repository.dart' show SelfActionFailed;

/// A linked identity of the person (`Identity` of `getMyCredentials`).
final class SelfIdentity {
  /// Creates the identity.
  const SelfIdentity({
    required this.id,
    required this.providerId,
    required this.providerKind,
    required this.subject,
    required this.linkedAt,
    this.issuer,
  });

  /// `id`.
  final String id;

  /// `provider_id`.
  final String providerId;

  /// `provider_kind`: `local`, `oidc` or `trusted_header`.
  final String providerKind;

  /// `subject`: the login name of a local identity, the subject at the login service otherwise.
  final String subject;

  /// `issuer` of an OIDC identity, or null.
  final String? issuer;

  /// `linked_at`.
  final DateTime linkedAt;
}

/// A passkey of the person (`Passkey` of `getMyCredentials`).
final class SelfPasskey {
  /// Creates the passkey.
  const SelfPasskey({
    required this.id,
    required this.name,
    required this.createdAt,
    this.backupEligible = false,
  });

  /// `id`.
  final String id;

  /// `name`.
  final String name;

  /// `created_at`.
  final DateTime createdAt;

  /// `backup_eligible`: a passkey that a password manager or a cloud keychain may copy.
  final bool backupEligible;
}

/// What `getMyCredentials` says about the person.
final class SelfCredentials {
  /// Creates the overview.
  const SelfCredentials({
    required this.identities,
    required this.passkeys,
    required this.passwordSet,
    required this.totpEnrolled,
  });

  /// The linked identities.
  final List<SelfIdentity> identities;

  /// The active passkeys.
  final List<SelfPasskey> passkeys;

  /// `local_password_set`.
  final bool passwordSet;

  /// `totp_enrolled`: the one-time code is confirmed, so the password path is set up (08 2.5).
  final bool totpEnrolled;

  /// Whether the person has an identity at the `local` driver; only then can a password be set
  /// (`setMyLocalCredential` answers `409 conflict` otherwise).
  bool get hasLocalIdentity => identities.any((i) => i.providerKind == 'local');
}

/// Creates a passkey on the platform over the WebAuthn creation options [options]
/// (`PublicKeyCredentialCreationOptions` in JSON form) and returns the `PublicKeyCredential` in
/// JSON form, or null when the person cancelled or no authenticator answered.
typedef PasskeyCreator = Future<Map<String, Object?>?> Function(
  Map<String, Object?> options,
);

/// The own sign-in methods and the calls that set them up.
///
/// The changing calls carry the purpose text the step-up dialog shows when the server asks for
/// a step-up (`401 step_up_required`): `local_credential.set` and `passkey.register` unless the
/// session has nothing to confirm with (05, `conditional`).
///
/// Errors: [SelfActionFailed] for an answer of the server or no answer, [StepUpCancelled] when
/// the person cancels the step-up.
abstract interface class SignInRepository {
  /// The identities, passkeys and the state of the password (`getMyCredentials`).
  Future<SelfCredentials> credentials();

  /// Sets the password ([password], null keeps it) and, with [enrollTotp], starts a new
  /// one-time code setup (`setMyLocalCredential`); returns the otpauth URI of the new secret,
  /// which the server sends only this once, or null without [enrollTotp].
  Future<String?> setPassword(
    String? password, {
    required bool enrollTotp,
    required String purpose,
  });

  /// Confirms the one-time code setup with the current [code] of the authenticator app
  /// (`setMyLocalCredential` with `totp_code`).
  Future<void> confirmTotp(String code, {required String purpose});

  /// Registers a passkey named [name] (`beginPasskeyRegistration`, [create] on the platform,
  /// `completePasskeyRegistration`); returns null when [create] reports a cancel.
  Future<SelfPasskey?> addPasskey(
    String name, {
    required PasskeyCreator create,
    required String purpose,
  });
}

/// [SignInRepository] over the generated client of `api_client`.
///
/// The `api` argument is the client of the signed-in session (the app's `AuthSession.api` or
/// the console's client), so DPoP, refresh and the step-up apply.
///
/// Example:
/// ```dart
/// final repository = ApiSignInRepository(session.api);
/// final overview = await repository.credentials();
/// ```
final class ApiSignInRepository implements SignInRepository {
  /// Creates the repository.
  ApiSignInRepository(this._api);

  final ApiClient _api;

  MeApi get _me => _api.getMeApi();

  @override
  Future<SelfCredentials> credentials() => _call(() async {
    final o = (await _me.getMyCredentials()).data!;
    return SelfCredentials(
      identities: [
        for (final i in o.identities)
          SelfIdentity(
            id: i.id,
            providerId: i.providerId,
            providerKind: switch (i.providerKind) {
              IdentityProviderKind.local => 'local',
              IdentityProviderKind.oidc => 'oidc',
              IdentityProviderKind.trustedHeader => 'trusted_header',
              _ => 'unknown',
            },
            subject: i.subject,
            issuer: i.issuer,
            linkedAt: i.linkedAt,
          ),
      ],
      passkeys: [for (final p in o.passkeys) _passkey(p)],
      passwordSet: o.localPasswordSet,
      totpEnrolled: o.totpEnrolled,
    );
  });

  @override
  Future<String?> setPassword(
    String? password, {
    required bool enrollTotp,
    required String purpose,
  }) => _call(() async {
    final r = await _me.setMyLocalCredential(
      localCredentialSet: LocalCredentialSet((b) {
        if (password != null) b.password = password;
        if (enrollTotp) b.totpEnroll = true;
      }),
      extra: {stepUpPurposeExtra: purpose},
    );
    return r.data!.totpSecretUri;
  });

  @override
  Future<void> confirmTotp(String code, {required String purpose}) =>
      _call(() async {
        await _me.setMyLocalCredential(
          localCredentialSet: LocalCredentialSet(
            (b) => b.totpCode = code.replaceAll(RegExp(r'\s'), ''),
          ),
          extra: {stepUpPurposeExtra: purpose},
        );
      });

  @override
  Future<SelfPasskey?> addPasskey(
    String name, {
    required PasskeyCreator create,
    required String purpose,
  }) => _call(() async {
    final extra = {stepUpPurposeExtra: purpose};
    final begun = (await _me.beginPasskeyRegistration(extra: extra)).data!;
    final credential = await create({
      for (final e in begun.publicKey.entries) e.key: e.value?.value,
    });
    if (credential == null) return null;
    final r = await _me.completePasskeyRegistration(
      passkeyRegisterComplete: PasskeyRegisterComplete(
        (b) => b
          ..challengeId = begun.challengeId
          ..name = name
          ..credential.replace(
            BuiltMap<String, JsonObject?>.of({
              for (final e in credential.entries)
                e.key: e.value == null ? null : JsonObject(e.value),
            }),
          ),
      ),
      extra: extra,
    );
    return _passkey(r.data!);
  });

  static SelfPasskey _passkey(Passkey p) => SelfPasskey(
    id: p.id,
    name: p.name,
    createdAt: p.createdAt,
    backupEligible: p.backupEligible,
  );

  /// Runs [call] and turns its errors into [SelfActionFailed]; an error of the login layer
  /// ([StepUpCancelled], a lost session) passes as it is.
  static Future<T> _call<T>(Future<T> Function() call) async {
    try {
      return await call();
    } on DioException catch (e) {
      final cause = e.error;
      if (cause is AuthException) throw cause;
      final response = e.response;
      throw SelfActionFailed(
        ApiProblem.of(e) ??
            (response == null ? null : ApiProblem.fromResponse(response)),
      );
    }
  }
}
