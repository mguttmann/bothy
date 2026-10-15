// SPDX-License-Identifier: Apache-2.0

/// Which link targets of a message are active (SEC-OUT-1).
library;

import 'package:flutter/foundation.dart';

/// An active link target and what its tooltip shows.
@immutable
final class DsLinkTarget {
  /// Creates the target.
  const DsLinkTarget(this.uri, this.display);

  /// The parsed target handed to the caller.
  final Uri uri;

  /// The domain of an `https` or `http` link, the address of a `mailto` link, or the path of
  /// an internal reference.
  final String display;
}

/// Classifies [href]: the active target, or null when the link must render as plain text.
///
/// Active are `https` and `http` with a host, `mailto` with an address, and the internal paths
/// `/t/...` and `/c/...` of docs/spec/15 section 11 (no scheme, no authority). Everything else,
/// for example `javascript:`, `data:`, `file:`, a relative path or a protocol-relative
/// `//host/...`, is inactive. Never throws.
///
/// ```dart
/// dsClassifyLink('https://example.com/a')?.display; // example.com
/// dsClassifyLink('javascript:alert(1)'); // null
/// ```
DsLinkTarget? dsClassifyLink(String href) {
  final uri = Uri.tryParse(href.trim());
  if (uri == null) return null;
  switch (uri.scheme) {
    case 'https' || 'http':
      return uri.host.isEmpty ? null : DsLinkTarget(uri, uri.host);
    case 'mailto':
      return uri.path.isEmpty ? null : DsLinkTarget(uri, uri.path);
    case '':
      final internal =
          !uri.hasAuthority &&
          (uri.path.startsWith('/t/') || uri.path.startsWith('/c/')) &&
          uri.path.length > 3;
      return internal ? DsLinkTarget(uri, uri.path) : null;
    default:
      return null;
  }
}
