// SPDX-License-Identifier: Apache-2.0

/// The one way this package prints a secret: by kind and length only (15 16, 19 SEC-LOG;
/// brief M4-09 Design note 7).
library;

/// The printable stand-in for a secret [value] of [kind], for example
/// `<access token, 812 chars>`; `null` prints as `<kind: none>`.
///
/// Every `toString`, exception message and log line of this package uses it for tokens,
/// proofs, codes, verifiers, passwords and TOTP values.
String redacted(String kind, String? value) =>
    value == null ? '<$kind: none>' : '<$kind, ${value.length} chars>';
