// SPDX-License-Identifier: Apache-2.0

/// The recently visited targets of this device for the quick switcher (docs/spec/01-product.md
/// 5.1 D6: "recently visited targets first"; brief M4-23 Design note 3). The list lives in the
/// local-only table `recent_targets` of `local_store` and is never synchronized: 01 11 has no
/// personal setting for it, so it is a fact of the device, not of the person.
library;

import 'package:auth/auth.dart' show Clock;
import 'package:local_store/local_store.dart';

import '../app/router.dart';

export 'package:local_store/local_store.dart'
    show RecentTarget, RecentTargetKind;

/// The target a visit of [location] records: a space (`/s/<id>`), a channel (`/c/<id>`) or a
/// thread (`/t/<id>`, the message of `?m=` does not matter); null for every other path.
///
/// Example: `recentTargetOf(Uri.parse('/c/<id>'))` is `(RecentTargetKind.channel, '<id>')`.
(RecentTargetKind, String)? recentTargetOf(Uri location) =>
    switch (parseLocation(location)) {
      SpaceLocation(:final spaceId) => (RecentTargetKind.space, spaceId.value),
      ChannelLocation(:final channelId) => (
        RecentTargetKind.channel,
        channelId.value,
      ),
      ThreadLocation(:final threadId) => (
        RecentTargetKind.thread,
        threadId.value,
      ),
      _ => null,
    };

/// The path of the route of [target], the form the switcher rows carry.
String recentPath(RecentTarget target) => switch (target.kind) {
  RecentTargetKind.space => '/s/${target.targetId}',
  RecentTargetKind.channel => '/c/${target.targetId}',
  RecentTargetKind.thread => '/t/${target.targetId}',
};

/// The recent targets of this device over `RecentTargetsDao`.
///
/// [keep] is `ui.quick_switch_max_results`: the switcher never shows more rows, so the table
/// keeps no more entries than that; the time of a visit comes from [clock] (the app's clock
/// port).
///
/// Example:
/// ```dart
/// final recent = RecentTargets(db.recentTargetsDao, clock: ports.clock, keep: 20);
/// await recent.recordLocation(Uri.parse('/t/<id>'));
/// ```
final class RecentTargets {
  /// Creates the list over [dao].
  RecentTargets(this._dao, {required this.clock, required this.keep});

  final RecentTargetsDao _dao;

  /// The app's clock port; the time of a visit.
  final Clock clock;

  /// How many targets the table keeps and the watch delivers.
  final int keep;

  /// Records a visit of [location] now; returns whether the path named a target (a space,
  /// channel or thread route), else records nothing.
  Future<bool> recordLocation(Uri location) async {
    final target = recentTargetOf(location);
    if (target == null) return false;
    await _dao.visit(target.$1, target.$2, at: clock.now(), keep: keep);
    return true;
  }

  /// The recent targets, newest first, on every change.
  Stream<List<RecentTarget>> watch() => _dao.watchRecent(limit: keep);
}
