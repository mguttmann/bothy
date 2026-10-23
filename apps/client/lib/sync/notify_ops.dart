// SPDX-License-Identifier: Apache-2.0

/// The writes of the notification settings through the outbox: the level of a space, channel
/// or thread and the quiet hours rules (13 3.1, 3.4; 04 14; brief M4-33 Design note 2).
library;

import 'dart:convert';

import 'package:api_client/api_client.dart' as wire;
import 'package:api_client/api_client.dart' show ApiClient;
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

/// The outbox operations of the notification settings and their payloads.
///
/// | Kind | Payload | Lane |
/// |---|---|---|
/// | `putNotificationPreference` | `target_kind`, `target_id`, `level`, optional `muted_until` | `notify:<kind>:<id>` |
/// | `deleteNotificationPreference` | `target_kind`, `target_id` | `notify:<kind>:<id>` |
/// | `createQuietHoursRule` | `local_id`, `weekdays`, `start_local`, `end_local`, optional `time_zone`, `enabled` | `quiet_hours` |
/// | `putQuietHoursRule` | `rule_id` (null while its create waits), the fields of the rule | `quiet_hours` |
/// | `deleteQuietHoursRule` | `rule_id` (null while its create waits) | `quiet_hours` |
///
/// Writes of one target run in order (the last one wins, 04 14), different targets in
/// parallel. The quiet hours share one lane, because the server runs the writes of one person
/// one after the other and each sends the whole list (13 3.4, D-M3-18-7). A new rule is shown
/// at once under a local ID, the ID of its outbox entry; its create returns the server's ID as
/// `rule_id`, which the outbox fills into the edits and the delete that wait for it
/// (D-M4-15-6). The `Idempotency-Key` is the entry's ID (06 7).
///
/// Example: `NotifyOps.register(executors, api, db);`
abstract final class NotifyOps {
  /// `PUT /me/notification-preferences/{target_kind}/{target_id}`.
  static const String putNotificationPreference = 'putNotificationPreference';

  /// `DELETE /me/notification-preferences/{target_kind}/{target_id}`.
  static const String deleteNotificationPreference =
      'deleteNotificationPreference';

  /// `POST /me/quiet-hours`.
  static const String createQuietHoursRule = 'createQuietHoursRule';

  /// `PUT /me/quiet-hours/{rule_id}`.
  static const String putQuietHoursRule = 'putQuietHoursRule';

  /// `DELETE /me/quiet-hours/{rule_id}`.
  static const String deleteQuietHoursRule = 'deleteQuietHoursRule';

  /// The lane of the quiet hours rules.
  static const String quietHoursLane = 'quiet_hours';

  /// Registers the five executors on [registry]; they call [api] and store the server's
  /// answer in [db] by the version rule, unless a later write of the same target waits in
  /// the outbox (its answer follows, the row keeps the newer local state meanwhile).
  static void register(
    OutboxExecutors registry,
    ApiClient api,
    LocalDatabase db,
  ) {
    final prefs = api.getPreferencesApi();
    registry
      ..register(putNotificationPreference, (e, ctx) async {
        final body = decodePayload(e.payload);
        final until = body['muted_until'];
        final response = await prefs.putNotificationPreference(
          targetKind: _str(body, 'target_kind'),
          targetId: _str(body, 'target_id'),
          notificationPreferencePut: wire.NotificationPreferencePut(
            (b) => b
              ..level = wire.NotificationLevel.valueOf(_str(body, 'level'))
              ..mutedUntil = until is String ? DateTime.parse(until) : null,
          ),
          extra: ctx.extra,
        );
        final pref = response.data;
        if (pref != null &&
            !await _laterWriteWaits(db, e, _preferenceWrites, _targetOf)) {
          final json = api.serializers.serializeWith(
            wire.NotificationPreference.serializer,
            pref,
          );
          if (json is Map) {
            await db.personalDao.upsertNotificationPreferenceIfNewer(
              preferenceFromJson(json.cast<String, Object?>()),
            );
          }
        }
        return OutboxResponse(status: response.statusCode, body: pref);
      }, lane: (e) => 'notify:${_targetOf(e)}')
      ..register(deleteNotificationPreference, (e, ctx) async {
        final body = decodePayload(e.payload);
        final response = await prefs.deleteNotificationPreference(
          targetKind: _str(body, 'target_kind'),
          targetId: _str(body, 'target_id'),
          extra: ctx.extra,
        );
        return OutboxResponse(status: response.statusCode);
      }, lane: (e) => 'notify:${_targetOf(e)}')
      ..register(createQuietHoursRule, (e, ctx) async {
        final body = decodePayload(e.payload);
        final response = await prefs.createQuietHoursRule(
          quietHoursRuleInput: _ruleInput(body),
          idempotencyKey: ctx.idempotencyKey,
          extra: ctx.extra,
        );
        final rule = response.data!;
        // The local row under the entry's ID gives way to the server's rule.
        await db.personalDao.deleteQuietHours(
          QuietHoursId.parse(_str(body, 'local_id')),
        );
        await _storeRule(api, db, rule);
        return OutboxResponse(
          status: response.statusCode,
          body: rule,
          result: {'rule_id': rule.id},
        );
      }, lane: (_) => quietHoursLane)
      ..register(putQuietHoursRule, (e, ctx) async {
        final body = decodePayload(e.payload);
        final response = await prefs.putQuietHoursRule(
          ruleId: _str(body, 'rule_id'),
          quietHoursRuleInput: _ruleInput(body),
          extra: ctx.extra,
        );
        final rule = response.data;
        if (rule != null &&
            !await _laterWriteWaits(db, e, _ruleWrites, _ruleOf)) {
          await _storeRule(api, db, rule);
        }
        return OutboxResponse(status: response.statusCode, body: rule);
      }, lane: (_) => quietHoursLane)
      ..register(deleteQuietHoursRule, (e, ctx) async {
        final response = await prefs.deleteQuietHoursRule(
          ruleId: _str(decodePayload(e.payload), 'rule_id'),
          extra: ctx.extra,
        );
        return OutboxResponse(status: response.statusCode);
      }, lane: (_) => quietHoursLane);
  }

  static wire.QuietHoursRuleInput _ruleInput(Map<String, Object?> body) =>
      wire.QuietHoursRuleInput(
        (b) => b
          ..weekdays = body['weekdays']! as int
          ..startLocal = _str(body, 'start_local')
          ..endLocal = _str(body, 'end_local')
          ..timeZone = body['time_zone'] as String?
          ..enabled = body['enabled'] != false,
      );

  static Future<void> _storeRule(
    ApiClient api,
    LocalDatabase db,
    wire.QuietHoursRule rule,
  ) async {
    final json = api.serializers.serializeWith(
      wire.QuietHoursRule.serializer,
      rule,
    );
    if (json is Map) {
      await db.personalDao.upsertQuietHoursIfNewer(
        quietHoursFromJson(json.cast<String, Object?>()),
      );
    }
  }

  /// Whether an entry of one of [kinds] with the same key ([keyOf]) after [e] waits in the
  /// outbox. [kinds] holds only kinds whose payload [keyOf] can read: the preference writes
  /// and the quiet hours writes have different payloads and share the outbox.
  static Future<bool> _laterWriteWaits(
    LocalDatabase db,
    OutboxEntry e,
    Set<String> kinds,
    String? Function(OutboxEntry) keyOf,
  ) async {
    final key = keyOf(e);
    for (final other in await db.outboxDao.allEntries()) {
      if (other.id == e.id ||
          other.state.isFinal ||
          other.createdAt.isBefore(e.createdAt) ||
          !kinds.contains(other.kind)) {
        continue;
      }
      if (keyOf(other) == key) return true;
    }
    return false;
  }

  static const Set<String> _preferenceWrites = {
    putNotificationPreference,
    deleteNotificationPreference,
  };

  static const Set<String> _ruleWrites = {
    putQuietHoursRule,
    deleteQuietHoursRule,
  };

  static String _targetOf(OutboxEntry e) {
    final body = decodePayload(e.payload);
    return '${_str(body, 'target_kind')}:${_str(body, 'target_id')}';
  }

  static String? _ruleOf(OutboxEntry e) =>
      decodePayload(e.payload)['rule_id'] as String?;
}

String _str(Map<String, Object?> body, String name) {
  final value = body[name];
  if (value is! String) throw StateError('member $name is missing');
  return value;
}

/// Writes the notification settings: the local row changes at once, the write goes through
/// the outbox and reaches the person's other devices as `notification_pref.updated` or
/// `quiet_hours.updated` (04 14; brief M4-33 Design note 2).
///
/// Example:
/// ```dart
/// final writer = NotifyWriter(engine, db, ids.next);
/// await writer.setLevel(NotificationTargetKind.channel, general, NotificationLevel.mute);
/// await writer.inherit(NotificationTargetKind.channel, general);
/// ```
final class NotifyWriter {
  /// Creates the writer over the outbox of the engine and the local database; the third
  /// argument gives the local ID of a new quiet hours rule (a UUIDv7, also the ID of its
  /// entry).
  NotifyWriter(this._engine, this._db, this._newId);

  final SyncEngine _engine;
  final LocalDatabase _db;
  final String Function() _newId;

  Future<NotificationPreference?> _own(
    NotificationTargetKind kind,
    String id,
  ) async => (await _db.personalDao.notificationPreferences())
      .where((p) => p.targetKind == kind && p.targetId == id)
      .firstOrNull;

  /// Sets the own [level] of target [kind] [id], muted until [mutedUntil] when given (the row
  /// applies [level] again after it, 13 3.1). Nothing happens when the row holds exactly
  /// this already.
  Future<void> setLevel(
    NotificationTargetKind kind,
    String id,
    NotificationLevel level, {
    DateTime? mutedUntil,
  }) async {
    final stored = await _own(kind, id);
    final until = mutedUntil?.toUtc();
    if (stored != null &&
        stored.level == level &&
        stored.mutedUntil?.toUtc() == until) {
      return;
    }
    await _db.personalDao.upsertNotificationPreferenceIfNewer(
      NotificationPreference(
        targetKind: kind,
        targetId: id,
        level: level,
        mutedUntil: until,
        version: ResourceVersion((stored?.version.value ?? 0) + 1),
      ),
    );
    await _engine.outbox.enqueue(
      OutboxRequest(
        kind: NotifyOps.putNotificationPreference,
        payload: {
          'target_kind': kind.code,
          'target_id': id,
          'level': level.code,
          'muted_until': ?until?.toIso8601String(),
        },
      ),
    );
  }

  /// Deletes the own level of target [kind] [id], so it inherits again (13 3.1); nothing
  /// happens without an own row.
  Future<void> inherit(NotificationTargetKind kind, String id) async {
    final stored = await _own(kind, id);
    if (stored == null) return;
    await _db.personalDao.deleteNotificationPreferenceIfNewer(
      kind,
      id,
      ResourceVersion(stored.version.value + 1),
    );
    await _engine.outbox.enqueue(
      OutboxRequest(
        kind: NotifyOps.deleteNotificationPreference,
        payload: {'target_kind': kind.code, 'target_id': id},
      ),
    );
  }

  /// Adds the quiet hours [rule] (its ID and version are ignored): stored at once under a
  /// new local ID, sent as `createQuietHoursRule`. Returns the local ID.
  Future<QuietHoursId> addQuietHours(QuietHours rule) async {
    final id = QuietHoursId.parse(_newId());
    await _db.personalDao.upsertQuietHoursIfNewer(
      _copy(rule, id: id, version: const ResourceVersion(1)),
    );
    await _engine.outbox.enqueue(
      OutboxRequest(
        id: id.value,
        kind: NotifyOps.createQuietHoursRule,
        payload: {'local_id': id.value, ..._fields(rule)},
      ),
    );
    return id;
  }

  /// Replaces the stored rule of the same ID with [rule]; nothing happens when the rule is
  /// gone or unchanged.
  Future<void> putQuietHours(QuietHours rule) async {
    final stored = (await _db.personalDao.quietHours())
        .where((r) => r.id == rule.id)
        .firstOrNull;
    if (stored == null ||
        jsonEncode(_fields(stored)) == jsonEncode(_fields(rule))) {
      return;
    }
    await _db.personalDao.upsertQuietHoursIfNewer(
      _copy(
        rule,
        id: rule.id,
        version: ResourceVersion(stored.version.value + 1),
      ),
    );
    final create = await _waitingCreate(rule.id);
    await _engine.outbox.enqueue(
      OutboxRequest(
        kind: NotifyOps.putQuietHoursRule,
        payload: {
          'rule_id': create == null ? rule.id.value : null,
          ..._fields(rule),
        },
        dependsOn: [?create],
      ),
    );
  }

  /// Deletes the rule [id]: a create that was never sent is dropped from the outbox, else the
  /// delete goes out (after its create, when that still waits).
  Future<void> deleteQuietHours(QuietHoursId id) async {
    await _db.personalDao.deleteQuietHours(id);
    final create = await _db.outboxDao.byId(id.value);
    if (create != null &&
        create.kind == NotifyOps.createQuietHoursRule &&
        !create.state.isFinal &&
        create.attempts == 0) {
      await _engine.outbox.discard(create.id);
      return;
    }
    final waiting = await _waitingCreate(id);
    await _engine.outbox.enqueue(
      OutboxRequest(
        kind: NotifyOps.deleteQuietHoursRule,
        payload: {'rule_id': waiting == null ? id.value : null},
        dependsOn: [?waiting],
      ),
    );
  }

  /// The ID of the create entry of the local rule [id] while it is not confirmed.
  Future<String?> _waitingCreate(QuietHoursId id) async {
    final entry = await _db.outboxDao.byId(id.value);
    return entry != null &&
            entry.kind == NotifyOps.createQuietHoursRule &&
            entry.state != OutboxState.confirmed
        ? entry.id
        : null;
  }

  static Map<String, Object?> _fields(QuietHours rule) => {
    'weekdays': rule.weekdays,
    'start_local': rule.startLocal,
    'end_local': rule.endLocal,
    'time_zone': ?rule.timeZone,
    'enabled': rule.enabled,
  };

  static QuietHours _copy(
    QuietHours rule, {
    required QuietHoursId id,
    required ResourceVersion version,
  }) => QuietHours(
    id: id,
    weekdays: rule.weekdays,
    startLocal: rule.startLocal,
    endLocal: rule.endLocal,
    timeZone: rule.timeZone,
    enabled: rule.enabled,
    version: version,
  );

  /// Loads the person's levels and quiet hours from the server and stores them by the
  /// version rule; a local row the server no longer lists goes, unless a write of it still
  /// waits in the outbox. The snapshot carries neither (04 6.3), so a device that signs in
  /// learns them here and from the events after it.
  ///
  /// Throws the `DioException` of a failed request; the stored rows stay.
  Future<void> refresh(ApiClient api) async {
    final prefs = api.getPreferencesApi();
    final levels = (await prefs.listMyNotificationPreferences()).data!.items;
    final rules = (await prefs.listMyQuietHours()).data!.items;
    final pending = [
      for (final e in await _db.outboxDao.allEntries())
        if (!e.state.isFinal) e,
    ];
    final busyTargets = {
      for (final e in pending)
        if (e.kind == NotifyOps.putNotificationPreference ||
            e.kind == NotifyOps.deleteNotificationPreference)
          NotifyOps._targetOf(e),
    };
    final quietBusy = pending.any(
      (e) =>
          e.kind == NotifyOps.createQuietHoursRule ||
          e.kind == NotifyOps.putQuietHoursRule ||
          e.kind == NotifyOps.deleteQuietHoursRule,
    );
    final listed = <String>{};
    for (final p in levels) {
      final json = api.serializers.serializeWith(
        wire.NotificationPreference.serializer,
        p,
      );
      if (json is! Map) continue;
      final pref = preferenceFromJson(json.cast<String, Object?>());
      final key = '${pref.targetKind.code}:${pref.targetId}';
      listed.add(key);
      if (busyTargets.contains(key)) continue;
      await _db.personalDao.upsertNotificationPreferenceIfNewer(pref);
    }
    for (final stored in await _db.personalDao.notificationPreferences()) {
      final key = '${stored.targetKind.code}:${stored.targetId}';
      if (listed.contains(key) || busyTargets.contains(key)) continue;
      await _db.personalDao.deleteNotificationPreferenceIfNewer(
        stored.targetKind,
        stored.targetId,
        ResourceVersion(stored.version.value + 1),
      );
    }
    // While a rule write waits the next `quiet_hours.updated` brings the list; nothing here.
    if (quietBusy) return;
    final ids = <QuietHoursId>{};
    for (final r in rules) {
      final json = api.serializers.serializeWith(
        wire.QuietHoursRule.serializer,
        r,
      );
      if (json is! Map) continue;
      final rule = quietHoursFromJson(json.cast<String, Object?>());
      ids.add(rule.id);
      await _db.personalDao.upsertQuietHoursIfNewer(rule);
    }
    for (final stored in await _db.personalDao.quietHours()) {
      if (!ids.contains(stored.id)) {
        await _db.personalDao.deleteQuietHours(stored.id);
      }
    }
  }
}
