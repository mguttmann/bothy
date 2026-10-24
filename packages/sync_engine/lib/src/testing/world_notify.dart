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

/// The notification preferences, quiet hours rules and the decision per recipient of 13 2, 3,
/// 4.6 and 6.4 in the fake. Simplified: no runs, no bundling, no pushes, quiet hour rules are
/// stored but not evaluated (the test lists the people inside quiet hours in
/// [FakeWorld.quietHours]).
extension _NotifyWorld on FakeWorld {
  static const Set<String> _targetKinds = {'thread', 'channel', 'space'};
  static const Set<String> _levels = {'all', 'mentions', 'mute'};
  static final RegExp _hhmm = RegExp(r'^([01][0-9]|2[0-3]):[0-5][0-9]$');

  static String _slot(String userId, String kind, String id) =>
      '$userId|$kind|$id';

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
    final slot = _slot(userId, targetKind, targetId);
    _notificationPrefs[slot] = {
      'target_kind': targetKind,
      'target_id': targetId,
      'level': level,
      if (mutedUntil != null) 'muted_until': stamp(mutedUntil),
      'version': ((_notificationPrefs[slot]?['version'] as int?) ?? 0) + 1,
    };
  }

  /// Whether [userId] can read the target; a target of another kind is unknown (404).
  bool _readsTarget(String userId, String kind, String id) => switch (kind) {
    'space' => spaces.containsKey(id) && seesSpace(userId, id),
    'channel' => seesChannel(userId, id),
    'thread' => seesThread(userId, id),
    _ => false,
  };

  /// Whether the target, its channel or its space is archived (D-M3-18-2).
  bool _archived(String kind, String id) {
    final channelId = switch (kind) {
      'thread' => threads[id]?['channel_id'] as String?,
      'channel' => id,
      _ => null,
    };
    final channel = channelId == null ? null : channels[channelId];
    final spaceId = kind == 'space' ? id : channel?.spaceId;
    return channel?.json['archived_at'] != null ||
        (spaceId != null && spaces[spaceId]?['archived_at'] != null);
  }

  List<JsonMap> _prefsOf(String userId) {
    final rows = [
      for (final MapEntry(:key, :value) in _notificationPrefs.entries)
        if (key.startsWith('$userId|') &&
            _readsTarget(
              userId,
              value['target_kind']! as String,
              value['target_id']! as String,
            ))
          value,
    ];
    int order(JsonMap a, JsonMap b) {
      final kind = (a['target_kind']! as String).compareTo(
        b['target_kind']! as String,
      );
      return kind != 0
          ? kind
          : (a['target_id']! as String).compareTo(b['target_id']! as String);
    }

    return rows..sort(order);
  }

  void _prefEvent(String userId, JsonMap pref, {required bool deleted}) =>
      append(
        'notification_pref.updated',
        resource: {'kind': 'user', 'id': userId, 'version': pref['version']},
        payload: {'preference': pref, if (deleted) 'deleted': true},
        actor: userId,
        audience: userId,
      );

  JsonMap _putPref(String userId, String kind, String id, JsonMap body) {
    if (!_readsTarget(userId, kind, id)) {
      throw const FakeProblem(404, 'not_found');
    }
    if (_archived(kind, id)) {
      throw const FakeProblem(409, 'conflict', reason: 'target_archived');
    }
    final level = body['level'];
    if (level is! String || !_levels.contains(level)) {
      throw const FakeProblem(
        400,
        'validation_failed',
        errors: [
          {'pointer': '/level', 'code': 'invalid'},
        ],
      );
    }
    final until = body['muted_until'] as String?;
    final slot = _slot(userId, kind, id);
    final stored = _notificationPrefs[slot];
    final normalized = until == null ? null : stamp(DateTime.parse(until));
    if (stored != null &&
        stored['level'] == level &&
        stored['muted_until'] == normalized) {
      return stored;
    }
    final pref = <String, Object?>{
      'target_kind': kind,
      'target_id': id,
      'level': level,
      'muted_until': ?normalized,
      'version': ((stored?['version'] as int?) ?? 0) + 1,
    };
    _notificationPrefs[slot] = pref;
    _prefEvent(userId, pref, deleted: false);
    return pref;
  }

  void _removePref(String userId, String kind, String id) {
    if (!_readsTarget(userId, kind, id)) {
      throw const FakeProblem(404, 'not_found');
    }
    final stored = _notificationPrefs.remove(_slot(userId, kind, id));
    if (stored == null) return;
    _prefEvent(userId, {
      ...stored,
      'version': (stored['version']! as int) + 1,
    }, deleted: true);
  }

  /// 13 3.2: a person (never an agent or app) who writes in a thread with `kind = thread`
  /// follows it at `all`, only when there is no own row yet (D-M3-19-14).
  void _follow(String userId, String threadId) {
    if (people[userId]?.kind != 'user' ||
        threads[threadId]?['kind'] != 'thread') {
      return;
    }
    final slot = _slot(userId, 'thread', threadId);
    if (_notificationPrefs.containsKey(slot)) return;
    final pref = <String, Object?>{
      'target_kind': 'thread',
      'target_id': threadId,
      'level': 'all',
      'version': 1,
    };
    _notificationPrefs[slot] = pref;
    _prefEvent(userId, pref, deleted: false);
  }

  List<JsonMap> _rulesOf(String userId) {
    final rules = [...?quietHoursRules[userId]?.values];
    return rules
      ..sort((a, b) => (a['id']! as String).compareTo(b['id']! as String));
  }

  /// The rule of [body] with [id] and [version]; `400 validation_failed` for weekdays outside
  /// 1 to 127, a time that is not `HH:MM` or an empty time zone.
  JsonMap _rule(String id, JsonMap body, int version) {
    final errors = <JsonMap>[
      if (body['weekdays'] case final int w when w < 1 || w > 127)
        {'pointer': '/weekdays', 'code': 'invalid'},
      if (body['weekdays'] is! int) {'pointer': '/weekdays', 'code': 'invalid'},
      for (final field in ['start_local', 'end_local'])
        if (body[field] is! String || !_hhmm.hasMatch(body[field]! as String))
          {'pointer': '/$field', 'code': 'invalid'},
      if (body['time_zone'] case final String zone when zone.isEmpty)
        {'pointer': '/time_zone', 'code': 'unknown_time_zone'},
    ];
    if (errors.isNotEmpty) {
      throw FakeProblem(400, 'validation_failed', errors: errors);
    }
    return {
      'id': id,
      'weekdays': body['weekdays'],
      'start_local': body['start_local'],
      'end_local': body['end_local'],
      'time_zone': ?body['time_zone'],
      'enabled': body['enabled'] ?? true,
      'version': version,
    };
  }

  void _rulesEvent(String userId) => append(
    'quiet_hours.updated',
    resource: {'kind': 'user', 'id': userId},
    payload: {'rules': _rulesOf(userId)},
    actor: userId,
    audience: userId,
  );

  JsonMap _createRule(String userId, JsonMap body) {
    final rule = _rule(nextId(), body, 1);
    quietHoursRules.putIfAbsent(userId, () => {})[rule['id']! as String] = rule;
    _rulesEvent(userId);
    return rule;
  }

  JsonMap _putRule(String userId, String ruleId, JsonMap body) {
    final mine = quietHoursRules[userId];
    final stored = mine?[ruleId];
    if (mine == null || stored == null) {
      throw const FakeProblem(404, 'not_found');
    }
    final rule = _rule(ruleId, body, stored['version']! as int);
    if (jsonEncode(rule) == jsonEncode(stored)) return stored;
    rule['version'] = (stored['version']! as int) + 1;
    mine[ruleId] = rule;
    _rulesEvent(userId);
    return rule;
  }

  void _deleteRule(String userId, String ruleId) {
    if (quietHoursRules[userId]?.remove(ruleId) == null) {
      throw const FakeProblem(404, 'not_found');
    }
    _rulesEvent(userId);
  }

  String _effectiveLevel(String userId, String threadId) {
    final thread = threads[threadId]!;
    final channel = channels[thread['channel_id']! as String]!;
    final space = channel.spaceId;
    for (final key in [
      _slot(userId, 'thread', threadId),
      _slot(userId, 'channel', channel.id),
      if (space != null) _slot(userId, 'space', space),
    ]) {
      final pref = _notificationPrefs[key];
      if (pref == null) continue;
      final until = pref['muted_until'] as String?;
      return until != null && DateTime.parse(until).isAfter(now())
          ? 'mute'
          : pref['level']! as String;
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
