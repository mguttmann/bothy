// SPDX-License-Identifier: Apache-2.0

/// The version rule of 04 9 item 3 and 15 5: a change applies only if `resource.version` is
/// greater than the local version; types without `resource.version` apply in `seq` order.
library;

import '../seq.dart';
import 'event_catalog.dart';

/// What the caller does with an incoming change.
enum ApplyDecision {
  /// Apply: the incoming version is newer, or nothing is stored locally.
  apply,

  /// Ignore: the incoming version is equal to or older than the local one (a duplicate or a
  /// stale REST page).
  ignoreStale,

  /// Apply in `seq` order: the type carries no `resource.version` (for example `reaction.*`,
  /// `pin.*`, `read_state.updated`); its payload holds absolute values.
  applyInSeqOrder,

  /// Skip: the event type is not in the catalogue of this version.
  unknownType,
}

/// Decides whether a change applies (04 9 item 3).
///
/// [local] is the stored version of the object, null when nothing is stored. [incoming] is
/// `resource.version` of the event or the version of a REST object. [typeCarriesVersion] comes
/// from [EventInfo.carriesResourceVersion]; REST objects always carry their version.
///
/// A type that carries a version but arrives without one is not applied: only a greater
/// version may change the object.
ApplyDecision decideApply({
  required ResourceVersion? local,
  required ResourceVersion? incoming,
  required bool typeCarriesVersion,
}) {
  if (!typeCarriesVersion) return ApplyDecision.applyInSeqOrder;
  if (incoming == null) return ApplyDecision.ignoreStale;
  if (local == null || incoming > local) return ApplyDecision.apply;
  return ApplyDecision.ignoreStale;
}

/// [decideApply] for an event of [type], looked up in the [EventCatalog]; an unknown type
/// gives [ApplyDecision.unknownType].
ApplyDecision decideApplyForEvent({
  required String type,
  required ResourceVersion? local,
  required ResourceVersion? incoming,
}) {
  final info = EventCatalog.lookup(type);
  if (info == null) return ApplyDecision.unknownType;
  return decideApply(
    local: local,
    incoming: incoming,
    typeCarriesVersion: info.carriesResourceVersion,
  );
}
