// SPDX-License-Identifier: Apache-2.0

/// The effective notification level of 13 3.1, computed for display only (brief M4-33 Design
/// note 1; docs/DECISIONS.md D-M4-33-1): it labels the tree of D9 and the menus, and decides
/// nothing about delivery, which is the server's alone.
library;

import 'package:domain/domain.dart';
import 'package:l10n/l10n.dart';

/// The step of the order of 13 3.1 that gave a level.
enum LevelSource {
  /// 1. the own row of the thread.
  thread,

  /// 2. the own row of the channel.
  channel,

  /// 3. the own row of the space.
  space,

  /// 4. `channels.default_notification_level`.
  channelDefault,

  /// 5. `all` in a DM channel.
  dm,

  /// 6. `user_settings['notify.default_level']`.
  personalDefault,

  /// 7. the instance default `notify.user_default_level`.
  instanceDefault,
}

/// A level with the step that gave it.
final class EffectiveLevel {
  /// Creates the result.
  const EffectiveLevel({
    required this.level,
    required this.source,
    this.storedLevel,
    this.mutedUntil,
  });

  /// The level that applies now: `mute` while [mutedUntil] lies in the future.
  final NotificationLevel level;

  /// The step of 13 3.1 that gave it.
  final LevelSource source;

  /// The level of the selected own row (steps 1 to 3), which applies again after
  /// [mutedUntil]; null for the defaults (steps 4 to 7).
  final NotificationLevel? storedLevel;

  /// The end of a temporary mute of the selected row, only while it lies in the future.
  final DateTime? mutedUntil;
}

/// The effective level of a thread after 13 3.1: the first existing value of [threadPref],
/// [channelPref], [spacePref], [channelDefault], `all` for a DM ([isDm]), [personalDefault] and
/// [instanceDefault]; when `muted_until` of the selected row lies after [now], `mute` applies.
///
/// For a channel or a space pass only the rows and defaults that lie above it (a space has
/// no channel default and is no DM). A row with a level this version does not know counts as
/// absent, so a newer server's level never shows as a wrong one.
///
/// Example:
/// ```dart
/// effectiveLevel(
///   channelPref: mutedChannel,
///   isDm: false,
///   instanceDefault: NotificationLevel.mentions,
///   now: DateTime.now(),
/// ).level; // NotificationLevel.mute
/// ```
EffectiveLevel effectiveLevel({
  NotificationPreference? threadPref,
  NotificationPreference? channelPref,
  NotificationPreference? spacePref,
  NotificationLevel? channelDefault,
  required bool isDm,
  NotificationLevel? personalDefault,
  required NotificationLevel instanceDefault,
  required DateTime now,
}) {
  for (final (row, source) in [
    (threadPref, LevelSource.thread),
    (channelPref, LevelSource.channel),
    (spacePref, LevelSource.space),
  ]) {
    if (row == null || row.level == NotificationLevel.unknown) continue;
    final until = row.mutedUntil;
    // 13 3.1: a passed `muted_until` has no effect (D-M3-18-3); the row's level applies.
    final muted = until != null && until.isAfter(now);
    return EffectiveLevel(
      level: muted ? NotificationLevel.mute : row.level,
      source: source,
      storedLevel: row.level,
      mutedUntil: muted ? until : null,
    );
  }
  if (_known(channelDefault) case final level?) {
    return EffectiveLevel(level: level, source: LevelSource.channelDefault);
  }
  if (isDm) {
    return const EffectiveLevel(
      level: NotificationLevel.all,
      source: LevelSource.dm,
    );
  }
  if (_known(personalDefault) case final level?) {
    return EffectiveLevel(level: level, source: LevelSource.personalDefault);
  }
  return EffectiveLevel(
    level: instanceDefault,
    source: LevelSource.instanceDefault,
  );
}

/// The level a target of [kind] would have without its own row: what "Inherit" gives and what
/// "inherits: ..." names. A space inherits only the personal and the instance default; a
/// channel the space row and the defaults; a thread the channel and everything above.
EffectiveLevel inheritedLevel(
  NotificationTargetKind kind, {
  NotificationPreference? channelPref,
  NotificationPreference? spacePref,
  NotificationLevel? channelDefault,
  required bool isDm,
  NotificationLevel? personalDefault,
  required NotificationLevel instanceDefault,
  required DateTime now,
}) {
  final space = kind == NotificationTargetKind.space;
  return effectiveLevel(
    channelPref: kind == NotificationTargetKind.thread ? channelPref : null,
    spacePref: space ? null : spacePref,
    channelDefault: space ? null : channelDefault,
    isDm: !space && isDm,
    personalDefault: personalDefault,
    instanceDefault: instanceDefault,
    now: now,
  );
}

NotificationLevel? _known(NotificationLevel? level) =>
    level == NotificationLevel.unknown ? null : level;

/// The name of [level] in the person's language.
String levelLabel(AppLocalizations l, NotificationLevel level) =>
    switch (level) {
      NotificationLevel.all => l.notifyLevelAll,
      NotificationLevel.mentions => l.notifyLevelMentions,
      NotificationLevel.mute || NotificationLevel.unknown => l.notifyLevelMute,
    };

/// "inherits: All" for the inherited [level].
String inheritsLabel(AppLocalizations l, EffectiveLevel level) =>
    l.notifyInherits(levelLabel(l, level.level));

/// The name of the step [source], as in "Inherit (All from the space)".
String levelSourceLabel(AppLocalizations l, LevelSource source) =>
    switch (source) {
      LevelSource.thread => l.notifySourceThread,
      LevelSource.channel => l.notifySourceChannel,
      LevelSource.space => l.notifySourceSpace,
      LevelSource.channelDefault => l.notifySourceChannelDefault,
      LevelSource.dm => l.notifySourceDm,
      LevelSource.personalDefault => l.notifySourcePersonalDefault,
      LevelSource.instanceDefault => l.notifySourceInstanceDefault,
    };
