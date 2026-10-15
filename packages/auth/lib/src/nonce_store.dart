// SPDX-License-Identifier: Apache-2.0

/// The newest DPoP nonce of one server (RFC 9449 8, 08 7.3, 06 3.1 item 2).
library;

/// Keeps the newest nonce the server issued; one store per server address (one per
/// `AuthSession`).
///
/// Sources: the `DPoP-Nonce` header of every response, also successful ones, and
/// `hello.dpop_nonce` and `reauth_required` of the gateway (04 6.3, through M4-13). The client
/// never decides how long a nonce lives (`auth.dpop_nonce_ttl` is the server's); it always
/// takes the one it received last, and the server answers `use_dpop_nonce` when it is stale.
///
/// Example:
/// ```dart
/// nonces.update(response.headers.value('dpop-nonce'));
/// final proof = await proofs.forRequest(method: 'GET', url: url); // carries the nonce
/// ```
final class NonceStore {
  String? _current;

  /// The newest nonce, or `null` before the server issued one.
  String? get current => _current;

  /// Takes [nonce] as the newest; `null` or an empty value changes nothing.
  void update(String? nonce) {
    if (nonce == null) return;
    final trimmed = nonce.trim();
    if (trimmed.isEmpty) return;
    _current = trimmed;
  }

  @override
  String toString() => 'NonceStore(${_current == null ? 'empty' : 'set'})';
}
