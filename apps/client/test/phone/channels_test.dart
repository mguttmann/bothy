// SPDX-License-Identifier: Apache-2.0

// The notification channels of Android (brief M5-03 fixed requirement 12, Design note 4; 01 10.3;
// 13 2; 15 10.2): four channels with stable IDs and l10n names in de and en, the importance of
// each, and a kind mapping that covers every kind of 13 2; the registration hands the list to
// the runner.

import 'package:api_client/api_client.dart' show NotificationKind;
import 'package:client/platform/notification_channels.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

class FakeChannelApi extends Fake implements AndroidPlatformApi {
  final List<List<AndroidNotificationChannel>> registered = [];

  @override
  Future<void> registerNotificationChannels(
    List<AndroidNotificationChannel> channels,
  ) async => registered.add(channels);
}

void main() {
  final de = lookupAppLocalizations(const Locale('de'));
  final en = lookupAppLocalizations(const Locale('en'));

  test('four channels with stable IDs in their order', () {
    expect(PushChannel.values.map((c) => c.id), [
      'mentions',
      'questions',
      'messages',
      'runs',
    ]);
  });

  test('mentions and questions are high, messages and runs default', () {
    expect(
      {for (final c in PushChannel.values) c.id: c.high},
      {'mentions': true, 'questions': true, 'messages': false, 'runs': false},
    );
  });

  test('the names and descriptions come from l10n in de and en', () {
    expect(PushChannel.values.map((c) => c.name(en)), [
      'Mentions',
      'Questions',
      'All messages',
      'Runs',
    ]);
    expect(PushChannel.values.map((c) => c.name(de)), [
      'Erwähnungen',
      'Fragen',
      'Alle Nachrichten',
      'Läufe',
    ]);
    for (final l in [de, en]) {
      final descriptions = PushChannel.values.map((c) => c.description(l));
      expect(descriptions.every((d) => d.isNotEmpty), isTrue);
      expect(descriptions.toSet(), hasLength(4));
    }
  });

  test('every kind of 13 2 has exactly one channel', () {
    final kinds = NotificationKind.values
        .where((k) => k != NotificationKind.unknownDefaultOpenApi)
        .map(
          (k) => k.name.replaceAllMapped(
            RegExp('[A-Z]'),
            (m) => '_${m[0]!.toLowerCase()}',
          ),
        )
        .toSet();
    expect(kinds, hasLength(8));
    final mapped = [for (final c in PushChannel.values) ...c.kinds];
    expect(mapped.toSet(), kinds);
    expect(mapped, hasLength(kinds.length), reason: 'no kind in two channels');
    expect(PushChannel.ofKind('mention'), PushChannel.mentions);
    expect(PushChannel.ofKind('dm'), PushChannel.mentions);
    expect(PushChannel.ofKind('input_needed'), PushChannel.questions);
    expect(PushChannel.ofKind('security'), PushChannel.questions);
    expect(PushChannel.ofKind('admin'), PushChannel.questions);
    expect(PushChannel.ofKind('message'), PushChannel.messages);
    expect(PushChannel.ofKind('run_done'), PushChannel.runs);
    expect(PushChannel.ofKind('run_failed'), PushChannel.runs);
  });

  test('a kind of a newer server goes to All messages', () {
    expect(PushChannel.ofKind('later_kind'), PushChannel.messages);
  });

  test(
    'the registration hands the four channels in the device language',
    () async {
      final api = FakeChannelApi();
      await registerNotificationChannels(api, de);
      expect(api.registered, hasLength(1));
      final channels = api.registered.single;
      expect(channels.map((c) => c.id), [
        'mentions',
        'questions',
        'messages',
        'runs',
      ]);
      expect(channels.first.name, 'Erwähnungen');
      expect(channels.first.description, de.pushChannelMentionsDescription);
      expect(channels.map((c) => c.high), [true, true, false, false]);
      expect(channels.first.kinds, ['mention', 'dm']);
      expect(channels.last.kinds, ['run_done', 'run_failed']);
    },
  );
}
