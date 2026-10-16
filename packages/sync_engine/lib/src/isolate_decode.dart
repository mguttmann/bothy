// SPDX-License-Identifier: Apache-2.0

/// Decoding of event envelopes, off the main isolate for large pages (15 7.1, brief M4-14
/// Design note 7): a page whose size exceeds `client.sync.parse_offload_bytes` is decoded with
/// `Isolate.run`; the results are applied on the caller's isolate (the database runs in its own
/// isolate).
library;

import 'dart:isolate';

import 'package:api_client/api_client.dart';

/// One envelope after decoding: [event] is null when the envelope does not fit the schema of
/// its type (it is skipped like an unknown type, but its `seq` still counts, 04 6.1).
final class DecodedEnvelope {
  /// Creates the result.
  const DecodedEnvelope(this.raw, this.event);

  /// The envelope as received.
  final Map<String, Object?> raw;

  /// The decoded event, or null for a malformed envelope.
  final DecodedEvent? event;

  /// The `seq` of a durable event, or null.
  int? get seq => raw['seq'] is int ? raw['seq']! as int : null;
}

/// An estimate of the JSON size of [value] in bytes: the length of every string and key plus a
/// few bytes per value. It costs one walk over the value, far less than the decoding it decides
/// about.
int estimateJsonBytes(Object? value) => switch (value) {
  String() => value.length + 2,
  Map() => value.entries.fold<int>(
    2,
    (sum, e) => sum + (e.key as String).length + 4 + estimateJsonBytes(e.value),
  ),
  List() => value.fold<int>(2, (sum, e) => sum + 1 + estimateJsonBytes(e)),
  _ => 8,
};

/// Decodes [envelopes] with `EventsPageDecoder.decodeEnvelope` of `api_client`; when their
/// estimated size exceeds [offloadBytes] the work runs in a background isolate.
///
/// Example:
/// ```dart
/// final decoded = await decodeEnvelopes(raws, offloadBytes: config.syncParseOffloadBytes);
/// ```
Future<List<DecodedEnvelope>> decodeEnvelopes(
  List<Map<String, Object?>> envelopes, {
  required int offloadBytes,
}) async {
  if (estimateJsonBytes(envelopes) > offloadBytes) {
    return Isolate.run(() => _decodeAll(envelopes));
  }
  return _decodeAll(envelopes);
}

List<DecodedEnvelope> _decodeAll(List<Map<String, Object?>> envelopes) => [
  for (final raw in envelopes) DecodedEnvelope(raw, _decodeOne(raw)),
];

DecodedEvent? _decodeOne(Map<String, Object?> raw) {
  try {
    return EventsPageDecoder.decodeEnvelope(raw);
  } on Object {
    // A known type whose payload breaks its schema: skipped, never thrown (04 6.1).
    return null;
  }
}
