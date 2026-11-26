// SPDX-License-Identifier: Apache-2.0

/// The JSON form of WebAuthn that the server speaks (binary members as base64url without
/// padding) and the binary form the browser API takes, kept apart from the browser so the
/// conversion runs on the test VM.
library;

import 'dart:convert';
import 'dart:typed_data';

/// Decodes base64url with or without padding.
///
/// Throws a [FormatException] for a value that is not base64url.
Uint8List base64UrlBytes(String value) =>
    base64Url.decode(value.padRight((value.length + 3) & ~3, '='));

/// Encodes [bytes] as base64url without padding, the form of the server.
String base64UrlText(List<int> bytes) =>
    base64Url.encode(bytes).replaceAll('=', '');

/// One entry of `allowCredentials`.
class AllowedCredential {
  /// Creates the entry.
  const AllowedCredential({required this.id, required this.transports});

  /// The credential ID.
  final Uint8List id;

  /// The transports the server stored, possibly empty.
  final List<String> transports;
}

/// `PublicKeyCredentialRequestOptions` in binary form.
class PasskeyRequest {
  /// Creates the request from its parts.
  const PasskeyRequest({
    required this.challenge,
    required this.allowCredentials,
    this.rpId,
    this.timeoutMs,
  });

  /// Reads the JSON form of the server. User verification is not read: the console always asks
  /// `required` (08 6.2), whatever the options say.
  ///
  /// Throws a [FormatException] when `challenge` is missing or not base64url.
  factory PasskeyRequest.fromJson(Map<String, Object?> json) {
    final challenge = json['challenge'];
    if (challenge is! String || challenge.isEmpty) {
      throw const FormatException('passkey options without a challenge');
    }
    final allow = <AllowedCredential>[];
    final list = json['allowCredentials'];
    if (list is List) {
      for (final entry in list) {
        if (entry is Map && entry['id'] is String) {
          allow.add(
            AllowedCredential(
              id: base64UrlBytes(entry['id'] as String),
              transports: [
                for (final t in (entry['transports'] as List?) ?? const [])
                  if (t is String) t,
              ],
            ),
          );
        }
      }
    }
    final timeout = json['timeout'];
    final rpId = json['rpId'];
    return PasskeyRequest(
      challenge: base64UrlBytes(challenge),
      allowCredentials: allow,
      rpId: rpId is String ? rpId : null,
      timeoutMs: timeout is num ? timeout.toInt() : null,
    );
  }

  /// The challenge to sign.
  final Uint8List challenge;

  /// The credentials the server allows; empty for discoverable credentials.
  final List<AllowedCredential> allowCredentials;

  /// The relying party ID, or null for the origin's host.
  final String? rpId;

  /// The timeout in milliseconds, or null.
  final int? timeoutMs;
}

/// The JSON form of an assertion (`PublicKeyCredential` with an
/// `AuthenticatorAssertionResponse`) that `completePasskeyLogin` and
/// `completeStepUpWithPasskey` take as `credential`.
Map<String, Object?> assertionJson({
  required String id,
  required List<int> rawId,
  required List<int> clientDataJson,
  required List<int> authenticatorData,
  required List<int> signature,
  List<int>? userHandle,
  String? authenticatorAttachment,
}) => {
  'id': id,
  'rawId': base64UrlText(rawId),
  'type': 'public-key',
  'authenticatorAttachment': ?authenticatorAttachment,
  'response': {
    'clientDataJSON': base64UrlText(clientDataJson),
    'authenticatorData': base64UrlText(authenticatorData),
    'signature': base64UrlText(signature),
    if (userHandle != null && userHandle.isNotEmpty)
      'userHandle': base64UrlText(userHandle),
  },
  'clientExtensionResults': <String, Object?>{},
};

/// One entry of `pubKeyCredParams`.
typedef CredentialParameter = ({String type, int alg});

/// `PublicKeyCredentialCreationOptions` in binary form, for registering a passkey
/// (`beginPasskeyRegistration`, V1-14).
class PasskeyCreation {
  /// Creates the options from their parts.
  const PasskeyCreation({
    required this.challenge,
    required this.rpName,
    required this.userId,
    required this.userName,
    required this.userDisplayName,
    required this.parameters,
    required this.excludeCredentials,
    this.rpId,
    this.timeoutMs,
    this.residentKey,
    this.authenticatorAttachment,
  });

  /// Reads the JSON form of the server. User verification is not read: the console always asks
  /// `required` (08 2.5), and attestation is always `none`.
  ///
  /// Throws a [FormatException] when `challenge` or `user.id` is missing or not base64url.
  factory PasskeyCreation.fromJson(Map<String, Object?> json) {
    final challenge = json['challenge'];
    if (challenge is! String || challenge.isEmpty) {
      throw const FormatException('passkey options without a challenge');
    }
    final rp = json['rp'] is Map
        ? json['rp']! as Map
        : const <String, Object?>{};
    final user = json['user'] is Map
        ? json['user']! as Map
        : const <String, Object?>{};
    final userId = user['id'];
    if (userId is! String || userId.isEmpty) {
      throw const FormatException('passkey options without a user id');
    }
    final parameters = <CredentialParameter>[
      for (final p in (json['pubKeyCredParams'] as List?) ?? const [])
        if (p is Map && p['alg'] is num)
          (
            type: p['type'] is String ? p['type']! as String : 'public-key',
            alg: (p['alg']! as num).toInt(),
          ),
    ];
    final exclude = <AllowedCredential>[
      for (final e in (json['excludeCredentials'] as List?) ?? const [])
        if (e is Map && e['id'] is String)
          AllowedCredential(
            id: base64UrlBytes(e['id'] as String),
            transports: [
              for (final t in (e['transports'] as List?) ?? const [])
                if (t is String) t,
            ],
          ),
    ];
    final selection = json['authenticatorSelection'] is Map
        ? json['authenticatorSelection']! as Map
        : const <String, Object?>{};
    final timeout = json['timeout'];
    String? text(Object? v) => v is String && v.isNotEmpty ? v : null;
    return PasskeyCreation(
      challenge: base64UrlBytes(challenge),
      rpId: text(rp['id']),
      rpName: text(rp['name']) ?? text(rp['id']) ?? '',
      userId: base64UrlBytes(userId),
      userName: text(user['name']) ?? '',
      userDisplayName: text(user['displayName']) ?? '',
      parameters: parameters,
      excludeCredentials: exclude,
      timeoutMs: timeout is num ? timeout.toInt() : null,
      residentKey: text(selection['residentKey']),
      authenticatorAttachment: text(selection['authenticatorAttachment']),
    );
  }

  /// The challenge to sign.
  final Uint8List challenge;

  /// The relying party ID, or null for the origin's host.
  final String? rpId;

  /// The relying party's name.
  final String rpName;

  /// The user handle.
  final Uint8List userId;

  /// The login name.
  final String userName;

  /// The name to show.
  final String userDisplayName;

  /// The algorithms the server accepts, in its order.
  final List<CredentialParameter> parameters;

  /// The passkeys of the person the authenticator must not register again.
  final List<AllowedCredential> excludeCredentials;

  /// The timeout in milliseconds, or null.
  final int? timeoutMs;

  /// `authenticatorSelection.residentKey`, or null.
  final String? residentKey;

  /// `authenticatorSelection.authenticatorAttachment`, or null.
  final String? authenticatorAttachment;
}

/// The JSON form of a new passkey (`PublicKeyCredential` with an
/// `AuthenticatorAttestationResponse`) that `completePasskeyRegistration` takes as `credential`.
Map<String, Object?> attestationJson({
  required String id,
  required List<int> rawId,
  required List<int> clientDataJson,
  required List<int> attestationObject,
  List<String> transports = const [],
  String? authenticatorAttachment,
}) => {
  'id': id,
  'rawId': base64UrlText(rawId),
  'type': 'public-key',
  'authenticatorAttachment': ?authenticatorAttachment,
  'response': {
    'clientDataJSON': base64UrlText(clientDataJson),
    'attestationObject': base64UrlText(attestationObject),
    if (transports.isNotEmpty) 'transports': transports,
  },
  'clientExtensionResults': <String, Object?>{},
};
