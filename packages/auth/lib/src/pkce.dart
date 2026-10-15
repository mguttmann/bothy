// SPDX-License-Identifier: Apache-2.0

/// PKCE of the app leg of the OIDC login (RFC 7636, 08 2.2 item 1): the app's
/// `client_verifier` and its `S256` `client_challenge`.
library;

import 'dart:convert';
import 'dart:math';

import 'package:auth/src/jose.dart';
import 'package:crypto/crypto.dart';

/// A PKCE pair: the secret [verifier] and its public [challenge].
///
/// The verifier never leaves the app except in `completeOidcLogin`; [toString] names neither.
final class PkcePair {
  /// Creates a pair from a [verifier]; the challenge is derived (`S256`).
  PkcePair(this.verifier) : challenge = pkceS256(verifier);

  /// Draws a new verifier from 32 bytes of [random] (a cryptographically secure source in
  /// the apps), encoded as base64url without padding: 43 characters, inside the 43 to 128
  /// of RFC 7636 4.1.
  ///
  /// Example: `final pkce = PkcePair.generate(Random.secure());`
  factory PkcePair.generate(Random random) => PkcePair(
    base64UrlNoPad(List<int>.generate(32, (_) => random.nextInt(256))),
  );

  /// The `client_verifier`; secret.
  final String verifier;

  /// The `client_challenge`: base64url(SHA-256(verifier)).
  final String challenge;

  @override
  String toString() => 'PkcePair(verifier: <redacted>, challenge: S256)';
}

/// The PKCE `S256` challenge of [verifier] (RFC 7636 4.2): base64url of SHA-256 of the ASCII
/// verifier, without padding.
String pkceS256(String verifier) =>
    base64UrlNoPad(sha256.convert(ascii.encode(verifier)).bytes);
