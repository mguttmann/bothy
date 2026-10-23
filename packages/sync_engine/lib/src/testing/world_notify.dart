// SPDX-License-Identifier: Apache-2.0

part of 'world.dart';

/// One hint the fake created and sent as `notification.created` (13 6.3), for assertions.
final class FakeNotification {
  /// Creates the record.
  const FakeNotification({
    required this.recipient,
    required this.kind,
    required this.threadId,
    required this.messageId,
    required this.quiet,
    required this.count,
    required this.devices,
    required this.payload,
  });

  /// The person the hint is for.
  final String recipient;

  /// The wire code of the kind (`message`, `mention`, `dm`).
  final String kind;

  /// The thread of the message.
  final String threadId;

  /// The message.
  final String messageId;

  /// Whether the recipient was inside quiet hours.
  final bool quiet;

  /// The count of the hint; the fake does not bundle, so always 1.
  final int count;

  /// The devices whose live connections the event went to.
  final Set<String> devices;

  /// The payload of the event (API `NotificationCreatedPayload`).
  final JsonMap payload;

  @override
  String toString() => 'FakeNotification($recipient, $kind)';
}

/// The notification preferences and the decision per recipient of 13 2, 3.1, 4.6 and 6.4 in
/// the fake. Simplified: no runs, no bundling, no pushes, no quiet hour rules (the test lists
/// the people inside quiet hours in [FakeWorld.quietHours]).
extension _NotifyWorld on FakeWorld {
  static const Set<String> _targetKinds = {'thread', 'channel', 'space'};
  static const Set<String> _levels = {'all', 'mentions', 'mute'};

  void _setPref(
    String userId,
    String targetKind,
    String targetId,
    String level,
    DateTime? mutedUntil,
  ) {
    if (!_targetKinds.contains(targetKind)) {
      throw ArgumentError.value(targetKind, 'targetKind');
    }
    if (!_levels.contains(level)) throw ArgumentError.value(level, 'level');
    _notificationPrefs['$userId|$targetKind|$targetId'] = (
      level: level,
      mutedUntil: mutedUntil,
    );
  }

  String _effectiveLevel(String userId, String threadId) {
    final thread = threads[threadId]!;
    final channel = channels[thread['channel_id']! as String]!;
    final space = channel.spaceId;
    for (final key in [
      '$userId|thread|$threadId',
      '$userId|channel|${channel.id}',
      if (space != null) '$userId|space|$space',
    ]) {
      final pref = _notificationPrefs[key];
      if (pref == null) continue;
      final until = pref.mutedUntil;
      return until != null && until.isAfter(now()) ? 'mute' : pref.level;
    }
    final channelDefault = channel.json['default_notification_level'];
    if (channelDefault is String) return channelDefault;
    if (channel.json['kind'] == 'dm') return 'all';
    final personal = settings[userId]?['notify.default_level']?['value'];
    if (personal is String) return personal;
    return userDefaultLevel;
  }

  /// Silent messages, imports, system messages and run messages create no hint (13 2).
  static bool _withoutHints(JsonMap m) =>
      m['silent'] == true ||
      const {'import', 'system'}.contains(m['origin']) ||
      const {'run', 'system'}.contains(m['kind']);

  /// Whether [userId] can receive a hint for a message of [author]: an active person, not the
  /// author (13 2).
  bool _isRecipient(String userId, String author) {
    final person = people[userId];
    return userId != author &&
        person != null &&
        person.kind == 'user' &&
        person.status == 'active';
  }

  /// The kind of the hint for [userId] (01 10.1): `dm` in a direct message channel, `mention`
  /// when mentioned, otherwise `message`.
  static String _kindFor(
    String userId, {
    required bool dm,
    required List<String> mentions,
  }) {
    if (dm) return 'dm';
    return mentions.contains(userId) ? 'mention' : 'message';
  }

  /// The active devices of [userId] with a live gateway connection (13 4.6).
  Set<String> _routeOf(String userId) {
    final connected = connectedDevices(userId);
    return {
      for (final d in devices.values)
        if (d.userId == userId &&
            d.state == 'active' &&
            connected.contains(d.id))
          d.id,
    };
  }

  List<FakeNotification> _notify(String messageId) {
    final m = messages[messageId];
    if (m == null) throw ArgumentError.value(messageId, 'messageId');
    if (_withoutHints(m)) return const [];
    final threadId = m['thread_id']! as String;
    final thread = threads[threadId]!;
    final channel = channels[thread['channel_id']! as String]!;
    final dm = channel.json['kind'] == 'dm';
    final author = (m['author']! as JsonMap)['id']! as String;
    final mentions = (m['mention_ids']! as List<Object?>).cast<String>();
    final created = <FakeNotification>[];
    for (final userId in channel.access.keys) {
      if (!_isRecipient(userId, author)) continue;
      final kind = _kindFor(userId, dm: dm, mentions: mentions);
      final level = _effectiveLevel(userId, threadId);
      if (level == 'mute' || (kind == 'message' && level != 'all')) {
        notificationSkips.add((userId: userId, reason: 'level'));
        continue;
      }
      final route = _routeOf(userId);
      if (route.isEmpty) {
        notificationSkips.add((userId: userId, reason: 'no_route'));
        continue;
      }
      final quiet = quietHours.contains(userId);
      final payload = <String, Object?>{
        'nid': nextId(),
        'kind': kind,
        'thread_id': threadId,
        'message_id': messageId,
        'quiet': quiet,
        'count': 1,
        ..._preview(userId, m, thread, channel),
      };
      final event = publishEphemeral(
        'notification.created',
        resource: {'kind': 'thread', 'id': threadId},
        scope: {'channel_id': channel.id, 'thread_id': threadId},
        payload: payload,
        audience: userId,
        devices: route,
      );
      final n = FakeNotification(
        recipient: userId,
        kind: kind,
        threadId: threadId,
        messageId: messageId,
        quiet: quiet,
        count: 1,
        devices: route,
        payload: event.envelope['payload']! as JsonMap,
      );
      notifications.add(n);
      created.add(n);
    }
    return created;
  }

  /// The preview fields of 13 6.4 by the `notify.preview_mode` of [userId].
  JsonMap _preview(
    String userId,
    JsonMap message,
    JsonMap thread,
    FakeChannel channel,
  ) {
    final mode =
        settings[userId]?['notify.preview_mode']?['value'] ??
        defaultPreviewMode;
    if (mode == 'none') return const {};
    final space = channel.spaceId;
    final text = message['content_md']! as String;
    return {
      if (space != null) 'space_name': ?spaces[space]?['name'],
      'channel_name': channel.json['name'],
      'thread_title': thread['title'],
      'sender_display_name': (message['author']! as JsonMap)['display_name'],
      if (mode == 'full')
        'excerpt': String.fromCharCodes(text.runes.take(previewMaxChars)),
    };
  }
}
