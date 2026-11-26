// SPDX-License-Identifier: Apache-2.0

/// Passkeys of the Windows app over Windows Hello: the Win32 WebAuthn API (`webauthn.h`) that
/// the runner serves on the method channel `bothy/webauthn` (windows/runner/webauthn_bridge.h).
///
/// The app registers a passkey from its settings and confirms a step-up with one, because the
/// Windows app has no step-up key (08 6.1 and 6.2; passkeys in native apps 08 2.5, 15 8). The
/// Win32 API takes the client data instead of a challenge, so this adapter is the WebAuthn
/// client: it writes `clientDataJSON` with the instance's origin, checks the relying party
/// against that origin as a browser would, and builds the `PublicKeyCredential` JSON the server
/// parses. The runner only fills the Win32 structs.
library;

import 'dart:convert';

import 'package:auth/auth.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

/// The channel the Windows runner serves (windows/runner/flutter_window.cpp).
const MethodChannel webauthnChannel = MethodChannel('bothy/webauthn');

/// The timeout of a ceremony whose options carry none, in milliseconds: the lower end of the
/// range WebAuthn Level 3 recommends when user verification is required (5.4.1, 5.5).
const int defaultPasskeyTimeoutMs = 300000;

/// [PasskeyAuthenticator] over Windows Hello for the instance at [origin].
///
/// A cancelled or declined Windows Hello prompt throws a [PlatformException] with the code
/// `cancelled`, the same error the iOS and Android adapters throw ([DomainPasskeys]); the login
/// flows show it as a refused passkey, and a step-up treats it like a refused signer: any error
/// that is not an `AuthException` becomes `StepUpCancelled` (packages/auth step_up.dart). Any other failure of Windows throws a
/// [PlatformException] with the code `failed` and the WebAuthn error name in its message.
/// Options for a relying party that does not cover the origin's host are refused with a
/// [StateError] before Windows is asked.
final class WindowsPasskeys implements PasskeyAuthenticator {
  /// Creates the adapter for [origin] over [channel]; use [create], which also asks whether
  /// Windows Hello is there.
  @visibleForTesting
  WindowsPasskeys({required String origin, MethodChannel? channel})
    : origin = Uri.parse(origin).origin,
      _channel = channel ?? webauthnChannel;

  /// The adapter for [origin] (the instance's `server.public_url`, for example
  /// `https://chat.example.com`), or `null` when this Windows has no user-verifying platform
  /// authenticator or the runner serves no WebAuthn channel; the app then hides passkeys.
  static Future<WindowsPasskeys?> create({
    required String origin,
    MethodChannel? channel,
  }) async {
    final passkeys = WindowsPasskeys(origin: origin, channel: channel);
    try {
      final available = await passkeys._channel.invokeMethod<bool>('available');
      return available == true ? passkeys : null;
    } on MissingPluginException {
      return null;
    } on PlatformException {
      return null;
    }
  }

  /// The WebAuthn origin of the client data: scheme, host and port, without a path.
  final String origin;

  final MethodChannel _channel;

  @override
  Future<Map<String, Object?>> getAssertion(
    Map<String, Object?> requestOptions,
  ) async {
    final rpId = _rpId(requestOptions['rpId']);
    final clientData = _clientData('webauthn.get', requestOptions);
    final result = await _invoke('getAssertion', {
      'rpId': rpId,
      'clientData': clientData,
      'allowIds': _credentialIds(requestOptions['allowCredentials']),
      'timeoutMs': _timeout(requestOptions['timeout']),
      'userVerification': _userVerification(requestOptions['userVerification']),
    });
    final id = _bytes(result['credentialId']);
    final userHandle = result['userHandle'];
    return _credential(id, {
      'clientDataJSON': _b64(clientData),
      'authenticatorData': _b64(_bytes(result['authenticatorData'])),
      'signature': _b64(_bytes(result['signature'])),
      if (userHandle is Uint8List && userHandle.isNotEmpty)
        'userHandle': _b64(userHandle),
    });
  }

  @override
  Future<Map<String, Object?>> createCredential(
    Map<String, Object?> creationOptions,
  ) async {
    final rp = _map(creationOptions['rp']);
    final user = _map(creationOptions['user']);
    final rpId = _rpId(rp['id']);
    final userId = user['id'];
    if (userId is! String || userId.isEmpty) {
      throw const FormatException('passkey options without a user id');
    }
    final selection = _map(creationOptions['authenticatorSelection']);
    final clientData = _clientData('webauthn.create', creationOptions);
    final rpName = rp['name'];
    final userName = user['name'];
    final displayName = user['displayName'];
    final result = await _invoke('makeCredential', {
      'rpId': rpId,
      'rpName': rpName is String && rpName.isNotEmpty ? rpName : rpId,
      'userId': _decode(userId),
      'userName': userName is String ? userName : '',
      'userDisplayName': displayName is String ? displayName : '',
      'clientData': clientData,
      'algs': _algorithms(creationOptions['pubKeyCredParams']),
      'excludeIds': _credentialIds(creationOptions['excludeCredentials']),
      'timeoutMs': _timeout(creationOptions['timeout']),
      'residentKeyRequired':
          selection['residentKey'] == 'required' ||
          selection['requireResidentKey'] == true,
      'userVerification': _userVerification(selection['userVerification']),
      // Windows Hello itself, unless the server asks for a security key.
      'platformOnly': selection['authenticatorAttachment'] != 'cross-platform',
    });
    final transports = result['transports'];
    return _credential(_bytes(result['credentialId']), {
      'clientDataJSON': _b64(clientData),
      'attestationObject': _b64(_bytes(result['attestationObject'])),
      if (transports is List && transports.isNotEmpty)
        'transports': [
          for (final t in transports)
            if (t is String) t,
        ],
    });
  }

  Future<Map<Object?, Object?>> _invoke(
    String method,
    Map<String, Object?> arguments,
  ) async {
    final result = await _channel.invokeMapMethod<Object?, Object?>(
      method,
      arguments,
    );
    if (result == null) {
      throw PlatformException(code: 'failed', message: '$method: no result');
    }
    return result;
  }

  /// The relying party: the one of the options, or the origin's host when they name none
  /// (WebAuthn Level 3 5.1.4.1 step 8). A relying party must be the host or a domain above it
  /// (the check a browser makes, 5.1.3 step 8), otherwise the server could have the app sign
  /// for another site's passkeys.
  String _rpId(Object? value) {
    final host = Uri.parse(origin).host;
    final rpId = value is String && value.isNotEmpty ? value : host;
    if (rpId != host && !host.endsWith('.$rpId')) {
      throw StateError('the relying party does not cover the instance origin');
    }
    return rpId;
  }

  /// The UTF-8 bytes of the client data (WebAuthn Level 3 5.8.1), with the challenge as the
  /// server sent it; the key order is the one of the specification's serialization (5.8.1.1).
  Uint8List _clientData(String type, Map<String, Object?> options) {
    final challenge = options['challenge'];
    if (challenge is! String || challenge.isEmpty) {
      throw const FormatException('passkey options without a challenge');
    }
    return utf8.encode(
      jsonEncode({
        'type': type,
        'challenge': challenge,
        'origin': origin,
        'crossOrigin': false,
      }),
    );
  }

  Map<String, Object?> _credential(
    Uint8List id,
    Map<String, Object?> response,
  ) => {
    'id': _b64(id),
    'rawId': _b64(id),
    'type': 'public-key',
    'authenticatorAttachment': 'platform',
    'clientExtensionResults': <String, Object?>{},
    'response': response,
  };

  static List<Uint8List> _credentialIds(Object? list) => [
    if (list is List)
      for (final entry in list)
        if (entry is Map && entry['id'] is String)
          _decode(entry['id'] as String),
  ];

  /// The COSE algorithms of `pubKeyCredParams` in the server's order; ES256 and RS256 when the
  /// list is empty (WebAuthn Level 3 5.1.3 step 10).
  static List<int> _algorithms(Object? list) {
    final algs = [
      if (list is List)
        for (final entry in list)
          if (entry is Map &&
              entry['type'] == 'public-key' &&
              entry['alg'] is int)
            entry['alg'] as int,
    ];
    return algs.isEmpty ? const [-7, -257] : algs;
  }

  static int _timeout(Object? value) =>
      value is num && value > 0 ? value.toInt() : defaultPasskeyTimeoutMs;

  static String _userVerification(Object? value) =>
      value is String && (value == 'required' || value == 'discouraged')
      ? value
      : 'preferred';

  static Map<String, Object?> _map(Object? value) =>
      value is Map ? value.cast<String, Object?>() : const {};

  static Uint8List _bytes(Object? value) {
    if (value is Uint8List && value.isNotEmpty) return value;
    throw PlatformException(code: 'failed', message: 'missing credential data');
  }

  static Uint8List _decode(String value) =>
      base64Url.decode(value.padRight((value.length + 3) & ~3, '='));

  static String _b64(List<int> bytes) =>
      base64Url.encode(bytes).replaceAll('=', '');
}
