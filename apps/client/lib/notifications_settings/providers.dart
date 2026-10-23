// SPDX-License-Identifier: Apache-2.0

/// Riverpod providers of the notification settings (brief M4-33 Scope): the personal switches
/// of 13 3.3 with their defaults, the writer through the outbox, the quiet hours rules, the
/// context that computes the effective level for display (13 3.1) and the tree of D9. Widgets
/// never query the database themselves (R-CL-03).
library;

import 'package:domain/domain.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../app/providers/settings.dart';
import '../sidebar/providers.dart';
import '../sync/notify_ops.dart';
import 'effective_level.dart';

part 'providers.g.dart';

/// The instance default `notify.user_default_level`, step 7 of 13 3.1. No API operation names
/// it (`getInstance` and the snapshot carry no notification defaults), so the display uses the
/// reference default of 13 17 (docs/DECISIONS.md D-M4-33-1); the server decides with its own
/// value.
const NotificationLevel instanceDefaultLevel = NotificationLevel.mentions;

/// The keys `notify.*` of 13 3.3, the only personal keys of this screen.
abstract final class NotifyKeys {
  /// `all`, `mentions`, `mute`: step 6 of 13 3.1.
  static const String defaultLevel = 'notify.default_level';

  /// `full`, `names`, `none` (13 6.4).
  static const String previewMode = 'notify.preview_mode';

  /// bool: `input_needed` stays audible in quiet hours.
  static const String inputBreaksQuietHours =
      'notify.input_needed_breaks_quiet_hours';

  /// bool: `admin` notifications to owners and administrators.
  static const String adminEnabled = 'notify.admin_enabled';
}

/// The personal switches of 13 3.3 with the reference defaults of 13 17 where a key is absent
/// (the server stores no defaults, 04 14).
final class NotifySettings {
  const NotifySettings._({
    required this.personalDefault,
    required this.previewMode,
    required this.inputBreaksQuietHours,
    required this.adminEnabled,
  });

  /// Reads [stored] (key to JSON value); a value outside the list of 13 3.3 counts as absent.
  ///
  /// Example: `NotifySettings.resolve({}).previewMode` is `full`.
  factory NotifySettings.resolve(Map<String, Object?> stored) {
    final level = stored[NotifyKeys.defaultLevel];
    final preview = stored[NotifyKeys.previewMode];
    final breaks = stored[NotifyKeys.inputBreaksQuietHours];
    final admin = stored[NotifyKeys.adminEnabled];
    final decoded = level is String ? NotificationLevel.fromCode(level) : null;
    return NotifySettings._(
      personalDefault: decoded == NotificationLevel.unknown ? null : decoded,
      previewMode: preview is String && previewModes.contains(preview)
          ? preview
          : 'full',
      inputBreaksQuietHours: breaks is bool && breaks,
      adminEnabled: admin is! bool || admin,
    );
  }

  /// The values of `notify.preview_mode` in their order.
  static const List<String> previewModes = ['full', 'names', 'none'];

  /// `notify.default_level`, or null while the person has none (step 7 applies).
  final NotificationLevel? personalDefault;

  /// `notify.preview_mode`; default `notify.default_preview_mode` (`full`).
  final String previewMode;

  /// `notify.input_needed_breaks_quiet_hours`; default `false`.
  final bool inputBreaksQuietHours;

  /// `notify.admin_enabled`; default `notify.default_admin_enabled` (`true`).
  final bool adminEnabled;
}

/// The personal switches of the signed-in person.
@Riverpod(keepAlive: true)
NotifySettings notifySettings(Ref ref) =>
    NotifySettings.resolve(ref.watch(userSettingsProvider).value ?? const {});

/// The writer of levels and quiet hours, or null before the login.
@Riverpod(keepAlive: true)
NotifyWriter? notifyWriter(Ref ref) {
  final services = ref.watch(signedInServicesProvider);
  if (services == null) return null;
  return NotifyWriter(services.engine, services.database, services.ids.next);
}

/// Loads the levels and quiet hours of the signed-in person once per session (the snapshot
/// carries neither, 04 6.3); a failed load leaves the stored rows and the events keep them
/// current.
@Riverpod(keepAlive: true)
Future<void> notifyRefresh(Ref ref) async {
  final services = ref.watch(signedInServicesProvider);
  final writer = ref.watch(notifyWriterProvider);
  if (services == null || writer == null) return;
  try {
    await writer.refresh(services.auth.api);
  } on Object {
    // Offline or refused: the stored rows stand; `notification_pref.updated` follows.
  }
}

/// The quiet hours rules of the person, by ID.
@riverpod
Stream<List<QuietHours>> quietHoursRules(Ref ref) {
  final db = ref.watch(localDatabaseProvider);
  return db == null ? Stream.value(const []) : db.personalDao.watchQuietHours();
}

/// Whether the signed-in person is an owner (`getMe`, `is_owner`): only owners see
/// `notify.admin_enabled` (13 3.3). False while unknown or offline.
@Riverpod(keepAlive: true)
Future<bool> isOwner(Ref ref) async {
  final services = ref.watch(signedInServicesProvider);
  if (services == null) return false;
  try {
    return (await services.auth.api.getMeApi().getMe()).data?.isOwner ?? false;
  } on Object {
    return false;
  }
}

/// What the effective level of a target needs: the person's own rows, the personal default
/// and the time; computes the levels for display only (D-M4-33-1).
final class LevelContext {
  /// Creates the context over [preferences].
  LevelContext({
    required Iterable<NotificationPreference> preferences,
    required this.personalDefault,
    required this.now,
  }) : _rows = {
         for (final p in preferences) '${p.targetKind.code}:${p.targetId}': p,
       };

  final Map<String, NotificationPreference> _rows;

  /// `notify.default_level`, or null.
  final NotificationLevel? personalDefault;

  /// The time `muted_until` is compared with.
  final DateTime now;

  /// The own row of [kind] [id], or null.
  NotificationPreference? own(NotificationTargetKind kind, String id) =>
      _rows['${kind.code}:$id'];

  /// The own rows of threads (followed or set), for the tree.
  Iterable<NotificationPreference> get threadRows =>
      _rows.values.where((p) => p.targetKind == NotificationTargetKind.thread);

  /// The effective level of [space] (its own row, then the defaults).
  EffectiveLevel ofSpace(SpaceId space) => effectiveLevel(
    spacePref: own(NotificationTargetKind.space, space.value),
    isDm: false,
    personalDefault: personalDefault,
    instanceDefault: instanceDefaultLevel,
    now: now,
  );

  /// The effective level of [channel].
  EffectiveLevel ofChannel(Channel channel) => effectiveLevel(
    channelPref: own(NotificationTargetKind.channel, channel.id.value),
    spacePref: _spaceRow(channel),
    channelDefault: channel.defaultNotificationLevel,
    isDm: channel.kind == ChannelKind.dm,
    personalDefault: personalDefault,
    instanceDefault: instanceDefaultLevel,
    now: now,
  );

  /// The effective level of [thread] in [channel] (the order of 13 3.1).
  EffectiveLevel ofThread(ThreadId thread, Channel channel) => effectiveLevel(
    threadPref: own(NotificationTargetKind.thread, thread.value),
    channelPref: own(NotificationTargetKind.channel, channel.id.value),
    spacePref: _spaceRow(channel),
    channelDefault: channel.defaultNotificationLevel,
    isDm: channel.kind == ChannelKind.dm,
    personalDefault: personalDefault,
    instanceDefault: instanceDefaultLevel,
    now: now,
  );

  /// What [kind] would inherit without its own row: for a space the defaults, for a channel
  /// [channel]'s space and defaults, for a thread also [channel]'s row.
  EffectiveLevel inherited(NotificationTargetKind kind, {Channel? channel}) =>
      inheritedLevel(
        kind,
        channelPref: channel == null
            ? null
            : own(NotificationTargetKind.channel, channel.id.value),
        spacePref: channel == null ? null : _spaceRow(channel),
        channelDefault: channel?.defaultNotificationLevel,
        isDm: channel?.kind == ChannelKind.dm,
        personalDefault: personalDefault,
        instanceDefault: instanceDefaultLevel,
        now: now,
      );

  NotificationPreference? _spaceRow(Channel channel) {
    final space = channel.spaceId;
    return space == null
        ? null
        : own(NotificationTargetKind.space, space.value);
  }
}

/// The [LevelContext] of the signed-in person; reading it starts the one load of
/// [notifyRefreshProvider].
@riverpod
LevelContext levelContext(Ref ref) {
  ref.watch(notifyRefreshProvider);
  return LevelContext(
    preferences: ref.watch(notificationPreferencesProvider).value ?? const [],
    personalDefault: ref.watch(notifySettingsProvider).personalDefault,
    now: ref.watch(appPortsProvider).clock.now(),
  );
}

/// A thread of the tree of D9: a stored thread with an own row.
final class LevelTreeThread {
  /// Creates the entry.
  const LevelTreeThread({required this.id, required this.title});

  /// The thread.
  final ThreadId id;

  /// Its title.
  final String title;
}

/// A channel of the tree of D9 with the threads that have an own row.
final class LevelTreeChannel {
  /// Creates the entry.
  const LevelTreeChannel({required this.channel, required this.threads});

  /// The channel.
  final Channel channel;

  /// Its threads with an own level, ordered by title.
  final List<LevelTreeThread> threads;
}

/// A space of the tree of D9 with its channels.
final class LevelTreeSpace {
  /// Creates the entry.
  const LevelTreeSpace({required this.space, required this.channels});

  /// The space.
  final Space space;

  /// Its channels in the sidebar's order.
  final List<LevelTreeChannel> channels;
}

/// The tree of D9: the spaces and channels of the sidebar in its order, below each channel
/// the threads with an own level (set or followed, 13 3.2).
@riverpod
Future<List<LevelTreeSpace>> levelTree(Ref ref) async {
  final tree = ref.watch(sidebarTreeProvider);
  final rows = ref.watch(levelContextProvider).threadRows.toList();
  final db = ref.watch(localDatabaseProvider);
  final threadsByChannel = <ChannelId, List<LevelTreeThread>>{};
  for (final row in rows) {
    final id = ThreadId.parse(row.targetId);
    final thread = await db?.threadsDao.byId(id);
    // The main thread's row belongs to the channel's history; the tree shows threads only.
    if (thread == null || thread.kind == ThreadKind.main) continue;
    threadsByChannel
        .putIfAbsent(thread.channelId, () => [])
        .add(LevelTreeThread(id: id, title: thread.title));
  }
  for (final list in threadsByChannel.values) {
    list.sort((a, b) => a.title.compareTo(b.title));
  }
  return [
    for (final node in tree)
      LevelTreeSpace(
        space: node.entry.space,
        channels: [
          for (final c in node.channels)
            LevelTreeChannel(
              channel: c.channel,
              threads: threadsByChannel[c.channel.id] ?? const [],
            ),
        ],
      ),
  ];
}
