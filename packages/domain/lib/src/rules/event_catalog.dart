// SPDX-License-Identifier: Apache-2.0

/// The event catalogue of the client, generated from `docs/spec/actions.yaml` (04 9 item 3,
/// ADR 0049); `make codegen-check` holds `event_catalog.g.dart` equal to a fresh generation.
library;

part 'event_catalog.g.dart';

/// What the client needs to know about one event type.
final class EventInfo {
  /// Creates an entry; only the generated table does.
  const EventInfo({
    required this.type,
    required this.durable,
    required this.carriesResourceVersion,
    required this.resourceKind,
    required this.personal,
  });

  /// The event type, for example `message.created`.
  final String type;

  /// Durable (in the event log, with `seq`) or ephemeral.
  final bool durable;

  /// Whether the envelope carries `resource.version` (04 9 item 3); without it the client
  /// applies the event in `seq` order.
  final bool carriesResourceVersion;

  /// Kind of the affected resource, for example `message` or `thread`.
  final String resourceKind;

  /// Whether the event is personal (`audience: self`, 04 14).
  final bool personal;
}

/// Lookup of event types.
abstract final class EventCatalog {
  /// The entry of [type], or null for a type this version does not know; the caller skips
  /// such an event (a newer server may add types, 06 13).
  static EventInfo? lookup(String type) => _catalogue[type];

  /// Every known event type, sorted.
  static Iterable<String> get types => _catalogue.keys;
}
