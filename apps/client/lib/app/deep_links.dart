// SPDX-License-Identifier: Apache-2.0

/// Deep links from outside the app (docs/spec/15-client-architektur.md 11; brief M4-18 Design
/// note 4; docs/DECISIONS.md D-M4-18-4).
library;

import 'dart:async';

import 'package:flutter/services.dart';

/// Where links of the URL scheme come from: the macOS app delegate and the Windows runner
/// forward them over a method channel.
abstract interface class DeepLinkSource {
  /// The links that arrived before the app listened (the app was started by a link).
  Future<List<Uri>> takePending();

  /// The links that arrive while the app runs.
  Stream<Uri> get links;
}

/// [DeepLinkSource] on the method channel `bothy/deep_links` of the runners: `takePending`
/// asks the runner for the links it kept, `open` brings a new one.
final class MethodChannelDeepLinks implements DeepLinkSource {
  /// Creates the source on [channel] (the runners' channel by default).
  MethodChannelDeepLinks({MethodChannel? channel})
    : _channel = channel ?? const MethodChannel('bothy/deep_links') {
    _channel.setMethodCallHandler((call) async {
      if (call.method != 'open') {
        throw MissingPluginException('no method ${call.method}');
      }
      final link = Uri.tryParse('${call.arguments}');
      if (link != null) _links.add(link);
    });
  }

  final MethodChannel _channel;
  // Single-subscription: a link that arrives before the app listens (macOS hands the start link
  // over the channel as soon as the engine runs) waits in the controller's buffer.
  final StreamController<Uri> _links = StreamController<Uri>();

  /// The links that arrive while the app runs; one listener (the app), which also gets every
  /// link that arrived before it listened.
  @override
  Stream<Uri> get links => _links.stream;

  @override
  Future<List<Uri>> takePending() async {
    final List<Object?>? raw;
    try {
      raw = await _channel.invokeListMethod<Object?>('takePending');
    } on MissingPluginException {
      // A host without the runner's channel (widget tests, a platform of M5).
      return const [];
    }
    return [
      for (final item in raw ?? const <Object?>[]) ?Uri.tryParse('$item'),
    ];
  }
}

/// The app path of an external link `<scheme>://t/<thread_id>?m=<message_id>` (and `s`, `c`
/// and the other paths of 15 11), or null when the link has another scheme or no path.
///
/// Both URI forms are accepted: `chat-example://t/<id>` (the target kind is the host) and
/// `chat-example:/t/<id>`. The query is kept; the scheme comparison ignores case (RFC 3986
/// 3.1). The returned path still goes through the router's parser, which rejects unknown
/// targets.
///
/// Example: `pathOfExternalLink(Uri.parse('chat-example://t/abc?m=def'), 'chat-example')`
/// returns `/t/abc?m=def`.
String? pathOfExternalLink(Uri link, String scheme) {
  if (link.scheme.toLowerCase() != scheme.toLowerCase()) return null;
  final segments = [
    if (link.host.isNotEmpty) link.host,
    ...link.pathSegments.where((s) => s.isNotEmpty),
  ];
  if (segments.isEmpty) return null;
  final path = '/${segments.map(Uri.encodeComponent).join('/')}';
  return link.hasQuery ? '$path?${link.query}' : path;
}
