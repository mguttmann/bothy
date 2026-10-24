// SPDX-License-Identifier: Apache-2.0

/// Handlers of the notification preferences, the quiet hours and the channel export (brief
/// M4-33 Scope; 13 3; 04 14; openapi `exportChannel`).
///
/// The preferences and rules live in the world ([FakeWorld.putNotificationPref],
/// [FakeWorld.createQuietHoursRule]), which appends `notification_pref.updated` and
/// `quiet_hours.updated` for the person's devices. The export streams one `Message` per line
/// for the channel's threads in the time range; it needs read access and the capability
/// `export` of the channel view, and the step-up of `export.channel` from
/// `docs/spec/actions.yaml` through [requireCatalogStepUp].
library;

import 'dart:convert';

import '../action_catalog.dart';
import '../rest_fake.dart';
import '../world.dart';
import 'admin_handlers.dart' show requireCatalogStepUp;

/// Registers the handlers on [rest]; [catalog] gives the step-up of `export.channel`.
void registerNotifyHandlers(RestFake rest, ActionCatalog catalog) {
  final world = rest.world;
  rest
    ..handle(
      'listMyNotificationPreferences',
      (r) =>
          FakeResponse(200, {'items': world.notificationPrefsOf(r.person.id)}),
    )
    ..handle(
      'putNotificationPreference',
      (r) => FakeResponse(
        200,
        world.putNotificationPref(
          r.person.id,
          r.param('target_kind'),
          r.param('target_id'),
          r.json,
        ),
      ),
    )
    ..handle('deleteNotificationPreference', (r) {
      world.removeNotificationPref(
        r.person.id,
        r.param('target_kind'),
        r.param('target_id'),
      );
      return const FakeResponse.noContent();
    })
    ..handle(
      'listMyQuietHours',
      (r) => FakeResponse(200, {'items': world.quietHoursOf(r.person.id)}),
    )
    ..handle(
      'createQuietHoursRule',
      (r) => FakeResponse(201, world.createQuietHoursRule(r.person.id, r.json)),
    )
    ..handle(
      'putQuietHoursRule',
      (r) => FakeResponse(
        200,
        world.putQuietHoursRule(r.person.id, r.param('rule_id'), r.json),
      ),
    )
    ..handle('deleteQuietHoursRule', (r) {
      world.deleteQuietHoursRule(r.person.id, r.param('rule_id'));
      return const FakeResponse.noContent();
    })
    ..handle('exportChannel', (r) {
      final channelId = r.param('channel_id');
      final me = r.person.id;
      if (!world.seesChannel(me, channelId)) {
        throw const FakeProblem(404, 'not_found');
      }
      if (!(world.channels[channelId]!.capabilities[me]?.contains('export') ??
          false)) {
        throw const FakeProblem(403, 'forbidden', reason: 'capability_missing');
      }
      requireCatalogStepUp(r, rest.contract, catalog);
      final from = DateTime.tryParse(r.query('from') ?? '');
      final to = DateTime.tryParse(r.query('to') ?? '');
      if (from == null || to == null || !from.isBefore(to)) {
        throw const FakeProblem(
          400,
          'validation_failed',
          errors: [
            {'pointer': '#query/to', 'code': 'invalid'},
          ],
        );
      }
      final out = StringBuffer();
      final threadIds = [
        for (final t in world.threads.values)
          if (t['channel_id'] == channelId) t['id']! as String,
      ]..sort();
      for (final threadId in threadIds) {
        for (final m in world.messagesOf(threadId)) {
          final at = DateTime.parse(m['created_at']! as String);
          if (at.isBefore(from) || !at.isBefore(to)) continue;
          final line = {...m}
            ..remove('content_truncated')
            ..remove('my_reactions');
          final problems = rest.contract.validate(line, 'Message');
          if (problems.isNotEmpty) {
            rest.violations.add(
              'response exportChannel line ${m['id']}: $problems',
            );
          }
          out.writeln(jsonEncode(line));
        }
      }
      return FakeResponse.bytes(
        utf8.encode(out.toString()),
        contentType: 'application/x-ndjson',
      );
    });
}
