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
